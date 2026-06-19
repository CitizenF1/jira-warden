package warden

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type ConfluencePage struct {
	Title string
	URL   string
	Date  time.Time
}

type ConfluenceClient struct {
	baseURL    string
	email      string
	token      string
	auth       string
	httpClient *http.Client
}

func NewConfluenceClient(baseURL, email, token, auth string) *ConfluenceClient {
	return &ConfluenceClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		email:      email,
		token:      token,
		auth:       auth,
		httpClient: http.DefaultClient,
	}
}

type confluenceCandidate struct {
	ID    string
	Title string
	WebUI string
}

// Pages возвращает активность пользователя в Confluence:
//   - правки страниц и блогпостов (по истории версий — точные даты)
//   - оставленные комментарии (дата создания, заголовок родительской страницы)
//
// out используется для вывода диагностических сообщений.
func (client *ConfluenceClient) Pages(ctx context.Context, contributor string, period Period, out io.Writer) ([]ConfluencePage, error) {
	candidates, baseURL, err := client.searchCandidates(ctx, contributor, period.From, out)
	if err != nil {
		return nil, err
	}

	_, _ = fmt.Fprintf(out, "найдено кандидатов Confluence: %d\n", len(candidates))

	var pages []ConfluencePage
	for _, candidate := range candidates {
		dates, err := client.contributionDates(ctx, candidate.ID, contributor, period)
		if err != nil {
			return nil, fmt.Errorf("не удалось получить историю страницы %q: %w", candidate.Title, err)
		}

		webURL := candidate.WebUI
		if !strings.HasPrefix(webURL, "http") {
			webURL = baseURL + webURL
		}

		for _, date := range dates {
			pages = append(pages, ConfluencePage{
				Title: candidate.Title,
				URL:   webURL,
				Date:  date,
			})
		}
	}

	comments, err := client.searchComments(ctx, contributor, period, out)
	if err != nil {
		return nil, err
	}
	for i := range comments {
		if !strings.HasPrefix(comments[i].URL, "http") {
			comments[i].URL = baseURL + comments[i].URL
		}
	}
	pages = append(pages, comments...)

	return pages, nil
}

// searchCandidates ищет страницы через CQL. Использует совместимый синтаксис:
// (creator = "X" OR lastmodifier = "X") вместо contributor = "X",
// (type = page OR type = blogpost) вместо type in (...).
// Верхняя граница даты намеренно отсутствует: страница могла быть изменена
// другим пользователем после окончания периода, но правка нашего contributor
// в нужный период всё равно найдётся при обходе истории версий.
func (client *ConfluenceClient) searchCandidates(
	ctx context.Context,
	contributor string,
	from time.Time,
	out io.Writer,
) ([]confluenceCandidate, string, error) {
	cql := client.candidatesCQL(contributor, from)
	_, _ = fmt.Fprintf(out, "CQL (страницы): %s\n", cql)

	var candidates []confluenceCandidate
	var baseURL string
	start := 0
	const limit = 50

	for {
		batch, hasNext, responseBase, err := client.fetchCandidatesBatch(ctx, cql, start, limit)
		if err != nil {
			return nil, "", err
		}
		if baseURL == "" {
			baseURL = responseBase
		}
		candidates = append(candidates, batch...)
		if !hasNext || len(batch) == 0 {
			break
		}
		start += limit
	}

	if baseURL == "" {
		baseURL = client.baseURL
	}

	return candidates, baseURL, nil
}

func (client *ConfluenceClient) candidatesCQL(contributor string, from time.Time) string {
	dateFrom := from.Format(time.DateOnly)
	typeExpr := `(type = page OR type = blogpost)`

	if contributor == "" {
		return fmt.Sprintf(`%s AND lastModified >= "%s"`, typeExpr, dateFrom)
	}

	authorExpr := fmt.Sprintf(`(creator = "%s" OR lastmodifier = "%s")`, contributor, contributor)
	return fmt.Sprintf(`%s AND %s AND lastModified >= "%s"`, typeExpr, authorExpr, dateFrom)
}

func (client *ConfluenceClient) fetchCandidatesBatch(
	ctx context.Context,
	cql string,
	start int,
	limit int,
) ([]confluenceCandidate, bool, string, error) {
	endpoint, err := url.JoinPath(client.baseURL, "rest/api/content/search")
	if err != nil {
		return nil, false, "", fmt.Errorf("не удалось собрать URL Confluence: %w", err)
	}

	requestURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, false, "", fmt.Errorf("не удалось разобрать URL Confluence: %w", err)
	}

	query := requestURL.Query()
	query.Set("cql", cql)
	query.Set("start", strconv.Itoa(start))
	query.Set("limit", strconv.Itoa(limit))
	requestURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return nil, false, "", fmt.Errorf("не удалось создать запрос Confluence: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	client.authorize(req)

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, false, "", fmt.Errorf("не удалось получить страницы из Confluence: %w", err)
	}
	defer resp.Body.Close()

	if err := client.checkStatus(resp); err != nil {
		return nil, false, "", err
	}

	var body struct {
		Results []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
			Links struct {
				WebUI string `json:"webui"`
			} `json:"_links"`
		} `json:"results"`
		Links struct {
			Next string `json:"next"`
			Base string `json:"base"`
		} `json:"_links"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, false, "", fmt.Errorf("не удалось декодировать ответ Confluence: %w", err)
	}

	candidates := make([]confluenceCandidate, 0, len(body.Results))
	for _, result := range body.Results {
		candidates = append(candidates, confluenceCandidate{
			ID:    result.ID,
			Title: result.Title,
			WebUI: result.Links.WebUI,
		})
	}

	return candidates, body.Links.Next != "", body.Links.Base, nil
}

// contributionDates обходит историю версий страницы и возвращает уникальные
// календарные дни, в которые contributor редактировал страницу в периоде.
func (client *ConfluenceClient) contributionDates(
	ctx context.Context,
	pageID string,
	contributor string,
	period Period,
) ([]time.Time, error) {
	seenDays := map[string]bool{}
	var dates []time.Time
	start := 0
	const limit = 200

	for {
		batch, hasNext, err := client.fetchVersionsBatch(ctx, pageID, start, limit)
		if err != nil {
			return nil, err
		}

		for _, version := range batch {
			if version.When.IsZero() {
				continue
			}
			if version.When.Before(period.From) || version.When.After(endOfDay(period.To)) {
				continue
			}
			if contributor != "" && !confluenceAuthorMatches(version.By, contributor) {
				continue
			}

			day := version.When.Format(time.DateOnly)
			if seenDays[day] {
				continue
			}
			seenDays[day] = true
			dates = append(dates, version.When)
		}

		if !hasNext || len(batch) == 0 {
			break
		}
		start += limit
	}

	return dates, nil
}

type confluenceVersion struct {
	When time.Time
	By   confluenceVersionAuthor
}

type confluenceVersionAuthor struct {
	AccountID   string `json:"accountId"`
	Username    string `json:"username"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
}

func (client *ConfluenceClient) fetchVersionsBatch(
	ctx context.Context,
	pageID string,
	start int,
	limit int,
) ([]confluenceVersion, bool, error) {
	endpoint, err := url.JoinPath(client.baseURL, "rest/api/content", pageID, "version")
	if err != nil {
		return nil, false, fmt.Errorf("не удалось собрать URL версий Confluence: %w", err)
	}

	requestURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, false, fmt.Errorf("не удалось разобрать URL версий Confluence: %w", err)
	}

	query := requestURL.Query()
	query.Set("start", strconv.Itoa(start))
	query.Set("limit", strconv.Itoa(limit))
	requestURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return nil, false, fmt.Errorf("не удалось создать запрос версий Confluence: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	client.authorize(req)

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("не удалось получить версии страницы Confluence: %w", err)
	}
	defer resp.Body.Close()

	if err := client.checkStatus(resp); err != nil {
		return nil, false, err
	}

	var body struct {
		Results []struct {
			When string                  `json:"when"`
			By   confluenceVersionAuthor `json:"by"`
		} `json:"results"`
		Links struct {
			Next string `json:"next"`
		} `json:"_links"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, false, fmt.Errorf("не удалось декодировать версии Confluence: %w", err)
	}

	versions := make([]confluenceVersion, 0, len(body.Results))
	for _, result := range body.Results {
		when, ok := parseJiraTime(result.When)
		if !ok {
			continue
		}
		versions = append(versions, confluenceVersion{
			When: when,
			By:   result.By,
		})
	}

	return versions, body.Links.Next != "", nil
}

// searchComments ищет комментарии, созданные contributor в периоде.
// Заголовок берётся из родительской страницы (container).
// Дублирующиеся (день + страница) отбрасываются.
func (client *ConfluenceClient) searchComments(
	ctx context.Context,
	contributor string,
	period Period,
	out io.Writer,
) ([]ConfluencePage, error) {
	cql := client.commentsCQL(contributor, period)
	_, _ = fmt.Fprintf(out, "CQL (комментарии): %s\n", cql)

	seen := map[string]bool{}
	var pages []ConfluencePage
	start := 0
	const limit = 50

	for {
		batch, hasNext, err := client.fetchCommentsBatch(ctx, cql, start, limit)
		if err != nil {
			return nil, err
		}

		for _, c := range batch {
			if c.Date.IsZero() || c.Title == "" {
				continue
			}
			key := c.Date.Format(time.DateOnly) + "\x00" + c.URL
			if seen[key] {
				continue
			}
			seen[key] = true
			pages = append(pages, c)
		}

		if !hasNext || len(batch) == 0 {
			break
		}
		start += limit
	}

	_, _ = fmt.Fprintf(out, "найдено комментариев Confluence: %d\n", len(pages))

	return pages, nil
}

func (client *ConfluenceClient) commentsCQL(contributor string, period Period) string {
	dateFrom := period.From.Format(time.DateOnly)
	dateTo := period.To.Format(time.DateOnly)

	if contributor == "" {
		return fmt.Sprintf(`type = comment AND created >= "%s" AND created <= "%s"`, dateFrom, dateTo)
	}

	return fmt.Sprintf(
		`type = comment AND creator = "%s" AND created >= "%s" AND created <= "%s"`,
		contributor, dateFrom, dateTo,
	)
}

func (client *ConfluenceClient) fetchCommentsBatch(
	ctx context.Context,
	cql string,
	start int,
	limit int,
) ([]ConfluencePage, bool, error) {
	endpoint, err := url.JoinPath(client.baseURL, "rest/api/content/search")
	if err != nil {
		return nil, false, fmt.Errorf("не удалось собрать URL комментариев Confluence: %w", err)
	}

	requestURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, false, fmt.Errorf("не удалось разобрать URL комментариев Confluence: %w", err)
	}

	query := requestURL.Query()
	query.Set("cql", cql)
	query.Set("start", strconv.Itoa(start))
	query.Set("limit", strconv.Itoa(limit))
	query.Set("expand", "version,container,container._links")
	requestURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return nil, false, fmt.Errorf("не удалось создать запрос комментариев Confluence: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	client.authorize(req)

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("не удалось получить комментарии из Confluence: %w", err)
	}
	defer resp.Body.Close()

	if err := client.checkStatus(resp); err != nil {
		return nil, false, err
	}

	var body struct {
		Results []struct {
			Version struct {
				When string `json:"when"`
			} `json:"version"`
			Container struct {
				Title string `json:"title"`
				Links struct {
					WebUI string `json:"webui"`
				} `json:"_links"`
			} `json:"container"`
		} `json:"results"`
		Links struct {
			Next string `json:"next"`
		} `json:"_links"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, false, fmt.Errorf("не удалось декодировать комментарии Confluence: %w", err)
	}

	pages := make([]ConfluencePage, 0, len(body.Results))
	for _, result := range body.Results {
		when, ok := parseJiraTime(result.Version.When)
		if !ok {
			continue
		}
		pages = append(pages, ConfluencePage{
			Title: result.Container.Title,
			URL:   result.Container.Links.WebUI,
			Date:  when,
		})
	}

	return pages, body.Links.Next != "", nil
}

func confluenceAuthorMatches(author confluenceVersionAuthor, contributor string) bool {
	contributor = strings.ToLower(strings.TrimSpace(contributor))
	values := []string{
		author.AccountID,
		author.Username,
		author.Email,
		author.DisplayName,
	}
	for _, v := range values {
		if strings.ToLower(strings.TrimSpace(v)) == contributor {
			return true
		}
	}
	return false
}

func (client *ConfluenceClient) checkStatus(resp *http.Response) error {
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	detail := strings.TrimSpace(string(body))

	// Попытка извлечь читаемое сообщение из JSON-ответа Confluence
	var errBody struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &errBody) == nil && errBody.Message != "" {
		detail = errBody.Message
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf(
			"Confluence вернула %s; проверь CONFLUENCE_TOKEN, CONFLUENCE_EMAIL и --confluence-auth\n%s",
			resp.Status, detail,
		)
	}

	if detail != "" {
		return fmt.Errorf("Confluence вернула %s: %s", resp.Status, detail)
	}
	return fmt.Errorf("Confluence вернула %s", resp.Status)
}

func (client *ConfluenceClient) authorize(req *http.Request) {
	switch client.resolvedAuthMode() {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+client.token)
	default:
		req.SetBasicAuth(client.email, client.token)
	}
}

func (client *ConfluenceClient) resolvedAuthMode() string {
	switch client.auth {
	case "bearer":
		return "bearer"
	case "basic":
		return "basic"
	default: // auto
		if client.email != "" {
			return "basic"
		}
		return "bearer"
	}
}

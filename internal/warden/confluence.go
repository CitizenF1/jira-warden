package warden

import (
	"context"
	"encoding/json"
	"fmt"
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

// Pages возвращает страницы Confluence с точными датами вклада.
// Для каждой страницы обходит историю версий и возвращает одну запись
// на каждый рабочий день, в который contributor вносил правки в периоде.
func (client *ConfluenceClient) Pages(ctx context.Context, contributor string, period Period) ([]ConfluencePage, error) {
	candidates, baseURL, err := client.searchCandidates(ctx, contributor, period.From)
	if err != nil {
		return nil, err
	}

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

	return pages, nil
}

// searchCandidates ищет страницы через CQL, в которых contributor участвовал
// начиная с from. Верхняя граница отсутствует намеренно: страница может быть
// отредактирована другим пользователем после окончания периода, при этом
// правка нашего contributor в нужный период всё равно должна попасть в результат.
func (client *ConfluenceClient) searchCandidates(
	ctx context.Context,
	contributor string,
	from time.Time,
) ([]confluenceCandidate, string, error) {
	contributorExpr := "contributor = currentUser()"
	if contributor != "" {
		contributorExpr = fmt.Sprintf(`contributor = "%s"`, contributor)
	}
	cql := fmt.Sprintf(
		`%s AND type in (page, blogpost) AND lastModified >= "%s"`,
		contributorExpr,
		from.Format(time.DateOnly),
	)

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
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf(
			"Confluence вернула %s; проверь CONFLUENCE_TOKEN, CONFLUENCE_EMAIL и --confluence-auth",
			resp.Status,
		)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("Confluence вернула %s", resp.Status)
	}
	return nil
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

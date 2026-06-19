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

func (client *ConfluenceClient) Pages(ctx context.Context, contributor string, period Period) ([]ConfluencePage, error) {
	cql := client.buildCQL(contributor, period)

	var pages []ConfluencePage
	start := 0
	const limit = 50

	for {
		batch, hasNext, responseBase, err := client.fetchPagesBatch(ctx, cql, start, limit)
		if err != nil {
			return nil, err
		}

		base := responseBase
		if base == "" {
			base = client.baseURL
		}
		for i := range batch {
			if !strings.HasPrefix(batch[i].URL, "http") {
				batch[i].URL = base + batch[i].URL
			}
		}

		pages = append(pages, batch...)
		if !hasNext || len(batch) == 0 {
			break
		}
		start += limit
	}

	return pages, nil
}

func (client *ConfluenceClient) buildCQL(contributor string, period Period) string {
	contributorExpr := "contributor = currentUser()"
	if contributor != "" {
		contributorExpr = fmt.Sprintf(`contributor = "%s"`, contributor)
	}

	return fmt.Sprintf(
		`%s AND type in (page, blogpost) AND lastModified >= "%s" AND lastModified <= "%s"`,
		contributorExpr,
		period.From.Format(time.DateOnly),
		period.To.Format(time.DateOnly),
	)
}

func (client *ConfluenceClient) fetchPagesBatch(
	ctx context.Context,
	cql string,
	start int,
	limit int,
) (pages []ConfluencePage, hasNext bool, responseBase string, err error) {
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
	query.Set("expand", "version")
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

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, false, "", fmt.Errorf(
			"Confluence вернула %s; проверь CONFLUENCE_TOKEN, CONFLUENCE_EMAIL и --confluence-auth",
			resp.Status,
		)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, false, "", fmt.Errorf("Confluence вернула %s", resp.Status)
	}

	var body struct {
		Results []struct {
			Title   string `json:"title"`
			Version struct {
				When string `json:"when"`
			} `json:"version"`
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

	pages = make([]ConfluencePage, 0, len(body.Results))
	for _, result := range body.Results {
		date, _ := parseJiraTime(result.Version.When)
		pages = append(pages, ConfluencePage{
			Title: result.Title,
			URL:   result.Links.WebUI,
			Date:  date,
		})
	}

	return pages, body.Links.Next != "", body.Links.Base, nil
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

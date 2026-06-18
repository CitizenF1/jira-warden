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

type MergeRequest struct {
	Title      string
	WebURL     string
	ActivityAt time.Time
}

type GitLabClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewGitLabClient(baseURL string, token string) *GitLabClient {
	return &GitLabClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      token,
		httpClient: http.DefaultClient,
	}
}

func (client *GitLabClient) MergeRequests(
	ctx context.Context,
	username string,
	period Period,
	state string,
) ([]MergeRequest, error) {
	result := []MergeRequest{}

	for page := 1; ; page++ {
		requestURL, err := client.mergeRequestsURL(username, period, state, page)
		if err != nil {
			return nil, err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
		if err != nil {
			return nil, fmt.Errorf("не удалось создать запрос GitLab: %w", err)
		}
		req.Header.Set("PRIVATE-TOKEN", client.token)

		resp, err := client.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("не удалось получить merge requests из GitLab: %w", err)
		}

		items, err := decodeGitLabMergeRequests(resp)
		if err != nil {
			return nil, err
		}

		for _, item := range items {
			if item.ActivityAt.Before(period.From) || item.ActivityAt.After(endOfDay(period.To)) {
				continue
			}
			result = append(result, item)
		}

		nextPage := resp.Header.Get("X-Next-Page")
		if nextPage == "" {
			break
		}
	}

	return result, nil
}

func (client *GitLabClient) mergeRequestsURL(
	username string,
	period Period,
	state string,
	page int,
) (string, error) {
	requestURL, err := url.Parse(client.baseURL + "/api/v4/merge_requests")
	if err != nil {
		return "", fmt.Errorf("не удалось разобрать GitLab URL: %w", err)
	}

	query := requestURL.Query()
	query.Set("scope", "all")
	query.Set("state", state)
	query.Set("author_username", username)
	query.Set("updated_after", period.From.Format(time.RFC3339))
	query.Set("updated_before", endOfDay(period.To).Format(time.RFC3339))
	query.Set("per_page", "100")
	query.Set("page", strconv.Itoa(page))
	requestURL.RawQuery = query.Encode()

	return requestURL.String(), nil
}

func decodeGitLabMergeRequests(resp *http.Response) ([]MergeRequest, error) {
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("GitLab вернул %s", resp.Status)
	}

	var apiItems []struct {
		Title     string `json:"title"`
		WebURL    string `json:"web_url"`
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
		MergedAt  string `json:"merged_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiItems); err != nil {
		return nil, fmt.Errorf("не удалось декодировать ответ GitLab: %w", err)
	}

	items := make([]MergeRequest, 0, len(apiItems))
	for _, apiItem := range apiItems {
		activityAtRaw := firstNotEmpty(apiItem.MergedAt, apiItem.UpdatedAt, apiItem.CreatedAt)
		if activityAtRaw == "" {
			continue
		}

		activityAt, err := time.Parse(time.RFC3339, activityAtRaw)
		if err != nil {
			return nil, fmt.Errorf("не удалось разобрать дату активности GitLab %q: %w", activityAtRaw, err)
		}

		items = append(items, MergeRequest{
			Title:      apiItem.Title,
			WebURL:     apiItem.WebURL,
			ActivityAt: activityAt,
		})
	}

	return items, nil
}

func firstNotEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}

	return ""
}

func endOfDay(day time.Time) time.Time {
	return day.Add(24*time.Hour - time.Nanosecond)
}

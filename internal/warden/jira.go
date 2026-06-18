package warden

import (
	"bytes"
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

type Worklog struct {
	IssueKey         string
	StartedDate      string
	TimeSpentSeconds int
	Comment          string
}

type JiraIssue struct {
	Key      string
	Assignee JiraAssignee
	Sprints  []JiraSprint
}

type JiraAssignee struct {
	AccountID    string `json:"accountId"`
	Name         string `json:"name"`
	EmailAddress string `json:"emailAddress"`
	DisplayName  string `json:"displayName"`
}

type JiraSprint struct {
	Name      string
	State     string
	StartDate string
	EndDate   string
}

type JiraWorklog struct {
	Author           JiraAssignee `json:"author"`
	Started          string       `json:"started"`
	TimeSpentSeconds int          `json:"timeSpentSeconds"`
}

type JiraClient struct {
	baseURL     string
	email       string
	token       string
	auth        string
	sprintField string
	httpClient  *http.Client
}

func NewJiraClient(
	baseURL string,
	email string,
	token string,
	auth string,
	sprintField string,
) *JiraClient {
	return &JiraClient{
		baseURL:     strings.TrimRight(baseURL, "/"),
		email:       email,
		token:       token,
		auth:        auth,
		sprintField: sprintField,
		httpClient:  http.DefaultClient,
	}
}

func (client *JiraClient) Issue(ctx context.Context, issueKey string) (JiraIssue, error) {
	originalSprintField := client.sprintField
	sprintField, err := client.effectiveSprintField(ctx)
	if err != nil {
		return JiraIssue{}, err
	}

	issue, err := client.issue(ctx, issueKey, sprintField)
	if err != nil {
		return JiraIssue{}, err
	}
	if len(issue.Sprints) > 0 || originalSprintField == "auto" {
		return issue, nil
	}

	discoveredField, err := client.discoverSprintField(ctx)
	if err != nil {
		return issue, nil
	}
	if discoveredField == sprintField {
		return issue, nil
	}

	return client.issue(ctx, issueKey, discoveredField)
}

func (client *JiraClient) issue(
	ctx context.Context,
	issueKey string,
	sprintField string,
) (JiraIssue, error) {
	endpoint, err := url.JoinPath(client.baseURL, "rest/api/2/issue", issueKey)
	if err != nil {
		return JiraIssue{}, fmt.Errorf("не удалось собрать URL задачи Jira: %w", err)
	}

	requestURL, err := url.Parse(endpoint)
	if err != nil {
		return JiraIssue{}, fmt.Errorf("не удалось разобрать URL задачи Jira: %w", err)
	}

	query := requestURL.Query()
	query.Set("fields", "assignee,"+sprintField)
	requestURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return JiraIssue{}, fmt.Errorf("не удалось создать запрос задачи Jira: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.do(req)
	if err != nil {
		return JiraIssue{}, fmt.Errorf("не удалось получить задачу Jira %s: %w", issueKey, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return JiraIssue{}, client.statusError(resp, fmt.Sprintf("получения задачи %s", issueKey))
	}

	var body struct {
		Key    string                     `json:"key"`
		Fields map[string]json.RawMessage `json:"fields"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return JiraIssue{}, fmt.Errorf("не удалось декодировать задачу Jira %s: %w", issueKey, err)
	}

	issue := JiraIssue{Key: body.Key, Sprints: []JiraSprint{}}
	if err := json.Unmarshal(body.Fields["assignee"], &issue.Assignee); err != nil {
		return JiraIssue{}, fmt.Errorf("не удалось декодировать исполнителя Jira для %s: %w", issueKey, err)
	}

	sprints, err := decodeJiraSprints(body.Fields[sprintField])
	if err != nil {
		return JiraIssue{}, fmt.Errorf("не удалось декодировать спринты Jira для %s: %w", issueKey, err)
	}
	issue.Sprints = sprints

	return issue, nil
}

func (client *JiraClient) effectiveSprintField(ctx context.Context) (string, error) {
	if client.sprintField != "auto" {
		return client.sprintField, nil
	}

	return client.discoverSprintField(ctx)
}

func (client *JiraClient) discoverSprintField(ctx context.Context) (string, error) {
	endpoint, err := url.JoinPath(client.baseURL, "rest/api/2/field")
	if err != nil {
		return "", fmt.Errorf("не удалось собрать URL полей Jira: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("не удалось создать запрос полей Jira: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.do(req)
	if err != nil {
		return "", fmt.Errorf("не удалось получить поля Jira: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", client.statusError(resp, "поиска поля спринта")
	}

	var fields []struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Custom bool   `json:"custom"`
		Schema struct {
			Custom string `json:"custom"`
		} `json:"schema"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&fields); err != nil {
		return "", fmt.Errorf("не удалось декодировать поля Jira: %w", err)
	}

	for _, field := range fields {
		if !field.Custom {
			continue
		}
		if isSprintField(field.Name, field.Schema.Custom) {
			client.sprintField = field.ID
			return field.ID, nil
		}
	}

	return "", fmt.Errorf("поле спринта Jira не найдено; укажи JIRA_SPRINT_FIELD явно")
}

func isSprintField(name string, schemaCustom string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	schemaCustom = strings.ToLower(schemaCustom)

	return name == "sprint" ||
		strings.Contains(name, "sprint") ||
		strings.Contains(schemaCustom, "sprint")
}

func (client *JiraClient) AddWorklog(ctx context.Context, worklog Worklog) error {
	endpoint, err := url.JoinPath(
		client.baseURL,
		"rest/api/2/issue",
		worklog.IssueKey,
		"worklog",
	)
	if err != nil {
		return fmt.Errorf("не удалось собрать URL Jira worklog: %w", err)
	}

	body := map[string]any{
		"comment":          worklog.Comment,
		"started":          worklog.StartedDate + "T09:00:00.000+0000",
		"timeSpentSeconds": worklog.TimeSpentSeconds,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("не удалось закодировать Jira worklog: %w", err)
	}

	reqFactory := func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("не удалось создать запрос Jira: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/json")

		return req, nil
	}

	resp, err := client.doWithFactory(reqFactory)
	if err != nil {
		return fmt.Errorf("не удалось отправить Jira worklog: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return client.statusError(resp, fmt.Sprintf("создания worklog для %s", worklog.IssueKey))
	}

	return nil
}

func (client *JiraClient) ExistingWorklogSeconds(
	ctx context.Context,
	date string,
	author string,
) (int, error) {
	issueKeys, err := client.worklogIssueKeys(ctx, date)
	if err != nil {
		return 0, err
	}

	var totalSeconds int
	for _, issueKey := range issueKeys {
		worklogs, err := client.issueWorklogs(ctx, issueKey)
		if err != nil {
			return 0, err
		}

		for _, worklog := range worklogs {
			if !worklogStartedOn(worklog.Started, date) {
				continue
			}
			if !assigneeMatches(worklog.Author, author) {
				continue
			}

			totalSeconds += worklog.TimeSpentSeconds
		}
	}

	return totalSeconds, nil
}

func (client *JiraClient) worklogIssueKeys(ctx context.Context, date string) ([]string, error) {
	keys := []string{}
	for startAt := 0; ; {
		endpoint, err := url.JoinPath(client.baseURL, "rest/api/2/search")
		if err != nil {
			return nil, fmt.Errorf("не удалось собрать URL поиска Jira worklog: %w", err)
		}

		requestURL, err := url.Parse(endpoint)
		if err != nil {
			return nil, fmt.Errorf("не удалось разобрать URL поиска Jira worklog: %w", err)
		}

		query := requestURL.Query()
		query.Set("jql", fmt.Sprintf(`worklogDate = "%s" AND worklogAuthor = currentUser()`, date))
		query.Set("fields", "key")
		query.Set("startAt", strconv.Itoa(startAt))
		query.Set("maxResults", "100")
		requestURL.RawQuery = query.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("не удалось создать запрос поиска Jira worklog: %w", err)
		}
		req.Header.Set("Accept", "application/json")

		resp, err := client.do(req)
		if err != nil {
			return nil, fmt.Errorf("не удалось найти задачи с Jira worklog за %s: %w", date, err)
		}

		var body struct {
			StartAt    int `json:"startAt"`
			MaxResults int `json:"maxResults"`
			Total      int `json:"total"`
			Issues     []struct {
				Key string `json:"key"`
			} `json:"issues"`
		}
		if err := decodeJiraResponse(resp, &body, fmt.Sprintf("поиска задач с worklog за %s", date), client); err != nil {
			return nil, err
		}

		for _, issue := range body.Issues {
			keys = append(keys, issue.Key)
		}

		startAt = body.StartAt + body.MaxResults
		if startAt >= body.Total || body.MaxResults == 0 {
			break
		}
	}

	return keys, nil
}

func (client *JiraClient) issueWorklogs(ctx context.Context, issueKey string) ([]JiraWorklog, error) {
	worklogs := []JiraWorklog{}
	for startAt := 0; ; {
		endpoint, err := url.JoinPath(client.baseURL, "rest/api/2/issue", issueKey, "worklog")
		if err != nil {
			return nil, fmt.Errorf("не удалось собрать URL Jira worklog: %w", err)
		}

		requestURL, err := url.Parse(endpoint)
		if err != nil {
			return nil, fmt.Errorf("не удалось разобрать URL Jira worklog: %w", err)
		}

		query := requestURL.Query()
		query.Set("startAt", strconv.Itoa(startAt))
		query.Set("maxResults", "100")
		requestURL.RawQuery = query.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("не удалось создать запрос Jira worklog: %w", err)
		}
		req.Header.Set("Accept", "application/json")

		resp, err := client.do(req)
		if err != nil {
			return nil, fmt.Errorf("не удалось получить Jira worklog для %s: %w", issueKey, err)
		}

		var body struct {
			StartAt    int           `json:"startAt"`
			MaxResults int           `json:"maxResults"`
			Total      int           `json:"total"`
			Worklogs   []JiraWorklog `json:"worklogs"`
		}
		if err := decodeJiraResponse(resp, &body, fmt.Sprintf("получения worklog для %s", issueKey), client); err != nil {
			return nil, err
		}

		worklogs = append(worklogs, body.Worklogs...)

		startAt = body.StartAt + body.MaxResults
		if startAt >= body.Total || body.MaxResults == 0 {
			break
		}
	}

	return worklogs, nil
}

func decodeJiraResponse(resp *http.Response, target any, action string, client *JiraClient) error {
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return client.statusError(resp, action)
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("не удалось декодировать ответ Jira во время %s: %w", action, err)
	}

	return nil
}

func worklogStartedOn(started string, date string) bool {
	startedAt, ok := parseJiraTime(started)
	if !ok {
		return false
	}

	return startedAt.Format(time.DateOnly) == date
}

func (client *JiraClient) do(req *http.Request) (*http.Response, error) {
	body, err := copyRequestBody(req)
	if err != nil {
		return nil, err
	}

	return client.doWithFactory(func() (*http.Request, error) {
		clone := req.Clone(req.Context())
		clone.Header = req.Header.Clone()
		if body != nil {
			clone.Body = io.NopCloser(bytes.NewReader(body))
			clone.ContentLength = int64(len(body))
		}

		return clone, nil
	})
}

func (client *JiraClient) doWithFactory(
	reqFactory func() (*http.Request, error),
) (*http.Response, error) {
	modes := client.authModes()
	for index, mode := range modes {
		req, err := reqFactory()
		if err != nil {
			return nil, err
		}
		client.authorize(req, mode)

		resp, err := client.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		isLastMode := index == len(modes)-1
		if resp.StatusCode != http.StatusUnauthorized || client.auth != "auto" || isLastMode {
			client.auth = mode
			return resp, nil
		}

		resp.Body.Close()
	}

	return nil, fmt.Errorf("нет доступных режимов авторизации Jira")
}

func copyRequestBody(req *http.Request) ([]byte, error) {
	if req.Body == nil {
		return nil, nil
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("не удалось скопировать тело запроса Jira: %w", err)
	}
	req.Body.Close()
	req.Body = io.NopCloser(bytes.NewReader(body))

	return body, nil
}

func (client *JiraClient) authModes() []string {
	switch client.auth {
	case "auto":
		if client.email == "" {
			return []string{"bearer"}
		}

		return []string{"bearer", "basic"}
	case "bearer":
		return []string{"bearer"}
	default:
		return []string{"basic"}
	}
}

func (client *JiraClient) authorize(req *http.Request, mode string) {
	if mode == "bearer" {
		req.Header.Set("Authorization", "Bearer "+client.token)
		return
	}

	req.SetBasicAuth(client.email, client.token)
}

func (client *JiraClient) statusError(resp *http.Response, action string) error {
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf(
			"Jira вернула %s во время %s; проверь JIRA_TOKEN, JIRA_EMAIL и -jira-auth=%s",
			resp.Status,
			action,
			client.auth,
		)
	}

	return fmt.Errorf("Jira вернула %s во время %s", resp.Status, action)
}

func decodeJiraSprints(raw json.RawMessage) ([]JiraSprint, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return []JiraSprint{}, nil
	}

	var objectSprints []JiraSprint
	if err := json.Unmarshal(raw, &objectSprints); err == nil {
		return objectSprints, nil
	}

	var stringSprints []string
	if err := json.Unmarshal(raw, &stringSprints); err != nil {
		return nil, err
	}

	sprints := make([]JiraSprint, 0, len(stringSprints))
	for _, value := range stringSprints {
		sprints = append(sprints, parseJiraSprintString(value))
	}

	return sprints, nil
}

func parseJiraSprintString(value string) JiraSprint {
	sprint := JiraSprint{}
	for _, part := range strings.Split(value, ",") {
		key, val, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}

		switch strings.TrimSpace(key) {
		case "name":
			sprint.Name = val
		case "state":
			sprint.State = val
		case "startDate":
			sprint.StartDate = val
		case "endDate":
			sprint.EndDate = val
		}
	}

	return sprint
}

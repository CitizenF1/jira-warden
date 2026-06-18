package warden

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
)

type IssueProvider interface {
	Issue(ctx context.Context, issueKey string) (JiraIssue, error)
}

func FilterEligibleWorklogs(
	ctx context.Context,
	worklogs []Worklog,
	provider IssueProvider,
	expectedAssignee string,
	out io.Writer,
) ([]Worklog, error) {
	issues := map[string]JiraIssue{}
	filtered := make([]Worklog, 0, len(worklogs))

	for _, worklog := range worklogs {
		issue, ok := issues[worklog.IssueKey]
		if !ok {
			var err error
			issue, err = provider.Issue(ctx, worklog.IssueKey)
			if err != nil {
				return nil, err
			}
			issues[worklog.IssueKey] = issue
		}

		if !assigneeMatches(issue.Assignee, expectedAssignee) {
			_, _ = fmt.Fprintf(out, "пропуск %s: задача назначена не на %s\n", worklog.IssueKey, expectedAssignee)
			continue
		}

		workDate, err := time.Parse(time.DateOnly, worklog.StartedDate)
		if err != nil {
			return nil, fmt.Errorf("не удалось разобрать дату worklog %q: %w", worklog.StartedDate, err)
		}

		if !hasSprintAt(issue.Sprints, workDate) {
			_, _ = fmt.Fprintf(out, "пропуск %s: задача не была в активном спринте на %s\n", worklog.IssueKey, worklog.StartedDate)
			continue
		}

		filtered = append(filtered, worklog)
	}

	return filtered, nil
}

func assigneeMatches(assignee JiraAssignee, expected string) bool {
	expected = strings.TrimSpace(strings.ToLower(expected))
	if expected == "" {
		return false
	}

	values := []string{
		assignee.AccountID,
		assignee.Name,
		assignee.EmailAddress,
		assignee.DisplayName,
	}
	for _, value := range values {
		if strings.ToLower(strings.TrimSpace(value)) == expected {
			return true
		}
	}

	return false
}

func hasSprintAt(sprints []JiraSprint, workDate time.Time) bool {
	for _, sprint := range sprints {
		start, ok := parseJiraTime(sprint.StartDate)
		if !ok || workDate.Before(dateOnly(start)) {
			continue
		}

		end, ok := parseJiraTime(sprint.EndDate)
		if ok && workDate.After(dateOnly(end)) {
			continue
		}

		return true
	}

	return false
}

func dateOnly(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

func parseJiraTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}

	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05.000-0700",
		"2006-01-02T15:04:05.000Z",
		time.DateOnly,
	}
	for _, format := range formats {
		parsed, err := time.Parse(format, value)
		if err == nil {
			return parsed, true
		}
	}

	return time.Time{}, false
}

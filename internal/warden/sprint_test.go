package warden

import (
	"bytes"
	"context"
	"testing"
)

func TestFilterEligibleWorklogs(t *testing.T) {
	provider := fakeIssueProvider{
		issues: map[string]JiraIssue{
			"PCS-123": {
				Assignee: JiraAssignee{EmailAddress: "me@example.com"},
				Sprints: []JiraSprint{
					{
						StartDate: "2026-06-01T00:00:00.000+0000",
						EndDate:   "2026-06-15T23:59:59.000+0000",
					},
				},
			},
			"PCS-456": {
				Assignee: JiraAssignee{EmailAddress: "other@example.com"},
				Sprints: []JiraSprint{
					{
						StartDate: "2026-06-01",
						EndDate:   "2026-06-30",
					},
				},
			},
			"PCS-789": {
				Assignee: JiraAssignee{EmailAddress: "me@example.com"},
				Sprints: []JiraSprint{
					{
						StartDate: "2026-06-20",
						EndDate:   "2026-06-30",
					},
				},
			},
		},
	}
	worklogs := []Worklog{
		{IssueKey: "PCS-123", StartedDate: "2026-06-10"},
		{IssueKey: "PCS-456", StartedDate: "2026-06-10"},
		{IssueKey: "PCS-789", StartedDate: "2026-06-10"},
		{IssueKey: "PCS-123", StartedDate: "2026-06-11"},
	}

	var out bytes.Buffer
	filtered, err := FilterEligibleWorklogs(
		context.Background(),
		worklogs,
		&provider,
		"me@example.com",
		&out,
	)
	if err != nil {
		t.Fatalf("filter worklogs: %v", err)
	}

	if len(filtered) != 2 {
		t.Fatalf("expected two worklogs, got %d", len(filtered))
	}
	if provider.calls != 3 {
		t.Fatalf("expected three issue fetches, got %d", provider.calls)
	}
	expectedOutput := "🛑 пропуск PCS-456: задача назначена не на me@example.com\n" +
		"🛑 пропуск PCS-789: задача не была в активном спринте на 2026-06-10\n"
	if out.String() != expectedOutput {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

type fakeIssueProvider struct {
	issues map[string]JiraIssue
	calls  int
}

func (provider *fakeIssueProvider) Issue(
	_ context.Context,
	issueKey string,
) (JiraIssue, error) {
	provider.calls++
	return provider.issues[issueKey], nil
}

package warden

import (
	"testing"
	"time"
)

func TestExtractContributions(t *testing.T) {
	requests := []MergeRequest{
		{
			Title:      "[PCS-123] add scoring",
			WebURL:     "https://gitlab.example.com/group/project/-/merge_requests/1",
			ActivityAt: mustDate(t, "2026-06-17"),
		},
		{
			Title:      "chore without issue key",
			WebURL:     "https://gitlab.example.com/group/project/-/merge_requests/2",
			ActivityAt: mustDate(t, "2026-06-17"),
		},
		{
			Title:      "feature/pcs-456-add-limit",
			WebURL:     "https://gitlab.example.com/group/project/-/merge_requests/3",
			ActivityAt: mustDate(t, "2026-06-17"),
		},
		{
			Title:      "fix: PCS-789 correct state",
			WebURL:     "https://gitlab.example.com/group/project/-/merge_requests/4",
			ActivityAt: mustDate(t, "2026-06-17"),
		},
		{
			Title:      "[PCS-111] [PCS-222] [PCS-111] shared changes",
			WebURL:     "https://gitlab.example.com/group/project/-/merge_requests/5",
			ActivityAt: mustDate(t, "2026-06-17"),
		},
		{
			Title:      "JIRA: PCS_333, ticket=pcs 444, refs #PCS-555",
			WebURL:     "https://gitlab.example.com/group/project/-/merge_requests/6",
			ActivityAt: mustDate(t, "2026-06-17"),
		},
		{
			Title:      "refactor/pcs_666-cleanup",
			WebURL:     "https://gitlab.example.com/group/project/-/merge_requests/7",
			ActivityAt: mustDate(t, "2026-06-17"),
		},
	}

	contributions, err := ExtractContributions(requests, []string{
		`(?i)\[([A-Z][A-Z0-9]+[-_ /]\d+)\]`,
		`(?i)\(([A-Z][A-Z0-9]+[-_ /]\d+)\)`,
		`(?i)#([A-Z][A-Z0-9]+[-_ /]\d+)\b`,
		`(?i)\b(?:jira|issue|ticket|task|ref|refs|related|close|closes|fix|fixes)[:=#\s]+([A-Z][A-Z0-9]+[-_ /]\d+)\b`,
		`(?i)\b(?:feature|feat|bugfix|fix|hotfix|release|task|chore|refactor|test|tests|docs|ci|dev|story|epic)[/_-]+([A-Z][A-Z0-9]+[-_ /]\d+)(?:[-_/][\w-]+)?`,
		`(?i)\b([A-Z][A-Z0-9]+[-_]\d+)\b`,
	})
	if err != nil {
		t.Fatalf("extract contributions: %v", err)
	}

	if len(contributions) != 9 {
		t.Fatalf("expected nine contributions, got %d", len(contributions))
	}
	if contributions[0].IssueKey != "PCS-123" {
		t.Fatalf("unexpected issue key: %s", contributions[0].IssueKey)
	}
	if contributions[1].IssueKey != "PCS-456" {
		t.Fatalf("unexpected issue key: %s", contributions[1].IssueKey)
	}
	if contributions[2].IssueKey != "PCS-789" {
		t.Fatalf("unexpected issue key: %s", contributions[2].IssueKey)
	}
	if contributions[3].IssueKey != "PCS-111" {
		t.Fatalf("unexpected issue key: %s", contributions[3].IssueKey)
	}
	if contributions[4].IssueKey != "PCS-222" {
		t.Fatalf("unexpected issue key: %s", contributions[4].IssueKey)
	}
	if contributions[5].IssueKey != "PCS-333" {
		t.Fatalf("unexpected issue key: %s", contributions[5].IssueKey)
	}
	if contributions[6].IssueKey != "PCS-444" {
		t.Fatalf("unexpected issue key: %s", contributions[6].IssueKey)
	}
	if contributions[7].IssueKey != "PCS-555" {
		t.Fatalf("unexpected issue key: %s", contributions[7].IssueKey)
	}
	if contributions[8].IssueKey != "PCS-666" {
		t.Fatalf("unexpected issue key: %s", contributions[8].IssueKey)
	}
}

func TestBuildWorklogsSplitsDayByIssues(t *testing.T) {
	period := Period{
		From: mustDate(t, "2026-06-17"),
		To:   mustDate(t, "2026-06-17"),
	}
	contributions := []Contribution{
		{
			IssueKey: "PCS-123",
			Title:    "[PCS-123] first",
			Date:     mustDate(t, "2026-06-17"),
		},
		{
			IssueKey: "PCS-456",
			Title:    "[PCS-456] second",
			Date:     mustDate(t, "2026-06-17"),
		},
	}

	worklogs := BuildWorklogs(contributions, period, 8, "GitLab MR")
	if len(worklogs) != 2 {
		t.Fatalf("expected two worklogs, got %d", len(worklogs))
	}

	for _, worklog := range worklogs {
		if worklog.TimeSpentSeconds != 4*3600 {
			t.Fatalf("expected 4 hours, got %d seconds", worklog.TimeSpentSeconds)
		}
	}
}

func TestBuildWorklogsCreatesTwoWorklogsForTwoIssuesInOneMergeRequest(t *testing.T) {
	period := Period{
		From: mustDate(t, "2026-06-17"),
		To:   mustDate(t, "2026-06-17"),
	}
	requests := []MergeRequest{
		{
			Title:      "[PCS-123] [PCS-124] shared implementation",
			WebURL:     "https://gitlab.example.com/group/project/-/merge_requests/1",
			ActivityAt: mustDate(t, "2026-06-17"),
		},
	}

	contributions, err := ExtractContributions(requests, []string{
		`(?i)\[([A-Z][A-Z0-9]+[-_ /]\d+)\]`,
	})
	if err != nil {
		t.Fatalf("extract contributions: %v", err)
	}

	worklogs := BuildWorklogs(contributions, period, 8, "GitLab MR")
	if len(worklogs) != 2 {
		t.Fatalf("expected two worklogs, got %d", len(worklogs))
	}
	if worklogs[0].IssueKey != "PCS-123" {
		t.Fatalf("unexpected first issue key: %s", worklogs[0].IssueKey)
	}
	if worklogs[1].IssueKey != "PCS-124" {
		t.Fatalf("unexpected second issue key: %s", worklogs[1].IssueKey)
	}
	for _, worklog := range worklogs {
		if worklog.TimeSpentSeconds != 4*3600 {
			t.Fatalf("expected 4 hours per issue, got %d seconds", worklog.TimeSpentSeconds)
		}
	}
}

func TestBuildWorklogsMovesWeekendToNextWorkday(t *testing.T) {
	period := Period{
		From: mustDate(t, "2026-06-19"),
		To:   mustDate(t, "2026-06-22"),
	}
	contributions := []Contribution{
		{
			IssueKey: "PCS-123",
			Title:    "[PCS-123] weekend mr",
			Date:     mustDate(t, "2026-06-20"),
		},
	}

	worklogs := BuildWorklogs(contributions, period, 8, "GitLab MR")
	if len(worklogs) != 1 {
		t.Fatalf("expected one worklog, got %d", len(worklogs))
	}
	if worklogs[0].StartedDate != "2026-06-22" {
		t.Fatalf("expected monday worklog, got %s", worklogs[0].StartedDate)
	}
}

func mustDate(t *testing.T, value string) time.Time {
	t.Helper()

	date, err := time.Parse(time.DateOnly, value)
	if err != nil {
		t.Fatalf("parse date: %v", err)
	}

	return date
}

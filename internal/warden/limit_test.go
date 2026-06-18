package warden

import (
	"bytes"
	"context"
	"testing"
)

func TestValidateDailyWorklogLimit(t *testing.T) {
	counter := fakeWorklogCounter{
		secondsByDate: map[string]int{
			"2026-06-18": 2 * 3600,
		},
	}
	worklogs := []Worklog{
		{StartedDate: "2026-06-18", TimeSpentSeconds: 3 * 3600},
		{StartedDate: "2026-06-18", TimeSpentSeconds: 2 * 3600},
	}

	var out bytes.Buffer
	err := ValidateDailyWorklogLimit(
		context.Background(),
		worklogs,
		&counter,
		"me@example.com",
		8,
		&out,
	)
	if err != nil {
		t.Fatalf("validate daily limit: %v", err)
	}
}

func TestValidateDailyWorklogLimitFailsWhenExceeded(t *testing.T) {
	counter := fakeWorklogCounter{
		secondsByDate: map[string]int{
			"2026-06-18": 4 * 3600,
		},
	}
	worklogs := []Worklog{
		{StartedDate: "2026-06-18", TimeSpentSeconds: 5 * 3600},
	}

	var out bytes.Buffer
	err := ValidateDailyWorklogLimit(
		context.Background(),
		worklogs,
		&counter,
		"me@example.com",
		8,
		&out,
	)
	if err == nil {
		t.Fatal("expected daily limit error")
	}
}

type fakeWorklogCounter struct {
	secondsByDate map[string]int
}

func (counter *fakeWorklogCounter) ExistingWorklogSeconds(
	_ context.Context,
	date string,
	_ string,
) (int, error) {
	return counter.secondsByDate[date], nil
}

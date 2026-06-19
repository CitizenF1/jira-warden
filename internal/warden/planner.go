package warden

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
)

type Contribution struct {
	IssueKey string
	Title    string
	URL      string
	Date     time.Time
}

func ExtractContributions(
	requests []MergeRequest,
	patterns []string,
) ([]Contribution, error) {
	matchers, err := compileIssuePatterns(patterns)
	if err != nil {
		return nil, err
	}

	contributions := make([]Contribution, 0, len(requests))
	for _, request := range requests {
		issueKeys := findIssueKeys(request.Title, matchers)
		if len(issueKeys) == 0 {
			continue
		}

		for _, issueKey := range issueKeys {
			contributions = append(contributions, Contribution{
				IssueKey: issueKey,
				Title:    request.Title,
				URL:      request.WebURL,
				Date:     request.ActivityAt,
			})
		}
	}

	return contributions, nil
}

func compileIssuePatterns(patterns []string) ([]*regexp.Regexp, error) {
	matchers := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("не удалось скомпилировать issue pattern %q: %w", pattern, err)
		}
		if re.NumSubexp() < 1 {
			return nil, fmt.Errorf("issue pattern %q должен содержать хотя бы одну capture group", pattern)
		}

		matchers = append(matchers, re)
	}

	return matchers, nil
}

func findIssueKeys(title string, matchers []*regexp.Regexp) []string {
	matches := []issueKeyMatch{}
	for _, matcher := range matchers {
		for _, indexes := range matcher.FindAllStringSubmatchIndex(title, -1) {
			hasFirstGroup := len(indexes) >= 4 && indexes[2] >= 0 && indexes[3] >= 0
			if !hasFirstGroup {
				continue
			}

			matches = append(matches, issueKeyMatch{
				start: indexes[2],
				key:   normalizeIssueKey(title[indexes[2]:indexes[3]]),
			})
		}
	}

	slices.SortFunc(matches, func(left issueKeyMatch, right issueKeyMatch) int {
		return left.start - right.start
	})

	seen := map[string]bool{}
	issueKeys := []string{}
	for _, match := range matches {
		if seen[match.key] {
			continue
		}

		seen[match.key] = true
		issueKeys = append(issueKeys, match.key)
	}

	return issueKeys
}

type issueKeyMatch struct {
	start int
	key   string
}

func normalizeIssueKey(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ToUpper(value)

	replacer := strings.NewReplacer(
		"_", "-",
		"/", "-",
		" ", "-",
	)

	return replacer.Replace(value)
}

func ExtractContributionsFromPages(pages []ConfluencePage, patterns []string) ([]Contribution, error) {
	matchers, err := compileIssuePatterns(patterns)
	if err != nil {
		return nil, err
	}

	contributions := make([]Contribution, 0, len(pages))
	for _, page := range pages {
		issueKeys := findIssueKeys(page.Title, matchers)
		if len(issueKeys) == 0 {
			continue
		}

		for _, issueKey := range issueKeys {
			contributions = append(contributions, Contribution{
				IssueKey: issueKey,
				Title:    page.Title,
				URL:      page.URL,
				Date:     page.Date,
			})
		}
	}

	return contributions, nil
}

func BuildWorklogs(
	contributions []Contribution,
	period Period,
	hoursPerDay float64,
	commentPrefix string,
) []Worklog {
	byDate := map[string]map[string][]Contribution{}
	for _, contribution := range contributions {
		date := contribution.Date.Format(time.DateOnly)
		if !isWorkday(contribution.Date) {
			date = nextWorkday(contribution.Date).Format(time.DateOnly)
		}

		if byDate[date] == nil {
			byDate[date] = map[string][]Contribution{}
		}
		byDate[date][contribution.IssueKey] = append(
			byDate[date][contribution.IssueKey],
			contribution,
		)
	}

	dates := make([]string, 0, len(byDate))
	for date := range byDate {
		dateTime, err := time.Parse(time.DateOnly, date)
		if err != nil {
			continue
		}
		if dateTime.Before(period.From) || dateTime.After(period.To) {
			continue
		}
		dates = append(dates, date)
	}
	slices.Sort(dates)

	worklogs := []Worklog{}
	for _, date := range dates {
		issues := make([]string, 0, len(byDate[date]))
		for issueKey := range byDate[date] {
			issues = append(issues, issueKey)
		}
		slices.Sort(issues)

		secondsPerIssue := int(hoursPerDay * 3600 / float64(len(issues)))
		for _, issueKey := range issues {
			items := byDate[date][issueKey]
			worklogs = append(worklogs, Worklog{
				IssueKey:         issueKey,
				StartedDate:      date,
				TimeSpentSeconds: secondsPerIssue,
				Comment:          buildComment(commentPrefix, items),
			})
		}
	}

	return worklogs
}

func buildComment(prefix string, contributions []Contribution) string {
	lines := []string{prefix}
	for _, contribution := range contributions {
		line := "- " + contribution.Title
		if contribution.URL != "" {
			line += " " + contribution.URL
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func isWorkday(day time.Time) bool {
	weekday := day.Weekday()
	return weekday != time.Saturday && weekday != time.Sunday
}

func nextWorkday(day time.Time) time.Time {
	next := day
	for !isWorkday(next) {
		next = next.AddDate(0, 0, 1)
	}

	return next
}

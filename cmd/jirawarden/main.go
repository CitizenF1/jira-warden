package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"warden/internal/warden"
)

var version = "dev"

var defaultIssuePatterns = []string{
	`(?i)\[([A-Z][A-Z0-9]+[-_ /]\d+)\]`,
	`(?i)\(([A-Z][A-Z0-9]+[-_ /]\d+)\)`,
	`(?i)#([A-Z][A-Z0-9]+[-_ /]\d+)\b`,
	`(?i)\b(?:jira|issue|ticket|task|ref|refs|related|close|closes|fix|fixes)[:=#\s]+([A-Z][A-Z0-9]+[-_ /]\d+)\b`,
	`(?i)\b(?:feature|feat|bugfix|fix|hotfix|release|task|chore|refactor|test|tests|docs|ci|dev|story|epic)[/_-]+([A-Z][A-Z0-9]+[-_ /]\d+)(?:[-_/][\w-]+)?`,
	`(?i)\b([A-Z][A-Z0-9]+[-_]\d+)\b`,
}

func main() {
	loadDotEnv(".env")

	cmd := newRootCommand(context.Background())
	cmd.SetArgs(normalizeLegacyFlags(os.Args[1:]))

	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func newRootCommand(ctx context.Context) *cobra.Command {
	var cfg warden.Config
	var from string
	var to string
	var showVersion bool
	issuePatterns := issuePatternFlag(defaultIssuePatterns)

	cmd := &cobra.Command{
		Use:           "jirawarden",
		Short:         "Создает Jira worklog по GitLab merge requests",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			if showVersion {
				fmt.Fprintln(os.Stdout, version)
				return nil
			}

			return run(ctx, cfg, issuePatterns.values(), from, to)
		},
	}

	cmd.AddCommand(newConfluenceCommand(ctx))

	flags := cmd.Flags()
	flags.StringVar(&cfg.GitLabURL, "gitlab-url", getenv("GITLAB_URL", ""), "базовый URL GitLab")
	flags.StringVar(&cfg.GitLabToken, "gitlab-token", getenv("GITLAB_TOKEN", ""), "private token GitLab")
	flags.StringVar(&cfg.GitLabUsername, "gitlab-user", getenv("GITLAB_USER", ""), "имя пользователя GitLab")
	flags.StringVar(&cfg.GitLabMRState, "gitlab-mr-state", getenv("GITLAB_MR_STATE", "all"), "состояние GitLab MR: all, opened, closed, locked или merged")
	flags.StringVar(&cfg.JiraURL, "jira-url", getenv("JIRA_URL", ""), "базовый URL Jira")
	flags.StringVar(&cfg.JiraEmail, "jira-email", getenv("JIRA_EMAIL", ""), "email пользователя Jira")
	flags.StringVar(&cfg.JiraToken, "jira-token", getenv("JIRA_TOKEN", ""), "API token Jira")
	flags.StringVar(&cfg.JiraAuth, "jira-auth", getenv("JIRA_AUTH", "auto"), "режим авторизации Jira: auto, basic или bearer")
	flags.StringVar(&cfg.JiraSprintField, "jira-sprint-field", getenv("JIRA_SPRINT_FIELD", "auto"), "custom field спринта Jira или auto")
	flags.StringVar(&cfg.JiraAssignee, "jira-assignee", getenv("JIRA_ASSIGNEE", getenv("JIRA_EMAIL", "")), "ожидаемый исполнитель Jira: accountId, name, email или display name")
	flags.Var(&issuePatterns, "issue-pattern", "дополнительная regexp с одной группой Jira key; можно повторять")
	flags.StringVar(&cfg.CommentPrefix, "comment-prefix", "GitLab MR", "префикс комментария Jira worklog")
	flags.Float64Var(&cfg.HoursPerDay, "hours-per-day", 8, "сколько часов распределять на рабочий день")
	flags.Float64Var(&cfg.MaxHoursPerDay, "max-hours-per-day", 8, "максимальный суммарный Jira worklog за день")
	flags.BoolVar(&cfg.RequireSprint, "require-sprint", true, "пропускать Jira задачи вне активного спринта")
	flags.BoolVar(&cfg.AssumeYes, "yes", false, "создавать Jira worklog без интерактивного подтверждения")
	flags.BoolVar(&cfg.DryRun, "dry-run", true, "показать worklog без отправки в Jira")
	flags.StringVar(&from, "from", "", "дата начала, YYYY-MM-DD")
	flags.StringVar(&to, "to", "", "дата окончания, YYYY-MM-DD")
	flags.BoolVar(&showVersion, "version", false, "вывести версию и выйти")

	return cmd
}

func run(
	ctx context.Context,
	cfg warden.Config,
	issuePatterns []string,
	from string,
	to string,
) error {
	startDate, endDate, err := parsePeriod(from, to)
	if err != nil {
		return err
	}

	cfg.IssuePatterns = issuePatterns
	cfg.Period = warden.Period{
		From: startDate,
		To:   endDate,
	}

	return warden.Run(ctx, cfg, os.Stdout, os.Stdin)
}

type issuePatternFlag []string

func (flag *issuePatternFlag) String() string {
	return strings.Join(*flag, ", ")
}

func (flag *issuePatternFlag) Set(value string) error {
	if value == "" {
		return fmt.Errorf("issue pattern не может быть пустым")
	}

	*flag = append(*flag, value)
	return nil
}

func (flag *issuePatternFlag) Type() string {
	return "regexp"
}

func (flag issuePatternFlag) values() []string {
	patterns := make([]string, 0, len(flag))
	for _, pattern := range flag {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		patterns = append(patterns, pattern)
	}

	return patterns
}

func newConfluenceCommand(ctx context.Context) *cobra.Command {
	var cfg warden.ConfluenceConfig
	var from, to string
	issuePatterns := issuePatternFlag(defaultIssuePatterns)

	cmd := &cobra.Command{
		Use:           "confluence",
		Short:         "Создает Jira worklog по активности в Confluence",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runConfluence(ctx, cfg, issuePatterns.values(), from, to)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&cfg.ConfluenceURL, "confluence-url", getenv("CONFLUENCE_URL", ""), "базовый URL Confluence")
	flags.StringVar(&cfg.ConfluenceToken, "confluence-token", getenv("CONFLUENCE_TOKEN", ""), "API token Confluence")
	flags.StringVar(&cfg.ConfluenceEmail, "confluence-email", getenv("CONFLUENCE_EMAIL", ""), "email для basic-авторизации Confluence")
	flags.StringVar(&cfg.ConfluenceAuth, "confluence-auth", getenv("CONFLUENCE_AUTH", "auto"), "режим авторизации Confluence: auto, basic или bearer")
	flags.StringVar(&cfg.ConfluenceUser, "confluence-user", getenv("CONFLUENCE_USER", ""), "пользователь Confluence для CQL (username, email или accountId; пусто = currentUser())")
	flags.StringVar(&cfg.JiraURL, "jira-url", getenv("JIRA_URL", ""), "базовый URL Jira")
	flags.StringVar(&cfg.JiraEmail, "jira-email", getenv("JIRA_EMAIL", ""), "email пользователя Jira")
	flags.StringVar(&cfg.JiraToken, "jira-token", getenv("JIRA_TOKEN", ""), "API token Jira")
	flags.StringVar(&cfg.JiraAuth, "jira-auth", getenv("JIRA_AUTH", "auto"), "режим авторизации Jira: auto, basic или bearer")
	flags.StringVar(&cfg.JiraSprintField, "jira-sprint-field", getenv("JIRA_SPRINT_FIELD", "auto"), "custom field спринта Jira или auto")
	flags.StringVar(&cfg.JiraAssignee, "jira-assignee", getenv("JIRA_ASSIGNEE", getenv("JIRA_EMAIL", "")), "ожидаемый исполнитель Jira: accountId, name, email или display name")
	flags.Var(&issuePatterns, "issue-pattern", "дополнительная regexp с одной группой Jira key; можно повторять")
	flags.StringVar(&cfg.CommentPrefix, "comment-prefix", "Confluence", "префикс комментария Jira worklog")
	flags.Float64Var(&cfg.HoursPerDay, "hours-per-day", 8, "сколько часов распределять на рабочий день")
	flags.Float64Var(&cfg.MaxHoursPerDay, "max-hours-per-day", 8, "максимальный суммарный Jira worklog за день")
	flags.BoolVar(&cfg.RequireSprint, "require-sprint", true, "пропускать Jira задачи вне активного спринта")
	flags.BoolVar(&cfg.AssumeYes, "yes", false, "создавать Jira worklog без интерактивного подтверждения")
	flags.BoolVar(&cfg.DryRun, "dry-run", true, "показать worklog без отправки в Jira")
	flags.StringVar(&from, "from", "", "дата начала, YYYY-MM-DD")
	flags.StringVar(&to, "to", "", "дата окончания, YYYY-MM-DD")

	return cmd
}

func parsePeriod(from, to string) (time.Time, time.Time, error) {
	today := time.Now().Truncate(24 * time.Hour)

	startDate := today
	if from != "" {
		parsed, err := time.Parse(time.DateOnly, from)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("не удалось разобрать -from: %w", err)
		}
		startDate = parsed
	}

	endDate := today
	if to != "" {
		parsed, err := time.Parse(time.DateOnly, to)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("не удалось разобрать -to: %w", err)
		}
		endDate = parsed
	}

	return startDate, endDate, nil
}

func runConfluence(
	ctx context.Context,
	cfg warden.ConfluenceConfig,
	issuePatterns []string,
	from string,
	to string,
) error {
	startDate, endDate, err := parsePeriod(from, to)
	if err != nil {
		return err
	}

	cfg.IssuePatterns = issuePatterns
	cfg.Period = warden.Period{
		From: startDate,
		To:   endDate,
	}

	return warden.RunConfluence(ctx, cfg, os.Stdout, os.Stdin)
}

func loadDotEnv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		if comment := strings.Index(value, " #"); comment >= 0 {
			value = strings.TrimSpace(value[:comment])
		}

		value = strings.Trim(value, `"'`)

		if key != "" && os.Getenv(key) == "" {
			_ = os.Setenv(key, value)
		}
	}
}

func getenv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}

func normalizeLegacyFlags(args []string) []string {
	normalized := make([]string, 0, len(args))
	for _, arg := range args {
		isLongSingleDash := strings.HasPrefix(arg, "-") &&
			!strings.HasPrefix(arg, "--") &&
			len(arg) > 2
		if isLongSingleDash {
			normalized = append(normalized, "-"+arg)
			continue
		}

		normalized = append(normalized, arg)
	}

	return normalized
}

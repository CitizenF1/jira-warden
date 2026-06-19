package warden

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
)

func Run(ctx context.Context, cfg Config, out io.Writer, in io.Reader) error {
	if err := cfg.validate(); err != nil {
		return err
	}

	gitlab := NewGitLabClient(cfg.GitLabURL, cfg.GitLabToken)
	requests, err := gitlab.MergeRequests(
		ctx,
		cfg.GitLabUsername,
		cfg.Period,
		cfg.GitLabMRState,
	)
	if err != nil {
		return err
	}

	contributions, err := ExtractContributions(requests, cfg.IssuePatterns)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(out, "найдено contributions: %d\n", len(contributions))

	worklogs := BuildWorklogs(
		contributions,
		cfg.Period,
		cfg.HoursPerDay,
		cfg.CommentPrefix,
	)
	if len(worklogs) == 0 {
		_, _ = fmt.Fprintln(out, "❌ нет worklog для создания")
		return nil
	}

	jira := NewJiraClient(
		cfg.JiraURL,
		cfg.JiraEmail,
		cfg.JiraToken,
		cfg.JiraAuth,
		cfg.JiraSprintField,
	)
	if cfg.RequireSprint {
		worklogs, err = FilterEligibleWorklogs(
			ctx,
			worklogs,
			jira,
			cfg.JiraAssignee,
			out,
		)
		if err != nil {
			return err
		}
		if len(worklogs) == 0 {
			_, _ = fmt.Fprintln(out, "❌ после фильтрации по спринту нет worklog для создания")
			return nil
		}
	}

	for _, worklog := range worklogs {
		_, _ = fmt.Fprintf(
			out,
			"🕛StartedDate: %s 🔷IssueKey: %s ⌛TimeSpent: %.2fч 💬Comment: %s\n",
			worklog.StartedDate,
			worklog.IssueKey,
			float64(worklog.TimeSpentSeconds)/3600,
			worklog.Comment,
		)
		_, _ = fmt.Fprintln(out, "-----------------------------------------")
	}

	if cfg.DryRun {
		_, _ = fmt.Fprintln(out, "⛔ включен dry-run: Jira не была изменена")
		return nil
	}

	if err := ValidateDailyWorklogLimit(
		ctx,
		worklogs,
		jira,
		cfg.JiraAssignee,
		cfg.MaxHoursPerDay,
		out,
	); err != nil {
		return err
	}

	if !cfg.AssumeYes {
		confirmed, err := confirmWorklogWrite(out, in)
		if err != nil {
			return err
		}
		if !confirmed {
			_, _ = fmt.Fprintln(out, "❌ отменено: Jira не была изменена")
			return nil
		}
	}

	for _, worklog := range worklogs {
		if err := jira.AddWorklog(ctx, worklog); err != nil {
			return err
		}
	}

	_, _ = fmt.Fprintln(out, "✅ Jira worklog созданы")

	return nil
}

func confirmWorklogWrite(out io.Writer, in io.Reader) (bool, error) {
	_, _ = fmt.Fprint(out, "Создать эти Jira worklog? Введите yes для продолжения [y/N]: ")

	reader := bufio.NewReader(in)
	answer, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return false, fmt.Errorf("не удалось прочитать подтверждение: %w", err)
	}

	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes", nil
}

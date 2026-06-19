package warden

import (
	"fmt"
	"strings"
	"time"
)

type Config struct {
	GitLabURL       string
	GitLabToken     string
	GitLabUsername  string
	GitLabMRState   string
	JiraURL         string
	JiraEmail       string
	JiraToken       string
	JiraAuth        string
	JiraSprintField string
	JiraAssignee    string
	IssuePatterns   []string
	CommentPrefix   string
	Period          Period
	HoursPerDay     float64
	MaxHoursPerDay  float64
	RequireSprint   bool
	AssumeYes       bool
	DryRun          bool
}

type Period struct {
	From time.Time
	To   time.Time
}

type ConfluenceConfig struct {
	ConfluenceURL   string
	ConfluenceEmail string
	ConfluenceToken string
	ConfluenceAuth  string
	ConfluenceUser  string
	JiraURL         string
	JiraEmail       string
	JiraToken       string
	JiraAuth        string
	JiraSprintField string
	JiraAssignee    string
	IssuePatterns   []string
	CommentPrefix   string
	Period          Period
	HoursPerDay     float64
	MaxHoursPerDay  float64
	RequireSprint   bool
	AssumeYes       bool
	DryRun          bool
}

func (cfg ConfluenceConfig) validate() error {
	if cfg.ConfluenceURL == "" {
		return fmt.Errorf("нужно указать Confluence URL")
	}
	if cfg.ConfluenceToken == "" {
		return fmt.Errorf("нужно указать Confluence token")
	}
	switch cfg.ConfluenceAuth {
	case "auto", "basic", "bearer":
	default:
		return fmt.Errorf("confluence auth должен быть auto, basic или bearer")
	}
	needsJira := !cfg.DryRun || cfg.RequireSprint
	if needsJira && cfg.JiraURL == "" {
		return fmt.Errorf("нужно указать Jira URL")
	}
	if needsJira && cfg.JiraToken == "" {
		return fmt.Errorf("нужно указать Jira token")
	}
	if needsJira && strings.TrimSpace(cfg.JiraSprintField) == "" {
		return fmt.Errorf("нужно указать Jira sprint field")
	}
	if needsJira && cfg.JiraAssignee == "" {
		return fmt.Errorf("нужно указать Jira assignee")
	}
	switch cfg.JiraAuth {
	case "auto":
	case "basic":
		if needsJira && cfg.JiraEmail == "" {
			return fmt.Errorf("для basic-авторизации нужно указать Jira email")
		}
	case "bearer":
	default:
		return fmt.Errorf("jira auth должен быть auto, basic или bearer")
	}
	if len(cfg.IssuePatterns) == 0 {
		return fmt.Errorf("нужен хотя бы один issue pattern")
	}
	if cfg.HoursPerDay <= 0 {
		return fmt.Errorf("hours per day должен быть больше нуля")
	}
	if cfg.MaxHoursPerDay <= 0 {
		return fmt.Errorf("max hours per day должен быть больше нуля")
	}
	if cfg.Period.From.IsZero() || cfg.Period.To.IsZero() {
		return fmt.Errorf("нужно указать период")
	}
	if cfg.Period.To.Before(cfg.Period.From) {
		return fmt.Errorf("дата окончания периода должна быть после даты начала")
	}
	return nil
}

func (cfg Config) validate() error {
	if cfg.GitLabURL == "" {
		return fmt.Errorf("нужно указать GitLab URL")
	}
	if cfg.GitLabToken == "" {
		return fmt.Errorf("нужно указать GitLab token")
	}
	if cfg.GitLabUsername == "" {
		return fmt.Errorf("нужно указать GitLab username")
	}
	switch cfg.GitLabMRState {
	case "all", "opened", "closed", "locked", "merged":
	default:
		return fmt.Errorf("состояние GitLab MR должно быть all, opened, closed, locked или merged")
	}
	needsJira := !cfg.DryRun || cfg.RequireSprint
	if needsJira && cfg.JiraURL == "" {
		return fmt.Errorf("нужно указать Jira URL")
	}
	if needsJira && cfg.JiraToken == "" {
		return fmt.Errorf("нужно указать Jira token")
	}
	if needsJira && strings.TrimSpace(cfg.JiraSprintField) == "" {
		return fmt.Errorf("нужно указать Jira sprint field")
	}
	if needsJira && cfg.JiraAssignee == "" {
		return fmt.Errorf("нужно указать Jira assignee")
	}
	switch cfg.JiraAuth {
	case "auto":
	case "basic":
		if needsJira && cfg.JiraEmail == "" {
			return fmt.Errorf("для basic-авторизации нужно указать Jira email")
		}
	case "bearer":
	default:
		return fmt.Errorf("jira auth должен быть auto, basic или bearer")
	}
	if len(cfg.IssuePatterns) == 0 {
		return fmt.Errorf("нужен хотя бы один issue pattern")
	}
	if cfg.HoursPerDay <= 0 {
		return fmt.Errorf("hours per day должен быть больше нуля")
	}
	if cfg.MaxHoursPerDay <= 0 {
		return fmt.Errorf("max hours per day должен быть больше нуля")
	}
	if cfg.Period.From.IsZero() || cfg.Period.To.IsZero() {
		return fmt.Errorf("нужно указать период")
	}
	if cfg.Period.To.Before(cfg.Period.From) {
		return fmt.Errorf("дата окончания периода должна быть после даты начала")
	}

	return nil
}

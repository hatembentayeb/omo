package main

import (
	"fmt"
	"strings"
	"time"
)

type PullRequest struct {
	Number    int
	Title     string
	State     string
	Author    string
	Branch    string
	Base      string
	Draft     bool
	Mergeable string
	Labels    []string
	Reviewers []string
	CreatedAt time.Time
	UpdatedAt time.Time
	URL       string
	Additions int
	Deletions int
	Comments  int
	Commits   int
}

func (pr *PullRequest) GetTableRow() []string {
	state := pr.State
	if pr.Draft {
		state = "draft"
	}
	labels := strings.Join(pr.Labels, ", ")
	if labels == "" {
		labels = "-"
	}
	age := formatAge(pr.CreatedAt)
	changes := fmt.Sprintf("+%d/-%d", pr.Additions, pr.Deletions)

	return []string{
		fmt.Sprintf("#%d", pr.Number),
		pr.Title,
		state,
		pr.Author,
		pr.Branch,
		pr.Base,
		changes,
		labels,
		age,
	}
}

type WorkflowRun struct {
	ID           int64
	Name         string
	WorkflowName string
	Status       string
	Conclusion   string
	Branch       string
	Event        string
	Actor        string
	Duration     string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	URL          string
	RunNumber    int
	Attempt      int
}

func (wr *WorkflowRun) GetTableRow() []string {
	status := wr.Status
	if wr.Conclusion != "" {
		status = wr.Conclusion
	}
	age := formatAge(wr.CreatedAt)

	return []string{
		fmt.Sprintf("%d", wr.ID),
		wr.WorkflowName,
		status,
		wr.Branch,
		wr.Event,
		wr.Actor,
		wr.Duration,
		age,
	}
}

type Workflow struct {
	ID        int64
	Name      string
	Path      string
	State     string
	CreatedAt time.Time
	UpdatedAt time.Time
	URL       string
	BadgeURL  string
}

func (w *Workflow) GetTableRow() []string {
	return []string{
		fmt.Sprintf("%d", w.ID),
		w.Name,
		w.Path,
		w.State,
		formatAge(w.UpdatedAt),
	}
}

type EnvVariable struct {
	Name      string
	Value     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (ev *EnvVariable) GetTableRow() []string {
	return []string{
		ev.Name,
		ev.Value,
		formatAge(ev.UpdatedAt),
	}
}

type RepoSecret struct {
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (rs *RepoSecret) GetTableRow() []string {
	return []string{
		rs.Name,
		"********",
		formatAge(rs.UpdatedAt),
	}
}

type Branch struct {
	Name      string
	SHA       string
	Protected bool
	Default   bool
}

func (b *Branch) GetTableRow() []string {
	protected := "No"
	if b.Protected {
		protected = "Yes"
	}
	isDefault := ""
	if b.Default {
		isDefault = "*"
	}
	shortSHA := b.SHA
	if len(shortSHA) > 8 {
		shortSHA = shortSHA[:8]
	}
	return []string{
		b.Name + isDefault,
		shortSHA,
		protected,
	}
}

type Release struct {
	ID          int64
	TagName     string
	Name        string
	Draft       bool
	Prerelease  bool
	Author      string
	CreatedAt   time.Time
	PublishedAt time.Time
	URL         string
	Body        string
	Assets      int
}

func (r *Release) GetTableRow() []string {
	status := "published"
	if r.Draft {
		status = "draft"
	} else if r.Prerelease {
		status = "prerelease"
	}
	return []string{
		r.TagName,
		r.Name,
		status,
		r.Author,
		fmt.Sprintf("%d", r.Assets),
		formatAge(r.PublishedAt),
	}
}

type PRReview struct {
	ID        int64
	User      string
	State     string
	Body      string
	CreatedAt time.Time
}

type PRCheck struct {
	Name       string
	Status     string
	Conclusion string
	URL        string
}

type WorkflowRunJob struct {
	ID         int64
	Name       string
	Status     string
	Conclusion string
	StartedAt  time.Time
	Duration   string
	RunnerName string
}

type Environment struct {
	ID        int64
	Name      string
	URL       string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func formatAge(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		m := int(d.Minutes())
		s := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm%ds", m, s)
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh%dm", h, m)
}

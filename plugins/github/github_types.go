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
		state = "[gray]draft"
	} else if state == "open" {
		state = "[green]open"
	} else if state == "closed" {
		state = "[red]closed"
	} else if state == "merged" {
		state = "[purple]merged"
	}
	labels := strings.Join(pr.Labels, ", ")
	if labels == "" {
		labels = "[gray]-"
	} else {
		labels = "[yellow]" + labels
	}
	age := "[gray]" + formatAge(pr.CreatedAt)
	changes := fmt.Sprintf("[green]+%d[white]/[red]-%d", pr.Additions, pr.Deletions)

	return []string{
		fmt.Sprintf("[white]#%d", pr.Number),
		"[white]" + pr.Title,
		state,
		"[aqua]" + pr.Author,
		"[green]" + pr.Branch,
		"[yellow]" + pr.Base,
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
	switch status {
	case "success":
		status = "[green]" + status
	case "failure":
		status = "[red]" + status
	case "cancelled":
		status = "[gray]" + status
	case "in_progress", "queued", "waiting":
		status = "[yellow]" + status
	default:
		status = "[white]" + status
	}

	return []string{
		"[gray]" + fmt.Sprintf("%d", wr.ID),
		"[white]" + wr.WorkflowName,
		status,
		"[green]" + wr.Branch,
		"[gray]" + wr.Event,
		"[aqua]" + wr.Actor,
		"[white]" + wr.Duration,
		"[gray]" + formatAge(wr.CreatedAt),
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
	state := w.State
	if state == "active" {
		state = "[green]" + state
	} else {
		state = "[red]" + state
	}
	return []string{
		"[gray]" + fmt.Sprintf("%d", w.ID),
		"[white]" + w.Name,
		"[gray]" + w.Path,
		state,
		"[gray]" + formatAge(w.UpdatedAt),
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
		"[yellow]" + ev.Name,
		"[white]" + ev.Value,
		"[gray]" + formatAge(ev.UpdatedAt),
	}
}

type RepoSecret struct {
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (rs *RepoSecret) GetTableRow() []string {
	return []string{
		"[yellow]" + rs.Name,
		"[gray]********",
		"[gray]" + formatAge(rs.UpdatedAt),
	}
}

type Branch struct {
	Name      string
	SHA       string
	Protected bool
	Default   bool
}

func (b *Branch) GetTableRow() []string {
	protected := "[gray]No"
	if b.Protected {
		protected = "[yellow]Yes"
	}
	name := "[white]" + b.Name
	if b.Default {
		name = "[green]" + b.Name + " *"
	}
	shortSHA := b.SHA
	if len(shortSHA) > 8 {
		shortSHA = shortSHA[:8]
	}
	return []string{
		name,
		"[gray]" + shortSHA,
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
	status := "[green]published"
	if r.Draft {
		status = "[yellow]draft"
	} else if r.Prerelease {
		status = "[yellow]prerelease"
	}
	return []string{
		"[green]" + r.TagName,
		"[white]" + r.Name,
		status,
		"[aqua]" + r.Author,
		"[white]" + fmt.Sprintf("%d", r.Assets),
		"[gray]" + formatAge(r.PublishedAt),
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

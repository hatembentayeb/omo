package jira

import (
	"net/url"
	"regexp"
	"strings"
	"time"
)

var (
	reColorTag = regexp.MustCompile(`\[[^\]]*\]`)
	reIssueKey = regexp.MustCompile(`^[A-Z][A-Z0-9]+-\d+$`)
)

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func stripColorTags(s string) string {
	return strings.TrimSpace(reColorTag.ReplaceAllString(s, ""))
}

func isPlaceholderKey(s string) bool {
	s = stripColorTags(s)
	switch strings.ToLower(s) {
	case "", "-", "—", "id", "key", "status", "property":
		return true
	}
	return false
}

func looksLikeIssueKey(s string) bool {
	return reIssueKey.MatchString(stripColorTags(s))
}

func normalizeSite(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return strings.TrimRight(raw, "/")
	}
	return strings.TrimRight(u.Scheme+"://"+u.Host, "/")
}

func dash(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "-"
	}
	return s
}

func formatWhen(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "-"
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Local().Format("2006-01-02 15:04")
	}
	if t, err := time.Parse("2006-01-02T15:04:05.000-0700", s); err == nil {
		return t.Local().Format("2006-01-02 15:04")
	}
	if len(s) >= 16 {
		return strings.ReplaceAll(s[:16], "T", " ")
	}
	return s
}

func colorStatus(name, category string) string {
	name = dash(name)
	switch strings.ToLower(category) {
	case "done":
		return "[green]" + name
	case "indeterminate":
		return "[yellow]" + name
	default:
		return "[white]" + name
	}
}

func colorPriority(name string) string {
	name = dash(name)
	switch strings.ToLower(name) {
	case "highest", "high", "blocker", "critical":
		return "[red]" + name
	case "medium":
		return "[yellow]" + name
	case "low", "lowest", "trivial", "minor":
		return "[green]" + name
	default:
		return "[white]" + name
	}
}

func pickCloseTransition(ts []Transition) *Transition {
	for i := range ts {
		if strings.EqualFold(ts[i].To.StatusCategory.Key, "done") {
			return &ts[i]
		}
	}
	for i := range ts {
		name := strings.ToLower(ts[i].Name)
		if strings.Contains(name, "done") || strings.Contains(name, "close") || strings.Contains(name, "resolve") {
			return &ts[i]
		}
	}
	return nil
}

func pickReopenTransition(ts []Transition) *Transition {
	var todo, prog *Transition
	for i := range ts {
		switch strings.ToLower(ts[i].To.StatusCategory.Key) {
		case "new":
			if todo == nil {
				todo = &ts[i]
			}
		case "indeterminate":
			if prog == nil {
				prog = &ts[i]
			}
		}
	}
	if todo != nil {
		return todo
	}
	if prog != nil {
		return prog
	}
	for i := range ts {
		name := strings.ToLower(ts[i].Name)
		if strings.Contains(name, "reopen") || strings.Contains(name, "to do") || strings.Contains(name, "open") {
			return &ts[i]
		}
	}
	return nil
}

func pickCreateType(types []IssueType) IssueType {
	prefer := []string{"Task", "Story", "Bug"}
	for _, want := range prefer {
		for _, t := range types {
			if !t.Subtask && strings.EqualFold(t.Name, want) {
				return t
			}
		}
	}
	for _, t := range types {
		if !t.Subtask {
			return t
		}
	}
	if len(types) > 0 {
		return types[0]
	}
	return IssueType{Name: "Task"}
}

func builtinFilters() []SavedFilter {
	return []SavedFilter{
		{ID: "mine", Name: "Mine open", JQL: jqlMine},
		{ID: "watched", Name: "Watched", JQL: jqlWatched},
		{ID: "reported", Name: "Reported by me", JQL: jqlReported},
		{ID: "done_today", Name: "Done today", JQL: jqlDoneToday},
		{ID: "deployed", Name: "Deployed", JQL: jqlDeployed},
	}
}

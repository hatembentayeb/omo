package jira

import (
	"encoding/json"
	"testing"

	"omo/pkg/pluginrpc"
)

func TestNormalizeSite(t *testing.T) {
	cases := map[string]string{
		"https://acme.atlassian.net":           "https://acme.atlassian.net",
		"https://acme.atlassian.net/":          "https://acme.atlassian.net",
		"https://acme.atlassian.net/jira":      "https://acme.atlassian.net",
		"acme.atlassian.net":                   "https://acme.atlassian.net",
		"  https://acme.atlassian.net/wiki/  ": "https://acme.atlassian.net",
	}
	for in, want := range cases {
		if got := normalizeSite(in); got != want {
			t.Errorf("normalizeSite(%q)=%q want %q", in, got, want)
		}
	}
}

func TestLooksLikeIssueKey(t *testing.T) {
	if !looksLikeIssueKey("PROJ-12") || !looksLikeIssueKey("[white]ABC-1") {
		t.Fatal("expected issue keys to match")
	}
	if looksLikeIssueKey("boards") || looksLikeIssueKey("-") || looksLikeIssueKey("12") {
		t.Fatal("did not expect these to look like issue keys")
	}
}

func TestADFRoundTrip(t *testing.T) {
	raw, err := json.Marshal(textToADF("hello\nworld"))
	if err != nil {
		t.Fatal(err)
	}
	got := adfToText(raw)
	if got != "hello\nworld" {
		t.Fatalf("adfToText=%q", got)
	}
	plain := adfToText(json.RawMessage(`"just a string"`))
	if plain != "just a string" {
		t.Fatalf("plain string ADF=%q", plain)
	}
}

func TestPickTransitions(t *testing.T) {
	ts := []Transition{
		{ID: "1", Name: "Start", To: Status{Name: "In Progress", StatusCategory: StatusCategory{Key: "indeterminate"}}},
		{ID: "2", Name: "Done", To: Status{Name: "Done", StatusCategory: StatusCategory{Key: "done"}}},
		{ID: "3", Name: "To Do", To: Status{Name: "To Do", StatusCategory: StatusCategory{Key: "new"}}},
	}
	closeT := pickCloseTransition(ts)
	if closeT == nil || closeT.ID != "2" {
		t.Fatalf("close=%v", closeT)
	}
	reopenT := pickReopenTransition(ts)
	if reopenT == nil || reopenT.ID != "3" {
		t.Fatalf("reopen=%v", reopenT)
	}
}

func TestPickCreateType(t *testing.T) {
	types := []IssueType{
		{Name: "Sub-task", Subtask: true},
		{Name: "Story"},
		{Name: "Bug"},
	}
	got := pickCreateType(types)
	if got.Name != "Story" {
		t.Fatalf("got %s", got.Name)
	}
}

func TestBuiltinFilters(t *testing.T) {
	fs := builtinFilters()
	if len(fs) != 5 {
		t.Fatalf("len=%d", len(fs))
	}
	if fs[0].JQL != jqlMine {
		t.Fatal("mine JQL mismatch")
	}
}

func TestViewNavBindings(t *testing.T) {
	nav := viewNavBindings()
	if len(nav) != 10 {
		t.Fatalf("want 10 views, got %d", len(nav))
	}
	if nav[0].Action != "goto_mine" || nav[9].Action != "goto_site" {
		t.Fatalf("unexpected ends: %s %s", nav[0].Action, nav[9].Action)
	}
	seen := map[string]bool{}
	for _, b := range nav {
		if seen[b.Key] {
			t.Fatalf("duplicate view key %s", b.Key)
		}
		seen[b.Key] = true
	}
}

func TestHelpSectionsCoverViews(t *testing.T) {
	help := helpSections()
	if len(help) < 10 {
		t.Fatalf("help sections=%d", len(help))
	}
	last := help[len(help)-1]
	if last.Title != "Global" {
		t.Fatalf("last section %q", last.Title)
	}
}

func TestParseDevDetail(t *testing.T) {
	raw := []byte(`{"detail":[{"pullRequests":[{"name":"feat/x","url":"https://example","status":"OPEN","lastUpdate":"2026-08-16T12:00:00Z"}]}]}`)
	items := parseDevDetail("pullrequest", raw)
	if len(items) != 1 || items[0].Kind != "PR" || items[0].Name != "feat/x" {
		t.Fatalf("%+v", items)
	}
}

func TestGetViewNotConnected(t *testing.T) {
	s := NewService()
	view, err := s.GetView(pluginrpc.ViewRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if view.View != viewMine {
		t.Fatalf("view=%s", view.View)
	}
	if len(view.ViewBindings) != 10 {
		t.Fatalf("view bindings=%d", len(view.ViewBindings))
	}
	if len(view.HelpSections) == 0 {
		t.Fatal("missing help sections")
	}
	res, err := s.DoAction(pluginrpc.ActionRequest{Action: "goto_boards"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || res.Next == nil || res.Next.View != viewBoards {
		t.Fatalf("goto boards: %+v", res)
	}
}

package host

import (
	"strings"
	"testing"

	"omo/pkg/pluginrpc"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestDashboardKeyboardSelectionAndOpen(t *testing.T) {
	entries := []installedPlugin{
		{Name: "docker", BinPath: "/plugins/docker"},
		{Name: "jira", BinPath: "/plugins/jira"},
		{Name: "redis", BinPath: "/plugins/redis"},
		{Name: "github", BinPath: "/plugins/github"},
	}
	var opened string
	closed := false
	d := NewDashboard(nil, nil, entries, func(entry installedPlugin) {
		opened = entry.Name
	}, func() {
		closed = true
	})

	if got := d.handleKey(tcell.NewEventKey(tcell.KeyRight, 0, 0)); got != nil {
		t.Fatal("right key was not consumed")
	}
	if d.selected != 1 {
		t.Fatalf("selected = %d, want 1", d.selected)
	}
	d.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
	if opened != "jira" {
		t.Fatalf("opened = %q, want jira", opened)
	}
	d.handleKey(tcell.NewEventKey(tcell.KeyEscape, 0, 0))
	if !closed {
		t.Fatal("escape did not close dashboard")
	}
}

func TestDashboardRenderCardLimitsRows(t *testing.T) {
	d := NewDashboard(nil, nil, []installedPlugin{{Name: "docker"}}, nil, nil)
	view := pluginrpc.Widget("Docker", "connected", "", [][2]string{
		{"One", "1"},
		{"Two", "2"},
		{"Three", "3"},
		{"Four", "4"},
		{"Five", "5"},
	})
	d.renderCard(0, view)

	text := d.cards[0].GetText(false)
	if !strings.Contains(text, "connected") || !strings.Contains(text, "One:") {
		t.Fatalf("card missing status/metric: %q", text)
	}
	if strings.Contains(text, "Five:") {
		t.Fatalf("card rendered more than four rows: %q", text)
	}
}

func TestDashStatusDefault(t *testing.T) {
	if got := dashStatus(""); got != "connected" {
		t.Fatalf("dashStatus empty = %q", got)
	}
}

func TestSplashDismissIsIdempotent(t *testing.T) {
	app := tview.NewApplication()
	pages := tview.NewPages()
	h := New(app, pages, nil, "test")
	pages.AddPage("main", tview.NewBox(), true, true)
	pages.AddAndSwitchToPage("splash", Splash(), true)
	if !h.SplashVisible() {
		t.Fatal("splash should be visible after ShowStartupSplash page add")
	}
	h.DismissSplash()
	h.DismissSplash()
	if h.SplashVisible() {
		t.Fatal("splash still visible after dismiss")
	}
	front, _ := pages.GetFrontPage()
	if front != "main" {
		t.Fatalf("front page = %q, want main", front)
	}
}

func TestCoverEnterOpensDashboard(t *testing.T) {
	app := tview.NewApplication()
	opened := false
	cover := Cover(app, "test", func() { opened = true })
	cover.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(tview.Primitive) {})
	if !opened {
		t.Fatal("cover Enter did not open dashboard")
	}
}

func TestPluginHomeEscapeReturnsToDashboard(t *testing.T) {
	app := tview.NewApplication()
	renderer := NewRPCRenderer(app, tview.NewPages(), "test", nil)
	returned := false
	renderer.SetHomeHook(func() { returned = true })
	renderer.Apply(pluginrpc.ViewData{
		View:    renderer.homeView,
		Title:   "Test",
		Headers: []string{"Metric", "Value"},
	})

	renderer.root.InputHandler()(tcell.NewEventKey(tcell.KeyEscape, 0, 0), func(tview.Primitive) {})
	if !returned {
		t.Fatal("plugin home Escape did not return to dashboard")
	}
}

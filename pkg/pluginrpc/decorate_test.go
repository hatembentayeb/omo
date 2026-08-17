package pluginrpc

import "testing"

func TestWidgetBuildsCompactDashboardView(t *testing.T) {
	view := Widget("Docker", "connected", "local", [][2]string{
		{"Running", "3"},
		{"Stopped", "1"},
	})

	if view.View != DashboardView {
		t.Fatalf("view = %q, want %q", view.View, DashboardView)
	}
	if view.Title != "Docker" || view.Status != "connected" || view.Info != "local" {
		t.Fatalf("unexpected widget metadata: %+v", view)
	}
	if len(view.Rows) != 2 || view.Rows[0][0] != "Running" || view.Rows[0][1] != "3" {
		t.Fatalf("unexpected widget rows: %#v", view.Rows)
	}
	if len(view.Actions) != 0 || len(view.ViewBindings) != 0 {
		t.Fatal("dashboard widgets must be action-free")
	}
}

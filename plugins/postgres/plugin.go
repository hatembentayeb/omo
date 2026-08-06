package postgres

import (
	"time"

	"omo/pkg/pluginapi"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Plugin is the PostgreSQL management plugin (host-embedded UI).
type Plugin struct {
	Name         string
	Description  string
	postgresView *PostgresView
}

// New returns a PostgreSQL plugin ready for host embedding.
func New() *Plugin {
	return &Plugin{
		Name:        "PostgreSQL Manager",
		Description: "Manage PostgreSQL users, roles, databases, and server configuration",
	}
}

// Start initializes and starts the PostgreSQL plugin UI inside the host app.
func (p *Plugin) Start(app *tview.Application) tview.Primitive {
	pluginapi.Log().Info("starting plugin")
	pages := tview.NewPages()
	postgresView := NewPostgresView(app, pages)
	p.postgresView = postgresView

	mainUI := postgresView.GetMainUI()

	pages.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlT {
			if p.postgresView != nil {
				p.postgresView.ShowConnectionSelector()
			}
			return nil
		}
		return event
	})

	pages.AddPage("postgres", mainUI, true, true)
	app.SetFocus(p.postgresView.usersView.GetTable())
	p.postgresView.AutoConnectToDefaultInstance()

	return pages
}

// Stop cleans up resources used by the PostgreSQL plugin.
func (p *Plugin) Stop() {
	if p.postgresView != nil {
		p.postgresView.Stop()
	}
}

// GetMetadata returns plugin metadata.
func (p *Plugin) GetMetadata() pluginapi.PluginMetadata {
	return pluginapi.PluginMetadata{
		Name:        "postgres",
		Version:     "1.0.0",
		Description: "PostgreSQL user & configuration management plugin",
		Author:      "OhMyOps Team",
		License:     "MIT",
		Tags:        []string{"database", "sql", "postgresql", "users", "management"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "https://github.com/hatembentayeb/omo/plugins/postgres",
	}
}

// OhmyopsPlugin is exported for native .so loaders.
var OhmyopsPlugin Plugin

func init() {
	OhmyopsPlugin.Name = "PostgreSQL Manager"
	OhmyopsPlugin.Description = "Manage PostgreSQL users, roles, databases, and server configuration"
}

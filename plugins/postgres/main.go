package main

import (
	"time"

	"omo/pkg/pluginapi"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// PostgresPlugin represents the PostgreSQL management plugin
type PostgresPlugin struct {
	Name         string
	Description  string
	postgresView *PostgresView
}

// Start initializes and starts the PostgreSQL plugin UI
func (p *PostgresPlugin) Start(app *tview.Application) tview.Primitive {
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

// Stop cleans up resources used by the PostgreSQL plugin
func (p *PostgresPlugin) Stop() {
	if p.postgresView != nil {
		p.postgresView.Stop()
	}
}

// GetMetadata returns plugin metadata
func (p *PostgresPlugin) GetMetadata() pluginapi.PluginMetadata {
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

// OhmyopsPlugin is exported as a variable to be loaded by the main application
var OhmyopsPlugin PostgresPlugin

func init() {
	OhmyopsPlugin.Name = "PostgreSQL Manager"
	OhmyopsPlugin.Description = "Manage PostgreSQL users, roles, databases, and server configuration"
}

// GetMetadata is exported for legacy loaders.
func GetMetadata() pluginapi.PluginMetadata {
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

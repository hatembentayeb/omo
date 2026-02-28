package main

import (
	"time"

	"omo/pkg/pluginapi"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type GitHubPlugin struct {
	Name        string
	Description string
	githubView  *GitHubView
}

func (g *GitHubPlugin) Start(app *tview.Application) tview.Primitive {
	pluginapi.Log().Info("starting plugin")
	pages := tview.NewPages()
	githubView := NewGitHubView(app, pages)
	g.githubView = githubView

	mainUI := githubView.GetMainUI()

	pages.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlG {
			if g.githubView != nil {
				g.githubView.ShowAccountSelector()
			}
			return nil
		}
		return event
	})

	pages.AddPage("github", mainUI, true, true)

	app.SetFocus(g.githubView.reposView.GetTable())

	g.githubView.reposView.Log("[blue]GitHub plugin initialized")
	g.githubView.reposView.Log("[yellow]Discovering accounts...")

	go func() {
		time.Sleep(100 * time.Millisecond)
		g.githubView.AutoDiscoverAccounts()
	}()

	return pages
}

func (g *GitHubPlugin) Stop() {
	if g.githubView != nil {
		g.githubView.Stop()
	}
}

func (g *GitHubPlugin) GetMetadata() pluginapi.PluginMetadata {
	return pluginapi.PluginMetadata{
		Name:        "github",
		Version:     "1.0.0",
		Description: "Manage GitHub PRs, Actions pipelines, environment variables, secrets, and releases",
		Author:      "OhMyOps Team",
		License:     "MIT",
		Tags:        []string{"github", "ci-cd", "devops", "pull-requests", "actions"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "https://github.com/hatembentayeb/omo/plugins/github",
	}
}

var OhmyopsPlugin GitHubPlugin

func init() {
	OhmyopsPlugin.Name = "GitHub Manager"
	OhmyopsPlugin.Description = "Manage GitHub PRs, Actions pipelines, environment variables, secrets, and releases"
}

func GetMetadata() pluginapi.PluginMetadata {
	return pluginapi.PluginMetadata{
		Name:        "github",
		Version:     "1.0.0",
		Description: "Manage GitHub PRs, Actions pipelines, environment variables, secrets, and releases",
		Author:      "OhMyOps Team",
		License:     "MIT",
		Tags:        []string{"github", "ci-cd", "devops", "pull-requests", "actions"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "https://github.com/hatembentayeb/omo/plugins/github",
	}
}

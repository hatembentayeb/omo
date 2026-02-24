package main

import (
	"fmt"
	"time"

	"omo/pkg/pluginapi"
	"omo/pkg/ui"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// GitPlugin represents the Git management plugin
type GitPlugin struct {
	Name        string
	Description string
	gitView     *GitView
}

// Start initializes and starts the Git plugin UI
func (g *GitPlugin) Start(app *tview.Application) tview.Primitive {
	pluginapi.Log().Info("starting plugin")
	pages := tview.NewPages()
	gitView := NewGitView(app, pages)
	g.gitView = gitView

	// Get the main UI component
	mainUI := gitView.GetMainUI()

	// Add keyboard handling to the pages
	pages.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Check for Ctrl+G to open repo selector
		if event.Key() == tcell.KeyCtrlG {
			if g.gitView != nil && len(g.gitView.repositories) > 0 {
				g.gitView.showRepoSelector()
			}
			return nil
		}
		// Pass all other keys through
		return event
	})

	pages.AddPage("git", mainUI, true, true)

	// Set initial focus to the table explicitly
	app.SetFocus(g.gitView.reposView.GetTable())

	// Show a detailed welcome message and instructions
	g.gitView.reposView.Log("[blue]Git plugin initialized")
	g.gitView.reposView.Log("[yellow]Searching for repositories...")

	// Run initial discovery in a goroutine
	go func() {
		time.Sleep(100 * time.Millisecond)
		g.gitView.AutoDiscoverRepositories()

		g.gitView.reposView.Log("[green]Git plugin ready")
		g.gitView.reposView.Log("[aqua]Navigation Keys:")
		g.gitView.reposView.Log("   [yellow]G[white] - Repositories")
		g.gitView.reposView.Log("   [yellow]S[white] - Status")
		g.gitView.reposView.Log("   [yellow]L[white] - Commits")
		g.gitView.reposView.Log("   [yellow]B[white] - Branches")
		g.gitView.reposView.Log("   [yellow]M[white] - Remotes")
		g.gitView.reposView.Log("   [yellow]T[white] - Tags")
		g.gitView.reposView.Log("   [yellow]H[white] - Stash")
		g.gitView.reposView.Log("   [yellow]?[white] - Help")
	}()

	return pages
}

// Stop cleans up resources used by the Git plugin
func (g *GitPlugin) Stop() {
	if g.gitView != nil {
		g.gitView.Stop()
	}
}

// GetMetadata returns plugin metadata
func (g *GitPlugin) GetMetadata() pluginapi.PluginMetadata {
	return pluginapi.PluginMetadata{
		Name:        "git",
		Version:     "2.0.0",
		Description: "Git repository management with status, commits, branches, remotes, stash, and tags",
		Author:      "OhMyOps Team",
		License:     "MIT",
		Tags:        []string{"version-control", "git", "development", "vcs"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "https://github.com/hatembentayeb/omo/plugins/git",
	}
}

// OhmyopsPlugin is exported as a variable to be loaded by the main application
var OhmyopsPlugin GitPlugin

func init() {
	OhmyopsPlugin.Name = "Git Manager"
	OhmyopsPlugin.Description = "Manage Git repositories with full status, commits, branches, remotes, stash, and tags support"
}

// GetMetadata is exported for legacy loaders.
func GetMetadata() pluginapi.PluginMetadata {
	return pluginapi.PluginMetadata{
		Name:        "git",
		Version:     "2.0.0",
		Description: "Git repository management with status, commits, branches, remotes, stash, and tags",
		Author:      "OhMyOps Team",
		License:     "MIT",
		Tags:        []string{"version-control", "git", "development", "vcs"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "https://github.com/hatembentayeb/omo/plugins/git",
	}
}

// showRepoSelector shows a modal for selecting a repository
func (gv *GitView) showRepoSelector() {
	if len(gv.repositories) == 0 {
		gv.reposView.Log("[yellow]No repositories found. Press D to add a directory.")
		return
	}

	items := make([][]string, len(gv.repositories))
	for i, repo := range gv.repositories {
		items[i] = []string{repo.Name, repo.Path}
	}

	ui.ShowStandardListSelectorModal(
		gv.pages,
		gv.app,
		"Select Repository",
		items,
		func(index int, name string, cancelled bool) {
			if !cancelled && index >= 0 && index < len(gv.repositories) {
				gv.currentRepoPath = gv.repositories[index].Path
				gv.reposView.Log(fmt.Sprintf("[blue]Selected: %s", gv.repositories[index].Name))
				gv.refresh()
			}
			gv.app.SetFocus(gv.reposView.GetTable())
		},
	)
}

package main

import (
	"time"

	"omo/pkg/pluginapi"

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
	// Initialize the Git view
	pages := tview.NewPages()
	gitView := NewGitView(app, pages)
	g.gitView = gitView

	// Get the main UI component
	mainUI := gitView.GetMainUI()

	// Add keyboard handling to the pages
	pages.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Check for specific Git shortcuts
		if event.Key() == tcell.KeyCtrlG {
			// Open repo selector
			if g.gitView != nil {
				go func() {
					// Run in goroutine with delay to prevent UI freeze
					time.Sleep(50 * time.Millisecond)

					g.gitView.ShowRepoSelector()

				}()
			}
			return nil // Consume the event
		}
		// Pass all other keys through
		return event
	})

	// Add page with a consistent name, removing any existing page first
	const pageName = "git-main"
	if pages.HasPage(pageName) {
		pages.RemovePage(pageName)
	}
	pages.AddPage(pageName, mainUI, true, true)

	// Set initial focus to the table explicitly
	app.SetFocus(g.gitView.cores.GetTable())

	// Show a welcome message and instructions
	g.gitView.cores.Log("[blue]Git plugin initialized")
	g.gitView.cores.Log("[yellow]Press 'D' to search for repositories in a directory")

	return pages
}

// Stop cleans up resources used by the Git plugin
func (g *GitPlugin) Stop() {
	if g.gitView != nil {
		// Log that we're cleaning up
		if g.gitView.cores != nil {
			g.gitView.cores.Log("[blue]Cleaning up Git plugin resources...")
		}

		// Clean up timer resources
		if g.gitView.refreshTimer != nil {
			g.gitView.refreshTimer.Stop()
			g.gitView.refreshTimer = nil
		}

		// Reset repositories data to prevent stale data
		g.gitView.repositories = []GitRepository{}

		// Reset current repo path
		g.gitView.currentRepoPath = ""

		// Clean up UI resources
		if g.gitView.cores != nil {
			// Unregister handlers to prevent input capture conflicts
			g.gitView.cores.UnregisterHandlers()

			// Stop auto-refresh if enabled
			g.gitView.cores.StopAutoRefresh()
		}

		// Clean up any modal pages that might still be open
		if g.gitView.pages != nil {
			// Remove known modal pages
			knownPages := []string{
				"git-main",
				"git-repo-selector",
				"git-directory-selector",
				"git-help-modal",
				"git-status-modal",
				"git-log-modal",
				"git-branches-modal",
			}

			for _, pageName := range knownPages {
				if g.gitView.pages.HasPage(pageName) {
					g.gitView.pages.RemovePage(pageName)
				}
			}
		}

		// Log final cleanup complete
		if g.gitView.cores != nil {
			g.gitView.cores.Log("[green]Git plugin resources cleaned up")
		}
	}
}

// GetMetadata returns plugin metadata
func (g *GitPlugin) GetMetadata() pluginapi.PluginMetadata {
	return pluginapi.PluginMetadata{
		Name:        "git",
		Version:     "1.0.0",
		Description: "Git repository management plugin",
		Author:      "Git Plugin Team",
		License:     "MIT",
		Tags:        []string{"version-control", "git", "development"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "https://github.com/hatembentayeb/omo/plugins/git",
	}
}

// OhmyopsPlugin is exported as a variable to be loaded by the main application
var OhmyopsPlugin GitPlugin

func init() {
	OhmyopsPlugin.Name = "Git Manager"
	OhmyopsPlugin.Description = "Manage Git repositories and monitor status"
}

// GetMetadata is exported for legacy loaders.
func GetMetadata() pluginapi.PluginMetadata {
	return pluginapi.PluginMetadata{
		Name:        "git",
		Version:     "1.0.0",
		Description: "Git repository management plugin",
		Author:      "Git Plugin Team",
		License:     "MIT",
		Tags:        []string{"version-control", "git", "development"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "https://github.com/hatembentayeb/omo/plugins/git",
	}
}

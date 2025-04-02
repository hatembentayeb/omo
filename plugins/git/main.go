package main

import (
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// PluginMetadata defines metadata for OhmyopsPlugin
type PluginMetadata struct {
	Name        string    // Name of the plugin
	Version     string    // Version of the plugin
	Description string    // Short description of the plugin
	Author      string    // Author of the plugin
	License     string    // License of the plugin
	Tags        []string  // Tags for categorizing the plugin
	Arch        []string  // Supported architectures
	LastUpdated time.Time // Last update time
	URL         string    // URL to the plugin repository or documentation
}

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
					app.QueueUpdateDraw(func() {
						g.gitView.ShowRepoSelector()
					})
				}()
			}
			return nil // Consume the event
		}
		// Pass all other keys through
		return event
	})

	pages.AddPage("main", mainUI, true, true)

	// Set initial focus to the table explicitly
	app.SetFocus(g.gitView.cores.GetTable())

	// Show a welcome message and instructions
	g.gitView.cores.Log("[blue]Git plugin initialized")
	g.gitView.cores.Log("[yellow]Press 'D' to search for repositories in a directory")

	// Use a timer to delay any heavy operations until after UI is shown
	time.AfterFunc(100*time.Millisecond, func() {
		app.QueueUpdateDraw(func() {
			// This ensures the UI is responsive first
		})
	})

	return pages
}

// Stop cleans up resources used by the Git plugin
func (g *GitPlugin) Stop() {
	if g.gitView != nil {
		// Clean up any resources
		if g.gitView.refreshTimer != nil {
			g.gitView.refreshTimer.Stop()
		}
	}
}

// GetMetadata returns plugin metadata
func (g *GitPlugin) GetMetadata() PluginMetadata {
	return PluginMetadata{
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

// GetMetadata is exported as a function to be called directly by the main application
// when the direct type assertion of OhmyopsPlugin fails
func GetMetadata() interface{} {
	return map[string]interface{}{
		"Name":        "git",
		"Version":     "1.0.0",
		"Description": "Git repository management plugin",
		"Author":      "Git Plugin Team",
		"License":     "MIT",
		"Tags":        []string{"version-control", "git", "development"},
		"Arch":        []string{"amd64", "arm64"},
		"LastUpdated": time.Now().Format(time.RFC3339),
		"URL":         "https://github.com/hatembentayeb/omo/plugins/git",
	}
}

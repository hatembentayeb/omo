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

// RedisPlugin represents the Redis management plugin
type RedisPlugin struct {
	Name        string
	Description string
	redisView   *RedisView
}

// Start initializes and starts the Redis plugin UI
func (r *RedisPlugin) Start(app *tview.Application) tview.Primitive {
	// Initialize the Redis view
	pages := tview.NewPages()
	redisView := NewRedisView(app, pages)
	r.redisView = redisView

	// Get the main UI component
	mainUI := redisView.GetMainUI()

	// Add keyboard handling to the pages
	pages.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Check for Ctrl+T to open instance selector (like other plugins)
		if event.Key() == tcell.KeyCtrlT {
			// Direct call without QueueUpdateDraw to prevent freezing
			if r.redisView != nil {
				r.redisView.ShowConnectionSelector()
			}
			return nil // Consume the event
		}
		// Pass all other keys through
		return event
	})

	pages.AddPage("redis", mainUI, true, true)

	// Set initial focus to the table explicitly
	app.SetFocus(r.redisView.cores.GetTable())

	// Auto-connect to the first Redis instance in config
	r.redisView.AutoConnectToDefaultInstance()

	return pages
}

// Stop cleans up resources used by the Redis plugin
func (r *RedisPlugin) Stop() {
	if r.redisView != nil {
		// If redisView has a redis client, disconnect
		if r.redisView.redisClient != nil {
			r.redisView.redisClient.Disconnect()
		}

		// Stop the auto-refresh timer
		if r.redisView.refreshTimer != nil {
			r.redisView.refreshTimer.Stop()
		}
	}
}

// GetMetadata returns plugin metadata
func (r *RedisPlugin) GetMetadata() PluginMetadata {
	return PluginMetadata{
		Name:        "redis",
		Version:     "1.0.0",
		Description: "Redis management plugin",
		Author:      "Redis Plugin Team",
		License:     "MIT",
		Tags:        []string{"database", "cache", "nosql"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "https://github.com/hatembentayeb/omo/plugins/redis",
	}
}

// OhmyopsPlugin is exported as a variable to be loaded by the main application
var OhmyopsPlugin RedisPlugin

func init() {
	OhmyopsPlugin.Name = "Redis Manager"
	OhmyopsPlugin.Description = "Manage Redis instances and monitor performance"
}

// GetMetadata is exported as a function to be called directly by the main application
// when the direct type assertion of OhmyopsPlugin fails
func GetMetadata() interface{} {
	return map[string]interface{}{
		"Name":        "redis",
		"Version":     "1.0.0",
		"Description": "Redis management plugin",
		"Author":      "Redis Plugin Team",
		"License":     "MIT",
		"Tags":        []string{"database", "cache", "nosql"},
		"Arch":        []string{"amd64", "arm64"},
		"LastUpdated": time.Now(),
		"URL":         "https://github.com/hatembentayeb/omo/plugins/redis",
	}
}

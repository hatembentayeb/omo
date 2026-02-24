package main

import (
	"time"

	"omo/pkg/pluginapi"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// RedisPlugin represents the Redis management plugin
type RedisPlugin struct {
	Name        string
	Description string
	redisView   *RedisView
}

// Start initializes and starts the Redis plugin UI
func (r *RedisPlugin) Start(app *tview.Application) tview.Primitive {
	pluginapi.Log().Info("starting plugin")
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

	// Auto-connect in a goroutine to avoid blocking the UI thread.
	// Network connect + key scan can take seconds on timeout.
	go func() {
		r.redisView.AutoConnectToDefaultInstance()
	}()

	return pages
}

// Stop cleans up resources used by the Redis plugin
func (r *RedisPlugin) Stop() {
	if r.redisView != nil {
		r.redisView.Stop()
	}
}

// GetMetadata returns plugin metadata
func (r *RedisPlugin) GetMetadata() pluginapi.PluginMetadata {
	return pluginapi.PluginMetadata{
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

// GetMetadata is exported for legacy loaders.
func GetMetadata() pluginapi.PluginMetadata {
	return pluginapi.PluginMetadata{
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

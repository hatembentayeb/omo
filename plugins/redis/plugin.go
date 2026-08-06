package redis

import (
	"time"

	"omo/pkg/pluginapi"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Plugin is the Redis management plugin (host-embedded UI).
type Plugin struct {
	Name        string
	Description string
	redisView   *RedisView
}

// New returns a Redis plugin ready for host embedding.
func New() *Plugin {
	return &Plugin{
		Name:        "Redis Manager",
		Description: "Manage Redis instances and monitor performance",
	}
}

// Start initializes and starts the Redis plugin UI inside the host app.
func (r *Plugin) Start(app *tview.Application) tview.Primitive {
	pluginapi.Log().Info("starting plugin")
	pages := tview.NewPages()
	redisView := NewRedisView(app, pages)
	r.redisView = redisView

	mainUI := redisView.GetMainUI()

	pages.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlT {
			if r.redisView != nil {
				r.redisView.ShowConnectionSelector()
			}
			return nil
		}
		return event
	})

	pages.AddPage("redis", mainUI, true, true)
	app.SetFocus(r.redisView.cores.GetTable())

	go func() {
		r.redisView.AutoConnectToDefaultInstance()
	}()

	return pages
}

// Stop cleans up resources used by the Redis plugin.
func (r *Plugin) Stop() {
	if r.redisView != nil {
		r.redisView.Stop()
	}
}

// GetMetadata returns plugin metadata.
func (r *Plugin) GetMetadata() pluginapi.PluginMetadata {
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

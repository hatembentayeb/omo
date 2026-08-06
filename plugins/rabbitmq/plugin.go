package rabbitmq

import (
	"time"

	"omo/pkg/pluginapi"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Plugin is the RabbitMQ management plugin (host-embedded UI).
type Plugin struct {
	Name        string
	Description string
	rmqView     *RabbitMQView
}

// New returns a RabbitMQ plugin ready for host embedding.
func New() *Plugin {
	return &Plugin{
		Name:        "RabbitMQ Manager",
		Description: "Manage RabbitMQ queues, exchanges, bindings, and connections",
	}
}

// Start initializes the plugin and returns the main UI component.
func (p *Plugin) Start(app *tview.Application) tview.Primitive {
	pluginapi.Log().Info("starting plugin")
	pages := tview.NewPages()

	p.rmqView = NewRabbitMQView(app, pages)
	mainUI := p.rmqView.GetMainUI()

	pages.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlT {
			if p.rmqView != nil {
				p.rmqView.ShowInstanceSelector()
			}
			return nil
		}
		return event
	})

	pages.AddPage("rabbitmq", mainUI, true, true)
	app.SetFocus(p.rmqView.cores.GetTable())

	return pages
}

// GetMetadata returns plugin metadata.
func (p *Plugin) GetMetadata() pluginapi.PluginMetadata {
	return pluginapi.PluginMetadata{
		Name:        "rabbitmq",
		Version:     "1.0.0",
		Description: "Manage RabbitMQ queues, exchanges, bindings, and connections",
		Author:      "HATMAN",
		License:     "MIT",
		Tags:        []string{"messaging", "broker", "amqp"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "https://github.com/ohmyops/omo-rabbitmq",
	}
}

// Stop cleans up resources used by the plugin.
func (p *Plugin) Stop() {
	if p.rmqView != nil {
		p.rmqView.Stop()
	}
}

// OhmyopsPlugin is exported for native .so loaders.
var OhmyopsPlugin Plugin

func init() {
	OhmyopsPlugin.Name = "RabbitMQ Manager"
	OhmyopsPlugin.Description = "Manage RabbitMQ queues, exchanges, bindings, and connections"
}

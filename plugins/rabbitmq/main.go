package main

import (
	"time"

	"omo/pkg/pluginapi"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// RabbitMQPlugin represents the RabbitMQ management plugin
type RabbitMQPlugin struct {
	Name        string
	Description string
	rmqView     *RabbitMQView
}

// Start initializes the plugin and returns the main UI component
func (p *RabbitMQPlugin) Start(app *tview.Application) tview.Primitive {
	pages := tview.NewPages()

	p.rmqView = NewRabbitMQView(app, pages)

	mainUI := p.rmqView.GetMainUI()

	// Add keyboard handling for Ctrl+T to open instance selector
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

	// Set initial focus
	app.SetFocus(p.rmqView.cores.GetTable())

	return pages
}

// GetMetadata returns plugin metadata.
func (p *RabbitMQPlugin) GetMetadata() pluginapi.PluginMetadata {
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

// GetMetadata is exported for legacy loaders.
func GetMetadata() pluginapi.PluginMetadata {
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

// Stop cleans up resources used by the plugin
func (p *RabbitMQPlugin) Stop() {
	if p.rmqView != nil {
		p.rmqView.Stop()
	}
}

// OhmyopsPlugin is exported as a variable to be loaded by the main application
var OhmyopsPlugin RabbitMQPlugin

func init() {
	OhmyopsPlugin.Name = "RabbitMQ Manager"
	OhmyopsPlugin.Description = "Manage RabbitMQ queues, exchanges, bindings, and connections"
}

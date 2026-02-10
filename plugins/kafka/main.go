package main

import (
	"time"

	"omo/pkg/pluginapi"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// KafkaPlugin represents the Kafka management plugin
type KafkaPlugin struct {
	Name        string
	Description string
	kafkaView   *KafkaView
}

// Start initializes the plugin and returns the main UI component
func (p *KafkaPlugin) Start(app *tview.Application) tview.Primitive {
	// Create pages component for modal dialogs
	pages := tview.NewPages()

	// Initialize the Kafka view
	p.kafkaView = NewKafkaView(app, pages)

	// Get the main UI component
	mainUI := p.kafkaView.GetMainUI()

	// Add keyboard handling for Ctrl+T to open cluster selector
	pages.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlT {
			if p.kafkaView != nil {
				p.kafkaView.ShowClusterSelector()
			}
			return nil
		}
		return event
	})

	pages.AddPage("kafka", mainUI, true, true)

	// Set initial focus
	app.SetFocus(p.kafkaView.cores.GetTable())

	return pages
}

// GetMetadata returns plugin metadata.
func (p *KafkaPlugin) GetMetadata() pluginapi.PluginMetadata {
	return pluginapi.PluginMetadata{
		Name:        "kafka",
		Version:     "2.0.0",
		Description: "Manage Kafka brokers, topics, and consumers",
		Author:      "HATMAN",
		License:     "MIT",
		Tags:        []string{"messaging", "streaming", "broker"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "https://github.com/hatembentayeb/ohmyops-v2/plugins/kafka",
	}
}

// GetMetadata is exported for legacy loaders.
func GetMetadata() pluginapi.PluginMetadata {
	return pluginapi.PluginMetadata{
		Name:        "kafka",
		Version:     "2.0.0",
		Description: "Manage Kafka brokers, topics, and consumers",
		Author:      "HATMAN",
		License:     "MIT",
		Tags:        []string{"messaging", "streaming", "broker"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "https://github.com/hatembentayeb/ohmyops-v2/plugins/kafka",
	}
}

// Stop cleans up resources used by the plugin
func (p *KafkaPlugin) Stop() {
	if p.kafkaView != nil {
		p.kafkaView.Stop()
	}
}

// OhmyopsPlugin is exported as a variable to be loaded by the main application
var OhmyopsPlugin KafkaPlugin

func init() {
	OhmyopsPlugin.Name = "Kafka Manager"
	OhmyopsPlugin.Description = "Manage Kafka brokers, topics, and consumers"
}

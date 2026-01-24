package main

import (
	"time"

	"omo/pkg/pluginapi"

	"github.com/rivo/tview"
)

type KafkaPlugin struct {
	Name        string
	Description string
	brokerView  *BrokerView
}

// Start initializes the plugin and returns the main UI component
func (p *KafkaPlugin) Start(app *tview.Application) tview.Primitive {
	// Create pages component for modal dialogs
	pages := tview.NewPages()

	// Initialize the broker view
	p.brokerView = NewBrokerView(app, pages)

	// Set initial breadcrumb view
	p.brokerView.cores.ClearViews()
	p.brokerView.cores.PushView("Kafka")
	p.brokerView.cores.PushView("brokers")

	// Add the broker view to the pages component as the main page
	pages.AddPage("kafka", p.brokerView.GetMainUI(), true, true)

	// Set initial focus on the broker view table
	app.SetFocus(p.brokerView.cores.GetTable())

	return pages
}

// GetMetadata returns plugin metadata.
func (p *KafkaPlugin) GetMetadata() pluginapi.PluginMetadata {
	return pluginapi.PluginMetadata{
		Name:        "kafka",
		Version:     "1.5.0",
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
		Version:     "1.5.0",
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
	// Clean up resources
}

// OhmyopsPlugin is exported as a variable to be loaded by the main application
var OhmyopsPlugin KafkaPlugin

func init() {
	OhmyopsPlugin.Name = "Kafka Manager"
	OhmyopsPlugin.Description = "Manage Kafka brokers, topics, and consumers"
}

package main

import (
	"time"

	"github.com/rivo/tview"
)

// OhmyopsPlugin is exported as a variable to be loaded by the main application
// This must implement the interface expected by the main application:
//
//	type OhmyopsPlugin interface {
//	  Start(*tview.Application) tview.Primitive
//    GetMetadata() PluginMetadata
//	}

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

	// Add the broker view to the pages component as the main page
	pages.AddPage("main", p.brokerView.GetMainUI(), true, true)

	// Set initial focus on the broker view table
	app.SetFocus(p.brokerView.cores.GetTable())

	return pages
}

// GetMetadata is exported as a function to be called directly by the main application
// when the direct type assertion of OhmyopsPlugin fails
func GetMetadata() interface{} {
	return map[string]interface{}{
		"Name":        "kafka",
		"Version":     "1.5.0",
		"Description": "Manage Kafka brokers, topics, and consumers",
		"Author":      "HATMAN",
		"License":     "MIT",
		"Tags":        []string{"messaging", "streaming", "broker"},
		"Arch":        []string{"amd64", "arm64"},
		"LastUpdated": time.Now(),
		"URL":         "https://github.com/hatembentayeb/ohmyops-v2/plugins/kafka",
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

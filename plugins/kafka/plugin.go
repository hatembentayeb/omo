package kafka

import (
	"time"

	"omo/pkg/pluginapi"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Plugin is the Kafka management plugin (host-embedded UI).
type Plugin struct {
	Name        string
	Description string
	kafkaView   *KafkaView
}

// New returns a Kafka plugin ready for host embedding.
func New() *Plugin {
	return &Plugin{
		Name:        "Kafka Manager",
		Description: "Manage Kafka brokers, topics, and consumers",
	}
}

// Start initializes the plugin and returns the main UI component.
func (p *Plugin) Start(app *tview.Application) tview.Primitive {
	pluginapi.Log().Info("starting plugin")
	pages := tview.NewPages()

	p.kafkaView = NewKafkaView(app, pages)
	mainUI := p.kafkaView.GetMainUI()

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
	app.SetFocus(p.kafkaView.cores.GetTable())

	return pages
}

// GetMetadata returns plugin metadata.
func (p *Plugin) GetMetadata() pluginapi.PluginMetadata {
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

// Stop cleans up resources used by the plugin.
func (p *Plugin) Stop() {
	if p.kafkaView != nil {
		p.kafkaView.Stop()
	}
}

// OhmyopsPlugin is exported for native .so loaders.
var OhmyopsPlugin Plugin

func init() {
	OhmyopsPlugin.Name = "Kafka Manager"
	OhmyopsPlugin.Description = "Manage Kafka brokers, topics, and consumers"
}

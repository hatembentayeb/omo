package main

import (
	"time"

	"omo/pkg/pluginapi"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type S3Plugin struct {
	Name        string
	Description string
	bucketsView *BucketsView
}

// Start initializes and starts the S3 plugin UI
func (s *S3Plugin) Start(app *tview.Application) tview.Primitive {
	pluginapi.Log().Info("starting plugin")
	pages := tview.NewPages()
	bucketsView := NewBucketsView(app, pages)
	s.bucketsView = bucketsView

	// Get the main UI component
	mainUI := bucketsView.GetMainUI()

	// Add keyboard handling to the pages instead
	pages.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Check for Ctrl+T
		if event.Key() == tcell.KeyCtrlT {
			// Show profile selector
			if s.bucketsView != nil {
				s.bucketsView.ShowProfileSelector()
			}
			return nil // Consume the event
		}
		return event // Pass other events through
	})

	pages.AddPage("s3", mainUI, true, true)

	return pages
}

// Stop cleans up resources used by the S3 plugin
func (s *S3Plugin) Stop() {
	// Clean up resources
}

// GetMetadata returns plugin metadata.
func (s *S3Plugin) GetMetadata() pluginapi.PluginMetadata {
	return pluginapi.PluginMetadata{
		Name:        "s3",
		Version:     "1.2.0",
		Description: "Manage AWS S3 buckets",
		Author:      "HATMAN",
		License:     "Apache-2.0",
		Tags:        []string{"storage", "cloud", "aws"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "https://github.com/hatembentayeb/ohmyops-v2/plugins/s3",
	}
}

// OhmyopsPlugin is exported as a variable to be loaded by the main application
var OhmyopsPlugin S3Plugin

func init() {
	OhmyopsPlugin.Name = "S3 Manager"
	OhmyopsPlugin.Description = "Manage AWS S3 buckets"
}

// GetMetadata is exported for legacy loaders.
func GetMetadata() pluginapi.PluginMetadata {
	return pluginapi.PluginMetadata{
		Name:        "s3",
		Version:     "1.2.0",
		Description: "Manage AWS S3 buckets",
		Author:      "HATMAN",
		License:     "Apache-2.0",
		Tags:        []string{"storage", "cloud", "aws"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "https://github.com/hatembentayeb/ohmyops-v2/plugins/s3",
	}
}

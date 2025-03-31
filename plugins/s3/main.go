package main

import (
	"time"

	"github.com/gdamore/tcell/v2"
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

type S3Plugin struct {
	Name        string
	Description string
	bucketsView *BucketsView
}

// Start initializes and starts the S3 plugin UI
func (s *S3Plugin) Start(app *tview.Application) tview.Primitive {
	// Initialize the bucket view
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

	pages.AddPage("main", mainUI, true, true)

	return pages
}

// Stop cleans up resources used by the S3 plugin
func (s *S3Plugin) Stop() {
	// Clean up resources
}

// OhmyopsPlugin is exported as a variable to be loaded by the main application
var OhmyopsPlugin S3Plugin

func init() {
	OhmyopsPlugin.Name = "S3 Manager"
	OhmyopsPlugin.Description = "Manage AWS S3 buckets"
}

// GetMetadata is exported as a function to be called directly by the main application
// when the direct type assertion of OhmyopsPlugin fails
func GetMetadata() interface{} {
	return map[string]interface{}{
		"Name":        "s3",
		"Version":     "1.2.0",
		"Description": "Manage AWS S3 buckets",
		"Author":      "HATMAN",
		"License":     "Apache-2.0",
		"Tags":        []string{"storage", "cloud", "aws"},
		"Arch":        []string{"amd64", "arm64"},
		"LastUpdated": time.Now(),
		"URL":         "https://github.com/hatembentayeb/ohmyops-v2/plugins/s3",
	}
}

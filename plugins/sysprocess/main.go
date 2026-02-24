package main

import (
	"time"

	"omo/pkg/pluginapi"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// SysProcessPlugin represents the user-space process monitor plugin
type SysProcessPlugin struct {
	Name        string
	Description string
	processView *ProcessView
}

// Start initializes and starts the plugin UI
func (s *SysProcessPlugin) Start(app *tview.Application) tview.Primitive {
	pluginapi.Log().Info("starting plugin")
	pages := tview.NewPages()
	processView := NewProcessView(app, pages)
	s.processView = processView

	mainUI := processView.GetMainUI()

	// Page-level keyboard handling
	pages.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlR {
			if s.processView != nil {
				go s.processView.loadProcessData()
			}
			return nil
		}
		return event
	})

	pages.AddPage("sysprocess", mainUI, true, true)

	// Set initial focus
	app.SetFocus(s.processView.processListView.GetTable())

	// Welcome messages
	s.processView.processListView.Log("[blue]Process monitor initialized")
	s.processView.processListView.Log("[yellow]Loading user processes...")

	// Initial data load in background
	go func() {
		time.Sleep(100 * time.Millisecond)
		s.processView.loadProcessData()

		s.processView.processListView.Log("[green]Process monitor ready")
		s.processView.processListView.Log("[aqua]Navigation Keys:")
		s.processView.processListView.Log("   [yellow]W[white] - Why Running? (witr-style details)")
		s.processView.processListView.Log("   [yellow]L[white] - Listening Ports")
		s.processView.processListView.Log("   [yellow]G[white] - Warnings")
		s.processView.processListView.Log("   [yellow]S[white] - System Metrics")
		s.processView.processListView.Log("   [yellow]D[white] - Disk Usage (ncdu)")
		s.processView.processListView.Log("   [yellow]K[white] - Kill Process")
		s.processView.processListView.Log("   [yellow]T[white] - Sort by CPU")
		s.processView.processListView.Log("   [yellow]M[white] - Sort by Memory")
		s.processView.processListView.Log("   [yellow]?[white] - Help")
	}()

	return pages
}

// Stop cleans up resources
func (s *SysProcessPlugin) Stop() {
	if s.processView != nil {
		s.processView.Stop()
	}
}

// GetMetadata returns plugin metadata
func (s *SysProcessPlugin) GetMetadata() pluginapi.PluginMetadata {
	return pluginapi.PluginMetadata{
		Name:        "sysprocess",
		Version:     "2.0.0",
		Description: "User-space process monitor — why is this running?",
		Author:      "OhMyOps Team",
		License:     "MIT",
		Tags:        []string{"process", "monitoring", "user", "witr"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "",
	}
}

// OhmyopsPlugin is exported as a variable to be loaded by the main application
var OhmyopsPlugin SysProcessPlugin

func init() {
	OhmyopsPlugin.Name = "Process Monitor"
	OhmyopsPlugin.Description = "User-space process monitor — why is this running?"
}

// GetMetadata is exported for legacy loaders.
func GetMetadata() pluginapi.PluginMetadata {
	return pluginapi.PluginMetadata{
		Name:        "sysprocess",
		Version:     "2.0.0",
		Description: "User-space process monitor — why is this running?",
		Author:      "OhMyOps Team",
		License:     "MIT",
		Tags:        []string{"process", "monitoring", "user", "witr"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "",
	}
}

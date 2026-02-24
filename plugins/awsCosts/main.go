package main

import (
	"time"

	"omo/pkg/pluginapi"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// OhmyopsPlugin is expected by the main application
var OhmyopsPlugin AWSCostsPlugin

// AWSCostsPlugin represents the AWS Costs monitoring plugin
type AWSCostsPlugin struct {
	Name        string
	Description string
	costsView   *AWSCostsView
}

// CostData represents AWS cost information
type CostData struct {
	service   string
	cost      float64
	date      string
	unit      string
	region    string
	usageType string
	trend     float64
	forecast  float64
	budget    float64
}

// safeGo runs a function in a goroutine with panic recovery
func safeGo(f func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				pluginapi.Log().Error("recovered from panic: %v", r)
			}
		}()
		f()
	}()
}

// getCostTimeRange returns a time range struct based on the string description
func getCostTimeRange(timeRange string) struct{ Start, End time.Time } {
	now := time.Now()
	result := struct{ Start, End time.Time }{
		End: now,
	}

	switch timeRange {
	case "LAST_7_DAYS":
		result.Start = now.AddDate(0, 0, -7)
	case "LAST_30_DAYS":
		result.Start = now.AddDate(0, 0, -30)
	case "LAST_3_MONTHS":
		result.Start = now.AddDate(0, -3, 0)
	case "LAST_6_MONTHS":
		result.Start = now.AddDate(0, -6, 0)
	case "LAST_12_MONTHS":
		result.Start = now.AddDate(-1, 0, 0)
	case "THIS_MONTH":
		result.Start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	default:
		result.Start = now.AddDate(0, 0, -30)
	}

	return result
}

// Start initializes the plugin
func (p *AWSCostsPlugin) Start(app *tview.Application) tview.Primitive {
	pluginapi.Log().Info("starting plugin")
	pages := tview.NewPages()
	costsView := NewAWSCostsView(app, pages)
	p.costsView = costsView

	mainUI := costsView.GetMainUI()

	pages.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlT {
			if p.costsView != nil {
				p.costsView.ShowProfileSelector()
			}
			return nil
		}
		return event
	})

	pages.AddPage("awscosts", mainUI, true, true)

	app.SetFocus(p.costsView.cores.GetTable())

	// Auto-connect to profile
	p.costsView.AutoConnect()

	return pages
}

// Stop cleans up resources when the plugin is unloaded.
func (p *AWSCostsPlugin) Stop() {
	if p.costsView != nil {
		p.costsView.Stop()
	}
}

// GetMetadata returns plugin metadata.
func (p *AWSCostsPlugin) GetMetadata() pluginapi.PluginMetadata {
	return pluginapi.PluginMetadata{
		Name:        "awsCosts",
		Version:     "1.0.0",
		Description: "AWS Cost Explorer and Budget Analyzer",
		Author:      "OhMyOps",
		License:     "MIT",
		Tags:        []string{"aws", "cost", "monitoring", "billing"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "",
	}
}

func init() {
	OhmyopsPlugin.Name = "AWS Costs"
	OhmyopsPlugin.Description = "AWS Cost Explorer and Budget Analyzer"
}

// GetMetadata is exported for legacy loaders.
func GetMetadata() pluginapi.PluginMetadata {
	return pluginapi.PluginMetadata{
		Name:        "awsCosts",
		Version:     "1.0.0",
		Description: "AWS Cost Explorer and Budget Analyzer",
		Author:      "OhMyOps",
		License:     "MIT",
		Tags:        []string{"aws", "cost", "monitoring", "billing"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "",
	}
}

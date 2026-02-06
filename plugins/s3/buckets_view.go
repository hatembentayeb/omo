package main

import (
	"fmt"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/rivo/tview"

	"omo/pkg/ui"
)

// BucketsView manages the UI for viewing S3 buckets
type BucketsView struct {
	app            *tview.Application
	pages          *tview.Pages
	cores          *ui.CoreView
	s3Client       *s3.S3
	currentRegion  string
	buckets        []*s3.Bucket
	currentProfile string
}

// NewBucketsView creates a new buckets view
func NewBucketsView(app *tview.Application, pages *tview.Pages) *BucketsView {
	bv := &BucketsView{
		app:           app,
		pages:         pages,
		currentRegion: "us-east-1", // Default region
		// Don't set a profile yet, we'll get it from credentials
	}

	// Create Cores UI component
	bv.cores = ui.NewCoreView(app, "S3 Buckets")
	bv.cores.SetModalPages(pages)

	// Set table headers
	bv.cores.SetTableHeaders([]string{"Name", "Creation Date", "Region"})

	// Set up refresh callback to make 'R' key work properly
	bv.cores.SetRefreshCallback(func() ([][]string, error) {
		// This will be called when RefreshData() is triggered from the core
		bv.refreshBuckets()
		// Return the updated data
		return bv.cores.GetTableData(), nil
	})

	// Add key bindings
	bv.cores.AddKeyBinding("R", "Refresh", bv.refreshBuckets)
	bv.cores.AddKeyBinding("^t", "Profile", bv.ShowProfileSelector)
	bv.cores.AddKeyBinding("?", "Help", bv.showHelp)
	bv.cores.AddKeyBinding("S", "Sort", bv.showSortModal)
	bv.cores.AddKeyBinding("M", "Metrics", bv.showBucketMetricsConfirmation)

	// Set action callback
	bv.cores.SetActionCallback(bv.handleAction)

	// Add row selection callback for debugging and selection tracking
	bv.cores.SetRowSelectedCallback(func(row int) {
		if row >= 0 && row < len(bv.buckets) {
			bv.cores.Log(fmt.Sprintf("[blue]Selected bucket: %s", *bv.buckets[row].Name))
		}
	})

	// Register the key handlers to actually handle the key events
	bv.cores.RegisterHandlers()

	// Get available profiles
	profiles, err := loadAWSProfilesFromCredentials()
	if err != nil {
		bv.cores.Log(fmt.Sprintf("[red]Error loading AWS profiles: %v", err))
	}

	// If we have profiles, use the first one
	if len(profiles) > 0 {
		bv.currentProfile = profiles[0]
		bv.cores.Log(fmt.Sprintf("[blue]Using AWS profile: %s", bv.currentProfile))
	} else {
		// Fallback to empty profile if none found
		bv.currentProfile = ""
		bv.cores.Log("[yellow]No AWS profiles found. Please configure AWS credentials.")
	}

	// Configure AWS session with the selected profile
	bv.configureAWSSession(bv.currentProfile, bv.currentRegion)

	// Initial data refresh
	bv.refreshBuckets()

	return bv
}

// GetMainUI returns the main UI component
func (bv *BucketsView) GetMainUI() tview.Primitive {
	return bv.cores.GetLayout()
}

// handleAction handles actions triggered by the UI
func (bv *BucketsView) handleAction(action string, payload map[string]interface{}) error {
	switch action {
	case "refresh":
		bv.refreshBuckets()
		return nil
	case "keypress":
		// Handle specific key presses
		if key, ok := payload["key"].(string); ok {
			switch key {
			case "?":
				bv.showHelp()
				return nil
			case "S":
				bv.showSortModal()
				return nil
			case "R":
				bv.refreshBuckets()
				return nil
			case "M":
				bv.showBucketMetricsConfirmation()
				return nil
			}
		}
	}
	return nil
}

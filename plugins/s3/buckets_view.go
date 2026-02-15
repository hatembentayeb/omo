package main

import (
	"fmt"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/rivo/tview"

	"omo/pkg/ui"
)

// BucketsView manages the UI for viewing S3 buckets
type BucketsView struct {
	app             *tview.Application
	pages           *tview.Pages
	viewPages       *tview.Pages
	cores           *ui.CoreView
	objectsView     *ui.CoreView
	s3Client        *s3.S3
	currentRegion   string
	buckets         []*s3.Bucket
	currentProfile  string
	currentViewName string
	currentBucket   string
	currentPrefix   string
}

// NewBucketsView creates a new buckets view
func NewBucketsView(app *tview.Application, pages *tview.Pages) *BucketsView {
	bv := &BucketsView{
		app:             app,
		pages:           pages,
		viewPages:       tview.NewPages(),
		currentRegion:   "us-east-1", // Default region
		currentViewName: s3ViewBuckets,
	}

	// Create Cores UI component for buckets
	bv.cores = ui.NewCoreView(app, "S3 Buckets")
	bv.cores.SetModalPages(pages)

	// Set table headers
	bv.cores.SetTableHeaders([]string{"Name", "Creation Date", "Region"})

	// Set up refresh callback to make 'R' key work properly
	bv.cores.SetRefreshCallback(func() ([][]string, error) {
		bv.refreshBuckets()
		return bv.cores.GetTableData(), nil
	})

	// Add key bindings
	bv.cores.AddKeyBinding("R", "Refresh", bv.refreshBuckets)
	bv.cores.AddKeyBinding("^t", "Profile", bv.ShowProfileSelector)
	bv.cores.AddKeyBinding("?", "Help", bv.showHelp)
	bv.cores.AddKeyBinding("S", "Sort", bv.showSortModal)
	bv.cores.AddKeyBinding("M", "Metrics", bv.showBucketMetricsConfirmation)
	bv.cores.AddKeyBinding("O", "Objects", nil)

	// Set action callback
	bv.cores.SetActionCallback(bv.handleAction)

	// Add row selection callback
	bv.cores.SetRowSelectedCallback(func(row int) {
		if row >= 0 && row < len(bv.buckets) {
			bv.cores.Log(fmt.Sprintf("[blue]Selected bucket: %s", *bv.buckets[row].Name))
		}
	})

	// Register the key handlers
	bv.cores.RegisterHandlers()

	// Create objects view
	bv.objectsView = bv.newObjectsView()
	bv.objectsView.SetModalPages(pages)

	// Add views to viewPages
	bv.viewPages.AddPage("s3-buckets", bv.cores.GetLayout(), true, true)
	bv.viewPages.AddPage("s3-objects", bv.objectsView.GetLayout(), true, false)

	// Set initial view stacks
	bv.setViewStack(bv.cores, s3ViewBuckets)
	bv.setViewStack(bv.objectsView, s3ViewObjects)

	// Get available profiles (from YAML config + ~/.aws/credentials + ~/.aws/config)
	profileInfos := bv.getAWSProfiles()

	// If we have profiles, use the first one
	if len(profileInfos) > 0 {
		bv.currentProfile = profileInfos[0].Name
		bv.currentRegion = profileInfos[0].Region
		bv.cores.Log(fmt.Sprintf("[blue]Using AWS profile: %s (region: %s)", bv.currentProfile, bv.currentRegion))
	} else {
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
	return bv.viewPages
}

// handleAction handles actions triggered by the UI
func (bv *BucketsView) handleAction(action string, payload map[string]interface{}) error {
	switch action {
	case "refresh":
		current := bv.currentCores()
		if current != nil {
			current.RefreshData()
		}
		return nil
	case "keypress":
		if key, ok := payload["key"].(string); ok {
			switch key {
			case "?":
				bv.showHelp()
				return nil
			case "S":
				bv.showSortModal()
				return nil
			case "R":
				current := bv.currentCores()
				if current != nil {
					current.RefreshData()
				}
				return nil
			case "M":
				bv.showBucketMetricsConfirmation()
				return nil
			case "O":
				bv.openSelectedBucketObjects()
				return nil
			case "B":
				if bv.currentViewName == s3ViewObjects {
					bv.showBucketsView()
					return nil
				}
			}
		}
	case "navigate_back":
		if view, ok := payload["current_view"].(string); ok {
			if view == s3ViewRoot {
				bv.showBucketsView()
				return nil
			}
			bv.switchView(view)
			return nil
		}
	}
	return nil
}

// openSelectedBucketObjects navigates to objects view for the selected bucket
func (bv *BucketsView) openSelectedBucketObjects() {
	selectedRow := bv.cores.GetSelectedRow()
	if selectedRow < 0 || selectedRow >= len(bv.buckets) {
		bv.cores.Log("[red]No bucket selected")
		return
	}

	bucketName := *bv.buckets[selectedRow].Name
	bv.currentBucket = bucketName
	bv.currentPrefix = ""
	bv.cores.Log(fmt.Sprintf("[blue]Opening objects for bucket: %s", bucketName))
	bv.showObjectsView()
}

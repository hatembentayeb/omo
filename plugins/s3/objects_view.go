package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"

	"omo/pkg/ui"
)

// newObjectsView creates the objects CoreView for browsing S3 objects
func (bv *BucketsView) newObjectsView() *ui.CoreView {
	view := ui.NewCoreView(bv.app, "S3 Objects")

	// Set table headers
	view.SetTableHeaders([]string{"Name", "Size", "Last Modified", "Storage Class"})

	// Set up refresh callback
	view.SetRefreshCallback(func() ([][]string, error) {
		return bv.refreshObjects()
	})

	// Add key bindings
	view.AddKeyBinding("R", "Refresh", nil)
	view.AddKeyBinding("B", "Buckets", nil)
	view.AddKeyBinding("?", "Help", nil)
	view.AddKeyBinding("I", "Info", nil)
	view.AddKeyBinding("U", "Up Dir", nil)

	// Set action callback (shares with buckets view)
	view.SetActionCallback(bv.handleObjectsAction)

	// Row selection callback
	view.SetRowSelectedCallback(func(row int) {
		tableData := view.GetTableData()
		if row >= 0 && row < len(tableData) && len(tableData[row]) > 0 {
			view.Log(fmt.Sprintf("[blue]Selected: %s", tableData[row][0]))
		}
	})

	// Double-click / Enter to navigate into directories
	view.GetTable().SetSelectedFunc(func(row, column int) {
		if row <= 0 {
			return
		}
		bv.navigateIntoObject()
	})

	// Register handlers
	view.RegisterHandlers()

	return view
}

// refreshObjects fetches and returns object data for the current bucket/prefix
func (bv *BucketsView) refreshObjects() ([][]string, error) {
	if bv.s3Client == nil {
		return [][]string{{"S3 client not initialized", "", "", ""}}, nil
	}

	if bv.currentBucket == "" {
		return [][]string{{"No bucket selected", "Press B to go back", "", ""}}, nil
	}

	// Get the correct region for the bucket
	bucketRegion, err := bv.getBucketRegion(bv.currentBucket)
	if err != nil {
		bucketRegion = bv.currentRegion
	}

	// Create S3 client for the bucket's region
	client := bv.createS3ClientForRegion(bv.currentProfile, bucketRegion)
	if client == nil {
		return [][]string{{"Error creating S3 client", "", "", ""}}, nil
	}

	// Use delimiter to get folder-like behavior
	input := &s3.ListObjectsV2Input{
		Bucket:    aws.String(bv.currentBucket),
		Delimiter: aws.String("/"),
		MaxKeys:   aws.Int64(1000),
	}

	if bv.currentPrefix != "" {
		input.Prefix = aws.String(bv.currentPrefix)
	}

	result, err := client.ListObjectsV2(input)
	if err != nil {
		return [][]string{{"Error listing objects", err.Error(), "", ""}}, nil
	}

	var tableData [][]string

	// Add ".." entry if we're in a subdirectory
	if bv.currentPrefix != "" {
		tableData = append(tableData, []string{"../", "", "", ""})
	}

	// Add "directories" (common prefixes)
	for _, prefix := range result.CommonPrefixes {
		if prefix.Prefix == nil {
			continue
		}
		// Display just the folder name, not the full prefix
		name := *prefix.Prefix
		if bv.currentPrefix != "" {
			name = strings.TrimPrefix(name, bv.currentPrefix)
		}
		tableData = append(tableData, []string{name, "-", "-", "Directory"})
	}

	// Add files
	for _, obj := range result.Contents {
		if obj.Key == nil {
			continue
		}

		// Skip the prefix itself (shows as empty entry)
		key := *obj.Key
		if key == bv.currentPrefix {
			continue
		}

		// Display just the filename, not the full key
		name := key
		if bv.currentPrefix != "" {
			name = strings.TrimPrefix(key, bv.currentPrefix)
		}

		size := ""
		if obj.Size != nil {
			size = formatSize(*obj.Size)
		}

		lastModified := ""
		if obj.LastModified != nil {
			lastModified = obj.LastModified.Format(time.RFC3339)
		}

		storageClass := ""
		if obj.StorageClass != nil {
			storageClass = *obj.StorageClass
		} else {
			storageClass = "STANDARD"
		}

		tableData = append(tableData, []string{name, size, lastModified, storageClass})
	}

	if len(tableData) == 0 {
		tableData = [][]string{{"Empty bucket", "", "", ""}}
	}

	// Update info panel
	prefix := bv.currentPrefix
	if prefix == "" {
		prefix = "/"
	}
	bv.objectsView.SetInfoText(fmt.Sprintf("[green]S3 Objects[white]\nBucket: %s\nPrefix: %s\nProfile: %s",
		bv.currentBucket, prefix, bv.currentProfile))

	return tableData, nil
}

// handleObjectsAction handles actions specific to the objects view
func (bv *BucketsView) handleObjectsAction(action string, payload map[string]interface{}) error {
	switch action {
	case "refresh":
		bv.objectsView.RefreshData()
		return nil
	case "keypress":
		if key, ok := payload["key"].(string); ok {
			switch key {
			case "R":
				bv.objectsView.RefreshData()
				return nil
			case "B":
				bv.showBucketsView()
				return nil
			case "?":
				bv.showObjectsHelp()
				return nil
			case "I":
				bv.showObjectInfo()
				return nil
			case "U":
				bv.navigateUp()
				return nil
			}
		}
	case "navigate_back":
		if view, ok := payload["current_view"].(string); ok {
			if view == s3ViewRoot || view == s3ViewBuckets {
				bv.showBucketsView()
				return nil
			}
		}
		// Default: go back to buckets
		bv.showBucketsView()
		return nil
	}
	return nil
}

// navigateIntoObject navigates into a directory or shows object info
func (bv *BucketsView) navigateIntoObject() {
	selectedRow := bv.objectsView.GetSelectedRow()
	tableData := bv.objectsView.GetTableData()
	if selectedRow < 0 || selectedRow >= len(tableData) {
		return
	}

	name := tableData[selectedRow][0]
	storageClass := tableData[selectedRow][3]

	// Handle ".." navigation
	if name == "../" {
		bv.navigateUp()
		return
	}

	// Handle directory navigation
	if storageClass == "Directory" || strings.HasSuffix(name, "/") {
		bv.currentPrefix = bv.currentPrefix + name
		bv.objectsView.Log(fmt.Sprintf("[blue]Navigating to: %s", bv.currentPrefix))
		bv.objectsView.RefreshData()
		return
	}

	// For files, show info
	bv.showObjectInfo()
}

// navigateUp navigates to the parent directory
func (bv *BucketsView) navigateUp() {
	if bv.currentPrefix == "" {
		// Already at root, go back to buckets
		bv.showBucketsView()
		return
	}

	// Remove the last directory from the prefix
	prefix := strings.TrimSuffix(bv.currentPrefix, "/")
	lastSlash := strings.LastIndex(prefix, "/")
	if lastSlash >= 0 {
		bv.currentPrefix = prefix[:lastSlash+1]
	} else {
		bv.currentPrefix = ""
	}

	bv.objectsView.Log(fmt.Sprintf("[blue]Navigating up to: %s", bv.currentPrefix))
	bv.objectsView.RefreshData()
}

// showObjectInfo shows detailed information about the selected object
func (bv *BucketsView) showObjectInfo() {
	selectedRow := bv.objectsView.GetSelectedRow()
	tableData := bv.objectsView.GetTableData()
	if selectedRow < 0 || selectedRow >= len(tableData) {
		bv.objectsView.Log("[red]No object selected")
		return
	}

	row := tableData[selectedRow]
	name := row[0]

	if name == "../" || row[3] == "Directory" {
		return
	}

	// Build full key
	fullKey := bv.currentPrefix + name

	infoText := fmt.Sprintf(`[yellow]Object Details[white]

[aqua]Key:[white] %s
[aqua]Bucket:[white] %s
[aqua]Size:[white] %s
[aqua]Last Modified:[white] %s
[aqua]Storage Class:[white] %s
[aqua]Profile:[white] %s
`,
		fullKey, bv.currentBucket, row[1], row[2], row[3], bv.currentProfile)

	ui.ShowInfoModal(
		bv.pages,
		bv.app,
		fmt.Sprintf("Object: %s", name),
		infoText,
		func() {
			bv.app.SetFocus(bv.objectsView.GetTable())
		},
	)
}

// showObjectsHelp shows help for the objects view
func (bv *BucketsView) showObjectsHelp() {
	content := `[yellow]S3 Objects View Help[white]

[green]Navigation[white]
  [aqua]↑/↓[white] - Navigate between objects
  [aqua]Enter[white] - Open directory / Show object info
  [aqua]U[white] - Navigate up to parent directory

[green]Actions[white]
  [aqua]R[white] - Refresh object list
  [aqua]I[white] - Show object details
  [aqua]B[white] - Back to buckets view
  [aqua]?[white] - Show this help screen
  [aqua]ESC[white] - Navigate back`

	ui.ShowInfoModal(
		bv.pages,
		bv.app,
		"S3 Objects Help",
		content,
		func() {
			bv.app.SetFocus(bv.objectsView.GetTable())
		},
	)
}

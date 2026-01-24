package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"omo/pkg/ui"
)

// showSortModal displays the sort options modal
func (bv *BucketsView) showSortModal() {
	// Get the column headers from the cores
	columns := bv.cores.GetTableHeaders()

	// Show the sort modal
	ui.ShowSortModal(
		bv.pages,
		bv.app,
		columns,
		func(options *ui.SortOptions, cancelled bool) {
			if !cancelled && options != nil {
				// Apply the sort
				bv.sortBuckets(options)
			} else {
				// Return focus to the table
				bv.app.SetFocus(bv.cores.GetTable())
			}
		},
	)
}

// sortBuckets sorts the bucket list based on the provided options
func (bv *BucketsView) sortBuckets(options *ui.SortOptions) {
	if len(bv.buckets) == 0 {
		bv.cores.Log("[yellow]No buckets to sort")
		return
	}

	// Get current table data
	tableData := bv.cores.GetTableData()
	if len(tableData) == 0 {
		bv.cores.Log("[yellow]No data to sort")
		return
	}

	// Find the column index
	colIndex := -1
	columns := bv.cores.GetTableHeaders()
	for i, col := range columns {
		if col == options.Column {
			colIndex = i
			break
		}
	}

	if colIndex == -1 {
		bv.cores.Log(fmt.Sprintf("[red]Column '%s' not found", options.Column))
		return
	}

	// Log the sort operation
	bv.cores.Log(fmt.Sprintf("[blue]Sorting by %s (%s, %s)",
		options.Column, options.SortType, options.Direction))

	// Create a copy of the table data for sorting
	sortedData := make([][]string, len(tableData))
	copy(sortedData, tableData)

	// Perform the sort based on the options
	sort.Slice(sortedData, func(i, j int) bool {
		// Get the values to compare
		valueI := sortedData[i][colIndex]
		valueJ := sortedData[j][colIndex]

		// Compare based on sort type
		var result bool
		switch options.SortType {
		case "alphabet":
			result = strings.ToLower(valueI) < strings.ToLower(valueJ)
		case "date":
			// Try to parse as time
			timeI, errI := time.Parse(time.RFC3339, valueI)
			timeJ, errJ := time.Parse(time.RFC3339, valueJ)
			if errI == nil && errJ == nil {
				result = timeI.Before(timeJ)
			} else {
				// Fall back to string comparison if parsing fails
				result = valueI < valueJ
			}
		case "number":
			// Try to parse as numbers
			var numI, numJ float64
			_, errI := fmt.Sscanf(valueI, "%f", &numI)
			_, errJ := fmt.Sscanf(valueJ, "%f", &numJ)
			if errI == nil && errJ == nil {
				result = numI < numJ
			} else {
				// Fall back to string comparison if parsing fails
				result = valueI < valueJ
			}
		default:
			// Default to string comparison
			result = valueI < valueJ
		}

		// Reverse result for descending order
		if options.Direction == "desc" {
			result = !result
		}

		return result
	})

	// Update the table with sorted data
	bv.cores.SetTableData(sortedData)
	bv.cores.Log(fmt.Sprintf("[green]Sorted by %s (%s order)",
		options.Column, options.Direction))

	// Return focus to the table
	bv.app.SetFocus(bv.cores.GetTable())
}

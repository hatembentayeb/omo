// Package ui provides terminal UI components for building consistent
// terminal applications with a unified interface.
package ui

import (
	"fmt"
	"strings"

	"github.com/rivo/tview"
)

// selectRow selects a table row by its data index.
// This function handles the selection logic including:
// - Triggering the row selection callback if set
// - Triggering the action callback with selection data
//
// Parameters:
//   - dataRow: The index of the row in the data array (0-based, excluding header)
func (c *Cores) selectRow(dataRow int) {
	if dataRow >= 0 && dataRow < len(c.tableData) {
		c.selectedRow = dataRow

		// Trigger callback if set
		if c.onRowSelected != nil {
			c.onRowSelected(dataRow)
		}

		// Trigger action callback if set
		if c.onAction != nil {
			selectedData := c.tableData[dataRow]
			payload := make(map[string]interface{})
			payload["rowIndex"] = dataRow
			payload["rowData"] = selectedData

			// Also include named data if we have headers
			if len(c.tableHeaders) > 0 {
				namedData := make(map[string]string)
				for i, header := range c.tableHeaders {
					if i < len(selectedData) {
						namedData[header] = selectedData[i]
					}
				}
				payload["namedData"] = namedData
			}

			c.onAction("rowSelected", payload)
		}
	}
}

// clearSelection clears the current row selection.
func (c *Cores) clearSelection() {
	c.selectedRow = -1
}

// getRowSignature creates a unique identifier for a row.
// This function generates a signature that can be used to identify a row
// even when the table data changes, by using either the selection key column
// or a combination of values from the first few columns.
//
// Parameters:
//   - row: The row data array
//
// Returns:
//   - A string identifier for the row
func (c *Cores) getRowSignature(row []string) string {
	// If a specific selection key column is set, use that
	if c.selectionKey != "" {
		for i, header := range c.tableHeaders {
			if header == c.selectionKey && i < len(row) {
				return row[i]
			}
		}
	}

	// Otherwise use a composite signature from first 3 columns (or fewer if not available)
	var sb strings.Builder
	for i := 0; i < min(3, len(row)); i++ {
		sb.WriteString(row[i])
		sb.WriteString("|")
	}
	return sb.String()
}

// refreshTable updates the table display with current data.
// This function updates the virtual table content and preserves selection.
func (c *Cores) refreshTable() {
	// Save current selection signature for restoring later
	var selectedSignature string
	selectedIndex := c.selectedRow
	if c.selectedRow >= 0 && c.selectedRow < len(c.tableData) {
		selectedSignature = c.getRowSignature(c.tableData[c.selectedRow])
	}

	// Update virtual table content
	c.tableContent.SetHeaders(c.tableHeaders)
	c.tableContent.SetData(c.tableData)

	// Try to restore selection by matching signature
	restored := false
	if selectedSignature != "" {
		for i, row := range c.tableData {
			if c.getRowSignature(row) == selectedSignature {
				c.selectedRow = i
				c.table.Select(i+1, 0) // +1 for header
				restored = true
				break
			}
		}
	}
	if !restored && selectedIndex >= 0 && len(c.tableData) > 0 {
		// Fallback to previous index if signature isn't available
		if selectedIndex >= len(c.tableData) {
			selectedIndex = len(c.tableData) - 1
		}
		c.selectedRow = selectedIndex
		c.table.Select(selectedIndex+1, 0) // +1 for header
	}
}

// SetTableHeaders sets the headers for the table.
// This function updates the table headers and refreshes the table display.
//
// Parameters:
//   - headers: Array of header strings
//
// Returns:
//   - The Cores instance for method chaining
func (c *Cores) SetTableHeaders(headers []string) *Cores {
	c.tableHeaders = headers
	c.refreshTable()
	return c
}

// SetTableData sets the data for the table.
// This function updates the table data and refreshes the table display.
//
// Parameters:
//   - data: 2D array of data strings
//
// Returns:
//   - The Cores instance for method chaining
func (c *Cores) SetTableData(data [][]string) *Cores {
	c.dataMutex.Lock()
	defer c.dataMutex.Unlock()
	c.rawTableData = data
	c.tableData = c.applyFilter(data)
	c.refreshTable()
	return c
}

// AppendTableData appends rows to the current data set.
func (c *Cores) AppendTableData(data [][]string) *Cores {
	if len(data) == 0 {
		return c
	}
	c.dataMutex.Lock()
	defer c.dataMutex.Unlock()
	c.rawTableData = append(c.rawTableData, data...)
	c.tableData = c.applyFilter(c.rawTableData)
	c.refreshTable()
	return c
}

// SetFilterQuery applies or clears the current table filter.
// Filters only already-loaded data - no blocking, instant response.
func (c *Cores) SetFilterQuery(query string) *Cores {
	c.dataMutex.Lock()
	defer c.dataMutex.Unlock()
	if c.isLoading {
		c.Log("[yellow]Loading in progress...")
		return c
	}
	c.filterQuery = strings.TrimSpace(query)
	beforeCount := len(c.rawTableData)
	c.tableData = c.applyFilter(c.rawTableData)
	c.refreshTable()
	afterCount := len(c.tableData)
	if c.filterQuery == "" {
		c.Log("[yellow]Filter cleared")
		c.table.SetTitle(fmt.Sprintf(" [yellow]%s[white] ", c.title))
	} else {
		c.Log(fmt.Sprintf("[green]Filter '%s': %d/%d rows", c.filterQuery, afterCount, beforeCount))
		c.table.SetTitle(fmt.Sprintf(" [yellow]%s[white] [gray](filter: %s)[white] ", c.title, c.filterQuery))
		if afterCount == 0 && c.lazyHasMore {
			c.Log("[gray]More data available - use PgDn to load more")
		}
	}
	return c
}

func (c *Cores) applyFilter(data [][]string) [][]string {
	if c.filterQuery == "" {
		c.filteredIndices = nil
		return data
	}
	query := strings.ToLower(c.filterQuery)
	filtered := make([][]string, 0, len(data))
	c.filteredIndices = make([]int, 0, len(data))
	for i, row := range data {
		for _, cell := range row {
			if strings.Contains(strings.ToLower(cell), query) {
				filtered = append(filtered, row)
				c.filteredIndices = append(c.filteredIndices, i)
				break
			}
		}
	}
	return filtered
}

// GetSelectedRowData returns the data of the currently selected row, or nil if none.
// This is the preferred method to access selected row data as it works correctly
// with both filtered and unfiltered views.
//
// Returns:
//   - The selected row data array, or nil if no row is selected
func (c *Cores) GetSelectedRowData() []string {
	if c.selectedRow >= 0 && c.selectedRow < len(c.tableData) {
		return c.tableData[c.selectedRow]
	}
	return nil
}

// GetTableHeaders returns the current table headers.
//
// Returns:
//   - Array of header strings
func (c *Cores) GetTableHeaders() []string {
	return c.tableHeaders
}

// GetTableData returns the current table data.
//
// Returns:
//   - 2D array of data strings
func (c *Cores) GetTableData() [][]string {
	return c.tableData
}

// SetTableTitle sets the title of the table.
//
// Parameters:
//   - title: The title string to display
//
// Returns:
//   - The Cores instance for method chaining
func (c *Cores) SetTableTitle(title string) *Cores {
	c.table.SetTitle(title)
	return c
}

// SetSelectionKey sets a specific column to use for tracking row selections.
// This column will be used to identify rows when restoring selection after refresh.
//
// Parameters:
//   - columnName: The name of the column to use as selection key
//
// Returns:
//   - The Cores instance for method chaining
func (c *Cores) SetSelectionKey(columnName string) *Cores {
	c.selectionKey = columnName
	return c
}

// UpdateRow updates a single row in the table.
// This function updates the underlying data - the virtual table will
// pick up the change on next draw.
//
// Parameters:
//   - index: The index of the row to update
//   - rowData: The new data for the row
func (c *Cores) UpdateRow(index int, rowData []string) {
	// Ensure index is valid
	if index < 0 || index >= len(c.tableData) {
		return
	}

	// Update the data in the table
	c.tableData[index] = rowData
	// Virtual table content will read from tableData on next draw
	c.tableContent.SetData(c.tableData)
}

// Table wraps tview.Table to provide additional functionality.
// This custom table component extends the standard tview.Table
// with OMO-specific enhancements for selection and display.
type Table struct {
	*tview.Table
}

// NewTable creates a new Table instance.
// This factory function initializes a new Table with default settings.
//
// Returns:
//   - A new Table instance
func NewTable() *Table {
	return &Table{
		Table: tview.NewTable(),
	}
}

// GetSelectedRow returns the currently selected row in the table.
//
// Returns:
//   - The index of the selected row
func (t *Table) GetSelectedRow() int {
	row, _ := t.GetSelection()
	return row
}

// SetupSelection configures the table for row selection.
// This function sets up the table to allow row selection.
func (t *Table) SetupSelection() {
	t.SetSelectable(true, false)
	t.Select(0, 0)
}

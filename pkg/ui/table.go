// Package ui provides terminal UI components for building consistent
// terminal applications with a unified interface.
package ui

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// selectRow selects a table row by its data index.
// This function handles the selection logic including:
// - Visual highlighting of the selected row
// - Triggering the row selection callback if set
// - Triggering the action callback with selection data
//
// Parameters:
//   - dataRow: The index of the row in the data array (0-based, excluding header)
func (c *Cores) selectRow(dataRow int) {
	// No need to clear selection since we'll just apply highlighting
	// where needed. The selectedRow is now set by SetSelectionChangedFunc.

	// Apply highlighting to this row
	if dataRow >= 0 && dataRow < len(c.tableData) {
		c.highlightRow(dataRow+1, true) // +1 for header row

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

// highlightRow visually highlights or unhighlights a row.
// This function applies visual styling to a row to indicate selection state.
//
// Parameters:
//   - row: The table row index (1-based, including header)
//   - highlight: Whether to highlight (true) or unhighlight (false) the row
func (c *Cores) highlightRow(row int, highlight bool) {
	for col := 0; col < len(c.tableHeaders); col++ {
		cell := c.table.GetCell(row, col)
		if highlight {
			// Remove the background color highlight but keep text slightly brighter
			cell.SetBackgroundColor(tcell.ColorDefault)
			cell.SetTextColor(tcell.ColorWhite).SetAttributes(tcell.AttrBold)
		} else {
			cell.SetBackgroundColor(tcell.ColorDefault)
			cell.SetAttributes(tcell.AttrNone)
			cell.SetTextColor(tcell.ColorWhite)
		}
	}
}

// clearSelection clears the current row selection.
// This removes any visual highlighting from the currently selected row
// and resets the selected row index.
func (c *Cores) clearSelection() {
	if c.selectedRow >= 0 {
		c.highlightRow(c.selectedRow+1, false) // +1 for header row
		c.selectedRow = -1
	}
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
// This function rebuilds the entire table with the current headers and data,
// preserving selection where possible by matching row signatures.
func (c *Cores) refreshTable() {
	// Save current selection signature for restoring later
	var selectedSignature string
	if c.selectedRow >= 0 && c.selectedRow < len(c.tableData) {
		selectedSignature = c.getRowSignature(c.tableData[c.selectedRow])
	}

	// Clear the table
	c.table.Clear()

	// Add headers - convert to uppercase
	for i, header := range c.tableHeaders {
		headerText := strings.ToUpper(header)

		cell := tview.NewTableCell(headerText).
			SetTextColor(tcell.ColorYellow).
			SetBackgroundColor(tcell.ColorDefault).
			SetAttributes(tcell.AttrBold).
			SetSelectable(false)

		// First column gets more space, others share equally
		if i == 0 {
			cell.SetExpansion(2) // Give first column more space
		} else {
			cell.SetExpansion(1) // Other columns share equally
		}

		c.table.SetCell(0, i, cell)
	}

	// Add data rows
	for rowIdx, row := range c.tableData {
		for colIdx, cellData := range row {
			if colIdx >= len(c.tableHeaders) {
				continue
			}

			cell := tview.NewTableCell(cellData).
				SetTextColor(tcell.ColorAqua).
				SetBackgroundColor(tcell.ColorDefault).
				SetSelectable(true).
				SetAlign(tview.AlignLeft)

			// First column gets more space, others share equally
			if colIdx == 0 {
				cell.SetExpansion(2) // Give first column more space
			} else {
				cell.SetExpansion(1) // Other columns share equally
			}

			c.table.SetCell(rowIdx+1, colIdx, cell)
		}
	}

	// Try to restore selection by matching signature
	if selectedSignature != "" {
		for i, row := range c.tableData {
			if c.getRowSignature(row) == selectedSignature {
				c.selectRow(i)
				c.table.Select(i+1, 0) // +1 for header
				break
			}
		}
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
	c.tableData = data
	c.refreshTable()
	return c
}

// GetSelectedRowData returns the data of the currently selected row, or nil if none.
// This provides access to the underlying data for the selected row.
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
// This function updates both the underlying data and the visual display
// for the specified row.
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

	// Update the visual table
	for j, value := range rowData {
		if j < c.table.GetColumnCount() {
			c.table.SetCell(index+1, j, // +1 for header row
				tview.NewTableCell(value).
					SetTextColor(tcell.ColorWhite).
					SetAlign(tview.AlignLeft))
		}
	}
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

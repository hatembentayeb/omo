// Package ui provides terminal UI components for building consistent
// terminal applications with a unified interface.
package ui

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// VirtualTableContent implements tview.TableContent for efficient rendering
// of large datasets. Only visible rows are rendered.
type VirtualTableContent struct {
	headers  []string
	data     [][]string
	selected int
}

// NewVirtualTableContent creates a new virtual table content.
func NewVirtualTableContent() *VirtualTableContent {
	return &VirtualTableContent{
		headers:  []string{},
		data:     [][]string{},
		selected: -1,
	}
}

// SetHeaders sets the table headers.
func (v *VirtualTableContent) SetHeaders(headers []string) {
	v.headers = headers
}

// SetData sets the table data.
func (v *VirtualTableContent) SetData(data [][]string) {
	v.data = data
}

// SetSelected sets the selected row index.
func (v *VirtualTableContent) SetSelected(row int) {
	v.selected = row
}

// GetCell returns the cell at the given position.
func (v *VirtualTableContent) GetCell(row, column int) *tview.TableCell {
	if column < 0 || column >= len(v.headers) {
		return nil
	}

	// Header row
	if row == 0 {
		return tview.NewTableCell(strings.ToUpper(v.headers[column])).
			SetTextColor(tcell.ColorYellow).
			SetBackgroundColor(tcell.ColorDefault).
			SetAttributes(tcell.AttrBold).
			SetSelectable(false).
			SetExpansion(v.getExpansion(column))
	}

	// Data rows
	dataRow := row - 1
	if dataRow < 0 || dataRow >= len(v.data) {
		return nil
	}

	rowData := v.data[dataRow]
	if column >= len(rowData) {
		return tview.NewTableCell("").
			SetTextColor(tcell.ColorAqua).
			SetBackgroundColor(tcell.ColorDefault).
			SetSelectable(true).
			SetExpansion(v.getExpansion(column))
	}

	cell := tview.NewTableCell(rowData[column]).
		SetTextColor(tcell.ColorAqua).
		SetBackgroundColor(tcell.ColorDefault).
		SetSelectable(true).
		SetAlign(tview.AlignLeft).
		SetExpansion(v.getExpansion(column))

	return cell
}

func (v *VirtualTableContent) getExpansion(column int) int {
	if column == 0 {
		return 2
	}
	return 1
}

// GetRowCount returns the total number of rows including header.
func (v *VirtualTableContent) GetRowCount() int {
	return len(v.data) + 1 // +1 for header
}

// GetColumnCount returns the number of columns.
func (v *VirtualTableContent) GetColumnCount() int {
	return len(v.headers)
}

// SetCell is required by the interface but we don't use it.
func (v *VirtualTableContent) SetCell(row, column int, cell *tview.TableCell) {
	// Not used - data is managed externally
}

// RemoveRow is required by the interface but we don't use it.
func (v *VirtualTableContent) RemoveRow(row int) {
	// Not used - data is managed externally
}

// RemoveColumn is required by the interface but we don't use it.
func (v *VirtualTableContent) RemoveColumn(column int) {
	// Not used - data is managed externally
}

// InsertRow is required by the interface but we don't use it.
func (v *VirtualTableContent) InsertRow(row int) {
	// Not used - data is managed externally
}

// InsertColumn is required by the interface but we don't use it.
func (v *VirtualTableContent) InsertColumn(column int) {
	// Not used - data is managed externally
}

// Clear is required by the interface but we don't use it.
func (v *VirtualTableContent) Clear() {
	v.data = [][]string{}
}

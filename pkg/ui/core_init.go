// Package ui provides terminal UI components for building consistent
// terminal applications with a unified interface.
package ui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func init() {
	// tview draws a double-line frame (╔═╗) when a bordered primitive has
	// focus. Keep a single-line box for the plugin table (and other chrome).
	tview.Borders.HorizontalFocus = tview.Borders.Horizontal
	tview.Borders.VerticalFocus = tview.Borders.Vertical
	tview.Borders.TopLeftFocus = tview.Borders.TopLeft
	tview.Borders.TopRightFocus = tview.Borders.TopRight
	tview.Borders.BottomLeftFocus = tview.Borders.BottomLeft
	tview.Borders.BottomRightFocus = tview.Borders.BottomRight
}

// initUI builds:
//
//	header: connection info | Views (0-9) | Actions
//	then content (table or in-place logs) + breadcrumbs
func (c *CoreView) initUI() {
	c.breadcrumbs = tview.NewTextView()
	c.breadcrumbs.SetDynamicColors(true)
	c.breadcrumbs.SetTextAlign(tview.AlignLeft)
	c.breadcrumbs.SetText(c.title)
	c.breadcrumbs.SetBackgroundColor(tcell.ColorDefault)
	c.breadcrumbs.SetBorder(false)

	c.infoPanel = tview.NewTextView()
	c.infoPanel.SetDynamicColors(true)
	c.infoPanel.SetTextAlign(tview.AlignLeft)
	c.infoPanel.SetText(fmt.Sprintf("[yellow]%s[white]\nStatus: Active", c.title))
	c.infoPanel.SetBackgroundColor(tcell.ColorDefault)
	c.infoPanel.SetBorder(false)

	c.viewsPanel = tview.NewTextView()
	c.viewsPanel.SetDynamicColors(true)
	c.viewsPanel.SetTextAlign(tview.AlignLeft)
	c.viewsPanel.SetText(c.getViewsText())
	c.viewsPanel.SetBackgroundColor(tcell.ColorDefault)
	c.viewsPanel.SetBorder(false)

	// Former logs column — expanded key shortcuts for the current view.
	c.keysPanel = tview.NewTextView()
	c.keysPanel.SetDynamicColors(true)
	c.keysPanel.SetTextAlign(tview.AlignLeft)
	c.keysPanel.SetScrollable(true)
	c.keysPanel.SetText(c.getKeysText())
	c.keysPanel.SetBackgroundColor(tcell.ColorDefault)
	c.keysPanel.SetBorder(false)

	c.table = NewTable()
	c.table.SetBorders(false)
	c.table.SetSelectable(true, false)
	c.table.SetBackgroundColor(tcell.ColorDefault)
	c.table.Box.SetBackgroundColor(tcell.ColorDefault)
	c.table.SetBorder(true)
	c.table.SetBorderPadding(0, 0, 1, 1)
	c.table.SetBorderStyle(tcell.StyleDefault.
		Foreground(tcell.ColorTeal).
		Background(tcell.ColorDefault))

	c.table.SetSelectedStyle(
		tcell.StyleDefault.
			Foreground(tcell.ColorBlack).
			Background(tcell.ColorAqua),
	)

	c.table.SetTitle(fmt.Sprintf(" [aqua]%s[white] ", c.title))
	c.table.SetTitleAlign(tview.AlignCenter)
	c.table.SetTitleColor(tcell.ColorTeal)

	c.tableContent = NewVirtualTableContent()
	c.table.SetContent(c.tableContent)
	c.table.SetFixed(1, 0)

	c.table.SetSelectionChangedFunc(func(row, column int) {
		if row <= 0 {
			return
		}
		if row-1 < len(c.tableData) {
			c.selectedRow = row - 1
			if c.onHighlightChanged != nil {
				c.onHighlightChanged(c.selectedRow)
			}
		}
	})

	c.table.SetSelectedFunc(func(row, column int) {
		if row <= 0 {
			return
		}
		c.selectRow(row - 1)
	})

	headerRow := tview.NewFlex()
	headerRow.SetDirection(tview.FlexColumn)
	headerRow.SetBackgroundColor(tcell.ColorDefault)
	headerRow.AddItem(c.infoPanel, 0, 1, false).
		AddItem(c.viewsPanel, 0, 1, false).
		AddItem(c.keysPanel, 0, 1, false)

	// Content area swaps between the table and the in-place logs viewer.
	c.contentPages = tview.NewPages()
	c.contentPages.SetBackgroundColor(tcell.ColorDefault)
	c.contentPages.AddPage(tableContentPage, c.table, true, true)

	c.mainLayout = tview.NewFlex()
	c.mainLayout.SetDirection(tview.FlexRow)
	c.mainLayout.SetBackgroundColor(tcell.ColorDefault)
	c.mainLayout.SetBorder(false)
	c.mainLayout.AddItem(headerRow, 5, 0, false).
		AddItem(c.contentPages, 0, 1, true).
		AddItem(c.breadcrumbs, 1, 0, false)
}

// Package ui provides terminal UI components for building consistent
// terminal applications with a unified interface.
package ui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// initUI builds:
//
//	header: connection info | Views (0-9) | Actions (former logs column)
//	then table + breadcrumbs
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
	c.table.SetBorderColor(tcell.ColorAqua)
	c.table.Box.SetBackgroundColor(tcell.ColorDefault)
	c.table.Box.SetBorderAttributes(tcell.AttrNone)
	c.table.SetBorder(false)

	c.table.SetSelectedStyle(
		tcell.StyleDefault.
			Foreground(tcell.ColorWhite).
			Background(tcell.ColorDarkSlateGray).
			Attributes(tcell.AttrBold),
	)

	c.table.SetTitle(fmt.Sprintf(" [yellow]%s[white] ", c.title))
	c.table.SetTitleAlign(tview.AlignCenter)
	c.table.SetTitleColor(tcell.ColorYellow)

	c.tableContent = NewVirtualTableContent()
	c.table.SetContent(c.tableContent)
	c.table.SetFixed(1, 0)

	c.table.SetSelectionChangedFunc(func(row, column int) {
		if row <= 0 {
			return
		}
		if row-1 < len(c.tableData) {
			c.selectedRow = row - 1
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

	separator := tview.NewBox().
		SetBackgroundColor(tcell.ColorDefault).
		SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
			for i := 0; i < width; i++ {
				screen.SetContent(x+i, y, tcell.RuneHLine, nil, tcell.StyleDefault.Background(tcell.ColorDefault).Foreground(tcell.ColorAqua))
			}
			return x, y, width, height
		})

	c.mainLayout = tview.NewFlex()
	c.mainLayout.SetDirection(tview.FlexRow)
	c.mainLayout.SetBackgroundColor(tcell.ColorDefault)
	c.mainLayout.SetBorder(false)
	c.mainLayout.AddItem(headerRow, 5, 0, false).
		AddItem(separator, 1, 0, false).
		AddItem(c.table, 0, 1, true).
		AddItem(c.breadcrumbs, 1, 0, false)
}

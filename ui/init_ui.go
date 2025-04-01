package ui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// initUI initializes all UI components
func (c *Cores) initUI() {
	// Initialize breadcrumbs
	c.breadcrumbs = tview.NewTextView()
	c.breadcrumbs.SetDynamicColors(true)
	c.breadcrumbs.SetTextAlign(tview.AlignLeft)
	c.breadcrumbs.SetText(c.title)
	c.breadcrumbs.SetBackgroundColor(tcell.ColorBlack)
	c.breadcrumbs.SetBorder(false)

	// Info panel (left)
	c.infoPanel = tview.NewTextView()
	c.infoPanel.SetDynamicColors(true)
	c.infoPanel.SetTextAlign(tview.AlignLeft)
	c.infoPanel.SetText(fmt.Sprintf("[yellow]%s[white]\nStatus: Active", c.title))
	c.infoPanel.SetBackgroundColor(tcell.ColorBlack)
	c.infoPanel.SetBorder(false)

	// Help panel (middle)
	c.helpPanel = tview.NewTextView()
	c.helpPanel.SetDynamicColors(true)
	c.helpPanel.SetTextAlign(tview.AlignLeft)
	c.helpPanel.SetText(c.getHelpText())
	c.helpPanel.SetBackgroundColor(tcell.ColorBlack)
	c.helpPanel.SetBorder(false)

	// Log panel (right)
	c.logPanel = tview.NewTextView()
	c.logPanel.SetDynamicColors(true)
	c.logPanel.SetChangedFunc(func() {
		c.app.Draw()
	})
	c.logPanel.SetScrollable(true)
	c.logPanel.SetBackgroundColor(tcell.ColorBlack)
	c.logPanel.SetBorder(false)
	c.logPanel.SetText("[blue::b]INFO[white::-] Plugin initialized")

	// Table view with styling to match Redis plugin
	c.table = tview.NewTable()
	c.table.SetBorders(false)
	c.table.SetSelectable(true, false)
	c.table.SetBackgroundColor(tcell.ColorBlack)
	c.table.SetBorderColor(tcell.ColorAqua)
	c.table.Box.SetBackgroundColor(tcell.ColorBlack)
	c.table.Box.SetBorderAttributes(tcell.AttrNone)
	c.table.SetBorder(false) // Remove border to match Redis style

	// Set selection style to match Redis plugin
	c.table.SetSelectedStyle(
		tcell.StyleDefault.
			Foreground(tcell.ColorBlack).
			Background(tcell.ColorAqua).
			Attributes(tcell.AttrBold),
	)

	// Set a title that matches Redis plugin style
	c.table.SetTitle(fmt.Sprintf(" [yellow]%s[white] ", c.title))
	c.table.SetTitleAlign(tview.AlignCenter)
	c.table.SetTitleColor(tcell.ColorYellow)

	// Update selection whenever cursor moves to a new row
	c.table.SetSelectionChangedFunc(func(row, column int) {
		if row <= 0 { // Ignore header row
			return
		}
		// Update the selected row index without triggering the full selection event
		if row-1 < len(c.tableData) {
			c.selectedRow = row - 1 // Adjust for header row
		}
	})

	// Keep this for backwards compatibility with code that expects Enter to "confirm" selection
	c.table.SetSelectedFunc(func(row, column int) {
		if row <= 0 { // Ignore header row
			return
		}
		// This now just calls the full highlightRow function for visual emphasis
		// and triggers any registered callbacks
		c.selectRow(row - 1) // Adjust for header row
	})

	// Build a header row with no borders to match Redis style
	headerRow := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(c.infoPanel, 0, 1, false).
		AddItem(c.helpPanel, 0, 1, false).
		AddItem(c.logPanel, 0, 1, false)
	headerRow.SetBackgroundColor(tcell.ColorBlack)

	// Create separator like Redis plugin
	separator := tview.NewBox().
		SetBackgroundColor(tcell.ColorBlack).
		SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
			// Draw a horizontal line
			for i := 0; i < width; i++ {
				screen.SetContent(x+i, y, tcell.RuneHLine, nil, tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorAqua))
			}
			return x, y, width, height
		})

	// Create main layout with header, separator, table, and breadcrumbs at the bottom
	c.mainLayout = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(headerRow, 6, 0, false).
		AddItem(separator, 1, 0, false).
		AddItem(c.table, 0, 1, true).
		AddItem(c.breadcrumbs, 1, 0, false)
	c.mainLayout.SetBackgroundColor(tcell.ColorBlack)
	c.mainLayout.SetBorder(false)
}

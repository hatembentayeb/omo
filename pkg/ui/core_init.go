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

	applyTviewStyles()
}

var (
	HexAppBg         = defaultPalette.AppBg
	HexRow           = defaultPalette.Row
	HexHighlight     = defaultPalette.Highlight
	HexHighlightText = defaultPalette.HighlightText
	HexBorder        = defaultPalette.Border
	HexViewKey       = defaultPalette.ViewKey
	HexActionKey     = defaultPalette.ActionKey
	HexLabel         = defaultPalette.Label
	HexInfoKey       = defaultPalette.InfoKey
	HexValue         = defaultPalette.Value

	// ColorAppBg is the full-screen OMO background.
	ColorAppBg = tcell.GetColor(HexAppBg)
	// ColorTableRow is unselected table body and plugin-list text.
	ColorTableRow = tcell.GetColor(HexRow)
	// ColorHighlight is the selected-row / list highlight fill.
	ColorHighlight = tcell.GetColor(HexHighlight)
	// ColorHighlightText is selected-row / list highlight foreground.
	ColorHighlightText = tcell.GetColor(HexHighlightText)
	// ColorBorder is frames, column headers, and table titles.
	ColorBorder = tcell.GetColor(HexBorder)
)

// initUI builds:
//
//	header: connection info | Views | Actions | logo
//	then content (table or in-place logs)
//
// The host may detach the header and mount it full-width above the sidebar.
func (c *CoreView) initUI() {
	c.breadcrumbs = tview.NewTextView()
	c.breadcrumbs.SetDynamicColors(true)
	c.breadcrumbs.SetTextAlign(tview.AlignLeft)
	c.breadcrumbs.SetText(c.title)
	c.breadcrumbs.SetTextStyle(tcell.StyleDefault)
	c.breadcrumbs.SetBackgroundColor(ColorAppBg)
	c.breadcrumbs.SetBorder(false)

	c.infoPanel = tview.NewTextView()
	styleHeaderPanel(c.infoPanel)
	c.infoPanel.SetText(fmt.Sprintf("[%s::b]%s[%s]\nStatus: Active", HexInfoKey, c.title, HexValue))

	c.viewsPanel = tview.NewTextView()
	styleHeaderPanel(c.viewsPanel)
	c.viewsPanel.SetText(c.getViewsText())

	// Former logs column — expanded key shortcuts for the current view.
	c.keysPanel = tview.NewTextView()
	styleHeaderPanel(c.keysPanel)
	c.keysPanel.SetText(c.getKeysText())

	c.table = NewTable()
	c.table.SetBorders(false)
	c.table.SetSelectable(true, false)
	c.table.SetBackgroundColor(ColorAppBg)
	c.table.Box.SetBackgroundColor(ColorAppBg)
	c.table.SetBorder(false)
	c.table.SetBorderPadding(0, 0, 1, 1)

	c.table.SetSelectedStyle(
		tcell.StyleDefault.
			Foreground(ColorHighlightText).
			Background(ColorHighlight),
	)

	c.table.SetTitle(fmt.Sprintf(" [%s]%s[%s] ", HexBorder, c.title, HexValue))
	c.table.SetTitleAlign(tview.AlignCenter)
	c.table.SetTitleColor(ColorBorder)

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

	c.logoSlot = tview.NewFlex()
	c.logoSlot.SetDirection(tview.FlexRow)
	c.logoSlot.SetBackgroundColor(ColorAppBg)

	c.headerRow = tview.NewFlex()
	c.headerRow.SetDirection(tview.FlexColumn)
	c.headerRow.SetBackgroundColor(ColorAppBg)
	c.headerRow.AddItem(c.infoPanel, 0, 1, false).
		AddItem(headerSpacer(), headerColGap, 0, false).
		AddItem(c.viewsPanel, 0, 1, false).
		AddItem(headerSpacer(), headerColGap, 0, false).
		AddItem(c.keysPanel, 0, 1, false).
		AddItem(headerSpacer(), headerColGap, 0, false).
		AddItem(c.logoSlot, HeaderLogoWidth, 0, false)

	// Content area swaps between the table and the in-place logs viewer.
	c.contentPages = tview.NewPages()
	c.contentPages.SetBackgroundColor(ColorAppBg)
	c.contentPages.AddPage(tableContentPage, c.table, true, true)

	c.mainLayout = tview.NewFlex()
	c.mainLayout.SetDirection(tview.FlexRow)
	c.mainLayout.SetBackgroundColor(ColorAppBg)
	c.mainLayout.SetBorder(false)
	c.mainLayout.AddItem(c.headerRow, 5, 0, false).
		AddItem(c.contentPages, 0, 1, true)
}

// HeaderLogoWidth is the fixed right-side slot for the OMO mark.
const HeaderLogoWidth = 16

func styleHeaderPanel(tv *tview.TextView) {
	tv.SetDynamicColors(true)
	tv.SetTextAlign(tview.AlignLeft)
	tv.SetWrap(false)
	tv.SetWordWrap(false)
	tv.SetScrollable(false)
	tv.SetBackgroundColor(ColorAppBg)
	tv.SetBorder(false)
}

func headerSpacer() tview.Primitive {
	b := tview.NewBox()
	b.SetBackgroundColor(ColorAppBg)
	return b
}

// Header returns the Info | Views | Actions | Logo row.
func (c *CoreView) Header() tview.Primitive {
	return c.headerRow
}

// DetachHeader lifts the header out of the stacked layout so the host can
// mount it full-width. GetLayout() then is table/content only.
func (c *CoreView) DetachHeader() tview.Primitive {
	if c != nil && c.mainLayout != nil && c.headerRow != nil {
		c.mainLayout.RemoveItem(c.headerRow)
	}
	return c.headerRow
}

// SetHeaderLogo places a primitive at the right of the plugin header,
// centered in the logo slot.
func (c *CoreView) SetHeaderLogo(p tview.Primitive) {
	if c == nil || c.logoSlot == nil {
		return
	}
	c.ClearHeaderLogo()
	if p == nil {
		return
	}
	c.headerLogo = p
	c.logoSlot.AddItem(headerSpacer(), 0, 1, false).
		AddItem(p, 2, 0, false).
		AddItem(headerSpacer(), 0, 1, false)
}

// ClearHeaderLogo removes the header logo slot contents.
func (c *CoreView) ClearHeaderLogo() {
	if c == nil || c.logoSlot == nil {
		return
	}
	c.logoSlot.Clear()
	c.headerLogo = nil
}

// SetBreadcrumbHook is called whenever the view stack text changes (host footer).
func (c *CoreView) SetBreadcrumbHook(fn func(string)) *CoreView {
	c.onCrumbs = fn
	c.updateBreadcrumbs()
	return c
}

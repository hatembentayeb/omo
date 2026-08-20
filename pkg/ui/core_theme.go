package ui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
)

// ApplyTheme restyles this CoreView after a palette change.
func (c *CoreView) ApplyTheme() {
	if c == nil {
		return
	}
	if c.breadcrumbs != nil {
		c.breadcrumbs.SetBackgroundColor(ColorAppBg)
		c.updateBreadcrumbs()
	}
	if c.infoPanel != nil {
		styleHeaderPanel(c.infoPanel)
	}
	if c.viewsPanel != nil {
		styleHeaderPanel(c.viewsPanel)
		c.viewsPanel.SetText(c.getViewsText())
	}
	if c.keysPanel != nil {
		styleHeaderPanel(c.keysPanel)
		c.keysPanel.SetText(c.getKeysText())
	}
	if c.headerRow != nil {
		c.headerRow.SetBackgroundColor(ColorAppBg)
	}
	if c.logoSlot != nil {
		c.logoSlot.SetBackgroundColor(ColorAppBg)
	}
	if c.contentPages != nil {
		c.contentPages.SetBackgroundColor(ColorAppBg)
	}
	if c.mainLayout != nil {
		c.mainLayout.SetBackgroundColor(ColorAppBg)
	}
	if c.table != nil {
		c.table.SetBackgroundColor(ColorAppBg)
		c.table.Box.SetBackgroundColor(ColorAppBg)
		c.table.SetSelectedStyle(tcell.StyleDefault.
			Foreground(ColorHighlightText).
			Background(ColorHighlight))
		c.table.SetTitle(fmt.Sprintf(" [%s]%s[%s] ", HexBorder, c.title, HexValue))
		c.table.SetTitleColor(ColorBorder)
		c.refreshTable()
	}
}

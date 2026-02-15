package ui

import (
	"strings"

	"github.com/rivo/tview"
)

// SetModalPages sets the pages container used for modals.
func (c *CoreView) SetModalPages(pages *tview.Pages) *CoreView {
	c.pages = pages
	return c
}

// ClearFilter removes any active filter.
func (c *CoreView) ClearFilter() *CoreView {
	return c.SetFilterQuery("")
}

// IsFiltered returns true if a filter is currently active.
func (c *CoreView) IsFiltered() bool {
	return c.filterQuery != ""
}

// GetFilterQuery returns the current filter query.
func (c *CoreView) GetFilterQuery() string {
	return c.filterQuery
}

func (c *CoreView) showFilterModal() {
	if c.pages == nil {
		c.Log("[red]Filter unavailable")
		return
	}

	ShowCompactStyledInputModal(
		c.pages,
		c.app,
		"Filter Rows",
		"Query",
		c.filterQuery,
		42,
		nil,
		func(text string, cancelled bool) {
			if cancelled {
				c.app.SetFocus(c.table)
				return
			}
			c.SetFilterQuery(strings.TrimSpace(text))
			c.app.SetFocus(c.table)
		},
	)
}

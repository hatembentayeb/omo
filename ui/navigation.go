package ui

import (
	"fmt"
	"strings"
)

// updateBreadcrumbs updates the breadcrumb display based on the current navigation stack
func (c *Cores) updateBreadcrumbs() {
	if len(c.navStack) == 0 {
		c.breadcrumbs.SetText("")
		return
	}

	var sb strings.Builder
	for i, view := range c.navStack {
		if i > 0 {
			sb.WriteString(" [yellow]>[white] ")
		}
		if i == len(c.navStack)-1 {
			// Current view in orange
			sb.WriteString(fmt.Sprintf("[black:orange]%s[-:-]", view))
		} else {
			// Previous views in aqua
			sb.WriteString(fmt.Sprintf("[black:aqua]%s[-:-]", view))
		}
	}
	c.breadcrumbs.SetText(sb.String())
}

// PushView adds a view to the navigation stack
func (c *Cores) PushView(view string) {
	c.navStack = append(c.navStack, view)
	c.updateBreadcrumbs()
}

// PopView removes the last view from the navigation stack
func (c *Cores) PopView() string {
	if len(c.navStack) <= 1 {
		// Don't pop if we're at root view or have no views
		return ""
	}
	lastView := c.navStack[len(c.navStack)-1]
	c.navStack = c.navStack[:len(c.navStack)-1]
	c.updateBreadcrumbs()
	return lastView
}

// ClearViews removes all views from the navigation stack except the root view
func (c *Cores) ClearViews() {
	if len(c.navStack) > 0 {
		// Keep only the root view if it exists
		c.navStack = c.navStack[:1]
	} else {
		c.navStack = []string{}
	}
	c.updateBreadcrumbs()
}

// GetCurrentView returns the name of the current view
func (c *Cores) GetCurrentView() string {
	if len(c.navStack) == 0 {
		return ""
	}
	return c.navStack[len(c.navStack)-1]
}

// SetViewStack sets the entire navigation stack at once
func (c *Cores) SetViewStack(stack []string) {
	if len(stack) > 0 {
		c.navStack = append([]string{}, stack...)
	} else {
		c.navStack = []string{}
	}
	c.updateBreadcrumbs()
}

// CopyNavigationStackFrom copies the navigation stack from another Cores instance
func (c *Cores) CopyNavigationStackFrom(other *Cores) {
	if len(other.navStack) > 0 {
		c.navStack = append([]string{}, other.navStack...)
	} else {
		c.navStack = []string{}
	}
	c.updateBreadcrumbs()
}

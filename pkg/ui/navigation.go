// Package ui provides terminal UI components for building consistent
// terminal applications with a unified interface.
package ui

import (
	"fmt"
	"strings"
)

// updateBreadcrumbs updates the breadcrumb display based on the current navigation stack.
// This function formats and renders the breadcrumb trail that shows the user's
// current position in the navigation hierarchy. It visually distinguishes
// the current view from previous views using different colors.
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

// PushView adds a view to the navigation stack.
// This function registers a new view in the navigation history and
// updates the breadcrumb display to reflect the new navigation state.
//
// Parameters:
//   - view: The name of the view to add to the navigation stack
func (c *Cores) PushView(view string) {
	c.navStack = append(c.navStack, view)
	c.updateBreadcrumbs()
}

// PopView removes the last view from the navigation stack.
// This function simulates navigating back to the previous view by
// removing the current view from the stack and updating the breadcrumbs.
// It will not remove the root view (first item in the stack).
//
// Returns:
//   - The name of the view that was removed, or empty string if no view was removed
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

// ClearViews removes all views from the navigation stack except the root view.
// This function resets the navigation history while preserving the root view,
// effectively returning to the starting point of the navigation.
func (c *Cores) ClearViews() {
	if len(c.navStack) > 0 {
		// Keep only the root view if it exists
		c.navStack = c.navStack[:1]
	} else {
		c.navStack = []string{}
	}
	c.updateBreadcrumbs()
}

// GetCurrentView returns the name of the current view.
// This function provides access to the name of the currently active view,
// which is the last item in the navigation stack.
//
// Returns:
//   - The name of the current view, or empty string if the stack is empty
func (c *Cores) GetCurrentView() string {
	if len(c.navStack) == 0 {
		return ""
	}
	return c.navStack[len(c.navStack)-1]
}

// SetViewStack sets the entire navigation stack at once.
// This function replaces the current navigation stack with a new one,
// allowing for complete control over the navigation history.
//
// Parameters:
//   - stack: The new navigation stack to use
func (c *Cores) SetViewStack(stack []string) {
	if len(stack) > 0 {
		c.navStack = append([]string{}, stack...)
	} else {
		c.navStack = []string{}
	}
	c.updateBreadcrumbs()
}

// CopyNavigationStackFrom copies the navigation stack from another Cores instance.
// This function is useful when transitioning between related views or when
// creating a new view that should inherit the navigation context of another view.
//
// Parameters:
//   - other: The Cores instance to copy the navigation stack from
func (c *Cores) CopyNavigationStackFrom(other *Cores) {
	if len(other.navStack) > 0 {
		c.navStack = append([]string{}, other.navStack...)
	} else {
		c.navStack = []string{}
	}
	c.updateBreadcrumbs()
}

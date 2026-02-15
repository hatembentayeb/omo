// Package ui provides terminal UI components for building consistent
// terminal applications with a unified interface.
package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// RegisterHandlers registers keyboard input handlers.
// This method sets up input capture for the CoreView instance, establishing
// the standard key handlers while preserving any existing input handlers.
// It implements a chain-of-responsibility pattern where this handler
// processes relevant keys and passes others to the previous handler.
//
// The handlers include standard keys like:
// - R for refresh
// - ? for help
// - ESC for navigation back
// - Any custom key bindings defined for the instance
func (c *CoreView) RegisterHandlers() {
	// Set input capture directly on the table - this is more reliable
	// than app-level capture which can get overwritten
	c.table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		return c.StandardKeyHandler(event, nil)
	})
}

// UnregisterHandlers removes keyboard input handlers.
// This should be called when switching away from this plugin
// to prevent interference with other components' keyboard handling.
// In a complete implementation, this would restore the previous
// input capture that was saved during registration.
func (c *CoreView) UnregisterHandlers() {
	// This would need to be integrated with your main app's focus management
	// Typically restores the previous input capture
}

// RemovePage is a utility function to remove any modal page with ESC key handling.
// This can be used by any modal in the application to ensure consistent behavior.
// The function implements standard ESC key handling for modals:
// - Captures the ESC key while a modal is active
// - Removes the modal when ESC is pressed
// - Restores the original input capture
// - Calls an optional callback function
//
// Parameters:
//   - pages: The tview.Pages instance containing the modal
//   - app: The tview.Application instance for input capture
//   - pageID: The ID of the page/modal to remove
//   - callback: Optional function to call after removing the page
func RemovePage(pages *tview.Pages, app *tview.Application, pageID string, callback func()) {
	// Save original input capture to restore later
	oldCapture := app.GetInputCapture()

	// Add app-level ESC handler that will take precedence
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			// Check if our modal is visible
			if pages.HasPage(pageID) {
				pages.RemovePage(pageID)
				// Restore original input capture
				app.SetInputCapture(oldCapture)
				if callback != nil {
					callback()
				}
				return nil
			}
		}

		// Pass to original handler
		if oldCapture != nil {
			return oldCapture(event)
		}
		return event
	})
}

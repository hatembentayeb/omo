package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// RegisterHandlers registers keyboard input handlers
func (c *Cores) RegisterHandlers() {
	// Save the current input capture if there is one
	oldCapture := c.app.GetInputCapture()

	// Set up a new input capture that handles our keys and passes through others
	c.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Use the standardized key handler
		return c.StandardKeyHandler(event, oldCapture)
	})
}

// UnregisterHandlers removes keyboard input handlers
// This should be called when switching away from this plugin
func (c *Cores) UnregisterHandlers() {
	// This would need to be integrated with your main app's focus management
	// Typically restores the previous input capture
}

// RemovePage is a utility function to remove any modal page with ESC key handling
// This can be used by any modal in the application to ensure consistent behavior
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

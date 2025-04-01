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
		// Handle key events for this plugin
		switch event.Key() {
		case tcell.KeyEscape:
			// Handle back navigation using the stack
			if len(c.navStack) > 1 { // We have more than just the root view
				// Get the current view before popping
				currentView := c.GetCurrentView()
				previousView := c.navStack[len(c.navStack)-2] // Get previous view without popping

				// Only pop if we're not at the root view
				c.PopView()

				if c.onAction != nil {
					// First trigger the back action
					c.onAction("back", map[string]interface{}{
						"from": currentView,
						"to":   previousView,
					})

					// Then trigger navigation to the previous view
					c.onAction("navigate_back", map[string]interface{}{
						"current_view": previousView,
					})
				}
				return nil
			}

			// If at root view or no views in stack, pass to original handler
			if oldCapture != nil {
				return oldCapture(event)
			}
			return event

		case tcell.KeyRune:
			switch event.Rune() {
			case 'R', 'r':
				c.RefreshData()
				return nil
			case '?':
				// First check if there's a custom handler for the '?' key
				if c.onAction != nil {
					handled := c.onAction("keypress", map[string]interface{}{
						"key": "?",
					})
					// If the custom handler returns non-nil, it means it handled the key
					if handled == nil {
						return nil
					}
				}

				// Default behavior - toggle help expanded/collapsed
				if c.helpPanel.GetTitle() == "Keybindings" {
					c.helpPanel.SetTitle("Keybindings (expanded)")
				} else {
					c.helpPanel.SetTitle("Keybindings")
				}
				return nil
			default:
				// Check for custom key bindings
				key := string(event.Rune())
				if _, exists := c.keyBindings[key]; exists {
					if c.onAction != nil {
						c.onAction("keypress", map[string]interface{}{
							"key": key,
						})
					}
					return nil
				}
			}
		}

		// Pass through to previous handler if we didn't handle it
		if oldCapture != nil {
			return oldCapture(event)
		}
		return event
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

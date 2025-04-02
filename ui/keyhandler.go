package ui

import (
	"github.com/gdamore/tcell/v2"
)

// KeyAction represents a key binding and its associated action
type KeyAction struct {
	Key         string
	Description string
	Callback    func()
}

// StandardKeyHandler provides a consistent way to register and handle keys
func (c *Cores) StandardKeyHandler(event *tcell.EventKey, oldCapture func(*tcell.EventKey) *tcell.EventKey) *tcell.EventKey {
	// Handle standard navigation keys
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
		// Handle standard key bindings
		switch event.Rune() {
		case 'R':
			c.RefreshData()
			return nil
		case '?':
			// First check if there's a custom handler for the '?' key
			if c.onAction != nil {
				handled := c.onAction("keypress", map[string]interface{}{
					"key": "?",
				})
				// If the custom handler returns nil, it means it handled the key
				if handled == nil {
					return nil
				}
			}

			// Default behavior - toggle help expanded/collapsed
			c.ToggleHelpExpanded()
			return nil
		default:
			// Check for registered key bindings
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
}

// RegisterStandardKeys adds common keybindings that should be consistent across plugins
func (c *Cores) RegisterStandardKeys() {
	c.keyBindings["R"] = "Refresh"
	c.keyBindings["?"] = "Help"
	c.keyBindings["ESC"] = "Back"
}

// ToggleHelpExpanded toggles the help panel between expanded and compact states
func (c *Cores) ToggleHelpExpanded() {
	if c.helpPanel.GetTitle() == "Keybindings" {
		c.helpPanel.SetTitle("Keybindings (expanded)")
		c.helpPanel.SetText(c.getExpandedHelpText())
	} else {
		c.helpPanel.SetTitle("Keybindings")
		c.helpPanel.SetText(c.getHelpText())
	}
}

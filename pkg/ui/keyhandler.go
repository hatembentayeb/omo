// Package ui provides terminal UI components for building consistent
// terminal applications with a unified interface.
package ui

import (
	"github.com/gdamore/tcell/v2"
)

// KeyAction represents a key binding and its associated action.
// This struct encapsulates everything needed to define and handle
// keyboard shortcuts in the UI, including the key, a human-readable
// description, and the callback function to execute when the key is pressed.
type KeyAction struct {
	Key         string // The key or key combination (e.g., "R", "Ctrl+C")
	Description string // Human-readable description of the action
	Callback    func() // Function to call when the key is pressed
}

// StandardKeyHandler provides a standardized approach to handling key events
// across all plugins. It implements common key actions like:
// - R for refreshing data
// - ? for toggling help view
// - ESC for navigation back
//
// This function follows the tview input capture pattern, where handlers can be chained.
// It processes key events, handles standard keys, and forwards unhandled events to the
// previous handler.
//
// Parameters:
//   - event: The key event to process
//   - oldCapture: The previous input capture function in the chain
//
// Returns:
//   - The event for the next handler, or nil if handled
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

	case tcell.KeyPgDn:
		if c.lazyLoader != nil {
			c.LoadMore()
			return nil
		}
	case tcell.KeyRune:
		// Handle standard key bindings
		switch event.Rune() {
		case 'R':
			// First check if there's a direct handler registered
			if handler, ok := c.keyHandlers["R"]; ok && handler != nil {
				handler()
				return nil
			}
			// Then check onAction callback
			if c.onAction != nil {
				handled := c.onAction("keypress", map[string]interface{}{
					"key": "R",
				})
				if handled == nil {
					return nil
				}
			}
			// Default behavior - refresh data
			c.RefreshData()
			return nil
		case '/':
			// First check if there's a direct handler registered
			if handler, ok := c.keyHandlers["/"]; ok && handler != nil {
				handler()
				return nil
			}
			if c.onAction != nil {
				handled := c.onAction("keypress", map[string]interface{}{
					"key": "/",
				})
				if handled == nil {
					return nil
				}
			}
			c.showFilterModal()
			return nil
		case '?':
			// First check if there's a direct handler registered
			if handler, ok := c.keyHandlers["?"]; ok && handler != nil {
				handler()
				return nil
			}
			// Then check onAction callback
			if c.onAction != nil {
				handled := c.onAction("keypress", map[string]interface{}{
					"key": "?",
				})
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
				// Call direct handler if registered
				if handler, ok := c.keyHandlers[key]; ok && handler != nil {
					handler()
					return nil
				}
				// Fall back to onAction callback
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

// RegisterStandardKeys registers the common keybindings that should be
// consistent across all plugins (R for refresh, ? for help, ESC for back).
// This ensures a consistent user experience across different plugins by
// maintaining the same basic key commands in every view.
func (c *Cores) RegisterStandardKeys() {
	c.keyBindings["R"] = "Refresh"
	c.keyBindings["?"] = "Help"
	c.keyBindings["ESC"] = "Back"
}

// ToggleHelpExpanded switches between basic and expanded help views
// to provide consistent help functionality across plugins.
// This function alternates between a compact help display showing
// just the key bindings, and an expanded view with more detailed
// information about each command.
func (c *Cores) ToggleHelpExpanded() {
	if c.helpPanel.GetTitle() == "Keybindings" {
		c.helpPanel.SetTitle("Keybindings (expanded)")
		c.helpPanel.SetText(c.getExpandedHelpText())
	} else {
		c.helpPanel.SetTitle("Keybindings")
		c.helpPanel.SetText(c.getHelpText())
	}
}

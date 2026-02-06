package ui

import (
	"github.com/gdamore/tcell/v2"
)

// KeyAction represents a key binding and its associated action.
type KeyAction struct {
	Key         string // The key or key combination (e.g., "R", "Ctrl+C")
	Description string // Human-readable description of the action
	Callback    func() // Function to call when the key is pressed
}

// StandardKeyHandler processes key events using a single dispatch path:
//
//  1. Direct handler (keyHandlers) — highest priority
//  2. Action callback (onAction "keypress") — plugin-level routing
//  3. Built-in default — only for standard keys (R=refresh, ?=help, /=filter)
//
// Non-rune keys (ESC, PgDn) are handled separately for navigation.
func (c *CoreView) StandardKeyHandler(event *tcell.EventKey, oldCapture func(*tcell.EventKey) *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEscape:
		return c.handleEscape(event, oldCapture)

	case tcell.KeyPgDn:
		if c.lazyLoader != nil {
			c.LoadMore()
			return nil
		}

	case tcell.KeyRune:
		key := string(event.Rune())

		// Only dispatch keys that are registered in keyBindings.
		if _, registered := c.keyBindings[key]; !registered {
			break
		}

		// Priority 1: direct handler function
		if handler, ok := c.keyHandlers[key]; ok && handler != nil {
			handler()
			return nil
		}

		// Priority 2: action callback (lets plugin handle it)
		if c.onAction != nil {
			err := c.onAction("keypress", map[string]interface{}{
				"key": key,
			})
			if err == nil {
				return nil
			}
		}

		// Priority 3: built-in defaults for standard keys
		switch key {
		case "R":
			c.RefreshData()
		case "/":
			c.showFilterModal()
		case "?":
			c.ToggleHelpExpanded()
		}
		return nil
	}

	if oldCapture != nil {
		return oldCapture(event)
	}
	return event
}

// handleEscape processes the ESC key for back-navigation through the view stack.
func (c *CoreView) handleEscape(event *tcell.EventKey, oldCapture func(*tcell.EventKey) *tcell.EventKey) *tcell.EventKey {
	if len(c.navStack) > 1 {
		currentView := c.GetCurrentView()
		previousView := c.navStack[len(c.navStack)-2]

		c.PopView()

		if c.onAction != nil {
			c.onAction("back", map[string]interface{}{
				"from": currentView,
				"to":   previousView,
			})
			c.onAction("navigate_back", map[string]interface{}{
				"current_view": previousView,
			})
		}
		return nil
	}

	if oldCapture != nil {
		return oldCapture(event)
	}
	return event
}

// ToggleHelpExpanded switches between basic and expanded help views.
func (c *CoreView) ToggleHelpExpanded() {
	if c.helpPanel.GetTitle() == "Keybindings" {
		c.helpPanel.SetTitle("Keybindings (expanded)")
		c.helpPanel.SetText(c.getExpandedHelpText())
	} else {
		c.helpPanel.SetTitle("Keybindings")
		c.helpPanel.SetText(c.getHelpText())
	}
}

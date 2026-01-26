package ui

// AddKeyBinding adds a custom key binding with description and handler
func (c *Cores) AddKeyBinding(key string, description string, handler func()) *Cores {
	// Add to the key bindings for help text
	c.keyBindings[key] = description

	// Store the handler for direct invocation
	if handler != nil {
		c.keyHandlers[key] = handler
	}

	// Update help text immediately
	c.helpPanel.SetText(c.getHelpText())

	return c
}

// ClearKeyBindings removes all custom key bindings while preserving standard ones
func (c *Cores) ClearKeyBindings() *Cores {
	// Create new maps with just the standard bindings
	standardBindings := make(map[string]string)
	standardHandlers := make(map[string]func())

	// Preserve standard key bindings if they exist
	standardKeys := []string{"R", "?", "ESC", "/", "PgDn"}
	for _, key := range standardKeys {
		if desc, exists := c.keyBindings[key]; exists {
			standardBindings[key] = desc
		}
		// Note: standard keys don't have handlers stored, they use defaults
	}

	// Replace the key bindings with just the standard ones
	c.keyBindings = standardBindings
	c.keyHandlers = standardHandlers

	// Update help text immediately to reflect the updated bindings
	c.helpPanel.SetText(c.getHelpText())

	return c
}

// SetActionCallback sets a function to be called for various plugin actions
func (c *Cores) SetActionCallback(callback func(action string, payload map[string]interface{}) error) *Cores {
	c.onAction = callback
	return c
}

// SetRowSelectedCallback sets a function to be called when a row is selected
func (c *Cores) SetRowSelectedCallback(callback func(row int)) *Cores {
	c.onRowSelected = callback
	return c
}

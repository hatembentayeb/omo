package ui

// AddKeyBinding adds a custom key binding with description
func (c *Cores) AddKeyBinding(key string, description string, handler func()) *Cores {
	// Add to the key bindings for help text
	c.keyBindings[key] = description

	// Update help text immediately
	c.helpPanel.SetText(c.getHelpText())

	// The actual handler will be called via the onAction callback
	// which is triggered through the RegisterHandlers method

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

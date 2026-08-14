package ui

// AddKeyBinding adds a shortcut to the right "Keys" column (former logs) and registers its handler.
func (c *CoreView) AddKeyBinding(key string, description string, handler func()) *CoreView {
	if c.keyBindings == nil {
		c.keyBindings = make(map[string]string)
	}
	if c.keyHandlers == nil {
		c.keyHandlers = make(map[string]func())
	}
	c.keyBindings[key] = description
	if handler != nil {
		c.keyHandlers[key] = handler
	}
	c.refreshHeaderPanels()
	return c
}

// AddViewBinding adds a view switch to the middle "Views" column (0-9).
// viewID is used to highlight the active view (e.g. "keys").
func (c *CoreView) AddViewBinding(key, description, viewID string, handler func()) *CoreView {
	if c.viewBindings == nil {
		c.viewBindings = make(map[string]string)
	}
	if c.viewBindingIDs == nil {
		c.viewBindingIDs = make(map[string]string)
	}
	if c.keyHandlers == nil {
		c.keyHandlers = make(map[string]func())
	}
	c.viewBindings[key] = description
	if viewID != "" {
		c.viewBindingIDs[key] = viewID
	}
	if handler != nil {
		c.keyHandlers[key] = handler
	}
	c.refreshHeaderPanels()
	return c
}

// SetActiveView highlights the matching entry in the Views column.
func (c *CoreView) SetActiveView(viewID string) *CoreView {
	c.activeViewID = viewID
	c.refreshHeaderPanels()
	return c
}

// BindKey registers a key handler without listing it in either header column.
func (c *CoreView) BindKey(key string, handler func()) *CoreView {
	if c.keyHandlers == nil {
		c.keyHandlers = make(map[string]func())
	}
	if handler != nil {
		c.keyHandlers[key] = handler
	}
	return c
}

// ClearKeyBindings clears view + key columns while preserving standard globals in Keys.
func (c *CoreView) ClearKeyBindings() *CoreView {
	standardBindings := make(map[string]string)
	for _, key := range []string{"R", "?", "ESC", "/", "PgDn", "^t"} {
		if desc, exists := c.keyBindings[key]; exists {
			standardBindings[key] = desc
		}
	}
	c.keyBindings = standardBindings
	c.viewBindings = make(map[string]string)
	c.viewBindingIDs = make(map[string]string)
	c.keyHandlers = make(map[string]func())
	c.refreshHeaderPanels()
	return c
}

// SetActionCallback sets a function to be called for various plugin actions
func (c *CoreView) SetActionCallback(callback func(action string, payload map[string]interface{}) error) *CoreView {
	c.onAction = callback
	return c
}

// SetRowSelectedCallback sets a function to be called when a row is selected
func (c *CoreView) SetRowSelectedCallback(callback func(row int)) *CoreView {
	c.onRowSelected = callback
	return c
}

// SetHighlightChangedCallback is called when the highlighted table row changes
// (arrow keys), without requiring Enter.
func (c *CoreView) SetHighlightChangedCallback(callback func(row int)) *CoreView {
	c.onHighlightChanged = callback
	return c
}

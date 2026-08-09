package ui

import (
	"fmt"
	"sort"
	"strings"
)

// SetInfoText updates the content of the info panel
func (c *CoreView) SetInfoText(text string) *CoreView {
	c.infoPanel.SetText(text)
	return c
}

// SetInfoTitle updates the title of the info panel
func (c *CoreView) SetInfoTitle(title string) *CoreView {
	c.infoPanel.SetTitle(title)
	return c
}

// Log is retained for callers; the on-screen logs panel was removed.
// Messages are discarded (host file logging remains elsewhere).
func (c *CoreView) Log(message string) *CoreView {
	_ = message
	return c
}

// ClearLogs is a no-op; the on-screen logs panel was removed.
func (c *CoreView) ClearLogs() *CoreView {
	return c
}

// SetInfoMap updates the info panel with a map of key-value pairs
// Keys will be shown in aqua color, values in white, both in bold
func (c *CoreView) SetInfoMap(infoMap map[string]string) *CoreView {
	// Build a stylized string from the map
	var sb strings.Builder

	// Sort the keys for consistent display order
	keys := make([]string, 0, len(infoMap))
	for key := range infoMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Find the longest key to align columns
	maxKeyLength := 0
	for _, key := range keys {
		if len(key) > maxKeyLength && infoMap[key] != "" {
			maxKeyLength = len(key)
		}
	}

	// Format each key-value pair with aligned columns
	for i, key := range keys {
		value := infoMap[key]
		if key != "" && value != "" {
			// Calculate padding: longest word + 1 space - current word length
			// This ensures exactly one space after the longest word
			paddingSpaces := maxKeyLength + 1 - len(key)
			padding := strings.Repeat(" ", paddingSpaces)

			sb.WriteString(fmt.Sprintf("[aqua::b]%s:%s[white::b]%s", key, padding, value))

			// Add newline for all but the last item
			if i < len(keys)-1 {
				sb.WriteString("\n")
			}
		}
	}

	// Set the formatted text to the info panel
	c.infoPanel.SetText(sb.String())
	return c
}

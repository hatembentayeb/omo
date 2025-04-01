package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// SetInfoText updates the content of the info panel
func (c *Cores) SetInfoText(text string) *Cores {
	c.infoPanel.SetText(text)
	return c
}

// SetInfoTitle updates the title of the info panel
func (c *Cores) SetInfoTitle(title string) *Cores {
	c.infoPanel.SetTitle(title)
	return c
}

// Log adds a new message to the log panel
func (c *Cores) Log(message string) *Cores {
	timestamp := time.Now().Format("15:04:05")
	content := c.logPanel.GetText(false)
	if content != "" {
		content += "\n"
	}

	// Format message in Redis style with gray timestamp
	content += fmt.Sprintf("[gray::d]%s[-] %s", timestamp, message)

	c.logPanel.SetText(content)
	c.logPanel.ScrollToEnd()
	return c
}

// ClearLogs clears all log messages
func (c *Cores) ClearLogs() *Cores {
	c.logPanel.SetText("")
	return c
}

// SetInfoMap updates the info panel with a map of key-value pairs
// Keys will be shown in aqua color, values in white, both in bold
func (c *Cores) SetInfoMap(infoMap map[string]string) *Cores {
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

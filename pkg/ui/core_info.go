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

// SetInfoMap updates the info panel with a map of key-value pairs (Label: value).
func (c *CoreView) SetInfoMap(infoMap map[string]string) *CoreView {
	keys := make([]string, 0, len(infoMap))
	for key := range infoMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	width := 0
	for _, key := range keys {
		if infoMap[key] == "" || key == "" {
			continue
		}
		if len(key) > width {
			width = len(key)
		}
	}

	var sb strings.Builder
	for _, key := range keys {
		value := infoMap[key]
		if key == "" || value == "" {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(fmt.Sprintf("%-*s : %s", width, key, value))
	}
	// Caller/host typically colorizes; keep readable plain tags for standalone use.
	c.infoPanel.SetText(sb.String())
	return c
}

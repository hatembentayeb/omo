package ui

import (
	"fmt"
	"sort"
	"strings"
)

type keyBinding struct {
	key         string
	description string
}

func buildSortedBindings(keyBindings map[string]string) []keyBinding {
	bindings := make([]keyBinding, 0, len(keyBindings))
	for key, description := range keyBindings {
		formattedDesc := ""
		if len(description) > 0 {
			formattedDesc = strings.ToUpper(description[:1])
			if len(description) > 1 {
				formattedDesc += strings.ToLower(description[1:])
			}
		}
		bindings = append(bindings, keyBinding{key, formattedDesc})
	}

	sort.Slice(bindings, func(i, j int) bool {
		isDigitI := len(bindings[i].key) == 1 && bindings[i].key[0] >= '0' && bindings[i].key[0] <= '9'
		isDigitJ := len(bindings[j].key) == 1 && bindings[j].key[0] >= '0' && bindings[j].key[0] <= '9'
		if isDigitI != isDigitJ {
			return isDigitI
		}
		if isDigitI && isDigitJ {
			return bindings[i].key < bindings[j].key
		}
		isSpecialI := len(bindings[i].key) > 1 || strings.ContainsAny(bindings[i].key, "^_")
		isSpecialJ := len(bindings[j].key) > 1 || strings.ContainsAny(bindings[j].key, "^_")
		if isSpecialI != isSpecialJ {
			return !isSpecialI
		}
		return bindings[i].key < bindings[j].key
	})
	return bindings
}

// formatBindingsColumns lays out bindings in a dense multi-column matrix.
func formatBindingsColumns(bindings []keyBinding, maxRows, maxCols int) string {
	if len(bindings) == 0 {
		return "[gray](none for this view)[white]"
	}
	if maxRows < 1 {
		maxRows = 1
	}
	if maxCols < 1 {
		maxCols = 1
	}

	longestDesc := 0
	for _, b := range bindings {
		if len(b.description) > longestDesc {
			longestDesc = len(b.description)
		}
	}

	numCols := (len(bindings) + maxRows - 1) / maxRows
	if numCols > maxCols {
		numCols = maxCols
	}
	rows := maxRows
	if needed := (len(bindings) + numCols - 1) / numCols; needed < rows {
		rows = needed
	}
	if rows < 1 {
		rows = 1
	}

	lines := make([]string, rows)
	for i, binding := range bindings {
		col := i / rows
		row := i % rows
		if col >= maxCols {
			break
		}
		paddingSpaces := longestDesc - len(binding.description) + 1
		if paddingSpaces < 1 {
			paddingSpaces = 1
		}
		padding := strings.Repeat(" ", paddingSpaces)
		cell := fmt.Sprintf("[purple::b]<%s>[white::-] %s%s", binding.key, binding.description, padding)
		if lines[row] != "" {
			lines[row] += cell
		} else {
			lines[row] = cell
		}
	}
	return strings.Join(lines, "\n")
}

// getViewsText renders the middle header column — view switches only (0-9).
func (c *CoreView) getViewsText() string {
	bindings := buildSortedBindings(c.viewBindings)
	if len(bindings) == 0 {
		return "[gray]0-9 switch views[white]"
	}

	format := func(b keyBinding) string {
		viewID := c.viewBindingIDs[b.key]
		if viewID != "" && viewID == c.activeViewID {
			return fmt.Sprintf("[green::b]<%s>[white::-] %-9s", b.key, b.description)
		}
		return fmt.Sprintf("[purple::b]<%s>[white::-] %-9s", b.key, b.description)
	}

	var sb strings.Builder
	// Prefer a single tall column when few; 2 columns for 0-9.
	if len(bindings) <= 6 {
		for _, b := range bindings {
			sb.WriteString(format(b))
			sb.WriteString("\n")
		}
		return strings.TrimRight(sb.String(), "\n")
	}

	rows := (len(bindings) + 1) / 2
	for row := 0; row < rows; row++ {
		sb.WriteString(format(bindings[row]))
		if row+rows < len(bindings) {
			sb.WriteString(format(bindings[row+rows]))
		}
		sb.WriteString("\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

// getKeysText renders the right header column (former logs) — current-view keys only.
func (c *CoreView) getKeysText() string {
	return formatBindingsColumns(buildSortedBindings(c.keyBindings), 5, 2)
}

// refreshHeaderPanels updates views + keys columns.
func (c *CoreView) refreshHeaderPanels() {
	if c.viewsPanel != nil {
		c.viewsPanel.SetText(c.getViewsText())
	}
	if c.keysPanel != nil {
		c.keysPanel.SetText(c.getKeysText())
	}
}

// SetHelpSections sets grouped help content for the "?" modal.
func (c *CoreView) SetHelpSections(sections []HelpSection) *CoreView {
	c.helpSections = sections
	return c
}

// ClearHelpSections drops plugin-provided help groups.
func (c *CoreView) ClearHelpSections() *CoreView {
	c.helpSections = nil
	return c
}

// getExpandedHelpText returns the "?" modal body, grouped by view when available.
func (c *CoreView) getExpandedHelpText() string {
	var sb strings.Builder

	if len(c.helpSections) > 0 {
		sb.WriteString("[yellow]Shortcuts by view[white]\n")
		sb.WriteString("[gray]Press Esc to close · ? again from a view for context[white]\n\n")
		for _, section := range c.helpSections {
			if section.Title == "" {
				continue
			}
			sb.WriteString(fmt.Sprintf("[yellow]%s[white]\n", section.Title))
			if len(section.Bindings) == 0 {
				sb.WriteString("  [gray](no actions)[white]\n\n")
				continue
			}
			for _, b := range section.Bindings {
				label := b.Label
				if label == "" {
					label = b.Key
				}
				sb.WriteString(fmt.Sprintf("  [aqua]%s[white]  %s\n", b.Key, label))
			}
			sb.WriteString("\n")
		}
		return sb.String()
	}

	sb.WriteString("[yellow]Views[white]\n")
	for _, b := range buildSortedBindings(c.viewBindings) {
		sb.WriteString(fmt.Sprintf("  [aqua]%s[white]  %s\n", b.key, b.description))
	}
	sb.WriteString("\n[yellow]Keys[white]\n")
	for _, b := range buildSortedBindings(c.keyBindings) {
		sb.WriteString(fmt.Sprintf("  [aqua]%s[white]  %s\n", b.key, b.description))
	}
	return sb.String()
}

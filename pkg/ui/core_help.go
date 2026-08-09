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

func formatBindingDescription(description string) string {
	if len(description) == 0 {
		return ""
	}
	formatted := strings.ToUpper(description[:1])
	if len(description) > 1 {
		formatted += strings.ToLower(description[1:])
	}
	return formatted
}

func isDigitKey(key string) bool {
	return len(key) == 1 && key[0] >= '0' && key[0] <= '9'
}

func isSpecialKey(key string) bool {
	return len(key) > 1 || strings.ContainsAny(key, "^_")
}

func bindingLess(a, b keyBinding) bool {
	digitA, digitB := isDigitKey(a.key), isDigitKey(b.key)
	if digitA != digitB {
		return digitA
	}
	if digitA {
		return a.key < b.key
	}
	specialA, specialB := isSpecialKey(a.key), isSpecialKey(b.key)
	if specialA != specialB {
		return !specialA
	}
	return a.key < b.key
}

func buildSortedBindings(keyBindings map[string]string) []keyBinding {
	bindings := make([]keyBinding, 0, len(keyBindings))
	for key, description := range keyBindings {
		bindings = append(bindings, keyBinding{key, formatBindingDescription(description)})
	}
	sort.Slice(bindings, func(i, j int) bool {
		return bindingLess(bindings[i], bindings[j])
	})
	return bindings
}

func longestDescription(bindings []keyBinding) int {
	longest := 0
	for _, b := range bindings {
		if len(b.description) > longest {
			longest = len(b.description)
		}
	}
	return longest
}

func bindingCell(b keyBinding, longestDesc int) string {
	paddingSpaces := longestDesc - len(b.description) + 1
	if paddingSpaces < 1 {
		paddingSpaces = 1
	}
	return fmt.Sprintf("[purple::b]<%s>[white::-] %s%s", b.key, b.description, strings.Repeat(" ", paddingSpaces))
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

	longestDesc := longestDescription(bindings)
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
		cell := bindingCell(binding, longestDesc)
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

func writeHelpSection(sb *strings.Builder, section HelpSection) {
	if section.Title == "" {
		return
	}
	sb.WriteString(fmt.Sprintf("[yellow]%s[white]\n", section.Title))
	if len(section.Bindings) == 0 {
		sb.WriteString("  [gray](no actions)[white]\n\n")
		return
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

func (c *CoreView) formatSectionedHelp() string {
	var sb strings.Builder
	sb.WriteString("[yellow]Shortcuts by view[white]\n")
	sb.WriteString("[gray]Press Esc to close · ? again from a view for context[white]\n\n")
	for _, section := range c.helpSections {
		writeHelpSection(&sb, section)
	}
	return sb.String()
}

func (c *CoreView) formatFlatHelp() string {
	var sb strings.Builder
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

// getExpandedHelpText returns the "?" modal body, grouped by view when available.
func (c *CoreView) getExpandedHelpText() string {
	if len(c.helpSections) > 0 {
		return c.formatSectionedHelp()
	}
	return c.formatFlatHelp()
}

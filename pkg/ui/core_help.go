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

// clipPad truncates s to width runes (ellipsis if needed) and right-pads so
// header columns stay aligned.
func clipPad(s string, width int) string {
	if width <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) > width {
		if width == 1 {
			return string(r[0])
		}
		return string(append(r[:width-1], '…'))
	}
	return s + strings.Repeat(" ", width-len(r))
}

func bindingCell(b keyBinding, labelWidth int) string {
	return fmt.Sprintf("%s %s ", shortcutAction(b.key), clipPad(b.description, labelWidth))
}

// Shortcuts follow the active theme: view keys, action keys, labels.
const (
	headerViewNameWidth   = 12
	headerActionNameWidth = 11
	headerColGap          = 3
)

func shortcutView(key string) string {
	return fmt.Sprintf("[%s]<%s>[%s]", HexViewKey, key, HexLabel)
}

func shortcutViewActive(key string) string {
	return fmt.Sprintf("[%s::b]<%s>[%s::-]", HexViewKey, key, HexLabel)
}

func shortcutAction(key string) string {
	return fmt.Sprintf("[%s]<%s>[%s]", HexActionKey, key, HexLabel)
}

func shortcutKey(key string) string {
	return shortcutAction(key)
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

	labelWidth := headerActionNameWidth
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
		cell := bindingCell(binding, labelWidth)
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
		label := clipPad(b.description, headerViewNameWidth)
		if viewID != "" && viewID == c.activeViewID {
			return fmt.Sprintf("%s %s ", shortcutViewActive(b.key), label)
		}
		return fmt.Sprintf("%s %s ", shortcutView(b.key), label)
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
	bindings := buildSortedBindings(c.keyBindings)
	cols := 2
	if c.logs != nil || len(bindings) > 10 {
		cols = 3 // logs actions need the extra column to fit in the 5-row header
	}
	return formatBindingsColumns(bindings, 5, cols)
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
	keyFmt := shortcutAction
	if strings.EqualFold(section.Title, "Views") || strings.EqualFold(section.Title, "More Views") {
		keyFmt = shortcutView
	}
	for _, b := range section.Bindings {
		label := b.Label
		if label == "" {
			label = b.Key
		}
		sb.WriteString(fmt.Sprintf("  %s  %s\n", keyFmt(b.Key), label))
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
		sb.WriteString(fmt.Sprintf("  %s  %s\n", shortcutView(b.key), b.description))
	}
	sb.WriteString("\n[yellow]Keys[white]\n")
	for _, b := range buildSortedBindings(c.keyBindings) {
		sb.WriteString(fmt.Sprintf("  %s  %s\n", shortcutAction(b.key), b.description))
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

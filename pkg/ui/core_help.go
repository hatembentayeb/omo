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
		isSpecialI := len(bindings[i].key) > 1 || strings.ContainsAny(bindings[i].key, "^_")
		isSpecialJ := len(bindings[j].key) > 1 || strings.ContainsAny(bindings[j].key, "^_")
		if isSpecialI != isSpecialJ {
			return !isSpecialI
		}
		return bindings[i].key < bindings[j].key
	})
	return bindings
}

func formatBindingsColumns(bindings []keyBinding) string {
	const maxRowsPerColumn = 4

	longestDesc := 0
	for _, b := range bindings {
		if len(b.description) > longestDesc {
			longestDesc = len(b.description)
		}
	}

	var sb strings.Builder
	numBindings := len(bindings)

	for i := 0; i < numBindings; i++ {
		binding := bindings[i]
		paddingSpaces := longestDesc - len(binding.description) + 1
		if paddingSpaces < 1 {
			paddingSpaces = 1
		}
		padding := strings.Repeat(" ", paddingSpaces)
		formattedBinding := fmt.Sprintf("[purple::b]<%s>[white::-]  %s%s", binding.key, binding.description, padding)

		if i < maxRowsPerColumn {
			sb.WriteString(formattedBinding)
			sb.WriteString("\n")
		} else {
			rowInColumn := (i - maxRowsPerColumn) % maxRowsPerColumn
			if rowInColumn < maxRowsPerColumn && i < (maxRowsPerColumn*2) {
				lines := strings.Split(sb.String(), "\n")
				if rowInColumn < len(lines) {
					lines[rowInColumn] = lines[rowInColumn] + formattedBinding
					sb.Reset()
					sb.WriteString(strings.Join(lines[:len(lines)-1], "\n"))
					if i < numBindings-1 {
						sb.WriteString("\n")
					}
				}
			}
		}
	}

	return sb.String()
}

// getHelpText generates the help text for keybindings
func (c *CoreView) getHelpText() string {
	bindings := buildSortedBindings(c.keyBindings)
	return formatBindingsColumns(bindings)
}

// getExpandedHelpText returns the expanded help text with more detailed information
func (c *CoreView) getExpandedHelpText() string {
	// Start with the basic help text as a foundation
	var sb strings.Builder

	sb.WriteString("[yellow]Keybinding Reference:[white]\n\n")

	// Add standard keybindings with more detailed descriptions
	sb.WriteString("[yellow]Standard Navigation:[white]\n")
	sb.WriteString("  [aqua]ESC[white]    - Navigate back to previous view\n")
	sb.WriteString("  [aqua]R[white]      - Refresh current data\n")
	sb.WriteString("  [aqua]?[white]      - Toggle between basic and detailed help\n\n")

	// Add custom keybindings with detailed explanations
	sb.WriteString("[yellow]Custom Actions:[white]\n")
	for key, description := range c.keyBindings {
		// Skip the standard keys we already covered
		if key == "R" || key == "ESC" || key == "?" {
			continue
		}
		sb.WriteString(fmt.Sprintf("  [aqua]%s[white]      - %s\n", key, description))
	}

	// Add additional help sections with more context
	sb.WriteString("\n[yellow]Navigation Tips:[white]\n")
	sb.WriteString("  - Use arrow keys to navigate the table\n")
	sb.WriteString("  - Press Enter to select an item\n")
	sb.WriteString("  - Use ESC to go back through navigation history\n\n")

	sb.WriteString("[yellow]Current View:[white] " + c.GetCurrentView() + "\n")

	return sb.String()
}

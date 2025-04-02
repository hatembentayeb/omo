package ui

import (
	"fmt"
	"sort"
	"strings"
)

// getHelpText generates the help text for keybindings
func (c *Cores) getHelpText() string {
	// Prepare keybindings for matrix display
	type keyBinding struct {
		key         string
		description string
	}

	// Build slice of keybindings with properly formatted descriptions
	bindings := make([]keyBinding, 0, len(c.keyBindings))
	for key, description := range c.keyBindings {
		// Convert description to Title case (first letter uppercase, rest lowercase)
		formattedDesc := ""
		if len(description) > 0 {
			formattedDesc = strings.ToUpper(description[:1])
			if len(description) > 1 {
				formattedDesc += strings.ToLower(description[1:])
			}
		}

		bindings = append(bindings, keyBinding{key, formattedDesc})
	}

	// Sort bindings for visual appeal (similar to Redis plugin)
	sort.Slice(bindings, func(i, j int) bool {
		// Special keys like "ESC", "^t" go to the end
		isSpecialI := len(bindings[i].key) > 1 || strings.ContainsAny(bindings[i].key, "^_")
		isSpecialJ := len(bindings[j].key) > 1 || strings.ContainsAny(bindings[j].key, "^_")

		// If one is special and the other isn't
		if isSpecialI != isSpecialJ {
			return !isSpecialI // Non-special keys come first
		}

		// Both are special or both are normal, sort alphabetically
		return bindings[i].key < bindings[j].key
	})

	// New layout: First 4 rows in a single column, then second column if needed
	var sb strings.Builder
	numBindings := len(bindings)

	// Calculate how many rows we need - maximum of 4 rows per column
	maxRowsPerColumn := 4

	// Find the longest description to align all others
	longestDesc := 0
	for _, binding := range bindings {
		if len(binding.description) > longestDesc {
			longestDesc = len(binding.description)
		}
	}

	// Format the keybindings
	for i := 0; i < numBindings; i++ {
		binding := bindings[i]

		// Calculate padding for description alignment
		paddingSpaces := 0
		if len(binding.description) < longestDesc {
			paddingSpaces = longestDesc - len(binding.description) + 1 // +1 ensures exactly one space after the longest description
		} else {
			paddingSpaces = 1 // Exactly one space for the longest description
		}
		padding := strings.Repeat(" ", paddingSpaces)

		// Format the key binding with proper padding for alignment
		formattedBinding := fmt.Sprintf("[purple::b]<%s>[white::-]  %s%s", binding.key, binding.description, padding)

		// Determine if this is in the first column or second column
		inFirstColumn := i < maxRowsPerColumn

		// If in first column, start a new line
		if inFirstColumn {
			sb.WriteString(formattedBinding)
			sb.WriteString("\n")
		} else {
			// In second column - go back up and add to the appropriate row
			rowInColumn := (i - maxRowsPerColumn) % maxRowsPerColumn

			// If starting a new row in the second column, don't add content
			// because we need to find the existing row

			// For rows that exist in the first column, this adds to those rows
			if rowInColumn < maxRowsPerColumn && i < (maxRowsPerColumn*2) {
				// Split the current text by newlines
				lines := strings.Split(sb.String(), "\n")

				// Make sure we have enough lines
				if rowInColumn < len(lines) {
					// Replace the line with the original line + new binding
					lines[rowInColumn] = lines[rowInColumn] + formattedBinding

					// Clear the builder and rebuild the string
					sb.Reset()
					sb.WriteString(strings.Join(lines[:len(lines)-1], "\n")) // Don't add the trailing empty line

					// Add a newline at the end of the last non-empty line
					if i < numBindings-1 {
						sb.WriteString("\n")
					}
				}
			}
		}
	}

	return sb.String()
}

// getExpandedHelpText returns the expanded help text with more detailed information
func (c *Cores) getExpandedHelpText() string {
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

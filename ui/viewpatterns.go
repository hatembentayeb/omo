// Package ui provides terminal UI components for building consistent
// terminal applications with a unified interface.
package ui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ViewPattern encapsulates common patterns for view initialization.
// It provides a standard way to configure views across different plugins,
// ensuring consistency in the user interface while reducing boilerplate code.
// This allows plugin developers to focus on implementing functionality
// rather than UI configuration details.
type ViewPattern struct {
	App          *tview.Application         // Reference to the main application
	Pages        *tview.Pages               // Pages component for navigation
	Cores        *Cores                     // Optional existing Cores instance
	Title        string                     // Title of the view
	HeaderText   string                     // Text to display in the header
	TableHeaders []string                   // Column headers for the table
	RefreshFunc  func() ([][]string, error) // Function to retrieve data
	KeyHandlers  map[string]string          // Key to description mapping
	SelectedFunc func(row int)              // Called when a row is selected
}

// InitializeView creates a standard view based on the provided pattern.
// This function creates a consistent UI layout following the OMO design patterns,
// by configuring headers, callbacks, and key bindings based on the supplied pattern.
//
// Parameters:
//   - pattern: The ViewPattern containing view configuration options
//
// Returns:
//   - A fully configured Cores instance ready to be displayed
func InitializeView(pattern ViewPattern) *Cores {
	// Create new Cores UI component if none was provided
	cores := pattern.Cores
	if cores == nil {
		cores = NewCores(pattern.App, pattern.Title)
	}

	// Set table headers
	if pattern.TableHeaders != nil {
		cores.SetTableHeaders(pattern.TableHeaders)
	}

	// Set refresh callback
	if pattern.RefreshFunc != nil {
		cores.SetRefreshCallback(pattern.RefreshFunc)
	}

	// Set row selection callback
	if pattern.SelectedFunc != nil {
		cores.SetRowSelectedCallback(pattern.SelectedFunc)
	}

	// Register standard keys
	cores.RegisterStandardKeys()

	// Add custom key bindings with descriptions
	if pattern.KeyHandlers != nil {
		for key, description := range pattern.KeyHandlers {
			cores.keyBindings[key] = description
		}
	}

	// Register standard keys
	cores.RegisterStandardKeys()

	// Register key bindings like in the Kafka and Git plugins
	if pattern.KeyHandlers != nil {
		for key, description := range pattern.KeyHandlers {
			cores.AddKeyBinding(key, description, nil)
		}
	}

	// Set header text if provided
	if pattern.HeaderText != "" {
		cores.SetInfoText(pattern.HeaderText)
	}

	// Register handlers
	cores.RegisterHandlers()

	return cores
}

// CreateStandardTableRow creates a consistently formatted table row.
// This function standardizes the appearance of table rows across the application,
// applying consistent styling and colors.
//
// Parameters:
//   - values: The strings to display in the row
//   - colors: Optional colors to apply to each cell (defaults to white if not specified)
//
// Returns:
//   - A styled table cell that can be added to a table
func CreateStandardTableRow(values []string, colors []tcell.Color) *tview.TableCell {
	row := tview.NewTableCell("")

	// Format the row with consistent styling
	for i, value := range values {
		color := tcell.ColorWhite
		if i < len(colors) {
			color = colors[i]
		}

		// Apply consistent styling
		row.SetText(fmt.Sprintf("%s", value))
		row.SetTextColor(color)
	}

	return row
}

// StandardDataFormatterFunc provides a standard way to format row data.
// This function type defines a transformation that can be applied to
// row data before displaying it in the UI.
type StandardDataFormatterFunc func([]string) []string

// FormatDataWithStandardPattern applies consistent formatting to table data.
// This helper function processes all rows in a dataset using the provided
// formatter function, ensuring consistent appearance across tables.
//
// Parameters:
//   - data: The raw data rows to format
//   - formatter: Optional function to transform each row (can be nil)
//
// Returns:
//   - Formatted data rows ready for display
func FormatDataWithStandardPattern(
	data [][]string,
	formatter StandardDataFormatterFunc,
) [][]string {
	result := make([][]string, len(data))

	for i, row := range data {
		if formatter != nil {
			result[i] = formatter(row)
		} else {
			result[i] = row
		}
	}

	return result
}

// CreateStandardSelectionHandler returns a standard selection handler function.
// This creates a consistent way to handle row selection events across different views,
// with optional formatting and actions triggered on selection.
//
// Parameters:
//   - cores: The Cores instance to log selection events to
//   - data: The underlying data objects represented by the rows
//   - formatFunc: Function to format the selected item for display in logs
//   - actionFunc: Function to perform an action when an item is selected
//
// Returns:
//   - A row selection handler function that can be registered with Cores
func CreateStandardSelectionHandler(
	cores *Cores,
	data []interface{},
	formatFunc func(item interface{}) string,
	actionFunc func(selectedItem interface{}),
) func(row int) {
	return func(row int) {
		if row >= 0 && row < len(data) {
			selectedItem := data[row]
			if formatFunc != nil {
				cores.Log(fmt.Sprintf("[blue]Selected: %s", formatFunc(selectedItem)))
			}
			if actionFunc != nil {
				actionFunc(selectedItem)
			}
		}
	}
}

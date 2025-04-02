package ui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ViewPattern encapsulates common patterns for view initialization
type ViewPattern struct {
	App          *tview.Application
	Pages        *tview.Pages
	Cores        *Cores
	Title        string
	HeaderText   string
	TableHeaders []string
	RefreshFunc  func() ([][]string, error)
	KeyHandlers  map[string]string // Key to description mapping
	SelectedFunc func(row int)
}

// InitializeView creates a standard view based on the provided pattern
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

// CreateStandardTableRow creates a consistently formatted table row
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

// StandardDataFormatterFunc provides a standard way to format row data
type StandardDataFormatterFunc func([]string) []string

// FormatDataWithStandardPattern applies consistent formatting to table data
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

// CreateStandardSelectionHandler returns a standard selection handler function
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

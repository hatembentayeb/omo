package ui

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ViewType represents the type of view to create
type ViewType int

const (
	// ViewTypeTable represents a standard table view
	ViewTypeTable ViewType = iota
	// ViewTypeSplit represents a split view with two panels
	ViewTypeSplit
	// ViewTypeForm represents a form view for data entry
	ViewTypeForm
	// ViewTypeDetail represents a detailed view of a selected item
	ViewTypeDetail
)

// ViewFactory creates standardized views for common patterns
type ViewFactory struct {
	app   *tview.Application
	pages *tview.Pages
}

// NewViewFactory creates a new view factory
func NewViewFactory(app *tview.Application, pages *tview.Pages) *ViewFactory {
	return &ViewFactory{
		app:   app,
		pages: pages,
	}
}

// TableViewConfig configures a table view
type TableViewConfig struct {
	Title          string
	TableHeaders   []string
	RefreshFunc    func() ([][]string, error)
	KeyHandlers    map[string]string
	SelectedFunc   func(row int)
	InitialData    [][]string
	AutoRefresh    bool
	RefreshSeconds int
}

// CreateTableView creates a standard table view based on the config
func (f *ViewFactory) CreateTableView(config TableViewConfig) *Cores {
	// Create new Cores UI component
	cores := NewCores(f.app, config.Title)

	// Set table headers
	if config.TableHeaders != nil {
		cores.SetTableHeaders(config.TableHeaders)
	}

	// Set refresh callback
	if config.RefreshFunc != nil {
		cores.SetRefreshCallback(config.RefreshFunc)
	}

	// Set row selection callback
	if config.SelectedFunc != nil {
		cores.SetRowSelectedCallback(config.SelectedFunc)
	}

	// Register standard keys
	cores.RegisterStandardKeys()

	// Register key bindings like in the Kafka and Git plugins
	if config.KeyHandlers != nil {
		for key, description := range config.KeyHandlers {
			cores.AddKeyBinding(key, description, nil)
		}
	}

	// Set initial data if provided
	if config.InitialData != nil {
		cores.SetTableData(config.InitialData)
	}

	// Set up auto-refresh if enabled
	if config.AutoRefresh && config.RefreshFunc != nil {
		interval := 10 // Default 10 seconds
		if config.RefreshSeconds > 0 {
			interval = config.RefreshSeconds
		}
		cores.StartAutoRefresh(time.Duration(interval) * time.Second)
	}

	// Register handlers
	cores.RegisterHandlers()

	return cores
}

// SplitViewConfig configures a split view
type SplitViewConfig struct {
	Title           string
	LeftTitle       string
	RightTitle      string
	LeftHeaders     []string
	RightHeaders    []string
	LeftRefreshFunc func() ([][]string, error)
	RightDataFunc   func(selectedRow int) ([][]string, error)
	KeyHandlers     map[string]string // Key to description mapping
	LeftSelected    func(row int)
}

// CreateSplitView creates a standard split view with master-detail layout
func (f *ViewFactory) CreateSplitView(config SplitViewConfig) tview.Primitive {
	// Create left panel (master)
	leftCores := NewCores(f.app, config.LeftTitle)
	leftCores.SetTableHeaders(config.LeftHeaders)

	// Create right panel (detail)
	rightCores := NewCores(f.app, config.RightTitle)
	rightCores.SetTableHeaders(config.RightHeaders)

	// Set up left panel refresh
	if config.LeftRefreshFunc != nil {
		leftCores.SetRefreshCallback(config.LeftRefreshFunc)
	}

	// Register standard keys for both panels
	leftCores.RegisterStandardKeys()
	rightCores.RegisterStandardKeys()

	// Set key handlers and callbacks for left panel
	if config.KeyHandlers != nil {
		for key, description := range config.KeyHandlers {
			leftCores.AddKeyBinding(key, description, nil)
		}
	}

	// Set up master-detail relationship
	if config.LeftSelected != nil && config.RightDataFunc != nil {
		leftCores.SetRowSelectedCallback(func(row int) {
			// Call the original callback
			config.LeftSelected(row)

			// Update the right panel with detail data
			rightData, err := config.RightDataFunc(row)
			if err != nil {
				rightCores.Log(fmt.Sprintf("[red]Error loading details: %v", err))
				return
			}

			rightCores.SetTableData(rightData)
			// Ensure table is refreshed visually
			f.app.QueueUpdateDraw(func() {
				rightCores.table.ScrollToBeginning()
			})
		})
	}

	// Register handlers for both cores
	leftCores.RegisterHandlers()
	rightCores.RegisterHandlers()

	// Create a flex container for the split view
	flex := tview.NewFlex()
	flex.SetDirection(tview.FlexColumn)
	flex.SetBackgroundColor(tcell.ColorDefault)
	flex.AddItem(leftCores.GetLayout(), 0, 1, true).
		AddItem(rightCores.GetLayout(), 0, 1, false)

	return flex
}

// DetailViewConfig configures a detail view
type DetailViewConfig struct {
	Title       string
	KeyHandlers map[string]string // Key to description mapping
	HeaderText  string
	DetailFunc  func() (map[string]string, error)
	ActionFunc  func(action string) error
}

// CreateDetailView creates a detailed view for a single item
func (f *ViewFactory) CreateDetailView(config DetailViewConfig) *Cores {
	// Create new Cores UI component
	cores := NewCores(f.app, config.Title)

	// Set info text
	if config.HeaderText != "" {
		cores.SetInfoText(config.HeaderText)
	}

	// Register standard keys
	cores.RegisterStandardKeys()

	// Register key bindings like in the Kafka and Git plugins
	if config.KeyHandlers != nil {
		for key, description := range config.KeyHandlers {
			cores.AddKeyBinding(key, description, nil)
		}
	}

	// Set up refresh to show details
	if config.DetailFunc != nil {
		cores.SetRefreshCallback(func() ([][]string, error) {
			// Get details
			details, err := config.DetailFunc()
			if err != nil {
				return nil, err
			}

			// Convert map to table data
			tableData := make([][]string, 0, len(details))
			for key, value := range details {
				tableData = append(tableData, []string{key, value})
			}

			return tableData, nil
		})

		// Set detail view headers
		cores.SetTableHeaders([]string{"Property", "Value"})

		// Initial refresh to show data
		cores.RefreshData()
	}

	// Set up action callback
	if config.ActionFunc != nil {
		cores.SetActionCallback(func(action string, payload map[string]interface{}) error {
			if action == "keypress" {
				if key, ok := payload["key"].(string); ok {
					return config.ActionFunc(key)
				}
			}
			return nil
		})
	}

	// Register handlers
	cores.RegisterHandlers()

	return cores
}

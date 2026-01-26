// Package ui provides terminal UI components for building consistent
// terminal applications with a unified interface.
// The UI package offers reusable components for building plugin UIs with a consistent
// look and feel throughout the OMO (Oh My Ops) system.
package ui

import (
	"sync"
	"time"

	"github.com/rivo/tview"
)

// Cores provides a standardized UI component that can be embedded in plugins
// with consistent layout and behavior. Cores is the central UI component that
// handles common UI patterns, user input, data display, and plugin interactions.
// It manages components like tables, logs, help text, and navigation breadcrumbs.
type Cores struct {
	// Core components
	app        *tview.Application // Reference to the main application
	pages      *tview.Pages       // Modal container (optional)
	mainLayout *tview.Flex        // Main component layout
	title      string             // Plugin title

	// Header row panels
	infoPanel   *tview.TextView // Left: context and status information
	helpPanel   *tview.TextView // Middle: keybinding and help information
	logPanel    *tview.TextView // Right: logs and messages
	breadcrumbs *tview.TextView // Navigation breadcrumbs

	// Table view
	table        *Table
	tableContent *VirtualTableContent

	// Table data
	tableHeaders    []string
	tableData       [][]string
	rawTableData    [][]string
	selectionKey    string // Column to track selected rows
	selectedRow     int    // Currently selected row index (-1 if none)
	filterQuery     string
	filteredIndices []int

	// Key binding management
	keyBindings  map[string]string
	keyHandlers  map[string]func()

	// Data refresh management
	refreshMutex  sync.Mutex
	refreshTicker *time.Ticker
	stopRefresh   chan struct{}
	onRefresh     func() ([][]string, error)

	// Data operation lock - prevents concurrent load/refresh/filter
	dataMutex sync.Mutex
	isLoading bool

	// Callbacks for plugin integration
	onRowSelected func(row int)
	onAction      func(action string, payload map[string]interface{}) error

	// Navigation stack
	navStack []string

	// Lazy loading
	lazyLoader   func(offset, limit int) ([][]string, error)
	lazyPageSize int
	lazyOffset   int
	lazyHasMore  bool
}

// NewCores creates a new Cores UI component with the specified plugin title
// that can be embedded in the main application. It initializes the UI components
// and sets up default key bindings.
//
// Parameters:
//   - app: The tview application instance to attach to
//   - title: The title of the plugin to display in the UI
//
// Returns:
//   - A fully initialized Cores instance ready to be used
func NewCores(app *tview.Application, title string) *Cores {
	c := &Cores{
		app:          app,
		title:        title,
		selectedRow:  -1,
		tableHeaders: []string{},
		tableData:    [][]string{},
		stopRefresh:  make(chan struct{}),
		keyBindings:  make(map[string]string),
		keyHandlers:  make(map[string]func()),
	}

	// Set default key bindings
	c.keyBindings = map[string]string{
		"R":   "Refresh",
		"ESC": "Back",
		"?":   "Help",
		"/":   "Filter",
	}

	// Initialize UI components
	c.initUI()

	return c
}

// GetLayout returns the main layout component to be embedded in the application.
// This is the primary method to retrieve the UI component for display.
//
// Returns:
//   - The main tview.Primitive component that can be added to the application
func (c *Cores) GetLayout() tview.Primitive {
	return c.mainLayout
}

// Destroy cleans up resources used by this component.
// This method should be called when the plugin is unloaded to prevent resource leaks
// by stopping background processes and unregistering handlers.
func (c *Cores) Destroy() {
	// Stop any background refresh
	if c.refreshTicker != nil {
		c.StopAutoRefresh()
	}

	// Remove handlers
	c.UnregisterHandlers()
}

// min is a helper function that returns the smaller of two integers.
//
// Parameters:
//   - a: First integer to compare
//   - b: Second integer to compare
//
// Returns:
//   - The smaller of the two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GetTable returns the underlying table primitive for focus management.
// This allows direct access to the table for advanced operations.
//
// Returns:
//   - The Table component instance
func (c *Cores) GetTable() *Table {
	return c.table
}

// GetSelectedRow returns the index of the currently selected row in the RAW (unfiltered) data.
// When filtering is active, this returns the original index in rawTableData, not the filtered index.
// Use GetSelectedRowData() to get the actual row data instead of indexing manually.
//
// Returns:
//   - The raw data index of the selected row, or -1 if none selected
func (c *Cores) GetSelectedRow() int {
	if c.selectedRow < 0 {
		return -1
	}
	if len(c.filteredIndices) > 0 {
		if c.selectedRow >= len(c.filteredIndices) {
			return -1
		}
		return c.filteredIndices[c.selectedRow]
	}
	return c.selectedRow
}

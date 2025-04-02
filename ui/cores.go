// Package ui provides terminal UI components for building consistent
// terminal applications with a unified interface
package ui

import (
	"sync"
	"time"

	"github.com/rivo/tview"
)

// Cores provides a standardized UI component that can be embedded in plugins
// with consistent layout and behavior.
type Cores struct {
	// Core components
	app        *tview.Application // Reference to the main application
	mainLayout *tview.Flex        // Main component layout
	title      string             // Plugin title

	// Header row panels
	infoPanel   *tview.TextView // Left: context and status information
	helpPanel   *tview.TextView // Middle: keybinding and help information
	logPanel    *tview.TextView // Right: logs and messages
	breadcrumbs *tview.TextView // Navigation breadcrumbs

	// Table view
	table *Table

	// Table data
	tableHeaders []string
	tableData    [][]string
	selectionKey string // Column to track selected rows
	selectedRow  int    // Currently selected row index (-1 if none)

	// Key binding management
	keyBindings map[string]string

	// Data refresh management
	refreshMutex  sync.Mutex
	refreshTicker *time.Ticker
	stopRefresh   chan struct{}
	onRefresh     func() ([][]string, error)

	// Callbacks for plugin integration
	onRowSelected func(row int)
	onAction      func(action string, payload map[string]interface{}) error

	// Navigation stack
	navStack []string
}

// NewCores creates a new Cores UI component with the specified plugin title
// that can be embedded in the main application.
func NewCores(app *tview.Application, title string) *Cores {
	c := &Cores{
		app:          app,
		title:        title,
		selectedRow:  -1,
		tableHeaders: []string{},
		tableData:    [][]string{},
		stopRefresh:  make(chan struct{}),
		keyBindings:  make(map[string]string),
	}

	// Set default key bindings
	c.keyBindings = map[string]string{
		"R":   "Refresh",
		"ESC": "Back",
		"?":   "Help",
	}

	// Initialize UI components
	c.initUI()

	return c
}

// GetLayout returns the main layout component to be embedded in the application
func (c *Cores) GetLayout() tview.Primitive {
	return c.mainLayout
}

// Destroy cleans up resources used by this component
// Should be called when the plugin is unloaded
func (c *Cores) Destroy() {
	// Stop any background refresh
	if c.refreshTicker != nil {
		c.StopAutoRefresh()
	}

	// Remove handlers
	c.UnregisterHandlers()
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GetTable returns the underlying table primitive for focus management
func (c *Cores) GetTable() *Table {
	return c.table
}

// GetSelectedRow returns the currently selected row in the table
func (c *Cores) GetSelectedRow() int {
	return c.table.GetSelectedRow()
}

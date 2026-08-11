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

// HelpSection is one titled group shown in the "?" help modal.
type HelpSection struct {
	Title    string
	Bindings []KeyBindingHelp
}

// KeyBindingHelp is a key + label pair for help display.
type KeyBindingHelp struct {
	Key   string
	Label string
}

// CoreView provides a standardized UI component that can be embedded in plugins
// with consistent layout and behavior. CoreView is the central UI component that
// handles common UI patterns, user input, data display, and plugin interactions.
// It manages components like tables, help text, and navigation breadcrumbs.
type CoreView struct {
	// Core components
	app          *tview.Application // Reference to the main application
	pages        *tview.Pages       // Modal container (optional)
	mainLayout   *tview.Flex        // Main component layout
	contentPages *tview.Pages       // Swaps table ↔ logs in the content area
	title        string             // Plugin title

	// Header row panels
	infoPanel    *tview.TextView // Left: connection / status
	viewsPanel   *tview.TextView // Middle: views 0-9
	keysPanel    *tview.TextView // Right (former logs): expanded key shortcuts
	helpExpanded bool
	breadcrumbs  *tview.TextView

	// In-place log viewer (replaces table; header stays).
	logs             *logsView
	logsKeysSaved    bool
	savedKeyBindings map[string]string
	savedKeyHandlers map[string]func()

	// Optional "?" modal content (grouped by view).
	helpSections []HelpSection

	// Separate binding maps for the two shortcut columns.
	viewBindings   map[string]string // middle: view switches (key -> label)
	viewBindingIDs map[string]string // key -> view id for highlight
	activeViewID   string
	keyBindings    map[string]string // right: action / global keys
	keyHandlers    map[string]func()

	// Table view
	table        *Table
	tableContent *VirtualTableContent

	// Table data
	tableHeaders    []string
	tableData       [][]string
	rawTableData    [][]string
	selectionKey    string
	selectedRow     int
	filterQuery     string
	filteredIndices []int

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

// NewCoreView creates a new CoreView UI component with the specified plugin title
// that can be embedded in the main application. It initializes the UI components
// and sets up default key bindings.
//
// Parameters:
//   - app: The tview application instance to attach to
//   - title: The title of the plugin to display in the UI
//
// Returns:
//   - A fully initialized CoreView instance ready to be used
func NewCoreView(app *tview.Application, title string) *CoreView {
	c := &CoreView{
		app:            app,
		title:          title,
		selectedRow:    -1,
		stopRefresh:    make(chan struct{}),
		viewBindings:   make(map[string]string),
		viewBindingIDs: make(map[string]string),
		keyBindings: map[string]string{
			"R":   "Refresh",
			"ESC": "Back",
			"?":   "Help",
			"/":   "Filter",
		},
		keyHandlers: make(map[string]func()),
	}

	c.initUI()

	return c
}

// GetLayout returns the main layout component to be embedded in the application.
// This is the primary method to retrieve the UI component for display.
//
// Returns:
//   - The main tview.Primitive component that can be added to the application
func (c *CoreView) GetLayout() tview.Primitive {
	return c.mainLayout
}

// Destroy cleans up resources used by this component.
// This method should be called when the plugin is unloaded to prevent resource leaks
// by stopping background processes and unregistering handlers.
func (c *CoreView) Destroy() {
	// Stop any background refresh
	if c.refreshTicker != nil {
		c.StopAutoRefresh()
	}

	// Remove handlers
	c.UnregisterHandlers()
}

// GetTable returns the underlying table primitive for focus management.
// This allows direct access to the table for advanced operations.
//
// Returns:
//   - The Table component instance
func (c *CoreView) GetTable() *Table {
	return c.table
}

// GetSelectedRow returns the index of the currently selected row in the RAW (unfiltered) data.
// When filtering is active, this returns the original index in rawTableData, not the filtered index.
// Use GetSelectedRowData() to get the actual row data instead of indexing manually.
//
// Returns:
//   - The raw data index of the selected row, or -1 if none selected
func (c *CoreView) GetSelectedRow() int {
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

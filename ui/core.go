// Package ui provides terminal UI components for building consistent
// terminal applications with a unified interface
package ui

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
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
	table *tview.Table

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

// initUI initializes all UI components
func (c *Cores) initUI() {
	// Initialize breadcrumbs
	c.breadcrumbs = tview.NewTextView()
	c.breadcrumbs.SetDynamicColors(true)
	c.breadcrumbs.SetTextAlign(tview.AlignLeft)
	c.breadcrumbs.SetText(c.title)
	c.breadcrumbs.SetBackgroundColor(tcell.ColorBlack)
	c.breadcrumbs.SetBorder(false)

	// Info panel (left)
	c.infoPanel = tview.NewTextView()
	c.infoPanel.SetDynamicColors(true)
	c.infoPanel.SetTextAlign(tview.AlignLeft)
	c.infoPanel.SetText(fmt.Sprintf("[yellow]%s[white]\nStatus: Active", c.title))
	c.infoPanel.SetBackgroundColor(tcell.ColorBlack)
	c.infoPanel.SetBorder(false)

	// Help panel (middle)
	c.helpPanel = tview.NewTextView()
	c.helpPanel.SetDynamicColors(true)
	c.helpPanel.SetTextAlign(tview.AlignLeft)
	c.helpPanel.SetText(c.getHelpText())
	c.helpPanel.SetBackgroundColor(tcell.ColorBlack)
	c.helpPanel.SetBorder(false)

	// Log panel (right)
	c.logPanel = tview.NewTextView()
	c.logPanel.SetDynamicColors(true)
	c.logPanel.SetChangedFunc(func() {
		c.app.Draw()
	})
	c.logPanel.SetScrollable(true)
	c.logPanel.SetBackgroundColor(tcell.ColorBlack)
	c.logPanel.SetBorder(false)
	c.logPanel.SetText("[blue::b]INFO[white::-] Plugin initialized")

	// Table view with styling to match Redis plugin
	c.table = tview.NewTable()
	c.table.SetBorders(false)
	c.table.SetSelectable(true, false)
	c.table.SetBackgroundColor(tcell.ColorBlack)
	c.table.SetBorderColor(tcell.ColorAqua)
	c.table.Box.SetBackgroundColor(tcell.ColorBlack)
	c.table.Box.SetBorderAttributes(tcell.AttrNone)
	c.table.SetBorder(false) // Remove border to match Redis style

	// Set selection style to match Redis plugin
	c.table.SetSelectedStyle(
		tcell.StyleDefault.
			Foreground(tcell.ColorBlack).
			Background(tcell.ColorAqua).
			Attributes(tcell.AttrBold),
	)

	// Set a title that matches Redis plugin style
	c.table.SetTitle(fmt.Sprintf(" [yellow]%s[white] ", c.title))
	c.table.SetTitleAlign(tview.AlignCenter)
	c.table.SetTitleColor(tcell.ColorYellow)

	// Update selection whenever cursor moves to a new row
	c.table.SetSelectionChangedFunc(func(row, column int) {
		if row <= 0 { // Ignore header row
			return
		}
		// Update the selected row index without triggering the full selection event
		if row-1 < len(c.tableData) {
			c.selectedRow = row - 1 // Adjust for header row
		}
	})

	// Keep this for backwards compatibility with code that expects Enter to "confirm" selection
	c.table.SetSelectedFunc(func(row, column int) {
		if row <= 0 { // Ignore header row
			return
		}
		// This now just calls the full highlightRow function for visual emphasis
		// and triggers any registered callbacks
		c.selectRow(row - 1) // Adjust for header row
	})

	// Build a header row with no borders to match Redis style
	headerRow := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(c.infoPanel, 0, 1, false).
		AddItem(c.helpPanel, 0, 1, false).
		AddItem(c.logPanel, 0, 1, false)
	headerRow.SetBackgroundColor(tcell.ColorBlack)

	// Create separator like Redis plugin
	separator := tview.NewBox().
		SetBackgroundColor(tcell.ColorBlack).
		SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
			// Draw a horizontal line
			for i := 0; i < width; i++ {
				screen.SetContent(x+i, y, tcell.RuneHLine, nil, tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorAqua))
			}
			return x, y, width, height
		})

	// Create main layout with header, separator, table, and breadcrumbs at the bottom
	c.mainLayout = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(headerRow, 6, 0, false).
		AddItem(separator, 1, 0, false).
		AddItem(c.table, 0, 1, true).
		AddItem(c.breadcrumbs, 1, 0, false)
	c.mainLayout.SetBackgroundColor(tcell.ColorBlack)
	c.mainLayout.SetBorder(false)
}

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

// RegisterHandlers registers keyboard input handlers
func (c *Cores) RegisterHandlers() {
	// Save the current input capture if there is one
	oldCapture := c.app.GetInputCapture()

	// Set up a new input capture that handles our keys and passes through others
	c.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Handle key events for this plugin
		switch event.Key() {
		case tcell.KeyEscape:
			// Pass to original handler
			if oldCapture != nil {
				return oldCapture(event)
			}
			return event

		case tcell.KeyRune:
			switch event.Rune() {
			case 'R', 'r':
				c.RefreshData()
				return nil
			case '?':
				// First check if there's a custom handler for the '?' key
				if c.onAction != nil {
					handled := c.onAction("keypress", map[string]interface{}{
						"key": "?",
					})
					// If the custom handler returns non-nil, it means it handled the key
					if handled == nil {
						return nil
					}
				}

				// Default behavior - toggle help expanded/collapsed
				if c.helpPanel.GetTitle() == "Keybindings" {
					c.helpPanel.SetTitle("Keybindings (expanded)")
				} else {
					c.helpPanel.SetTitle("Keybindings")
				}
				return nil
			default:
				// Check for custom key bindings
				key := string(event.Rune())
				if _, exists := c.keyBindings[key]; exists {
					if c.onAction != nil {
						c.onAction("keypress", map[string]interface{}{
							"key": key,
						})
					}
					return nil
				}
			}
		}

		// Pass through to previous handler if we didn't handle it
		if oldCapture != nil {
			return oldCapture(event)
		}
		return event
	})
}

// UnregisterHandlers removes keyboard input handlers
// This should be called when switching away from this plugin
func (c *Cores) UnregisterHandlers() {
	// This would need to be integrated with your main app's focus management
	// Typically restores the previous input capture
}

// selectRow selects a table row by its data index
func (c *Cores) selectRow(dataRow int) {
	// No need to clear selection since we'll just apply highlighting
	// where needed. The selectedRow is now set by SetSelectionChangedFunc.

	// Apply highlighting to this row
	if dataRow >= 0 && dataRow < len(c.tableData) {
		c.highlightRow(dataRow+1, true) // +1 for header row

		// Trigger callback if set
		if c.onRowSelected != nil {
			c.onRowSelected(dataRow)
		}

		// Trigger action callback if set
		if c.onAction != nil {
			selectedData := c.tableData[dataRow]
			payload := make(map[string]interface{})
			payload["rowIndex"] = dataRow
			payload["rowData"] = selectedData

			// Also include named data if we have headers
			if len(c.tableHeaders) > 0 {
				namedData := make(map[string]string)
				for i, header := range c.tableHeaders {
					if i < len(selectedData) {
						namedData[header] = selectedData[i]
					}
				}
				payload["namedData"] = namedData
			}

			c.onAction("rowSelected", payload)
		}
	}
}

// highlightRow visually highlights or unhighlights a row
func (c *Cores) highlightRow(row int, highlight bool) {
	for col := 0; col < len(c.tableHeaders); col++ {
		cell := c.table.GetCell(row, col)
		if highlight {
			// Remove the background color highlight but keep text slightly brighter
			cell.SetBackgroundColor(tcell.ColorDefault)
			cell.SetTextColor(tcell.ColorWhite).SetAttributes(tcell.AttrBold)
		} else {
			cell.SetBackgroundColor(tcell.ColorDefault)
			cell.SetAttributes(tcell.AttrNone)
			cell.SetTextColor(tcell.ColorWhite)
		}
	}
}

// clearSelection clears the current row selection
func (c *Cores) clearSelection() {
	if c.selectedRow >= 0 {
		c.highlightRow(c.selectedRow+1, false) // +1 for header row
		c.selectedRow = -1
	}
}

// getRowSignature creates a unique identifier for a row
func (c *Cores) getRowSignature(row []string) string {
	// If a specific selection key column is set, use that
	if c.selectionKey != "" {
		for i, header := range c.tableHeaders {
			if header == c.selectionKey && i < len(row) {
				return row[i]
			}
		}
	}

	// Otherwise use a composite signature from first 3 columns (or fewer if not available)
	var sb strings.Builder
	for i := 0; i < min(3, len(row)); i++ {
		sb.WriteString(row[i])
		sb.WriteString("|")
	}
	return sb.String()
}

// refreshTable updates the table display with current data
func (c *Cores) refreshTable() {
	// Save current selection signature for restoring later
	var selectedSignature string
	if c.selectedRow >= 0 && c.selectedRow < len(c.tableData) {
		selectedSignature = c.getRowSignature(c.tableData[c.selectedRow])
	}

	// Clear the table
	c.table.Clear()

	// Add headers - convert to uppercase
	for i, header := range c.tableHeaders {
		headerText := strings.ToUpper(header)

		cell := tview.NewTableCell(headerText).
			SetTextColor(tcell.ColorYellow).
			SetBackgroundColor(tcell.ColorBlack).
			SetAttributes(tcell.AttrBold).
			SetSelectable(false)

		// First column gets more space, others share equally
		if i == 0 {
			cell.SetExpansion(2) // Give first column more space
		} else {
			cell.SetExpansion(1) // Other columns share equally
		}

		c.table.SetCell(0, i, cell)
	}

	// Add data rows
	for rowIdx, row := range c.tableData {
		for colIdx, cellData := range row {
			if colIdx >= len(c.tableHeaders) {
				continue
			}

			cell := tview.NewTableCell(cellData).
				SetTextColor(tcell.ColorAqua).
				SetBackgroundColor(tcell.ColorBlack).
				SetSelectable(true).
				SetAlign(tview.AlignLeft)

			// First column gets more space, others share equally
			if colIdx == 0 {
				cell.SetExpansion(2) // Give first column more space
			} else {
				cell.SetExpansion(1) // Other columns share equally
			}

			c.table.SetCell(rowIdx+1, colIdx, cell)
		}
	}

	// Try to restore selection by matching signature
	if selectedSignature != "" {
		for i, row := range c.tableData {
			if c.getRowSignature(row) == selectedSignature {
				c.selectRow(i)
				c.table.Select(i+1, 0) // +1 for header
				break
			}
		}
	}
}

//
// Public API
//

// GetLayout returns the main layout component to be embedded in the application
func (c *Cores) GetLayout() tview.Primitive {
	return c.mainLayout
}

// SetInfoText updates the content of the info panel
func (c *Cores) SetInfoText(text string) *Cores {
	c.infoPanel.SetText(text)
	return c
}

// SetInfoTitle updates the title of the info panel
func (c *Cores) SetInfoTitle(title string) *Cores {
	c.infoPanel.SetTitle(title)
	return c
}

// Log adds a new message to the log panel
func (c *Cores) Log(message string) *Cores {
	timestamp := time.Now().Format("15:04:05")
	content := c.logPanel.GetText(false)
	if content != "" {
		content += "\n"
	}

	// Format message in Redis style with gray timestamp
	content += fmt.Sprintf("[gray::d]%s[-] %s", timestamp, message)

	c.logPanel.SetText(content)
	c.logPanel.ScrollToEnd()
	return c
}

// ClearLogs clears all log messages
func (c *Cores) ClearLogs() *Cores {
	c.logPanel.SetText("")
	return c
}

// SetTableHeaders sets the headers for the table
func (c *Cores) SetTableHeaders(headers []string) *Cores {
	c.tableHeaders = headers
	c.refreshTable()
	return c
}

// SetTableData sets the data for the table
func (c *Cores) SetTableData(data [][]string) *Cores {
	c.tableData = data
	c.refreshTable()
	return c
}

// SetRefreshCallback sets a function to be called when refresh is triggered
func (c *Cores) SetRefreshCallback(callback func() ([][]string, error)) *Cores {
	c.onRefresh = callback
	return c
}

// StartAutoRefresh starts automatic refreshing at the given interval
func (c *Cores) StartAutoRefresh(interval time.Duration) *Cores {
	c.refreshMutex.Lock()
	defer c.refreshMutex.Unlock()

	// Stop any existing refresh
	if c.refreshTicker != nil {
		c.StopAutoRefresh()
	}

	c.refreshTicker = time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-c.refreshTicker.C:
				c.RefreshData()
			case <-c.stopRefresh:
				return
			}
		}
	}()

	c.Log(fmt.Sprintf("Auto-refresh enabled (%s)", interval))
	return c
}

// StopAutoRefresh stops the automatic refresh
func (c *Cores) StopAutoRefresh() *Cores {
	c.refreshMutex.Lock()
	defer c.refreshMutex.Unlock()

	if c.refreshTicker != nil {
		c.refreshTicker.Stop()
		c.refreshTicker = nil
		close(c.stopRefresh)
		c.stopRefresh = make(chan struct{})
		c.Log("Auto-refresh disabled")
	}

	return c
}

// RefreshData manually triggers a refresh of the data
func (c *Cores) RefreshData() *Cores {
	if c.onRefresh != nil {
		c.Log("Refreshing data...")
		data, err := c.onRefresh()
		if err != nil {
			c.Log(fmt.Sprintf("[red]Error refreshing data: %v", err))
		} else {
			c.SetTableData(data)
			c.Log("[green]Data refreshed successfully")
		}
	}
	return c
}

// AddKeyBinding adds a custom key binding with description
func (c *Cores) AddKeyBinding(key string, description string, handler func()) *Cores {
	// Add to the key bindings for help text
	c.keyBindings[key] = description

	// Update help text immediately
	c.helpPanel.SetText(c.getHelpText())

	// The actual handler will be called via the onAction callback
	// which is triggered through the RegisterHandlers method

	return c
}

// SetRowSelectedCallback sets a function to be called when a row is selected
func (c *Cores) SetRowSelectedCallback(callback func(row int)) *Cores {
	c.onRowSelected = callback
	return c
}

// SetActionCallback sets a function to be called for various plugin actions
func (c *Cores) SetActionCallback(callback func(action string, payload map[string]interface{}) error) *Cores {
	c.onAction = callback
	return c
}

// GetSelectedRow returns the index of the currently selected row, or -1 if none
func (c *Cores) GetSelectedRow() int {
	return c.selectedRow
}

// GetSelectedRowData returns the data of the currently selected row, or nil if none
func (c *Cores) GetSelectedRowData() []string {
	if c.selectedRow >= 0 && c.selectedRow < len(c.tableData) {
		return c.tableData[c.selectedRow]
	}
	return nil
}

// GetTableHeaders returns the current table headers
func (c *Cores) GetTableHeaders() []string {
	return c.tableHeaders
}

// GetTableData returns the current table data
func (c *Cores) GetTableData() [][]string {
	return c.tableData
}

// SetTableTitle sets the title of the table
func (c *Cores) SetTableTitle(title string) *Cores {
	c.table.SetTitle(title)
	return c
}

// SetSelectionKey sets a specific column to use for tracking row selections
func (c *Cores) SetSelectionKey(columnName string) *Cores {
	c.selectionKey = columnName
	return c
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
func (c *Cores) GetTable() tview.Primitive {
	return c.table
}

// SetInfoMap updates the info panel with a map of key-value pairs
// Keys will be shown in aqua color, values in white, both in bold
func (c *Cores) SetInfoMap(infoMap map[string]string) *Cores {
	// Build a stylized string from the map
	var sb strings.Builder

	// Sort the keys for consistent display order
	keys := make([]string, 0, len(infoMap))
	for key := range infoMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Find the longest key to align columns
	maxKeyLength := 0
	for _, key := range keys {
		if len(key) > maxKeyLength && infoMap[key] != "" {
			maxKeyLength = len(key)
		}
	}

	// Format each key-value pair with aligned columns
	for i, key := range keys {
		value := infoMap[key]
		if key != "" && value != "" {
			// Calculate padding: longest word + 1 space - current word length
			// This ensures exactly one space after the longest word
			paddingSpaces := maxKeyLength + 1 - len(key)
			padding := strings.Repeat(" ", paddingSpaces)

			sb.WriteString(fmt.Sprintf("[aqua::b]%s:%s[white::b]%s", key, padding, value))

			// Add newline for all but the last item
			if i < len(keys)-1 {
				sb.WriteString("\n")
			}
		}
	}

	// Set the formatted text to the info panel
	c.infoPanel.SetText(sb.String())
	return c
}

// RemovePage is a utility function to remove any modal page with ESC key handling
// This can be used by any modal in the application to ensure consistent behavior
func RemovePage(pages *tview.Pages, app *tview.Application, pageID string, callback func()) {
	// Save original input capture to restore later
	oldCapture := app.GetInputCapture()

	// Add app-level ESC handler that will take precedence
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			// Check if our modal is visible
			if pages.HasPage(pageID) {
				pages.RemovePage(pageID)
				// Restore original input capture
				app.SetInputCapture(oldCapture)
				if callback != nil {
					callback()
				}
				return nil
			}
		}

		// Pass to original handler
		if oldCapture != nil {
			return oldCapture(event)
		}
		return event
	})
}

// updateBreadcrumbs updates the breadcrumb display based on the current navigation stack
func (c *Cores) updateBreadcrumbs() {
	if len(c.navStack) == 0 {
		c.breadcrumbs.SetText("")
		return
	}

	var sb strings.Builder
	for i, view := range c.navStack {
		if i > 0 {
			sb.WriteString(" [yellow]>[white] ")
		}
		if i == len(c.navStack)-1 {
			// Current view in orange
			sb.WriteString(fmt.Sprintf("[black:orange]%s[-:-]", view))
		} else {
			// Previous views in aqua
			sb.WriteString(fmt.Sprintf("[black:aqua]%s[-:-]", view))
		}
	}
	c.breadcrumbs.SetText(sb.String())
}

// PushView adds a view to the navigation stack
func (c *Cores) PushView(view string) {
	c.navStack = append(c.navStack, view)
	c.updateBreadcrumbs()
}

// PopView removes the last view from the navigation stack
func (c *Cores) PopView() string {
	if len(c.navStack) == 0 {
		return ""
	}
	lastView := c.navStack[len(c.navStack)-1]
	c.navStack = c.navStack[:len(c.navStack)-1]
	c.updateBreadcrumbs()
	return lastView
}

// ClearViews removes all views from the navigation stack
func (c *Cores) ClearViews() {
	c.navStack = []string{}
	c.updateBreadcrumbs()
}

// GetCurrentView returns the name of the current view
func (c *Cores) GetCurrentView() string {
	if len(c.navStack) == 0 {
		return ""
	}
	return c.navStack[len(c.navStack)-1]
}

// SetViewStack sets the entire navigation stack at once
func (c *Cores) SetViewStack(stack []string) {
	c.navStack = append([]string{}, stack...)
	c.updateBreadcrumbs()
}

// CopyNavigationStackFrom copies the navigation stack from another Cores instance
func (c *Cores) CopyNavigationStackFrom(other *Cores) {
	c.navStack = append([]string{}, other.navStack...)
	c.updateBreadcrumbs()
}

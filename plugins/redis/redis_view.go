package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rivo/tview"

	"omo/pkg/ui"
)

// RedisView manages the UI for interacting with Redis
type RedisView struct {
	app               *tview.Application
	pages             *tview.Pages
	cores             *ui.Cores
	redisClient       *RedisClient
	currentConnection *RedisConnection
	keys              []string
	currentDatabase   int
	refreshTimer      *time.Timer
	refreshInterval   time.Duration
}

// NewRedisView creates a new Redis view
func NewRedisView(app *tview.Application, pages *tview.Pages) *RedisView {
	rv := &RedisView{
		app:             app,
		pages:           pages,
		currentDatabase: 0,
		keys:            []string{},
		refreshInterval: 10 * time.Second, // Default refresh interval
	}

	// Create Cores UI component
	rv.cores = ui.NewCores(app, "Redis Manager")

	// Set table headers
	rv.cores.SetTableHeaders([]string{"Key", "Type", "TTL", "Size"})

	// Set up refresh callback
	rv.cores.SetRefreshCallback(func() ([][]string, error) {
		// This will be called when RefreshData() is triggered from the core
		return rv.refreshKeys()
	})

	// Add key bindings
	rv.cores.AddKeyBinding("R", "Refresh", rv.refresh)
	// Note: Ctrl+T is handled at the plugin level for consistency
	rv.cores.AddKeyBinding("?", "Help", rv.showHelp)
	rv.cores.AddKeyBinding("D", "Del Key", rv.showDeleteKeyConfirmation)
	rv.cores.AddKeyBinding("F", "Flush DB", rv.showFlushDBConfirmation)
	rv.cores.AddKeyBinding("N", "New Key", rv.showNewKeyForm)

	// Database selection with S key
	rv.cores.AddKeyBinding("S", "DB Select", rv.showDBSelector)
	rv.cores.AddKeyBinding("Enter", "View Key", rv.showSelectedKeyContent)

	// Set action callback
	rv.cores.SetActionCallback(rv.handleAction)

	// Add row selection callback - just log the selection, don't show content automatically
	rv.cores.SetRowSelectedCallback(func(row int) {
		if row >= 0 && row < len(rv.keys) {
			rv.cores.Log(fmt.Sprintf("[blue]Selected key: %s", rv.keys[row]))
		}
	})

	// Register the key handlers
	rv.cores.RegisterHandlers()

	// Initialize Redis client
	rv.redisClient = NewRedisClient()

	// Set initial status
	rv.cores.SetInfoText("[yellow]Redis Manager[white]\nStatus: Not Connected\nUse [green]Ctrl+T[white] to select instance")

	// Start auto-refresh timer
	rv.startAutoRefresh()

	return rv
}

// GetMainUI returns the main UI component
func (rv *RedisView) GetMainUI() tview.Primitive {
	return rv.cores.GetLayout()
}

// ShowConnectionSelector displays the connection selector form
func (rv *RedisView) ShowConnectionSelector() {
	// Debug log to verify this method is called
	rv.cores.Log("[blue]Opening connection selector...")

	// Get available Redis instances from config
	instances, err := GetAvailableInstances()
	if err != nil {
		rv.cores.Log(fmt.Sprintf("[red]Failed to load Redis instances: %v", err))
		// Fall back to manual configuration if no instances are configured
		rv.showManualConnectionForm()
		return
	}

	if len(instances) == 0 {
		rv.cores.Log("[yellow]No Redis instances configured, using manual configuration")
		rv.showManualConnectionForm()
		return
	}

	// Create a list of instance items for the selector
	items := make([][]string, len(instances))
	for i, instance := range instances {
		items[i] = []string{
			instance.Name,
			fmt.Sprintf("%s:%d - %s", instance.Host, instance.Port, instance.Description),
		}
	}

	// Show selection modal - do NOT use QueueUpdate here
	ui.ShowStandardListSelectorModal(
		rv.pages,
		rv.app,
		"Select Redis Instance",
		items,
		func(index int, name string, cancelled bool) {
			// Always return focus to the table, whether cancelled or selected
			if !cancelled && index >= 0 && index < len(instances) {
				// Connect to the selected instance
				rv.connectToRedisInstance(instances[index])
			} else {
				// Log that selection was cancelled
				rv.cores.Log("[blue]Connection selection cancelled")
			}

			// Always return focus to the table
			rv.app.SetFocus(rv.cores.GetTable())
		},
	)
}

// showConnectionForm shows a form for one connection parameter
func (rv *RedisView) showConnectionForm(items [][]string, index int) {
	if index >= len(items) {
		// All fields collected, connect to Redis
		host := items[0][1]
		port := items[1][1]
		password := items[2][1]
		dbStr := items[3][1]

		// Parse database number
		db, err := strconv.Atoi(dbStr)
		if err != nil {
			rv.cores.Log(fmt.Sprintf("[red]Invalid database number: %s", dbStr))
			return
		}

		rv.connectToRedis(host, port, password, db)
		return
	}

	// Show input modal for current field
	label := items[index][0]
	defaultValue := items[index][1]

	ui.ShowCompactStyledInputModal(
		rv.pages,
		rv.app,
		fmt.Sprintf("Redis Connection - %s", label),
		label,
		defaultValue,
		20,
		nil,
		func(value string, cancelled bool) {
			if cancelled {
				// Return focus to table
				rv.app.SetFocus(rv.cores.GetTable())
				return
			}

			// Store the value and move to next field
			items[index][1] = value
			rv.showConnectionForm(items, index+1)
		},
	)
}

// connectToRedis attempts to connect to a Redis instance
func (rv *RedisView) connectToRedis(host, port, password string, db int) {
	// Create a new Redis client if needed
	if rv.redisClient == nil {
		rv.redisClient = NewRedisClient()
	}

	// Connect to Redis
	err := rv.redisClient.Connect(host, port, password, db)
	if err != nil {
		rv.cores.Log(fmt.Sprintf("[red]Failed to connect to Redis: %v", err))
		return
	}

	rv.currentConnection = rv.redisClient.GetCurrentConnection()
	rv.currentDatabase = db

	// Update the UI
	rv.cores.SetInfoText(fmt.Sprintf("[green]Redis Manager[white]\nServer: %s:%s\nDB: %d\nStatus: Connected",
		host, port, db))
	rv.cores.Log(fmt.Sprintf("[green]Connected to Redis at %s:%s, DB: %d", host, port, db))

	// Refresh keys
	rv.refresh()
}

// refreshKeys updates the table with keys from Redis
func (rv *RedisView) refreshKeys() ([][]string, error) {
	if rv.redisClient == nil || !rv.redisClient.IsConnected() {
		// No client or not connected, show empty data
		rv.keys = []string{}
		rv.cores.SetTableData([][]string{})
		rv.cores.SetInfoText("[yellow]Redis Manager[white]\nStatus: Not Connected\nUse [green]Ctrl+T[white] to select instance")
		return [][]string{}, nil
	}

	// Get keys from Redis client
	keys, err := rv.redisClient.GetKeys("*")
	if err != nil {
		rv.cores.Log(fmt.Sprintf("[red]Failed to get keys: %v", err))

		// Check if we need to reset the connection
		if strings.Contains(err.Error(), "connection") ||
			strings.Contains(err.Error(), "timeout") {
			rv.cores.Log("[red]Connection to Redis lost. Please reconnect.")
			// Mark as disconnected
			if rv.redisClient != nil {
				rv.redisClient.Disconnect()
			}
			rv.cores.SetInfoText("[yellow]Redis Manager[white]\nStatus: Not Connected\nUse [green]Ctrl+T[white] to select instance")
		}
		return [][]string{}, err
	}

	// Prepare table data
	tableData := make([][]string, 0, len(keys))
	rv.keys = make([]string, 0, len(keys))

	for _, key := range keys {
		// Get key info
		keyInfo, err := rv.redisClient.GetKeyInfo(key)
		if err != nil {
			continue
		}

		keyType := keyInfo["type"]
		ttl := keyInfo["ttl"]
		size := keyInfo["size"]

		tableData = append(tableData, []string{key, keyType, ttl, size})
		rv.keys = append(rv.keys, key)
	}

	// Update the table
	rv.cores.SetTableData(tableData)

	// Update last refresh time
	rv.redisClient.SetLastRefreshTime(time.Now())

	// Update info text
	if rv.currentConnection != nil {
		rv.cores.SetInfoText(fmt.Sprintf("[green]Redis Manager[white]\nServer: %s:%s\nDB: %d\nStatus: Connected",
			rv.currentConnection.Host, rv.currentConnection.Port, rv.currentDatabase))
	}

	return tableData, nil
}

// refresh manually refreshes the keys
func (rv *RedisView) refresh() {
	rv.cores.RefreshData()
}

// showKeyContent displays the content of a Redis key
func (rv *RedisView) showKeyContent(key string) {
	if rv.redisClient == nil || !rv.redisClient.IsConnected() {
		return
	}

	// Get content
	content, err := rv.redisClient.GetKeyContent(key)
	if err != nil {
		rv.cores.Log(fmt.Sprintf("[red]Failed to get content for key %s: %v", key, err))
		return
	}

	// Show content in info modal
	ui.ShowInfoModal(
		rv.pages,
		rv.app,
		fmt.Sprintf("Key: %s", key),
		content,
		func() {
			rv.app.SetFocus(rv.cores.GetTable())
		},
	)
}

// showHelp displays Redis plugin help
func (rv *RedisView) showHelp() {
	helpText := `
[yellow]Redis Manager Help[white]

[green]Key Bindings:[white]
R       - Refresh keys list
Ctrl+T  - Connect to Redis
?       - Show this help
D       - Delete selected key
F       - Flush current database
N       - Create new key
S       - Select database (0-15)
Enter   - View key content

[green]Navigation:[white]
Arrow keys - Navigate keys list
Enter      - View key content
Esc        - Close modal dialogs
`

	ui.ShowInfoModal(
		rv.pages,
		rv.app,
		"Redis Help",
		helpText,
		func() {
			rv.app.SetFocus(rv.cores.GetTable())
		},
	)
}

// handleAction handles actions triggered by the UI
func (rv *RedisView) handleAction(action string, payload map[string]interface{}) error {
	switch action {
	case "refresh":
		rv.refresh()
		return nil
	case "keypress":
		// Handle specific key presses
		if key, ok := payload["key"].(string); ok {
			switch key {
			case "?":
				rv.showHelp()
				return nil
			case "R":
				rv.refresh()
				return nil
			case "D":
				rv.showDeleteKeyConfirmation()
				return nil
			case "F":
				rv.showFlushDBConfirmation()
				return nil
			case "N":
				rv.showNewKeyForm()
				return nil
			case "S":
				rv.showDBSelector()
				return nil
			case "Enter":
				rv.showSelectedKeyContent()
				return nil
			}
		}
	}
	return nil
}

// selectDatabase switches to the specified Redis database
func (rv *RedisView) selectDatabase(db int) {
	if rv.redisClient == nil || !rv.redisClient.IsConnected() {
		rv.cores.Log("[yellow]Not connected to Redis")
		return
	}

	// Validate database number
	if db < 0 || db > 15 {
		rv.cores.Log("[red]Invalid database number. Must be 0-15.")
		return
	}

	// Select the database
	rv.cores.Log(fmt.Sprintf("[blue]Selecting database %d...", db))
	if err := rv.redisClient.SelectDB(db); err != nil {
		rv.cores.Log(fmt.Sprintf("[red]Failed to select database: %v", err))
	} else {
		// Update the current database in the view
		rv.currentDatabase = db

		// Log success before updating connection
		rv.cores.Log(fmt.Sprintf("[green]Selected database: %d", db))

		// Update connection info to reflect the new database
		if rv.currentConnection != nil {
			rv.currentConnection.Database = db

			// Update the UI header
			rv.cores.SetInfoText(fmt.Sprintf("[green]Redis Manager[white]\nServer: %s:%s\nDB: %d\nStatus: Connected",
				rv.currentConnection.Host, rv.currentConnection.Port, db))
		}

		// Refresh keys to show data from the new database
		rv.refresh()
	}
}

// showDeleteKeyConfirmation shows confirmation dialog for key deletion
func (rv *RedisView) showDeleteKeyConfirmation() {
	selectedRow := rv.cores.GetSelectedRow()
	if selectedRow < 0 || selectedRow >= len(rv.keys) {
		rv.cores.Log("[yellow]No key selected")
		return
	}

	key := rv.keys[selectedRow]

	ui.ShowStandardConfirmationModal(
		rv.pages,
		rv.app,
		"Delete Key",
		fmt.Sprintf("Are you sure you want to delete the key '[red]%s[white]'?", key),
		func(confirmed bool) {
			if confirmed {
				// Delete the key
				if err := rv.redisClient.DeleteKey(key); err != nil {
					rv.cores.Log(fmt.Sprintf("[red]Failed to delete key: %v", err))
				} else {
					rv.cores.Log(fmt.Sprintf("[yellow]Deleted key: %s", key))
					rv.refresh()
				}
			}
			// Return focus to the table
			rv.app.SetFocus(rv.cores.GetTable())
		},
	)
}

// showFlushDBConfirmation shows confirmation dialog for flushing the database
func (rv *RedisView) showFlushDBConfirmation() {
	if rv.redisClient == nil || !rv.redisClient.IsConnected() {
		rv.cores.Log("[yellow]Not connected to Redis")
		return
	}

	ui.ShowStandardConfirmationModal(
		rv.pages,
		rv.app,
		"Flush Database",
		fmt.Sprintf("Are you sure you want to [red]FLUSH[white] database [red]%d[white]?\nThis will delete [red]ALL[white] keys in the database!", rv.currentDatabase),
		func(confirmed bool) {
			if confirmed {
				// Flush the database
				if err := rv.redisClient.FlushDB(); err != nil {
					rv.cores.Log(fmt.Sprintf("[red]Failed to flush database: %v", err))
				} else {
					rv.cores.Log(fmt.Sprintf("[red]Flushed database %d", rv.currentDatabase))
					rv.refresh()
				}
			}
			// Return focus to the table
			rv.app.SetFocus(rv.cores.GetTable())
		},
	)
}

// showNewKeyForm shows a form to create a new key
func (rv *RedisView) showNewKeyForm() {
	if rv.redisClient == nil || !rv.redisClient.IsConnected() {
		rv.cores.Log("[yellow]Not connected to Redis")
		return
	}

	// First get the key name
	ui.ShowCompactStyledInputModal(
		rv.pages,
		rv.app,
		"New Key",
		"Key Name",
		"",
		30,
		nil,
		func(key string, cancelled bool) {
			if cancelled || key == "" {
				rv.app.SetFocus(rv.cores.GetTable())
				if key == "" && !cancelled {
					rv.cores.Log("[yellow]Key name cannot be empty")
				}
				return
			}

			// Now get the value
			ui.ShowCompactStyledInputModal(
				rv.pages,
				rv.app,
				"New Key Value",
				"Value",
				"",
				30,
				nil,
				func(value string, cancelled bool) {
					if cancelled {
						rv.app.SetFocus(rv.cores.GetTable())
						return
					}

					// Set the key
					if err := rv.redisClient.SetKey(key, value, -1); err != nil {
						rv.cores.Log(fmt.Sprintf("[red]Failed to set key: %v", err))
					} else {
						rv.cores.Log(fmt.Sprintf("[green]Created key: %s", key))
						rv.refresh()
					}
					rv.app.SetFocus(rv.cores.GetTable())
				},
			)
		},
	)
}

// showDBSelector shows a form to select a database
func (rv *RedisView) showDBSelector() {
	if rv.redisClient == nil || !rv.redisClient.IsConnected() {
		rv.cores.Log("[yellow]Not connected to Redis")
		return
	}

	ui.ShowCompactStyledInputModal(
		rv.pages,
		rv.app,
		"Select Database",
		"Database Number (0-15)",
		strconv.Itoa(rv.currentDatabase),
		5,
		nil,
		func(dbStr string, cancelled bool) {
			if cancelled {
				rv.app.SetFocus(rv.cores.GetTable())
				return
			}

			// Parse database number
			db, err := strconv.Atoi(dbStr)
			if err != nil || db < 0 || db > 15 {
				rv.cores.Log("[red]Invalid database number. Must be 0-15.")
				rv.app.SetFocus(rv.cores.GetTable())
				return
			}

			// Select the database
			if err := rv.redisClient.SelectDB(db); err != nil {
				rv.cores.Log(fmt.Sprintf("[red]Failed to select database: %v", err))
			} else {
				rv.currentDatabase = db
				rv.cores.Log(fmt.Sprintf("[green]Selected database: %d", db))
				rv.cores.SetInfoText(fmt.Sprintf("[green]Redis Manager[white]\nServer: %s:%s\nDB: %d\nStatus: Connected",
					rv.currentConnection.Host, rv.currentConnection.Port, db))
				rv.refresh()
			}
			rv.app.SetFocus(rv.cores.GetTable())
		},
	)
}

// showManualConnectionForm shows the manual connection form
func (rv *RedisView) showManualConnectionForm() {
	// Create input fields for the connection form
	items := [][]string{
		{"Host", "localhost"},
		{"Port", "6379"},
		{"Password", ""},
		{"Database", "0"},
	}

	// Use CompactInputModal for each field
	rv.showConnectionForm(items, 0)
}

// startAutoRefresh sets up and starts the auto-refresh timer
func (rv *RedisView) startAutoRefresh() {
	// Load the refresh interval from config
	if uiConfig, err := GetUIConfig(); err == nil {
		rv.refreshInterval = time.Duration(uiConfig.RefreshInterval) * time.Second
	}

	// Cancel any existing timer
	if rv.refreshTimer != nil {
		rv.refreshTimer.Stop()
	}

	// Create a new timer
	rv.refreshTimer = time.AfterFunc(rv.refreshInterval, func() {
		// Use a direct refresh call without QueueUpdateDraw to prevent freezing
		if rv.redisClient != nil && rv.redisClient.IsConnected() {
			// We need to use QueueUpdate because we're in a goroutine
			rv.app.QueueUpdate(func() {
				rv.refresh()
				// Restart the timer for next refresh
				rv.startAutoRefresh()
			})
		} else {
			// Just restart the timer without refreshing
			rv.startAutoRefresh()
		}
	})
}

// connectToRedisInstance connects to a preconfigured Redis instance
func (rv *RedisView) connectToRedisInstance(instance RedisInstance) {
	// Create a new Redis client if needed
	if rv.redisClient == nil {
		rv.redisClient = NewRedisClient()
	}

	// Set status to connecting
	rv.cores.SetInfoText(fmt.Sprintf("[yellow]Redis Manager[white]\nServer: %s:%d\nStatus: Connecting...",
		instance.Host, instance.Port))

	// Connect to Redis
	err := rv.redisClient.ConnectToInstance(instance)
	if err != nil {
		rv.cores.Log(fmt.Sprintf("[red]Connection failed: %v", err))
		rv.cores.SetInfoText("[yellow]Redis Manager[white]\nStatus: Not Connected\nUse [green]Ctrl+T[white] to select instance")
		return
	}

	rv.currentConnection = rv.redisClient.GetCurrentConnection()
	rv.currentDatabase = instance.Database

	// Update the UI
	rv.cores.SetInfoText(fmt.Sprintf("[green]Redis Manager[white]\nServer: %s:%d\nDB: %d\nStatus: Connected",
		instance.Host, instance.Port, instance.Database))
	rv.cores.Log(fmt.Sprintf("[green]Connected to %s:%d", instance.Host, instance.Port))

	// Refresh keys immediately
	rv.refresh()

	// Ensure auto-refresh is running
	rv.startAutoRefresh()
}

// showSelectedKeyContent shows the content of the currently selected key
func (rv *RedisView) showSelectedKeyContent() {
	selectedRow := rv.cores.GetSelectedRow()
	if selectedRow < 0 || selectedRow >= len(rv.keys) {
		rv.cores.Log("[yellow]No key selected")
		return
	}

	key := rv.keys[selectedRow]
	rv.showKeyContent(key)
}

// AutoConnectToDefaultInstance automatically connects to the default Redis instance
func (rv *RedisView) AutoConnectToDefaultInstance() {
	// Get available Redis instances from config
	instances, err := GetAvailableInstances()
	if err != nil {
		rv.cores.Log(fmt.Sprintf("[yellow]Failed to load Redis instances: %v", err))
		return
	}

	if len(instances) == 0 {
		rv.cores.Log("[yellow]No Redis instances configured in config/redis.yaml")
		return
	}

	// Connect to the first instance in the list
	rv.cores.Log(fmt.Sprintf("[blue]Auto-connecting to Redis instance: %s", instances[0].Name))
	rv.connectToRedisInstance(instances[0])
}

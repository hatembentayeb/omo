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
	viewPages         *tview.Pages
	cores             *ui.Cores
	keysView          *ui.Cores
	infoView          *ui.Cores
	slowlogView       *ui.Cores
	statsView         *ui.Cores
	clientsView       *ui.Cores
	configView        *ui.Cores
	memoryView        *ui.Cores
	replicationView   *ui.Cores
	persistenceView   *ui.Cores
	redisClient       *RedisClient
	currentConnection *RedisConnection
	keys              []string
	currentDatabase   int
	currentView       string
	refreshTimer      *time.Timer
	refreshInterval   time.Duration
}

// NewRedisView creates a new Redis view
func NewRedisView(app *tview.Application, pages *tview.Pages) *RedisView {
	rv := &RedisView{
		app:             app,
		pages:           pages,
		viewPages:       tview.NewPages(),
		currentDatabase: 0,
		keys:            []string{},
		refreshInterval: 10 * time.Second, // Default refresh interval
	}

	// Create Cores UI component
	rv.keysView = ui.NewCores(app, "Redis Manager")
	rv.cores = rv.keysView

	// Set table headers
	rv.keysView.SetTableHeaders([]string{"Key", "Type", "TTL", "Size"})

	// Set up refresh callback
	rv.keysView.SetRefreshCallback(func() ([][]string, error) {
		// This will be called when RefreshData() is triggered from the core
		return rv.refreshKeys()
	})

	// Add key bindings
	rv.keysView.AddKeyBinding("R", "Refresh", rv.refresh)
	// Note: Ctrl+T is handled at the plugin level for consistency
	rv.keysView.AddKeyBinding("?", "Help", rv.showHelp)
	rv.keysView.AddKeyBinding("D", "Del Key", rv.showDeleteKeyConfirmation)
	rv.keysView.AddKeyBinding("F", "Flush DB", rv.showFlushDBConfirmation)
	rv.keysView.AddKeyBinding("N", "New Key", rv.showNewKeyForm)
	rv.keysView.AddKeyBinding("E", "View Key", rv.showSelectedKeyContent)
	rv.keysView.AddKeyBinding("I", "Server Info", rv.showServerInfo)
	rv.keysView.AddKeyBinding("L", "Slowlog", rv.showSlowlog)
	rv.keysView.AddKeyBinding("T", "Stats", rv.showStats)
	rv.keysView.AddKeyBinding("C", "Clients", rv.showClients)
	rv.keysView.AddKeyBinding("G", "Config", rv.showConfig)
	rv.keysView.AddKeyBinding("M", "Memory", rv.showMemory)
	rv.keysView.AddKeyBinding("P", "Persistence", rv.showPersistence)
	rv.keysView.AddKeyBinding("Y", "Replication", rv.showReplication)

	// Database selection with S key
	rv.keysView.AddKeyBinding("S", "DB Select", rv.showDBSelector)

	// Set action callback
	rv.keysView.SetActionCallback(rv.handleAction)

	// Add row selection callback - just log the selection, don't show content automatically
	rv.keysView.SetRowSelectedCallback(func(row int) {
		if row >= 0 && row < len(rv.keys) {
			rv.keysView.Log(fmt.Sprintf("[blue]Selected key: %s", rv.keys[row]))
		}
	})

	rv.keysView.GetTable().SetSelectedFunc(func(row, column int) {
		if row <= 0 {
			return
		}
		rv.showSelectedKeyContent()
	})

	// Register the key handlers
	rv.keysView.RegisterHandlers()

	// Initialize Redis client
	rv.redisClient = NewRedisClient()

	// Set initial status
	rv.keysView.SetInfoText("[yellow]Redis Manager[white]\nStatus: Not Connected\nUse [green]Ctrl+T[white] to select instance")

	rv.infoView = rv.newInfoView()
	rv.slowlogView = rv.newSlowlogView()
	rv.statsView = rv.newStatsView()
	rv.clientsView = rv.newClientsView()
	rv.configView = rv.newConfigView()
	rv.memoryView = rv.newMemoryView()
	rv.replicationView = rv.newReplicationView()
	rv.persistenceView = rv.newPersistenceView()

	rv.viewPages.AddPage("redis-keys", rv.keysView.GetLayout(), true, true)
	rv.viewPages.AddPage("redis-info", rv.infoView.GetLayout(), true, false)
	rv.viewPages.AddPage("redis-slowlog", rv.slowlogView.GetLayout(), true, false)
	rv.viewPages.AddPage("redis-stats", rv.statsView.GetLayout(), true, false)
	rv.viewPages.AddPage("redis-clients", rv.clientsView.GetLayout(), true, false)
	rv.viewPages.AddPage("redis-config", rv.configView.GetLayout(), true, false)
	rv.viewPages.AddPage("redis-memory", rv.memoryView.GetLayout(), true, false)
	rv.viewPages.AddPage("redis-replication", rv.replicationView.GetLayout(), true, false)
	rv.viewPages.AddPage("redis-persistence", rv.persistenceView.GetLayout(), true, false)
	rv.currentView = viewKeys
	rv.setViewStack(rv.keysView, viewKeys)
	rv.setViewStack(rv.infoView, viewInfo)
	rv.setViewStack(rv.slowlogView, viewSlowlog)
	rv.setViewStack(rv.statsView, viewStats)
	rv.setViewStack(rv.clientsView, viewClients)
	rv.setViewStack(rv.configView, viewConfig)
	rv.setViewStack(rv.memoryView, viewMemory)
	rv.setViewStack(rv.replicationView, viewRepl)
	rv.setViewStack(rv.persistenceView, viewPersist)

	// Start auto-refresh timer
	rv.startAutoRefresh()

	return rv
}

// GetMainUI returns the main UI component
func (rv *RedisView) GetMainUI() tview.Primitive {
	return rv.viewPages
}

// Stop cleans up resources when the view is no longer used.
func (rv *RedisView) Stop() {
	if rv.refreshTimer != nil {
		rv.refreshTimer.Stop()
	}

	if rv.redisClient != nil && rv.redisClient.IsConnected() {
		rv.redisClient.Disconnect()
	}

	views := []*ui.Cores{
		rv.keysView,
		rv.infoView,
		rv.slowlogView,
		rv.statsView,
		rv.clientsView,
		rv.configView,
		rv.memoryView,
		rv.replicationView,
		rv.persistenceView,
	}
	for _, view := range views {
		if view != nil {
			view.StopAutoRefresh()
			view.UnregisterHandlers()
		}
	}
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
	currentView := rv.currentCores()
	if currentView != nil {
		currentView.RefreshData()
	}
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
E       - View key content
I       - Server info view
L       - Slowlog view
T       - Stats view
K       - Keys view
C       - Clients view
G       - Config view
M       - Memory view
P       - Persistence view
Y       - Replication view
D       - Memory Doctor (memory view)

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
			current := rv.currentCores()
			if current != nil {
				rv.app.SetFocus(current.GetTable())
			}
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
			case "I":
				rv.showServerInfo()
				return nil
			case "L":
				rv.showSlowlog()
				return nil
			case "T":
				rv.showStats()
				return nil
			case "C":
				rv.showClients()
				return nil
			case "G":
				rv.showConfig()
				return nil
			case "M":
				rv.showMemory()
				return nil
			case "P":
				rv.showPersistence()
				return nil
			case "Y":
				rv.showReplication()
				return nil
			case "K":
				rv.switchView(viewKeys)
				return nil
			case "?":
				rv.showHelp()
				return nil
			case "R":
				rv.refresh()
				return nil
			case "D":
				if rv.currentView == viewKeys {
					rv.showDeleteKeyConfirmation()
					return nil
				}
				if rv.currentView == viewMemory {
					rv.showMemoryDoctor()
					return nil
				}
			case "F":
				if rv.currentView == viewKeys {
					rv.showFlushDBConfirmation()
					return nil
				}
			case "N":
				if rv.currentView == viewKeys {
					rv.showNewKeyForm()
					return nil
				}
			case "S":
				if rv.currentView == viewKeys {
					rv.showDBSelector()
					return nil
				}
			case "E":
				if rv.currentView == viewKeys {
					rv.showSelectedKeyContent()
					return nil
				}
			case "Enter":
				if rv.currentView == viewKeys {
					rv.showSelectedKeyContent()
					return nil
				}
			}
		}
	case "navigate_back":
		if view, ok := payload["current_view"].(string); ok {
			if view == viewRoot {
				// Stay on keys when reaching root breadcrumb.
				rv.switchView(viewKeys)
				return nil
			}
			rv.switchView(view)
			return nil
		}
	}
	return fmt.Errorf("unhandled")
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

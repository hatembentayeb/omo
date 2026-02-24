package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/rivo/tview"

	"omo/pkg/ui"
)

// PostgresView manages the UI for interacting with PostgreSQL
type PostgresView struct {
	app               *tview.Application
	pages             *tview.Pages
	viewPages         *tview.Pages
	usersView         *ui.CoreView
	databasesView     *ui.CoreView
	tablesView        *ui.CoreView
	schemasView       *ui.CoreView
	extensionsView    *ui.CoreView
	connectionsView   *ui.CoreView
	statsView         *ui.CoreView
	configView        *ui.CoreView
	logsView          *ui.CoreView
	locksView         *ui.CoreView
	indexesView       *ui.CoreView
	replicationView   *ui.CoreView
	tablespacesView   *ui.CoreView
	dbStatsView       *ui.CoreView
	pgClient          *PostgresClient
	currentConnection *PostgresConnection
	currentView       string
	refreshTimer      *time.Timer
	refreshInterval   time.Duration
}

// NewPostgresView creates a new PostgreSQL view
func NewPostgresView(app *tview.Application, pages *tview.Pages) *PostgresView {
	pv := &PostgresView{
		app:             app,
		pages:           pages,
		viewPages:       tview.NewPages(),
		refreshInterval: 10 * time.Second,
	}

	// --- Users view (primary/default) ---
	pv.usersView = ui.NewCoreView(app, "PostgreSQL Users")
	pv.usersView.SetSelectionKey("User")
	pv.usersView.SetTableHeaders([]string{"User", "Login", "Super", "CreateDB", "CreateRole", "Repl", "Conns", "Member Of", "Valid Until"})

	pv.usersView.SetRefreshCallback(pv.refreshUsers)

	pv.usersView.AddKeyBinding("R", "Refresh", pv.refresh)
	pv.usersView.AddKeyBinding("?", "Help", pv.showHelp)
	pv.usersView.AddKeyBinding("N", "New User", pv.showCreateUserForm)
	pv.usersView.AddKeyBinding("D", "Drop User", pv.showDropUserConfirmation)
	pv.usersView.AddKeyBinding("P", "Password", pv.showChangePasswordForm)
	pv.usersView.AddKeyBinding("G", "Grant Role", pv.showGrantRoleForm)
	pv.usersView.AddKeyBinding("V", "Revoke Role", pv.showRevokeRoleForm)
	pv.addCommonBindings(pv.usersView)

	pv.usersView.SetActionCallback(pv.handleAction)

	pv.usersView.SetRowSelectedCallback(func(row int) {
		tableData := pv.usersView.GetTableData()
		if row >= 0 && row < len(tableData) && len(tableData[row]) > 0 {
			pv.usersView.Log(fmt.Sprintf("[blue]Selected user: %s", tableData[row][0]))
		}
	})

	pv.usersView.SetModalPages(pv.pages)
	pv.usersView.RegisterHandlers()

	// Initialize PostgreSQL client
	pv.pgClient = NewPostgresClient()

	pv.usersView.SetInfoText("[yellow]PostgreSQL Manager[white]\nStatus: Not Connected\nUse [green]Ctrl+T[white] to select instance")

	// --- Create all sub-views ---
	pv.databasesView = pv.newDatabasesView()
	pv.tablesView = pv.newTablesView()
	pv.schemasView = pv.newSchemasView()
	pv.extensionsView = pv.newExtensionsView()
	pv.connectionsView = pv.newConnectionsView()
	pv.statsView = pv.newStatsView()
	pv.configView = pv.newConfigView()
	pv.logsView = pv.newLogsView()
	pv.locksView = pv.newLocksView()
	pv.indexesView = pv.newIndexesView()
	pv.replicationView = pv.newReplicationView()
	pv.tablespacesView = pv.newTablespacesView()
	pv.dbStatsView = pv.newDbStatsView()

	// Set modal pages for all views
	allViews := []*ui.CoreView{
		pv.usersView,
		pv.databasesView,
		pv.tablesView,
		pv.schemasView,
		pv.extensionsView,
		pv.connectionsView,
		pv.statsView,
		pv.configView,
		pv.logsView,
		pv.locksView,
		pv.indexesView,
		pv.replicationView,
		pv.tablespacesView,
		pv.dbStatsView,
	}
	for _, v := range allViews {
		if v != nil {
			v.SetModalPages(pv.pages)
		}
	}

	// Register view pages
	pv.viewPages.AddPage("pg-users", pv.usersView.GetLayout(), true, true)
	pv.viewPages.AddPage("pg-databases", pv.databasesView.GetLayout(), true, false)
	pv.viewPages.AddPage("pg-tables", pv.tablesView.GetLayout(), true, false)
	pv.viewPages.AddPage("pg-schemas", pv.schemasView.GetLayout(), true, false)
	pv.viewPages.AddPage("pg-extensions", pv.extensionsView.GetLayout(), true, false)
	pv.viewPages.AddPage("pg-connections", pv.connectionsView.GetLayout(), true, false)
	pv.viewPages.AddPage("pg-stats", pv.statsView.GetLayout(), true, false)
	pv.viewPages.AddPage("pg-config", pv.configView.GetLayout(), true, false)
	pv.viewPages.AddPage("pg-logs", pv.logsView.GetLayout(), true, false)
	pv.viewPages.AddPage("pg-locks", pv.locksView.GetLayout(), true, false)
	pv.viewPages.AddPage("pg-indexes", pv.indexesView.GetLayout(), true, false)
	pv.viewPages.AddPage("pg-replication", pv.replicationView.GetLayout(), true, false)
	pv.viewPages.AddPage("pg-tablespaces", pv.tablespacesView.GetLayout(), true, false)
	pv.viewPages.AddPage("pg-dbstats", pv.dbStatsView.GetLayout(), true, false)

	pv.currentView = viewUsers

	// Set breadcrumb stacks
	pv.setViewStack(pv.usersView, viewUsers)
	pv.setViewStack(pv.databasesView, viewDatabases)
	pv.setViewStack(pv.tablesView, viewTables)
	pv.setViewStack(pv.schemasView, viewSchemas)
	pv.setViewStack(pv.extensionsView, viewExtensions)
	pv.setViewStack(pv.connectionsView, viewConnections)
	pv.setViewStack(pv.statsView, viewStats)
	pv.setViewStack(pv.configView, viewConfig)
	pv.setViewStack(pv.logsView, viewLogs)
	pv.setViewStack(pv.locksView, viewLocks)
	pv.setViewStack(pv.indexesView, viewIndexes)
	pv.setViewStack(pv.replicationView, viewReplication)
	pv.setViewStack(pv.tablespacesView, viewTablespaces)
	pv.setViewStack(pv.dbStatsView, viewDbStats)

	pv.startAutoRefresh()

	return pv
}

// addCommonBindings adds navigation key bindings shared across all views
func (pv *PostgresView) addCommonBindings(cores *ui.CoreView) {
	cores.AddKeyBinding("1", "Users", pv.showUsers)
	cores.AddKeyBinding("2", "Databases", pv.showDatabases)
	cores.AddKeyBinding("3", "Tables", pv.showTables)
	cores.AddKeyBinding("4", "Schemas", pv.showSchemas)
	cores.AddKeyBinding("5", "Extensions", pv.showExtensions)
	cores.AddKeyBinding("6", "Connections", pv.showConnections)
	cores.AddKeyBinding("7", "Stats", pv.showStats)
	cores.AddKeyBinding("8", "Config", pv.showConfig)
	cores.AddKeyBinding("9", "Logs", pv.showLogs)
	cores.AddKeyBinding("0", "Locks", pv.showLocks)
	cores.AddKeyBinding("I", "Indexes", pv.showIndexes)
	cores.AddKeyBinding("Y", "Replication", pv.showReplication)
	cores.AddKeyBinding("T", "Tablespaces", pv.showTablespaces)
	cores.AddKeyBinding("B", "DB Stats", pv.showDbStats)
}

// GetMainUI returns the main UI component
func (pv *PostgresView) GetMainUI() tview.Primitive {
	return pv.viewPages
}

// Stop cleans up resources when the view is no longer used.
func (pv *PostgresView) Stop() {
	if pv.refreshTimer != nil {
		pv.refreshTimer.Stop()
	}

	if pv.pgClient != nil && pv.pgClient.IsConnected() {
		pv.pgClient.Disconnect()
	}

	allViews := []*ui.CoreView{
		pv.usersView,
		pv.databasesView,
		pv.tablesView,
		pv.schemasView,
		pv.extensionsView,
		pv.connectionsView,
		pv.statsView,
		pv.configView,
		pv.logsView,
		pv.locksView,
		pv.indexesView,
		pv.replicationView,
		pv.tablespacesView,
		pv.dbStatsView,
	}
	for _, v := range allViews {
		if v != nil {
			v.StopAutoRefresh()
			v.UnregisterHandlers()
		}
	}
}

// ShowConnectionSelector displays the connection selector
func (pv *PostgresView) ShowConnectionSelector() {
	pv.usersView.Log("[blue]Opening connection selector...")

	instances, err := GetAvailableInstances()
	if err != nil {
		pv.usersView.Log(fmt.Sprintf("[red]Failed to load instances: %v", err))
		pv.showManualConnectionForm()
		return
	}

	if len(instances) == 0 {
		pv.usersView.Log("[yellow]No instances configured, using manual configuration")
		pv.showManualConnectionForm()
		return
	}

	items := make([][]string, len(instances))
	for i, inst := range instances {
		items[i] = []string{
			inst.Name,
			fmt.Sprintf("%s:%d/%s - %s", inst.Host, inst.Port, inst.Database, inst.Description),
		}
	}

	ui.ShowStandardListSelectorModal(
		pv.pages,
		pv.app,
		"Select PostgreSQL Instance",
		items,
		func(index int, name string, cancelled bool) {
			if !cancelled && index >= 0 && index < len(instances) {
				pv.connectToInstance(instances[index])
			} else {
				pv.usersView.Log("[blue]Connection selection cancelled")
			}
			pv.app.SetFocus(pv.currentCores().GetTable())
		},
	)
}

// showManualConnectionForm shows the manual connection form
func (pv *PostgresView) showManualConnectionForm() {
	items := [][]string{
		{"Host", "localhost"},
		{"Port", "5432"},
		{"Username", "postgres"},
		{"Password", ""},
		{"Database", "postgres"},
		{"SSLMode", "disable"},
	}
	pv.showConnectionFormStep(items, 0)
}

func (pv *PostgresView) showConnectionFormStep(items [][]string, index int) {
	if index >= len(items) {
		host := items[0][1]
		portStr := items[1][1]
		username := items[2][1]
		password := items[3][1]
		database := items[4][1]
		sslmode := items[5][1]

		port, err := strconv.Atoi(portStr)
		if err != nil {
			pv.usersView.Log(fmt.Sprintf("[red]Invalid port: %s", portStr))
			return
		}

		pv.connectToPostgres(host, port, username, password, database, sslmode)
		return
	}

	label := items[index][0]
	defaultValue := items[index][1]

	ui.ShowCompactStyledInputModal(
		pv.pages,
		pv.app,
		fmt.Sprintf("PostgreSQL Connection - %s", label),
		label,
		defaultValue,
		25,
		nil,
		func(value string, cancelled bool) {
			if cancelled {
				pv.app.SetFocus(pv.currentCores().GetTable())
				return
			}
			items[index][1] = value
			pv.showConnectionFormStep(items, index+1)
		},
	)
}

// connectToPostgres connects to a PostgreSQL server
func (pv *PostgresView) connectToPostgres(host string, port int, username, password, database, sslmode string) {
	if pv.pgClient == nil {
		pv.pgClient = NewPostgresClient()
	}

	// Disconnect if already connected
	if pv.pgClient.IsConnected() {
		pv.pgClient.Disconnect()
	}

	err := pv.pgClient.Connect(host, port, username, password, database, sslmode)
	if err != nil {
		pv.usersView.Log(fmt.Sprintf("[red]Failed to connect: %v", err))
		return
	}

	pv.currentConnection = pv.pgClient.GetCurrentConnection()

	pv.updateInfoText()
	pv.usersView.Log(fmt.Sprintf("[green]Connected to %s:%d/%s", host, port, database))

	pv.refresh()
}

// connectToInstance connects to a preconfigured PostgreSQL instance
func (pv *PostgresView) connectToInstance(instance PostgresInstance) {
	if pv.pgClient == nil {
		pv.pgClient = NewPostgresClient()
	}

	if pv.pgClient.IsConnected() {
		pv.pgClient.Disconnect()
	}

	cores := pv.currentCores()
	cores.SetInfoText(fmt.Sprintf("[yellow]PostgreSQL Manager[white]\nServer: %s:%d\nStatus: Connecting...",
		instance.Host, instance.Port))

	err := pv.pgClient.ConnectToInstance(instance)
	if err != nil {
		cores.Log(fmt.Sprintf("[red]Connection failed: %v", err))
		cores.SetInfoText("[yellow]PostgreSQL Manager[white]\nStatus: Not Connected\nUse [green]Ctrl+T[white] to select instance")
		return
	}

	pv.currentConnection = pv.pgClient.GetCurrentConnection()

	pv.updateInfoText()
	cores.Log(fmt.Sprintf("[green]Connected to %s:%d/%s", instance.Host, instance.Port, instance.Database))

	pv.refresh()
	pv.startAutoRefresh()
}

// updateInfoText updates the info panel with current connection info
func (pv *PostgresView) updateInfoText() {
	if pv.currentConnection == nil {
		return
	}
	text := fmt.Sprintf("[green]PostgreSQL Manager[white]\nServer: %s:%d\nDB: %s\nUser: %s\nSSL: %s\nStatus: Connected",
		pv.currentConnection.Host, pv.currentConnection.Port,
		pv.currentConnection.Database, pv.currentConnection.Username,
		pv.currentConnection.SSLMode)
	pv.currentCores().SetInfoText(text)
}

// refresh manually refreshes the current view
func (pv *PostgresView) refresh() {
	current := pv.currentCores()
	if current != nil {
		current.RefreshData()
	}
}

// refreshUsers returns the users table data
func (pv *PostgresView) refreshUsers() ([][]string, error) {
	if pv.pgClient == nil || !pv.pgClient.IsConnected() {
		pv.usersView.SetInfoText("[yellow]PostgreSQL Manager[white]\nStatus: Not Connected\nUse [green]Ctrl+T[white] to select instance")
		return [][]string{}, nil
	}

	users, err := pv.pgClient.GetUsers()
	if err != nil {
		return [][]string{}, err
	}

	data := make([][]string, 0, len(users))
	for _, u := range users {
		data = append(data, []string{
			u.Name,
			boolToStr(u.CanLogin),
			boolToStr(u.Super),
			boolToStr(u.CreateDB),
			boolToStr(u.CreateRole),
			boolToStr(u.Replication),
			fmt.Sprintf("%d", u.ConnectionCount),
			u.MemberOf,
			u.ValidUntil,
		})
	}

	if pv.currentConnection != nil {
		pv.usersView.SetInfoText(fmt.Sprintf("[green]PostgreSQL Manager[white]\nServer: %s:%d\nDB: %s\nUsers: %d",
			pv.currentConnection.Host, pv.currentConnection.Port,
			pv.currentConnection.Database, len(users)))
	}

	return data, nil
}

// selectedUser returns the currently selected user name
func (pv *PostgresView) selectedUser() (string, bool) {
	row := pv.usersView.GetSelectedRowData()
	if len(row) == 0 {
		return "", false
	}
	return row[0], true
}

// --- User Management Actions ---

func (pv *PostgresView) showCreateUserForm() {
	if pv.pgClient == nil || !pv.pgClient.IsConnected() {
		pv.usersView.Log("[yellow]Not connected")
		return
	}

	ui.ShowCompactStyledInputModal(
		pv.pages, pv.app, "Create User", "Username", "", 25, nil,
		func(username string, cancelled bool) {
			if cancelled || username == "" {
				pv.app.SetFocus(pv.usersView.GetTable())
				return
			}
			ui.ShowCompactStyledInputModal(
				pv.pages, pv.app, "Create User", "Password", "", 25, nil,
				func(password string, cancelled bool) {
					if cancelled {
						pv.app.SetFocus(pv.usersView.GetTable())
						return
					}
					err := pv.pgClient.CreateUser(username, password, true, false, false, false)
					if err != nil {
						pv.usersView.Log(fmt.Sprintf("[red]Failed to create user: %v", err))
					} else {
						pv.usersView.Log(fmt.Sprintf("[green]Created user: %s", username))
						pv.refresh()
					}
					pv.app.SetFocus(pv.usersView.GetTable())
				},
			)
		},
	)
}

func (pv *PostgresView) showDropUserConfirmation() {
	user, ok := pv.selectedUser()
	if !ok {
		pv.usersView.Log("[yellow]No user selected")
		return
	}

	ui.ShowStandardConfirmationModal(
		pv.pages, pv.app, "Drop User",
		fmt.Sprintf("Are you sure you want to drop user '[red]%s[white]'?", user),
		func(confirmed bool) {
			if confirmed {
				if err := pv.pgClient.DropUser(user); err != nil {
					pv.usersView.Log(fmt.Sprintf("[red]Failed to drop user: %v", err))
				} else {
					pv.usersView.Log(fmt.Sprintf("[yellow]Dropped user: %s", user))
					pv.refresh()
				}
			}
			pv.app.SetFocus(pv.usersView.GetTable())
		},
	)
}

func (pv *PostgresView) showChangePasswordForm() {
	user, ok := pv.selectedUser()
	if !ok {
		pv.usersView.Log("[yellow]No user selected")
		return
	}

	ui.ShowCompactStyledInputModal(
		pv.pages, pv.app, fmt.Sprintf("Change Password - %s", user),
		"New Password", "", 25, nil,
		func(password string, cancelled bool) {
			if cancelled {
				pv.app.SetFocus(pv.usersView.GetTable())
				return
			}
			if err := pv.pgClient.AlterUserPassword(user, password); err != nil {
				pv.usersView.Log(fmt.Sprintf("[red]Failed to change password: %v", err))
			} else {
				pv.usersView.Log(fmt.Sprintf("[green]Password changed for: %s", user))
			}
			pv.app.SetFocus(pv.usersView.GetTable())
		},
	)
}

func (pv *PostgresView) showGrantRoleForm() {
	user, ok := pv.selectedUser()
	if !ok {
		pv.usersView.Log("[yellow]No user selected")
		return
	}

	ui.ShowCompactStyledInputModal(
		pv.pages, pv.app, fmt.Sprintf("Grant Role to %s", user),
		"Role Name", "", 25, nil,
		func(role string, cancelled bool) {
			if cancelled || role == "" {
				pv.app.SetFocus(pv.usersView.GetTable())
				return
			}
			if err := pv.pgClient.GrantRole(role, user); err != nil {
				pv.usersView.Log(fmt.Sprintf("[red]Failed to grant role: %v", err))
			} else {
				pv.usersView.Log(fmt.Sprintf("[green]Granted %s to %s", role, user))
				pv.refresh()
			}
			pv.app.SetFocus(pv.usersView.GetTable())
		},
	)
}

func (pv *PostgresView) showRevokeRoleForm() {
	user, ok := pv.selectedUser()
	if !ok {
		pv.usersView.Log("[yellow]No user selected")
		return
	}

	ui.ShowCompactStyledInputModal(
		pv.pages, pv.app, fmt.Sprintf("Revoke Role from %s", user),
		"Role Name", "", 25, nil,
		func(role string, cancelled bool) {
			if cancelled || role == "" {
				pv.app.SetFocus(pv.usersView.GetTable())
				return
			}
			if err := pv.pgClient.RevokeRole(role, user); err != nil {
				pv.usersView.Log(fmt.Sprintf("[red]Failed to revoke role: %v", err))
			} else {
				pv.usersView.Log(fmt.Sprintf("[green]Revoked %s from %s", role, user))
				pv.refresh()
			}
			pv.app.SetFocus(pv.usersView.GetTable())
		},
	)
}

// showHelp displays PostgreSQL plugin help
func (pv *PostgresView) showHelp() {
	helpText := `
[yellow]PostgreSQL Manager Help[white]

[green]Navigation Keys:[white]
1       - Users view (default)
2       - Databases view
3       - Tables view
4       - Schemas view
5       - Extensions view
6       - Active Connections
7       - Server Stats
8       - Configuration
9       - Activity / Logs
0       - Locks view
I       - Indexes view
Y       - Replication view
T       - Tablespaces view
B       - Database Stats

[green]User Management (Users view):[white]
N       - Create new user
D       - Drop selected user
P       - Change password
G       - Grant role to user
V       - Revoke role from user

[green]Database Management (Databases view):[white]
N       - Create new database
D       - Drop selected database

[green]Extension Management (Extensions view):[white]
N       - Install extension
D       - Remove extension

[green]Connection Management (Connections view):[white]
K       - Kill (terminate) connection
C       - Cancel query

[green]General:[white]
R       - Refresh current view
Ctrl+T  - Select instance
?       - Show this help
/       - Filter table
Esc     - Navigate back
`

	ui.ShowInfoModal(
		pv.pages, pv.app, "PostgreSQL Help", helpText,
		func() {
			current := pv.currentCores()
			if current != nil {
				pv.app.SetFocus(current.GetTable())
			}
		},
	)
}

func (pv *PostgresView) handleNavKeys(key string) bool {
	switch key {
	case "1":
		pv.showUsers()
	case "2":
		pv.showDatabases()
	case "3":
		pv.showTables()
	case "4":
		pv.showSchemas()
	case "5":
		pv.showExtensions()
	case "6":
		pv.showConnections()
	case "7":
		pv.showStats()
	case "8":
		pv.showConfig()
	case "9":
		pv.showLogs()
	case "0":
		pv.showLocks()
	case "I":
		pv.showIndexes()
	case "Y":
		pv.showReplication()
	case "T":
		pv.showTablespaces()
	case "B":
		pv.showDbStats()
	case "?":
		pv.showHelp()
	case "R":
		pv.refresh()
	default:
		return false
	}
	return true
}

func (pv *PostgresView) handleViewSpecificKeys(key string) bool {
	switch key {
	case "N":
		switch pv.currentView {
		case viewUsers:
			pv.showCreateUserForm()
		case viewDatabases:
			pv.showCreateDatabaseForm()
		case viewExtensions:
			pv.showInstallExtensionForm()
		default:
			return false
		}
	case "D":
		switch pv.currentView {
		case viewUsers:
			pv.showDropUserConfirmation()
		case viewDatabases:
			pv.showDropDatabaseConfirmation()
		case viewExtensions:
			pv.showDropExtensionConfirmation()
		default:
			return false
		}
	case "P":
		if pv.currentView == viewUsers {
			pv.showChangePasswordForm()
		} else {
			return false
		}
	case "G":
		if pv.currentView == viewUsers {
			pv.showGrantRoleForm()
		} else {
			return false
		}
	case "V":
		if pv.currentView == viewUsers {
			pv.showRevokeRoleForm()
		} else {
			return false
		}
	case "K":
		if pv.currentView == viewConnections {
			pv.showTerminateConnectionConfirmation()
		} else {
			return false
		}
	case "C":
		if pv.currentView == viewConnections {
			pv.showCancelQueryConfirmation()
		} else {
			return false
		}
	default:
		return false
	}
	return true
}

// handleAction handles actions triggered by the UI
func (pv *PostgresView) handleAction(action string, payload map[string]interface{}) error {
	switch action {
	case "refresh":
		pv.refresh()
		return nil
	case "keypress":
		if key, ok := payload["key"].(string); ok {
			handled := pv.handleNavKeys(key)
			if !handled {
				handled = pv.handleViewSpecificKeys(key)
			}
			if handled {
				return nil
			}
		}
	case "navigate_back":
		if view, ok := payload["current_view"].(string); ok {
			if view == viewRoot {
				pv.switchView(viewUsers)
				return nil
			}
			pv.switchView(view)
			return nil
		}
	}
	return fmt.Errorf("unhandled")
}

// startAutoRefresh sets up the auto-refresh timer
func (pv *PostgresView) startAutoRefresh() {
	if uiConfig, err := GetUIConfig(); err == nil {
		pv.refreshInterval = time.Duration(uiConfig.RefreshInterval) * time.Second
	}

	if pv.refreshTimer != nil {
		pv.refreshTimer.Stop()
	}

	pv.refreshTimer = time.AfterFunc(pv.refreshInterval, func() {
		if pv.pgClient != nil && pv.pgClient.IsConnected() {
			pv.app.QueueUpdate(func() {
				pv.refresh()
				pv.startAutoRefresh()
			})
		} else {
			pv.startAutoRefresh()
		}
	})
}

// AutoConnectToDefaultInstance automatically connects to the default instance
func (pv *PostgresView) AutoConnectToDefaultInstance() {
	instances, err := GetAvailableInstances()
	if err != nil {
		pv.usersView.Log(fmt.Sprintf("[yellow]Failed to load instances: %v", err))
		return
	}

	if len(instances) == 0 {
		pv.usersView.Log("[yellow]No PostgreSQL instances configured in KeePass (create entries under postgres/<environment>/<name>)")
		return
	}

	pv.usersView.Log(fmt.Sprintf("[blue]Auto-connecting to: %s", instances[0].Name))
	pv.connectToInstance(instances[0])
}

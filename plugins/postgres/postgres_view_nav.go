package main

import (
	"omo/pkg/ui"
)

const (
	viewRoot        = "postgres"
	viewUsers       = "users"
	viewDatabases   = "databases"
	viewTables      = "tables"
	viewSchemas     = "schemas"
	viewExtensions  = "extensions"
	viewConnections = "connections"
	viewStats       = "stats"
	viewConfig      = "config"
	viewLogs        = "logs"
	viewLocks       = "locks"
	viewIndexes     = "indexes"
	viewReplication = "replication"
	viewTablespaces = "tablespaces"
	viewDbStats     = "dbstats"
)

func (pv *PostgresView) currentCores() *ui.CoreView {
	switch pv.currentView {
	case viewDatabases:
		return pv.databasesView
	case viewTables:
		return pv.tablesView
	case viewSchemas:
		return pv.schemasView
	case viewExtensions:
		return pv.extensionsView
	case viewConnections:
		return pv.connectionsView
	case viewStats:
		return pv.statsView
	case viewConfig:
		return pv.configView
	case viewLogs:
		return pv.logsView
	case viewLocks:
		return pv.locksView
	case viewIndexes:
		return pv.indexesView
	case viewReplication:
		return pv.replicationView
	case viewTablespaces:
		return pv.tablespacesView
	case viewDbStats:
		return pv.dbStatsView
	default:
		return pv.usersView
	}
}

func (pv *PostgresView) setViewStack(cores *ui.CoreView, viewName string) {
	if cores == nil {
		return
	}

	stack := []string{viewRoot, viewUsers}
	if viewName != viewUsers {
		stack = append(stack, viewName)
	}
	cores.SetViewStack(stack)
}

func (pv *PostgresView) switchView(viewName string) {
	pageName := "pg-" + viewName
	pv.currentView = viewName
	pv.viewPages.SwitchToPage(pageName)

	pv.setViewStack(pv.currentCores(), viewName)
	pv.refresh()
	current := pv.currentCores()
	if current != nil {
		pv.app.SetFocus(current.GetTable())
	}
}

func (pv *PostgresView) showUsers()       { pv.switchView(viewUsers) }
func (pv *PostgresView) showDatabases()   { pv.switchView(viewDatabases) }
func (pv *PostgresView) showTables()      { pv.switchView(viewTables) }
func (pv *PostgresView) showSchemas()     { pv.switchView(viewSchemas) }
func (pv *PostgresView) showExtensions()  { pv.switchView(viewExtensions) }
func (pv *PostgresView) showConnections() { pv.switchView(viewConnections) }
func (pv *PostgresView) showStats()       { pv.switchView(viewStats) }
func (pv *PostgresView) showConfig()      { pv.switchView(viewConfig) }
func (pv *PostgresView) showLogs()        { pv.switchView(viewLogs) }
func (pv *PostgresView) showLocks()       { pv.switchView(viewLocks) }
func (pv *PostgresView) showIndexes()     { pv.switchView(viewIndexes) }
func (pv *PostgresView) showReplication() { pv.switchView(viewReplication) }
func (pv *PostgresView) showTablespaces() { pv.switchView(viewTablespaces) }
func (pv *PostgresView) showDbStats()     { pv.switchView(viewDbStats) }

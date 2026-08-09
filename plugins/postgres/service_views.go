package postgres

import (
	"fmt"

	"omo/pkg/pluginrpc"
)

// Primary views on digit keys 0-9. Remaining views use letter shortcuts and appear in "?".
func viewNavBindings() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "0", Label: "Users", Action: "goto_users"},
		{Key: "1", Label: "Databases", Action: "goto_databases"},
		{Key: "2", Label: "Tables", Action: "goto_tables"},
		{Key: "3", Label: "Schemas", Action: "goto_schemas"},
		{Key: "4", Label: "Extensions", Action: "goto_extensions"},
		{Key: "5", Label: "Connections", Action: "goto_connections"},
		{Key: "6", Label: "Stats", Action: "goto_stats"},
		{Key: "7", Label: "Config", Action: "goto_config"},
		{Key: "8", Label: "Locks", Action: "goto_locks"},
		{Key: "9", Label: "Indexes", Action: "goto_indexes"},
	}
}

func moreViewBindings() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "Y", Label: "Replication", Action: "goto_replication"},
		{Key: "T", Label: "Tablespaces", Action: "goto_tablespaces"},
		{Key: "B", Label: "DB Stats", Action: "goto_dbstats"},
	}
}

func usersActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "N", Label: "New User", Action: "create_user"},
		{Key: "D", Label: "Drop User", Action: "drop_user"},
		{Key: "P", Label: "Password", Action: "change_password"},
		{Key: "G", Label: "Grant Role", Action: "grant_role"},
		{Key: "V", Label: "Revoke Role", Action: "revoke_role"},
	}
}

func databasesActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "N", Label: "New DB", Action: "create_database"},
		{Key: "D", Label: "Drop DB", Action: "drop_database"},
	}
}

func tablesActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "E", Label: "Columns", Action: "view_columns"},
	}
}

func extensionsActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "N", Label: "Install Ext", Action: "install_extension"},
		{Key: "D", Label: "Drop Ext", Action: "drop_extension"},
	}
}

func connectionsActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "K", Label: "Kill Conn", Action: "kill_connection"},
		{Key: "C", Label: "Cancel Query", Action: "cancel_query"},
	}
}

func configActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "E", Label: "Edit Param", Action: "edit_config"},
	}
}

func indexesActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "E", Label: "Index Def", Action: "view_index"},
	}
}

func helpSections() []pluginrpc.HelpSection {
	return []pluginrpc.HelpSection{
		{Title: "Views (0-9)", Bindings: viewNavBindings()},
		{Title: "More Views", Bindings: moreViewBindings()},
		{Title: "Users", Bindings: usersActions()},
		{Title: "Databases", Bindings: databasesActions()},
		{Title: "Tables", Bindings: tablesActions()},
		{Title: "Extensions", Bindings: extensionsActions()},
		{Title: "Connections", Bindings: connectionsActions()},
		{Title: "Config", Bindings: configActions()},
		{Title: "Indexes", Bindings: indexesActions()},
		{
			Title: "Global",
			Bindings: []pluginrpc.KeyBinding{
				{Key: "R", Label: "Refresh"},
				{Key: "?", Label: "Help (this screen)"},
				{Key: "/", Label: "Filter"},
				{Key: "^t", Label: "Switch target"},
				{Key: "ESC", Label: "Back / home"},
			},
		},
	}
}

// decorate splits UI roles:
//   - ViewBindings → middle Views column (0-9)
//   - Actions → former logs / Actions column (this view only)
//   - more views (Y/T/B) → silent binds + "?" help only
func decorate(view pluginrpc.ViewData, actions ...pluginrpc.KeyBinding) pluginrpc.ViewData {
	view.ViewBindings = viewNavBindings()
	view.KeyBindings = moreViewBindings()
	view.Actions = actions
	view.HelpSections = helpSections()
	return view
}

func (s *Service) baseInfo(extra string) string {
	msg := fmt.Sprintf("[green]PostgreSQL Manager[white]\nServer: %s:%d\nDB: %s\nStatus: Connected\nView: %s",
		s.host, s.port, s.database, s.currentView)
	if extra != "" {
		msg += "\n" + extra
	}
	return msg
}

func (s *Service) buildViewLocked(viewID string) (pluginrpc.ViewData, error) {
	if viewID == "" {
		viewID = viewUsers
	}
	s.currentView = viewID

	if err := s.ensureConnectedLocked(); err != nil {
		return pluginrpc.ViewData{
			View:    viewID,
			Title:   "PostgreSQL Manager",
			Info:    "[yellow]PostgreSQL Manager[white]\nStatus: Not Connected\n" + err.Error(),
			Status:  "not connected",
			Headers: []string{"Status", "Detail"},
			Rows:    [][]string{{"error", err.Error()}},
			KeyBindings: []pluginrpc.KeyBinding{
				{Key: "R", Label: "Refresh", Action: "refresh"},
			},
		}, nil
	}

	switch viewID {
	case viewDatabases:
		return s.viewDatabasesLocked()
	case viewTables:
		return s.viewTablesLocked()
	case viewSchemas:
		return s.viewSchemasLocked()
	case viewExtensions:
		return s.viewExtensionsLocked()
	case viewConnections:
		return s.viewConnectionsLocked()
	case viewStats:
		return s.viewStatsLocked()
	case viewConfig:
		return s.viewConfigLocked()
	case viewLocks:
		return s.viewLocksLocked()
	case viewIndexes:
		return s.viewIndexesLocked()
	case viewReplication:
		return s.viewReplicationLocked()
	case viewTablespaces:
		return s.viewTablespacesLocked()
	case viewDbStats:
		return s.viewDbStatsLocked()
	default:
		return s.viewUsersLocked()
	}
}

func (s *Service) viewUsersLocked() (pluginrpc.ViewData, error) {
	users, err := s.client.GetUsers()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(users))
	for _, u := range users {
		rows = append(rows, []string{
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
	return decorate(pluginrpc.ViewData{
		View:         viewUsers,
		Title:        "PostgreSQL Users",
		Info:         s.baseInfo(fmt.Sprintf("Users: %d", len(users))),
		Status:       "connected",
		Headers:      []string{"User", "Login", "Super", "CreateDB", "CreateRole", "Repl", "Conns", "Member Of", "Valid Until"},
		Rows:         rows,
		SelectionKey: "User",
	}, usersActions()...), nil
}

func (s *Service) viewDatabasesLocked() (pluginrpc.ViewData, error) {
	databases, err := s.client.GetDatabases()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(databases))
	for _, d := range databases {
		rows = append(rows, []string{
			d.Name, d.Owner, d.Encoding, d.Collation, d.Size, d.Tablespace, fmt.Sprintf("%d", d.ConnLimit),
		})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"-", "-", "-", "-", "-", "-", "No databases"})
	}
	return decorate(pluginrpc.ViewData{
		View:         viewDatabases,
		Title:        "PostgreSQL Databases",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"Name", "Owner", "Encoding", "Collation", "Size", "Tablespace", "Conn Limit"},
		Rows:         rows,
		SelectionKey: "Name",
	}, databasesActions()...), nil
}

func (s *Service) viewTablesLocked() (pluginrpc.ViewData, error) {
	tables, err := s.client.GetTables()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(tables))
	for _, t := range tables {
		hasIdx := "No"
		if t.HasIndexes {
			hasIdx = "Yes"
		}
		rows = append(rows, []string{
			t.Schema, t.Name, t.Owner, fmt.Sprintf("%d", t.RowCount), t.Size, t.TotalSize, hasIdx, t.Tablespace,
		})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"-", "-", "-", "-", "-", "-", "-", "No tables"})
	}
	return decorate(pluginrpc.ViewData{
		View:         viewTables,
		Title:        "PostgreSQL Tables",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"Schema", "Table", "Owner", "Rows", "Size", "Total Size", "Indexes", "Tablespace"},
		Rows:         rows,
		SelectionKey: "Table",
	}, tablesActions()...), nil
}

func (s *Service) viewSchemasLocked() (pluginrpc.ViewData, error) {
	schemas, err := s.client.GetSchemas()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(schemas))
	for _, sch := range schemas {
		rows = append(rows, []string{sch.Name, sch.Owner})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"-", "No schemas"})
	}
	return decorate(pluginrpc.ViewData{
		View:         viewSchemas,
		Title:        "PostgreSQL Schemas",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"Schema", "Owner"},
		Rows:         rows,
		SelectionKey: "Schema",
	}), nil
}

func (s *Service) viewExtensionsLocked() (pluginrpc.ViewData, error) {
	extensions, err := s.client.GetExtensions()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(extensions))
	for _, ext := range extensions {
		installed := "No"
		if ext.Installed {
			installed = "Yes"
		}
		comment := ext.Comment
		if len(comment) > 80 {
			comment = comment[:80] + "..."
		}
		rows = append(rows, []string{ext.Name, ext.Version, ext.Schema, installed, comment})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"-", "-", "-", "-", "No extensions"})
	}
	return decorate(pluginrpc.ViewData{
		View:         viewExtensions,
		Title:        "PostgreSQL Extensions",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"Name", "Version", "Schema", "Installed", "Comment"},
		Rows:         rows,
		SelectionKey: "Name",
	}, extensionsActions()...), nil
}

func (s *Service) viewConnectionsLocked() (pluginrpc.ViewData, error) {
	conns, err := s.client.GetActiveConnections()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(conns))
	for _, conn := range conns {
		query := conn.Query
		if len(query) > 80 {
			query = query[:80] + "..."
		}
		rows = append(rows, []string{
			fmt.Sprintf("%d", conn.PID), conn.User, conn.Database, conn.ClientAddr,
			conn.State, conn.BackendType, conn.WaitEvent, conn.Duration, query,
		})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"-", "-", "-", "-", "-", "-", "-", "-", "No active connections"})
	}
	return decorate(pluginrpc.ViewData{
		View:         viewConnections,
		Title:        "PostgreSQL Active Connections",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"PID", "User", "Database", "Client", "State", "Backend", "Wait", "Duration", "Query"},
		Rows:         rows,
		SelectionKey: "PID",
	}, connectionsActions()...), nil
}

func (s *Service) viewStatsLocked() (pluginrpc.ViewData, error) {
	stats, err := s.client.GetServerInfo()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(stats))
	for _, st := range stats {
		rows = append(rows, []string{st.Key, st.Value})
	}
	return decorate(pluginrpc.ViewData{
		View:         viewStats,
		Title:        "PostgreSQL Server Stats",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"Property", "Value"},
		Rows:         rows,
		SelectionKey: "Property",
	}), nil
}

func (s *Service) viewConfigLocked() (pluginrpc.ViewData, error) {
	configs, err := s.client.GetConfig("%")
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(configs))
	for _, cfg := range configs {
		restart := "No"
		if cfg.PendRestart {
			restart = "Yes"
		}
		rows = append(rows, []string{cfg.Name, cfg.Setting, cfg.Unit, cfg.Category, cfg.Source, cfg.BootVal, restart})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"-", "-", "-", "-", "-", "-", "No settings"})
	}
	return decorate(pluginrpc.ViewData{
		View:         viewConfig,
		Title:        "PostgreSQL Configuration",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"Parameter", "Value", "Unit", "Category", "Source", "Boot Value", "Restart?"},
		Rows:         rows,
		SelectionKey: "Parameter",
	}, configActions()...), nil
}

func (s *Service) viewLocksLocked() (pluginrpc.ViewData, error) {
	locks, err := s.client.GetLocks()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(locks))
	for _, lk := range locks {
		granted := "Yes"
		if !lk.Granted {
			granted = "No (Waiting)"
		}
		rows = append(rows, []string{
			fmt.Sprintf("%d", lk.PID), lk.LockType, lk.Mode, lk.Relation, granted, lk.WaitStart,
		})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"-", "-", "-", "-", "-", "No locks"})
	}
	return decorate(pluginrpc.ViewData{
		View:         viewLocks,
		Title:        "PostgreSQL Locks",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"PID", "Lock Type", "Mode", "Relation", "Granted", "Wait Start"},
		Rows:         rows,
		SelectionKey: "PID",
	}), nil
}

func (s *Service) viewIndexesLocked() (pluginrpc.ViewData, error) {
	indexes, err := s.client.GetIndexes()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(indexes))
	for _, idx := range indexes {
		rows = append(rows, []string{
			idx.Schema, idx.Table, idx.Name, idx.Size,
			fmt.Sprintf("%d", idx.Scans), fmt.Sprintf("%d", idx.TupRead), fmt.Sprintf("%d", idx.TupFetch),
		})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"-", "-", "-", "-", "-", "-", "No indexes"})
	}
	return decorate(pluginrpc.ViewData{
		View:         viewIndexes,
		Title:        "PostgreSQL Indexes",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"Schema", "Table", "Index", "Size", "Scans", "Tuples Read", "Tuples Fetched"},
		Rows:         rows,
		SelectionKey: "Index",
	}, indexesActions()...), nil
}

func (s *Service) viewReplicationLocked() (pluginrpc.ViewData, error) {
	replicas, err := s.client.GetReplicationStatus()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(replicas))
	for _, r := range replicas {
		rows = append(rows, []string{
			fmt.Sprintf("%d", r.PID), r.Application, r.ClientAddr, r.State,
			r.SentLSN, r.WriteLSN, r.FlushLSN, r.ReplayLSN, r.WriteLag, r.FlushLag, r.ReplayLag,
		})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"-", "-", "-", "No replicas", "-", "-", "-", "-", "-", "-", "-"})
	}
	return decorate(pluginrpc.ViewData{
		View:         viewReplication,
		Title:        "PostgreSQL Replication",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"PID", "Application", "Client", "State", "Sent LSN", "Write LSN", "Flush LSN", "Replay LSN", "Write Lag", "Flush Lag", "Replay Lag"},
		Rows:         rows,
		SelectionKey: "PID",
	}), nil
}

func (s *Service) viewTablespacesLocked() (pluginrpc.ViewData, error) {
	tablespaces, err := s.client.GetTablespaces()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(tablespaces))
	for _, ts := range tablespaces {
		rows = append(rows, []string{ts.Name, ts.Owner, ts.Location, ts.Size})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"-", "-", "-", "No tablespaces"})
	}
	return decorate(pluginrpc.ViewData{
		View:         viewTablespaces,
		Title:        "PostgreSQL Tablespaces",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"Name", "Owner", "Location", "Size"},
		Rows:         rows,
		SelectionKey: "Name",
	}), nil
}

func (s *Service) viewDbStatsLocked() (pluginrpc.ViewData, error) {
	data, err := s.client.GetDatabaseStats()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	if len(data) == 0 {
		empty := make([]string, 14)
		empty[0] = "No data"
		data = append(data, empty)
	}
	return decorate(pluginrpc.ViewData{
		View:   viewDbStats,
		Title:  "PostgreSQL Database Stats",
		Info:   s.baseInfo(""),
		Status: "connected",
		Headers: []string{
			"Database", "Backends", "Commits", "Rollbacks",
			"Blks Read", "Blks Hit", "Cache Hit%",
			"Returned", "Fetched", "Inserted", "Updated", "Deleted",
			"Conflicts", "Deadlocks",
		},
		Rows:         data,
		SelectionKey: "Database",
	}), nil
}

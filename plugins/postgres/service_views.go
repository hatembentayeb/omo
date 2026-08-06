package postgres

import (
	"fmt"

	"omo/pkg/pluginrpc"
)

func navBindings() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "1", Label: "Users", Action: "goto_users"},
		{Key: "2", Label: "Databases", Action: "goto_databases"},
		{Key: "3", Label: "Tables", Action: "goto_tables"},
		{Key: "4", Label: "Schemas", Action: "goto_schemas"},
		{Key: "5", Label: "Extensions", Action: "goto_extensions"},
		{Key: "6", Label: "Connections", Action: "goto_connections"},
		{Key: "7", Label: "Stats", Action: "goto_stats"},
		{Key: "8", Label: "Config", Action: "goto_config"},
		{Key: "0", Label: "Locks", Action: "goto_locks"},
		{Key: "I", Label: "Indexes", Action: "goto_indexes"},
		{Key: "Y", Label: "Replication", Action: "goto_replication"},
		{Key: "T", Label: "Tablespaces", Action: "goto_tablespaces"},
		{Key: "B", Label: "DB Stats", Action: "goto_dbstats"},
	}
}

func withNav(extra ...pluginrpc.KeyBinding) []pluginrpc.KeyBinding {
	out := make([]pluginrpc.KeyBinding, 0, len(extra)+len(navBindings())+1)
	out = append(out, pluginrpc.KeyBinding{Key: "R", Label: "Refresh", Action: "refresh"})
	out = append(out, extra...)
	out = append(out, navBindings()...)
	return out
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
	return pluginrpc.ViewData{
		View:         viewUsers,
		Title:        "PostgreSQL Users",
		Info:         s.baseInfo(fmt.Sprintf("Users: %d", len(users))),
		Status:       "connected",
		Headers:      []string{"User", "Login", "Super", "CreateDB", "CreateRole", "Repl", "Conns", "Member Of", "Valid Until"},
		Rows:         rows,
		SelectionKey: "User",
		KeyBindings: withNav(
			pluginrpc.KeyBinding{Key: "N", Label: "New User", Action: "create_user"},
			pluginrpc.KeyBinding{Key: "D", Label: "Drop User", Action: "drop_user"},
			pluginrpc.KeyBinding{Key: "P", Label: "Password", Action: "change_password"},
			pluginrpc.KeyBinding{Key: "G", Label: "Grant Role", Action: "grant_role"},
			pluginrpc.KeyBinding{Key: "V", Label: "Revoke Role", Action: "revoke_role"},
		),
	}, nil
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
	return pluginrpc.ViewData{
		View:         viewDatabases,
		Title:        "PostgreSQL Databases",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"Name", "Owner", "Encoding", "Collation", "Size", "Tablespace", "Conn Limit"},
		Rows:         rows,
		SelectionKey: "Name",
		KeyBindings: withNav(
			pluginrpc.KeyBinding{Key: "N", Label: "New DB", Action: "create_database"},
			pluginrpc.KeyBinding{Key: "D", Label: "Drop DB", Action: "drop_database"},
		),
	}, nil
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
	return pluginrpc.ViewData{
		View:         viewTables,
		Title:        "PostgreSQL Tables",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"Schema", "Table", "Owner", "Rows", "Size", "Total Size", "Indexes", "Tablespace"},
		Rows:         rows,
		SelectionKey: "Table",
		KeyBindings: withNav(
			pluginrpc.KeyBinding{Key: "E", Label: "Columns", Action: "view_columns"},
		),
	}, nil
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
	return pluginrpc.ViewData{
		View:         viewSchemas,
		Title:        "PostgreSQL Schemas",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"Schema", "Owner"},
		Rows:         rows,
		SelectionKey: "Schema",
		KeyBindings:  withNav(),
	}, nil
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
	return pluginrpc.ViewData{
		View:         viewExtensions,
		Title:        "PostgreSQL Extensions",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"Name", "Version", "Schema", "Installed", "Comment"},
		Rows:         rows,
		SelectionKey: "Name",
		KeyBindings: withNav(
			pluginrpc.KeyBinding{Key: "N", Label: "Install Ext", Action: "install_extension"},
			pluginrpc.KeyBinding{Key: "D", Label: "Drop Ext", Action: "drop_extension"},
		),
	}, nil
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
	return pluginrpc.ViewData{
		View:         viewConnections,
		Title:        "PostgreSQL Active Connections",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"PID", "User", "Database", "Client", "State", "Backend", "Wait", "Duration", "Query"},
		Rows:         rows,
		SelectionKey: "PID",
		KeyBindings: withNav(
			pluginrpc.KeyBinding{Key: "K", Label: "Kill Conn", Action: "kill_connection"},
			pluginrpc.KeyBinding{Key: "C", Label: "Cancel Query", Action: "cancel_query"},
		),
	}, nil
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
	return pluginrpc.ViewData{
		View:         viewStats,
		Title:        "PostgreSQL Server Stats",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"Property", "Value"},
		Rows:         rows,
		SelectionKey: "Property",
		KeyBindings:  withNav(),
	}, nil
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
	return pluginrpc.ViewData{
		View:         viewConfig,
		Title:        "PostgreSQL Configuration",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"Parameter", "Value", "Unit", "Category", "Source", "Boot Value", "Restart?"},
		Rows:         rows,
		SelectionKey: "Parameter",
		KeyBindings: withNav(
			pluginrpc.KeyBinding{Key: "E", Label: "Edit Param", Action: "edit_config"},
		),
	}, nil
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
	return pluginrpc.ViewData{
		View:         viewLocks,
		Title:        "PostgreSQL Locks",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"PID", "Lock Type", "Mode", "Relation", "Granted", "Wait Start"},
		Rows:         rows,
		SelectionKey: "PID",
		KeyBindings:  withNav(),
	}, nil
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
	return pluginrpc.ViewData{
		View:         viewIndexes,
		Title:        "PostgreSQL Indexes",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"Schema", "Table", "Index", "Size", "Scans", "Tuples Read", "Tuples Fetched"},
		Rows:         rows,
		SelectionKey: "Index",
		KeyBindings: withNav(
			pluginrpc.KeyBinding{Key: "E", Label: "Index Def", Action: "view_index"},
		),
	}, nil
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
	return pluginrpc.ViewData{
		View:         viewReplication,
		Title:        "PostgreSQL Replication",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"PID", "Application", "Client", "State", "Sent LSN", "Write LSN", "Flush LSN", "Replay LSN", "Write Lag", "Flush Lag", "Replay Lag"},
		Rows:         rows,
		SelectionKey: "PID",
		KeyBindings:  withNav(),
	}, nil
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
	return pluginrpc.ViewData{
		View:         viewTablespaces,
		Title:        "PostgreSQL Tablespaces",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"Name", "Owner", "Location", "Size"},
		Rows:         rows,
		SelectionKey: "Name",
		KeyBindings:  withNav(),
	}, nil
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
	return pluginrpc.ViewData{
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
		KeyBindings:  withNav(),
	}, nil
}

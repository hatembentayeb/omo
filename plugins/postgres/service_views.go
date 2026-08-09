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
	return pluginrpc.HelpNav(viewNavBindings(), moreViewBindings(),
		pluginrpc.HelpSection{Title: "Users", Bindings: usersActions()},
		pluginrpc.HelpSection{Title: "Databases", Bindings: databasesActions()},
		pluginrpc.HelpSection{Title: "Tables", Bindings: tablesActions()},
		pluginrpc.HelpSection{Title: "Extensions", Bindings: extensionsActions()},
		pluginrpc.HelpSection{Title: "Connections", Bindings: connectionsActions()},
		pluginrpc.HelpSection{Title: "Config", Bindings: configActions()},
		pluginrpc.HelpSection{Title: "Indexes", Bindings: indexesActions()},
	)
}

var ui = pluginrpc.ViewUI{
	Views: viewNavBindings,
	More:  moreViewBindings,
	Help:  helpSections,
}

func (s *Service) baseInfo(extra string) string {
	msg := fmt.Sprintf("[green]PostgreSQL Manager[white]\nServer: %s:%d\nDB: %s\nStatus: Connected\nView: %s",
		s.host, s.port, s.database, s.currentView)
	return pluginrpc.FormatInfo(msg, extra)
}

func (s *Service) buildViewLocked(viewID string) (pluginrpc.ViewData, error) {
	if viewID == "" {
		viewID = viewUsers
	}
	s.currentView = viewID

	if err := s.ensureConnectedLocked(); err != nil {
		return ui.NotConnectedErr(viewID, "PostgreSQL Manager", err)
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
	return ui.Connected(viewUsers, "PostgreSQL Users", s.baseInfo(fmt.Sprintf("Users: %d", len(users))), []string{"User", "Login", "Super", "CreateDB", "CreateRole", "Repl", "Conns", "Member Of", "Valid Until"}, rows, "User", usersActions()...), nil
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
	rows = pluginrpc.EnsureRows(rows, []string{"-", "-", "-", "-", "-", "-", "No databases"})
	return ui.Connected(viewDatabases, "PostgreSQL Databases", s.baseInfo(""), []string{"Name", "Owner", "Encoding", "Collation", "Size", "Tablespace", "Conn Limit"}, rows, "Name", databasesActions()...), nil
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
	rows = pluginrpc.EnsureRows(rows, []string{"-", "-", "-", "-", "-", "-", "-", "No tables"})
	return ui.Connected(viewTables, "PostgreSQL Tables", s.baseInfo(""), []string{"Schema", "Table", "Owner", "Rows", "Size", "Total Size", "Indexes", "Tablespace"}, rows, "Table", tablesActions()...), nil
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
	rows = pluginrpc.EnsureRows(rows, []string{"-", "No schemas"})
	return ui.Connected(viewSchemas, "PostgreSQL Schemas", s.baseInfo(""), []string{"Schema", "Owner"}, rows, "Schema"), nil
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
	rows = pluginrpc.EnsureRows(rows, []string{"-", "-", "-", "-", "No extensions"})
	return ui.Connected(viewExtensions, "PostgreSQL Extensions", s.baseInfo(""), []string{"Name", "Version", "Schema", "Installed", "Comment"}, rows, "Name", extensionsActions()...), nil
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
	rows = pluginrpc.EnsureRows(rows, []string{"-", "-", "-", "-", "-", "-", "-", "-", "No active connections"})
	return ui.Connected(viewConnections, "PostgreSQL Active Connections", s.baseInfo(""), []string{"PID", "User", "Database", "Client", "State", "Backend", "Wait", "Duration", "Query"}, rows, "PID", connectionsActions()...), nil
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
	return ui.Connected(viewStats, "PostgreSQL Server Stats", s.baseInfo(""), []string{"Property", "Value"}, rows, "Property"), nil
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
	rows = pluginrpc.EnsureRows(rows, []string{"-", "-", "-", "-", "-", "-", "No settings"})
	return ui.Connected(viewConfig, "PostgreSQL Configuration", s.baseInfo(""), []string{"Parameter", "Value", "Unit", "Category", "Source", "Boot Value", "Restart?"}, rows, "Parameter", configActions()...), nil
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
	rows = pluginrpc.EnsureRows(rows, []string{"-", "-", "-", "-", "-", "No locks"})
	return ui.Connected(viewLocks, "PostgreSQL Locks", s.baseInfo(""), []string{"PID", "Lock Type", "Mode", "Relation", "Granted", "Wait Start"}, rows, "PID"), nil
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
	rows = pluginrpc.EnsureRows(rows, []string{"-", "-", "-", "-", "-", "-", "No indexes"})
	return ui.Connected(viewIndexes, "PostgreSQL Indexes", s.baseInfo(""), []string{"Schema", "Table", "Index", "Size", "Scans", "Tuples Read", "Tuples Fetched"}, rows, "Index", indexesActions()...), nil
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
	rows = pluginrpc.EnsureRows(rows, []string{"-", "-", "-", "No replicas", "-", "-", "-", "-", "-", "-", "-"})
	return ui.Connected(viewReplication, "PostgreSQL Replication", s.baseInfo(""), []string{"PID", "Application", "Client", "State", "Sent LSN", "Write LSN", "Flush LSN", "Replay LSN", "Write Lag", "Flush Lag", "Replay Lag"}, rows, "PID"), nil
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
	rows = pluginrpc.EnsureRows(rows, []string{"-", "-", "-", "No tablespaces"})
	return ui.Connected(viewTablespaces, "PostgreSQL Tablespaces", s.baseInfo(""), []string{"Name", "Owner", "Location", "Size"}, rows, "Name"), nil
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
	return ui.Connected(viewDbStats, "PostgreSQL Database Stats", s.baseInfo(""), []string{
		"Database", "Backends", "Commits", "Rollbacks",
		"Blks Read", "Blks Hit", "Cache Hit%",
		"Returned", "Fetched", "Inserted", "Updated", "Deleted",
		"Conflicts", "Deadlocks",
	}, data, "Database"), nil
}

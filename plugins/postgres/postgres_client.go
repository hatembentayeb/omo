package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// PostgresConnection represents a connection to a PostgreSQL server
type PostgresConnection struct {
	Host     string
	Port     int
	Username string
	Password string
	Database string
	SSLMode  string
}

// PostgresClient is a client for interacting with PostgreSQL
type PostgresClient struct {
	conn        *PostgresConnection
	db          *sql.DB
	ctx         context.Context
	connected   bool
	lastRefresh time.Time
}

// --- Data types ---

// PgUser represents a PostgreSQL role/user
type PgUser struct {
	Name            string
	Super           bool
	Inherit         bool
	CreateRole      bool
	CreateDB        bool
	CanLogin        bool
	Replication     bool
	BypassRLS       bool
	ConnLimit       int
	ValidUntil      string
	MemberOf        string
	ConnectionCount int
}

// PgDatabase represents a PostgreSQL database
type PgDatabase struct {
	Name       string
	Owner      string
	Encoding   string
	Collation  string
	CType      string
	Size       string
	Tablespace string
	ConnLimit  int
}

// PgTable represents a PostgreSQL table
type PgTable struct {
	Schema     string
	Name       string
	Owner      string
	Tablespace string
	RowCount   int64
	Size       string
	TotalSize  string
	HasIndexes bool
}

// PgSchema represents a PostgreSQL schema
type PgSchema struct {
	Name  string
	Owner string
}

// PgExtension represents a PostgreSQL extension
type PgExtension struct {
	Name      string
	Version   string
	Schema    string
	Comment   string
	Installed bool
}

// PgConnection represents an active connection
type PgConnection struct {
	PID         int
	User        string
	Database    string
	ClientAddr  string
	State       string
	Query       string
	BackendType string
	WaitEvent   string
	Duration    string
}

// PgStat represents server statistics
type PgStat struct {
	Key   string
	Value string
}

// PgLogEntry represents a log entry from pg_stat_activity
type PgLogEntry struct {
	PID       int
	User      string
	Database  string
	Query     string
	State     string
	StartedAt string
	Duration  string
}

// PgConfigEntry represents a PostgreSQL configuration parameter
type PgConfigEntry struct {
	Name        string
	Setting     string
	Unit        string
	Category    string
	Source      string
	BootVal     string
	PendRestart bool
}

// PgTablespace represents a PostgreSQL tablespace
type PgTablespace struct {
	Name     string
	Owner    string
	Location string
	Size     string
}

// PgIndex represents a PostgreSQL index
type PgIndex struct {
	Schema   string
	Table    string
	Name     string
	Size     string
	Scans    int64
	TupRead  int64
	TupFetch int64
	IndexDef string
}

// PgLock represents a PostgreSQL lock
type PgLock struct {
	PID       int
	Mode      string
	LockType  string
	Relation  string
	Granted   bool
	WaitStart string
}

// PgReplication represents replication status
type PgReplication struct {
	PID         int
	User        string
	Application string
	ClientAddr  string
	State       string
	SentLSN     string
	WriteLSN    string
	FlushLSN    string
	ReplayLSN   string
	WriteLag    string
	FlushLag    string
	ReplayLag   string
}

// NewPostgresClient creates a new PostgreSQL client
func NewPostgresClient() *PostgresClient {
	return &PostgresClient{
		conn:        nil,
		db:          nil,
		ctx:         context.Background(),
		connected:   false,
		lastRefresh: time.Time{},
	}
}

// Connect connects to a PostgreSQL server
func (c *PostgresClient) Connect(host string, port int, username, password, database, sslmode string) error {
	if host == "" {
		return errors.New("host cannot be empty")
	}
	if database == "" {
		database = "postgres"
	}
	if sslmode == "" {
		sslmode = "disable"
	}

	c.conn = &PostgresConnection{
		Host:     host,
		Port:     port,
		Username: username,
		Password: password,
		Database: database,
		SSLMode:  sslmode,
	}

	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=5",
		host, port, username, password, database, sslmode,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to open connection: %v", err)
	}

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(c.ctx, 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return fmt.Errorf("failed to connect to PostgreSQL: %v", err)
	}

	c.db = db
	c.connected = true
	c.lastRefresh = time.Now()

	return nil
}

// ConnectToInstance connects to a preconfigured PostgreSQL instance
func (c *PostgresClient) ConnectToInstance(instance PostgresInstance) error {
	port := instance.Port
	if port == 0 {
		port = 5432
	}
	return c.Connect(
		instance.Host,
		port,
		instance.Username,
		instance.Password,
		instance.Database,
		instance.SSLMode,
	)
}

// Disconnect disconnects from the PostgreSQL server
func (c *PostgresClient) Disconnect() error {
	if !c.connected {
		return errors.New("not connected to any PostgreSQL server")
	}
	if c.db != nil {
		if err := c.db.Close(); err != nil {
			return fmt.Errorf("error closing PostgreSQL connection: %v", err)
		}
	}
	c.conn = nil
	c.db = nil
	c.connected = false
	return nil
}

// IsConnected returns whether the client is connected
func (c *PostgresClient) IsConnected() bool {
	if !c.connected || c.db == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(c.ctx, 2*time.Second)
	defer cancel()
	return c.db.PingContext(ctx) == nil
}

// GetCurrentConnection returns the current connection details
func (c *PostgresClient) GetCurrentConnection() *PostgresConnection {
	return c.conn
}

// GetLastRefreshTime returns the time of the last refresh
func (c *PostgresClient) GetLastRefreshTime() time.Time {
	return c.lastRefresh
}

// SetLastRefreshTime sets the time of the last refresh
func (c *PostgresClient) SetLastRefreshTime(t time.Time) {
	c.lastRefresh = t
}

// --- User / Role Management ---

// GetUsers returns all PostgreSQL roles/users
func (c *PostgresClient) GetUsers() ([]PgUser, error) {
	if !c.connected || c.db == nil {
		return nil, errors.New("not connected")
	}

	query := `
		SELECT r.rolname,
			   r.rolsuper, r.rolinherit, r.rolcreaterole, r.rolcreatedb,
			   r.rolcanlogin, r.rolreplication, r.rolbypassrls,
			   r.rolconnlimit,
			   COALESCE(r.rolvaliduntil::text, 'never'),
			   COALESCE(string_agg(m.rolname, ', '), ''),
			   (SELECT count(*) FROM pg_stat_activity WHERE usename = r.rolname)
		FROM pg_roles r
		LEFT JOIN pg_auth_members am ON am.member = r.oid
		LEFT JOIN pg_roles m ON m.oid = am.roleid
		GROUP BY r.rolname, r.rolsuper, r.rolinherit, r.rolcreaterole,
				 r.rolcreatedb, r.rolcanlogin, r.rolreplication,
				 r.rolbypassrls, r.rolconnlimit, r.rolvaliduntil
		ORDER BY r.rolname`

	rows, err := c.db.QueryContext(c.ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %v", err)
	}
	defer rows.Close()

	var users []PgUser
	for rows.Next() {
		var u PgUser
		if err := rows.Scan(
			&u.Name, &u.Super, &u.Inherit, &u.CreateRole, &u.CreateDB,
			&u.CanLogin, &u.Replication, &u.BypassRLS,
			&u.ConnLimit, &u.ValidUntil, &u.MemberOf, &u.ConnectionCount,
		); err != nil {
			return nil, fmt.Errorf("failed to scan user: %v", err)
		}
		users = append(users, u)
	}
	return users, nil
}

// CreateUser creates a new PostgreSQL user/role
func (c *PostgresClient) CreateUser(name, password string, canLogin, createDB, createRole, superuser bool) error {
	if !c.connected || c.db == nil {
		return errors.New("not connected")
	}

	var opts []string
	if canLogin {
		opts = append(opts, "LOGIN")
	} else {
		opts = append(opts, "NOLOGIN")
	}
	if createDB {
		opts = append(opts, "CREATEDB")
	}
	if createRole {
		opts = append(opts, "CREATEROLE")
	}
	if superuser {
		opts = append(opts, "SUPERUSER")
	}
	if password != "" {
		opts = append(opts, fmt.Sprintf("PASSWORD '%s'", password))
	}

	query := fmt.Sprintf("CREATE ROLE %s %s", quoteIdent(name), strings.Join(opts, " "))
	_, err := c.db.ExecContext(c.ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create user: %v", err)
	}
	return nil
}

// DropUser drops a PostgreSQL user/role
func (c *PostgresClient) DropUser(name string) error {
	if !c.connected || c.db == nil {
		return errors.New("not connected")
	}

	query := fmt.Sprintf("DROP ROLE IF EXISTS %s", quoteIdent(name))
	_, err := c.db.ExecContext(c.ctx, query)
	if err != nil {
		return fmt.Errorf("failed to drop user: %v", err)
	}
	return nil
}

// AlterUserPassword changes a user's password
func (c *PostgresClient) AlterUserPassword(name, password string) error {
	if !c.connected || c.db == nil {
		return errors.New("not connected")
	}

	query := fmt.Sprintf("ALTER ROLE %s PASSWORD '%s'", quoteIdent(name), password)
	_, err := c.db.ExecContext(c.ctx, query)
	if err != nil {
		return fmt.Errorf("failed to alter password: %v", err)
	}
	return nil
}

// GrantRole grants a role to a user
func (c *PostgresClient) GrantRole(role, user string) error {
	if !c.connected || c.db == nil {
		return errors.New("not connected")
	}

	query := fmt.Sprintf("GRANT %s TO %s", quoteIdent(role), quoteIdent(user))
	_, err := c.db.ExecContext(c.ctx, query)
	if err != nil {
		return fmt.Errorf("failed to grant role: %v", err)
	}
	return nil
}

// RevokeRole revokes a role from a user
func (c *PostgresClient) RevokeRole(role, user string) error {
	if !c.connected || c.db == nil {
		return errors.New("not connected")
	}

	query := fmt.Sprintf("REVOKE %s FROM %s", quoteIdent(role), quoteIdent(user))
	_, err := c.db.ExecContext(c.ctx, query)
	if err != nil {
		return fmt.Errorf("failed to revoke role: %v", err)
	}
	return nil
}

// AlterUserAttribute alters a boolean attribute on a user
func (c *PostgresClient) AlterUserAttribute(name, attribute string, value bool) error {
	if !c.connected || c.db == nil {
		return errors.New("not connected")
	}

	prefix := ""
	if !value {
		prefix = "NO"
	}
	query := fmt.Sprintf("ALTER ROLE %s %s%s", quoteIdent(name), prefix, attribute)
	_, err := c.db.ExecContext(c.ctx, query)
	if err != nil {
		return fmt.Errorf("failed to alter user attribute: %v", err)
	}
	return nil
}

// --- Database Management ---

// GetDatabases returns all PostgreSQL databases
func (c *PostgresClient) GetDatabases() ([]PgDatabase, error) {
	if !c.connected || c.db == nil {
		return nil, errors.New("not connected")
	}

	query := `
		SELECT d.datname,
			   pg_catalog.pg_get_userbyid(d.datdba),
			   pg_catalog.pg_encoding_to_char(d.encoding),
			   d.datcollate,
			   d.datctype,
			   pg_catalog.pg_size_pretty(pg_catalog.pg_database_size(d.datname)),
			   COALESCE(t.spcname, 'pg_default'),
			   d.datconnlimit
		FROM pg_database d
		LEFT JOIN pg_tablespace t ON d.dattablespace = t.oid
		ORDER BY d.datname`

	rows, err := c.db.QueryContext(c.ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query databases: %v", err)
	}
	defer rows.Close()

	var databases []PgDatabase
	for rows.Next() {
		var d PgDatabase
		if err := rows.Scan(
			&d.Name, &d.Owner, &d.Encoding, &d.Collation,
			&d.CType, &d.Size, &d.Tablespace, &d.ConnLimit,
		); err != nil {
			return nil, fmt.Errorf("failed to scan database: %v", err)
		}
		databases = append(databases, d)
	}
	return databases, nil
}

// CreateDatabase creates a new PostgreSQL database
func (c *PostgresClient) CreateDatabase(name, owner, encoding string) error {
	if !c.connected || c.db == nil {
		return errors.New("not connected")
	}

	opts := []string{}
	if owner != "" {
		opts = append(opts, fmt.Sprintf("OWNER = %s", quoteIdent(owner)))
	}
	if encoding != "" {
		opts = append(opts, fmt.Sprintf("ENCODING = '%s'", encoding))
	}

	query := fmt.Sprintf("CREATE DATABASE %s", quoteIdent(name))
	if len(opts) > 0 {
		query += " " + strings.Join(opts, " ")
	}

	_, err := c.db.ExecContext(c.ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create database: %v", err)
	}
	return nil
}

// DropDatabase drops a PostgreSQL database
func (c *PostgresClient) DropDatabase(name string) error {
	if !c.connected || c.db == nil {
		return errors.New("not connected")
	}

	query := fmt.Sprintf("DROP DATABASE IF EXISTS %s", quoteIdent(name))
	_, err := c.db.ExecContext(c.ctx, query)
	if err != nil {
		return fmt.Errorf("failed to drop database: %v", err)
	}
	return nil
}

// --- Table Management ---

// GetTables returns all tables in the current database
func (c *PostgresClient) GetTables() ([]PgTable, error) {
	if !c.connected || c.db == nil {
		return nil, errors.New("not connected")
	}

	query := `
		SELECT schemaname,
			   relname,
			   pg_catalog.pg_get_userbyid(c.relowner),
			   COALESCE(t.spcname, 'pg_default'),
			   c.reltuples::bigint,
			   pg_size_pretty(pg_relation_size(c.oid)),
			   pg_size_pretty(pg_total_relation_size(c.oid)),
			   c.relhasindex
		FROM pg_stat_user_tables s
		JOIN pg_class c ON c.relname = s.relname
			AND c.relnamespace = (SELECT oid FROM pg_namespace WHERE nspname = s.schemaname)
		LEFT JOIN pg_tablespace t ON c.reltablespace = t.oid
		ORDER BY schemaname, relname`

	rows, err := c.db.QueryContext(c.ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %v", err)
	}
	defer rows.Close()

	var tables []PgTable
	for rows.Next() {
		var t PgTable
		if err := rows.Scan(
			&t.Schema, &t.Name, &t.Owner, &t.Tablespace,
			&t.RowCount, &t.Size, &t.TotalSize, &t.HasIndexes,
		); err != nil {
			return nil, fmt.Errorf("failed to scan table: %v", err)
		}
		tables = append(tables, t)
	}
	return tables, nil
}

// GetTableColumns returns columns for a specific table
func (c *PostgresClient) GetTableColumns(schema, table string) ([][]string, error) {
	if !c.connected || c.db == nil {
		return nil, errors.New("not connected")
	}

	query := `
		SELECT column_name, data_type, is_nullable,
			   COALESCE(column_default, ''),
			   COALESCE(character_maximum_length::text, '')
		FROM information_schema.columns
		WHERE table_schema = $1 AND table_name = $2
		ORDER BY ordinal_position`

	rows, err := c.db.QueryContext(c.ctx, query, schema, table)
	if err != nil {
		return nil, fmt.Errorf("failed to query columns: %v", err)
	}
	defer rows.Close()

	var columns [][]string
	for rows.Next() {
		var name, dtype, nullable, defaultVal, maxLen string
		if err := rows.Scan(&name, &dtype, &nullable, &defaultVal, &maxLen); err != nil {
			return nil, fmt.Errorf("failed to scan column: %v", err)
		}
		columns = append(columns, []string{name, dtype, nullable, defaultVal, maxLen})
	}
	return columns, nil
}

// --- Schema Management ---

// GetSchemas returns all schemas
func (c *PostgresClient) GetSchemas() ([]PgSchema, error) {
	if !c.connected || c.db == nil {
		return nil, errors.New("not connected")
	}

	query := `
		SELECT nspname, pg_catalog.pg_get_userbyid(nspowner)
		FROM pg_namespace
		WHERE nspname NOT LIKE 'pg_%' AND nspname != 'information_schema'
		ORDER BY nspname`

	rows, err := c.db.QueryContext(c.ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query schemas: %v", err)
	}
	defer rows.Close()

	var schemas []PgSchema
	for rows.Next() {
		var s PgSchema
		if err := rows.Scan(&s.Name, &s.Owner); err != nil {
			return nil, fmt.Errorf("failed to scan schema: %v", err)
		}
		schemas = append(schemas, s)
	}
	return schemas, nil
}

// --- Extension Management ---

// GetExtensions returns installed and available extensions
func (c *PostgresClient) GetExtensions() ([]PgExtension, error) {
	if !c.connected || c.db == nil {
		return nil, errors.New("not connected")
	}

	query := `
		SELECT a.name,
			   COALESCE(e.extversion, a.default_version),
			   COALESCE(n.nspname, ''),
			   COALESCE(a.comment, ''),
			   e.extname IS NOT NULL AS installed
		FROM pg_available_extensions a
		LEFT JOIN pg_extension e ON e.extname = a.name
		LEFT JOIN pg_namespace n ON n.oid = e.extnamespace
		ORDER BY a.name`

	rows, err := c.db.QueryContext(c.ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query extensions: %v", err)
	}
	defer rows.Close()

	var extensions []PgExtension
	for rows.Next() {
		var ext PgExtension
		if err := rows.Scan(&ext.Name, &ext.Version, &ext.Schema, &ext.Comment, &ext.Installed); err != nil {
			return nil, fmt.Errorf("failed to scan extension: %v", err)
		}
		extensions = append(extensions, ext)
	}
	return extensions, nil
}

// CreateExtension installs an extension
func (c *PostgresClient) CreateExtension(name string) error {
	if !c.connected || c.db == nil {
		return errors.New("not connected")
	}

	query := fmt.Sprintf("CREATE EXTENSION IF NOT EXISTS %s", quoteIdent(name))
	_, err := c.db.ExecContext(c.ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create extension: %v", err)
	}
	return nil
}

// DropExtension removes an extension
func (c *PostgresClient) DropExtension(name string) error {
	if !c.connected || c.db == nil {
		return errors.New("not connected")
	}

	query := fmt.Sprintf("DROP EXTENSION IF EXISTS %s", quoteIdent(name))
	_, err := c.db.ExecContext(c.ctx, query)
	if err != nil {
		return fmt.Errorf("failed to drop extension: %v", err)
	}
	return nil
}

// --- Active Connections ---

// GetActiveConnections returns active connections
func (c *PostgresClient) GetActiveConnections() ([]PgConnection, error) {
	if !c.connected || c.db == nil {
		return nil, errors.New("not connected")
	}

	query := `
		SELECT pid, COALESCE(usename, ''), COALESCE(datname, ''),
			   COALESCE(client_addr::text, 'local'),
			   COALESCE(state, 'unknown'),
			   COALESCE(LEFT(query, 200), ''),
			   COALESCE(backend_type, ''),
			   COALESCE(wait_event, ''),
			   COALESCE(EXTRACT(EPOCH FROM (now() - backend_start))::int::text || 's', '')
		FROM pg_stat_activity
		WHERE pid != pg_backend_pid()
		ORDER BY backend_start DESC`

	rows, err := c.db.QueryContext(c.ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query connections: %v", err)
	}
	defer rows.Close()

	var conns []PgConnection
	for rows.Next() {
		var conn PgConnection
		if err := rows.Scan(
			&conn.PID, &conn.User, &conn.Database, &conn.ClientAddr,
			&conn.State, &conn.Query, &conn.BackendType, &conn.WaitEvent, &conn.Duration,
		); err != nil {
			return nil, fmt.Errorf("failed to scan connection: %v", err)
		}
		conns = append(conns, conn)
	}
	return conns, nil
}

// TerminateConnection terminates a backend connection
func (c *PostgresClient) TerminateConnection(pid int) error {
	if !c.connected || c.db == nil {
		return errors.New("not connected")
	}

	var terminated bool
	err := c.db.QueryRowContext(c.ctx, "SELECT pg_terminate_backend($1)", pid).Scan(&terminated)
	if err != nil {
		return fmt.Errorf("failed to terminate connection: %v", err)
	}
	if !terminated {
		return fmt.Errorf("connection %d could not be terminated", pid)
	}
	return nil
}

// CancelQuery cancels a running query
func (c *PostgresClient) CancelQuery(pid int) error {
	if !c.connected || c.db == nil {
		return errors.New("not connected")
	}

	var cancelled bool
	err := c.db.QueryRowContext(c.ctx, "SELECT pg_cancel_backend($1)", pid).Scan(&cancelled)
	if err != nil {
		return fmt.Errorf("failed to cancel query: %v", err)
	}
	if !cancelled {
		return fmt.Errorf("query on connection %d could not be cancelled", pid)
	}
	return nil
}

// --- Server Stats ---

// GetServerInfo returns key server information
func (c *PostgresClient) GetServerInfo() ([]PgStat, error) {
	if !c.connected || c.db == nil {
		return nil, errors.New("not connected")
	}

	var version string
	c.db.QueryRowContext(c.ctx, "SELECT version()").Scan(&version)

	var uptime string
	c.db.QueryRowContext(c.ctx, "SELECT now() - pg_postmaster_start_time()").Scan(&uptime)

	var dbSize string
	c.db.QueryRowContext(c.ctx, "SELECT pg_size_pretty(sum(pg_database_size(datname))) FROM pg_database").Scan(&dbSize)

	var activeConns int
	c.db.QueryRowContext(c.ctx, "SELECT count(*) FROM pg_stat_activity WHERE state = 'active'").Scan(&activeConns)

	var totalConns int
	c.db.QueryRowContext(c.ctx, "SELECT count(*) FROM pg_stat_activity").Scan(&totalConns)

	var maxConns int
	c.db.QueryRowContext(c.ctx, "SHOW max_connections").Scan(&maxConns)

	var dbCount int
	c.db.QueryRowContext(c.ctx, "SELECT count(*) FROM pg_database WHERE NOT datistemplate").Scan(&dbCount)

	var userCount int
	c.db.QueryRowContext(c.ctx, "SELECT count(*) FROM pg_roles WHERE rolcanlogin").Scan(&userCount)

	stats := []PgStat{
		{"version", version},
		{"uptime", uptime},
		{"total_size", dbSize},
		{"active_connections", fmt.Sprintf("%d", activeConns)},
		{"total_connections", fmt.Sprintf("%d", totalConns)},
		{"max_connections", fmt.Sprintf("%d", maxConns)},
		{"databases", fmt.Sprintf("%d", dbCount)},
		{"login_roles", fmt.Sprintf("%d", userCount)},
	}

	return stats, nil
}

// GetDatabaseStats returns per-database statistics
func (c *PostgresClient) GetDatabaseStats() ([][]string, error) {
	if !c.connected || c.db == nil {
		return nil, errors.New("not connected")
	}

	query := `
		SELECT datname,
			   numbackends::text,
			   xact_commit::text,
			   xact_rollback::text,
			   blks_read::text,
			   blks_hit::text,
			   CASE WHEN blks_hit + blks_read > 0
					THEN round(100.0 * blks_hit / (blks_hit + blks_read), 2)::text || '%'
					ELSE 'N/A'
			   END,
			   tup_returned::text,
			   tup_fetched::text,
			   tup_inserted::text,
			   tup_updated::text,
			   tup_deleted::text,
			   conflicts::text,
			   deadlocks::text
		FROM pg_stat_database
		WHERE datname IS NOT NULL AND NOT datistemplate
		ORDER BY datname`

	rows, err := c.db.QueryContext(c.ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query database stats: %v", err)
	}
	defer rows.Close()

	var data [][]string
	for rows.Next() {
		row := make([]string, 14)
		ptrs := make([]interface{}, 14)
		for i := range row {
			ptrs[i] = &row[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %v", err)
		}
		data = append(data, row)
	}
	return data, nil
}

// --- Configuration ---

// GetConfig returns server configuration parameters
func (c *PostgresClient) GetConfig(pattern string) ([]PgConfigEntry, error) {
	if !c.connected || c.db == nil {
		return nil, errors.New("not connected")
	}

	query := `
		SELECT name, setting, COALESCE(unit, ''), category,
			   source, boot_val, pending_restart
		FROM pg_settings
		WHERE name LIKE $1
		ORDER BY category, name`

	if pattern == "" {
		pattern = "%"
	}

	rows, err := c.db.QueryContext(c.ctx, query, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to query config: %v", err)
	}
	defer rows.Close()

	var configs []PgConfigEntry
	for rows.Next() {
		var cfg PgConfigEntry
		if err := rows.Scan(
			&cfg.Name, &cfg.Setting, &cfg.Unit, &cfg.Category,
			&cfg.Source, &cfg.BootVal, &cfg.PendRestart,
		); err != nil {
			return nil, fmt.Errorf("failed to scan config: %v", err)
		}
		configs = append(configs, cfg)
	}
	return configs, nil
}

// AlterConfig changes a configuration parameter (runtime)
func (c *PostgresClient) AlterConfig(name, value string) error {
	if !c.connected || c.db == nil {
		return errors.New("not connected")
	}

	query := fmt.Sprintf("ALTER SYSTEM SET %s = '%s'", name, value)
	_, err := c.db.ExecContext(c.ctx, query)
	if err != nil {
		return fmt.Errorf("failed to alter config: %v", err)
	}

	// Reload configuration
	_, err = c.db.ExecContext(c.ctx, "SELECT pg_reload_conf()")
	if err != nil {
		return fmt.Errorf("config set but reload failed: %v", err)
	}
	return nil
}

// --- Logs (from pg_stat_activity) ---

// GetActivityLog returns current activity as a log-like view
func (c *PostgresClient) GetActivityLog() ([]PgLogEntry, error) {
	if !c.connected || c.db == nil {
		return nil, errors.New("not connected")
	}

	query := `
		SELECT pid,
			   COALESCE(usename, ''),
			   COALESCE(datname, ''),
			   COALESCE(LEFT(query, 300), ''),
			   COALESCE(state, 'unknown'),
			   COALESCE(query_start::text, ''),
			   COALESCE(EXTRACT(EPOCH FROM (now() - query_start))::int::text || 's', '')
		FROM pg_stat_activity
		WHERE pid != pg_backend_pid()
			AND state IS NOT NULL
		ORDER BY query_start DESC NULLS LAST`

	rows, err := c.db.QueryContext(c.ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query activity: %v", err)
	}
	defer rows.Close()

	var entries []PgLogEntry
	for rows.Next() {
		var e PgLogEntry
		if err := rows.Scan(&e.PID, &e.User, &e.Database, &e.Query, &e.State, &e.StartedAt, &e.Duration); err != nil {
			return nil, fmt.Errorf("failed to scan activity: %v", err)
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// --- Locks ---

// GetLocks returns current locks
func (c *PostgresClient) GetLocks() ([]PgLock, error) {
	if !c.connected || c.db == nil {
		return nil, errors.New("not connected")
	}

	query := `
		SELECT l.pid,
			   l.mode,
			   l.locktype,
			   COALESCE(c.relname, l.locktype),
			   l.granted,
			   COALESCE(l.waitstart::text, '')
		FROM pg_locks l
		LEFT JOIN pg_class c ON c.oid = l.relation
		WHERE l.pid != pg_backend_pid()
		ORDER BY l.pid`

	rows, err := c.db.QueryContext(c.ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query locks: %v", err)
	}
	defer rows.Close()

	var locks []PgLock
	for rows.Next() {
		var lk PgLock
		if err := rows.Scan(&lk.PID, &lk.Mode, &lk.LockType, &lk.Relation, &lk.Granted, &lk.WaitStart); err != nil {
			return nil, fmt.Errorf("failed to scan lock: %v", err)
		}
		locks = append(locks, lk)
	}
	return locks, nil
}

// --- Tablespaces ---

// GetTablespaces returns all tablespaces
func (c *PostgresClient) GetTablespaces() ([]PgTablespace, error) {
	if !c.connected || c.db == nil {
		return nil, errors.New("not connected")
	}

	query := `
		SELECT spcname,
			   pg_catalog.pg_get_userbyid(spcowner),
			   COALESCE(pg_catalog.pg_tablespace_location(oid), ''),
			   pg_size_pretty(pg_tablespace_size(spcname))
		FROM pg_tablespace
		ORDER BY spcname`

	rows, err := c.db.QueryContext(c.ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query tablespaces: %v", err)
	}
	defer rows.Close()

	var tablespaces []PgTablespace
	for rows.Next() {
		var ts PgTablespace
		if err := rows.Scan(&ts.Name, &ts.Owner, &ts.Location, &ts.Size); err != nil {
			return nil, fmt.Errorf("failed to scan tablespace: %v", err)
		}
		tablespaces = append(tablespaces, ts)
	}
	return tablespaces, nil
}

// --- Indexes ---

// GetIndexes returns index statistics
func (c *PostgresClient) GetIndexes() ([]PgIndex, error) {
	if !c.connected || c.db == nil {
		return nil, errors.New("not connected")
	}

	query := `
		SELECT schemaname, relname, indexrelname,
			   pg_size_pretty(pg_relation_size(indexrelid)),
			   idx_scan, idx_tup_read, idx_tup_fetch,
			   pg_get_indexdef(indexrelid)
		FROM pg_stat_user_indexes
		ORDER BY schemaname, relname, indexrelname`

	rows, err := c.db.QueryContext(c.ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query indexes: %v", err)
	}
	defer rows.Close()

	var indexes []PgIndex
	for rows.Next() {
		var idx PgIndex
		if err := rows.Scan(
			&idx.Schema, &idx.Table, &idx.Name, &idx.Size,
			&idx.Scans, &idx.TupRead, &idx.TupFetch, &idx.IndexDef,
		); err != nil {
			return nil, fmt.Errorf("failed to scan index: %v", err)
		}
		indexes = append(indexes, idx)
	}
	return indexes, nil
}

// --- Replication ---

// GetReplicationStatus returns replication status
func (c *PostgresClient) GetReplicationStatus() ([]PgReplication, error) {
	if !c.connected || c.db == nil {
		return nil, errors.New("not connected")
	}

	query := `
		SELECT pid, COALESCE(usesysid::text, ''),
			   COALESCE(application_name, ''),
			   COALESCE(client_addr::text, 'local'),
			   COALESCE(state, ''),
			   COALESCE(sent_lsn::text, ''),
			   COALESCE(write_lsn::text, ''),
			   COALESCE(flush_lsn::text, ''),
			   COALESCE(replay_lsn::text, ''),
			   COALESCE(write_lag::text, ''),
			   COALESCE(flush_lag::text, ''),
			   COALESCE(replay_lag::text, '')
		FROM pg_stat_replication
		ORDER BY pid`

	rows, err := c.db.QueryContext(c.ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query replication: %v", err)
	}
	defer rows.Close()

	var replicas []PgReplication
	for rows.Next() {
		var r PgReplication
		if err := rows.Scan(
			&r.PID, &r.User, &r.Application, &r.ClientAddr,
			&r.State, &r.SentLSN, &r.WriteLSN, &r.FlushLSN, &r.ReplayLSN,
			&r.WriteLag, &r.FlushLag, &r.ReplayLag,
		); err != nil {
			return nil, fmt.Errorf("failed to scan replication: %v", err)
		}
		replicas = append(replicas, r)
	}
	return replicas, nil
}

// --- Helpers ---

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func boolToStr(b bool) string {
	if b {
		return "[green]Yes"
	}
	return "[red]No"
}

package postgres

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"omo/pkg/pluginapi"
	"omo/pkg/pluginrpc"
)

// Service is the RPC-facing postgres backend (no tview).
type Service struct {
	mu          sync.Mutex
	client      *PostgresClient
	host        string
	port        int
	username    string
	password    string
	database    string
	sslmode     string
	name        string
	currentView string
}

// NewService creates a postgres RPC service.
func NewService() *Service {
	return &Service{
		client:      NewPostgresClient(),
		port:        5432,
		database:    "postgres",
		sslmode:     "disable",
		currentView: viewUsers,
	}
}

func (s *Service) GetMetadata() (pluginapi.PluginMetadata, error) {
	return pluginapi.PluginMetadata{
		Name:        "postgres",
		Version:     "1.0.0",
		Description: "PostgreSQL user & configuration management plugin",
		Author:      "OhMyOps Team",
		License:     "MIT",
		Tags:        []string{"database", "sql", "postgresql", "users", "management"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "https://github.com/hatembentayeb/omo/plugins/postgres",
	}, nil
}

func (s *Service) Configure(req pluginrpc.ConfigureRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	pluginrpc.RPCLog("Service.Configure begin")

	if req.Settings == nil {
		return fmt.Errorf("missing settings")
	}
	s.host = req.Settings["host"]
	s.port = 5432
	if p := req.Settings["port"]; p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			s.port = n
		}
	}
	s.username = req.Settings["username"]
	s.password = req.Settings["password"]
	s.name = req.Settings["name"]
	s.database = req.Settings["database"]
	if s.database == "" {
		s.database = "postgres"
	}
	s.sslmode = req.Settings["sslmode"]
	if s.sslmode == "" {
		s.sslmode = "disable"
	}
	if s.host == "" {
		return fmt.Errorf("host is required")
	}

	pluginrpc.RPCLog("Service.Configure host=%s port=%d user=%s db=%s", s.host, s.port, s.username, s.database)

	if s.client != nil && s.client.IsConnected() {
		_ = s.client.Disconnect()
	}
	return nil
}

func (s *Service) GetView(req pluginrpc.ViewRequest) (pluginrpc.ViewData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	viewID := req.View
	if viewID == "" {
		viewID = s.currentView
	}
	if viewID == "" {
		viewID = viewUsers
	}
	pluginrpc.RPCLog("Service.GetView begin view=%s host=%s connected=%v", viewID, s.host, s.client != nil && s.client.IsConnected())
	start := time.Now()
	view, err := s.buildViewLocked(viewID)
	if err != nil {
		pluginrpc.RPCLog("Service.GetView err=%v", err)
		return pluginrpc.ViewData{}, err
	}
	pluginrpc.RPCLog("Service.GetView OK view=%s rows=%d dur=%s", view.View, len(view.Rows), time.Since(start))
	return view, nil
}

func (s *Service) DoAction(req pluginrpc.ActionRequest) (pluginrpc.ActionResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	action := req.Action
	if strings.HasPrefix(action, "goto_") {
		viewID := strings.TrimPrefix(action, "goto_")
		view, err := s.buildViewLocked(viewID)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "switched to " + viewID, Next: &view}, nil
	}

	switch action {
	case "refresh", "":
		view, err := s.buildViewLocked(s.currentView)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "refreshed", Next: &view}, nil

	case "create_user":
		user := firstNonEmpty(req.Payload["username"], req.Payload["key"], req.Payload["name"])
		password := req.Payload["password"]
		if user == "" {
			return pluginrpc.ActionResult{OK: false, Message: "username required"}, nil
		}
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		if err := s.client.CreateUser(user, password, true, false, false, false); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewUsers)
		return pluginrpc.ActionResult{OK: true, Message: "created user " + user, Next: &view}, nil

	case "drop_user", "delete":
		user := firstNonEmpty(req.Payload["username"], req.Payload["key"])
		if user == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no user selected"}, nil
		}
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		if s.currentView == viewUsers || action == "drop_user" {
			if err := s.client.DropUser(user); err != nil {
				return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
			}
			view, _ := s.buildViewLocked(viewUsers)
			return pluginrpc.ActionResult{OK: true, Message: "dropped user " + user, Next: &view}, nil
		}
		return pluginrpc.ActionResult{OK: false, Message: "delete not supported in this view"}, nil

	case "change_password":
		user := firstNonEmpty(req.Payload["username"], req.Payload["key"])
		password := req.Payload["password"]
		if user == "" || password == "" {
			return pluginrpc.ActionResult{OK: false, Message: "username and password required"}, nil
		}
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		if err := s.client.AlterUserPassword(user, password); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewUsers)
		return pluginrpc.ActionResult{OK: true, Message: "password updated for " + user, Next: &view}, nil

	case "grant_role":
		user := firstNonEmpty(req.Payload["username"], req.Payload["key"])
		role := req.Payload["role"]
		if user == "" || role == "" {
			return pluginrpc.ActionResult{OK: false, Message: "username and role required"}, nil
		}
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		if err := s.client.GrantRole(role, user); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewUsers)
		return pluginrpc.ActionResult{OK: true, Message: fmt.Sprintf("granted %s to %s", role, user), Next: &view}, nil

	case "revoke_role":
		user := firstNonEmpty(req.Payload["username"], req.Payload["key"])
		role := req.Payload["role"]
		if user == "" || role == "" {
			return pluginrpc.ActionResult{OK: false, Message: "username and role required"}, nil
		}
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		if err := s.client.RevokeRole(role, user); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewUsers)
		return pluginrpc.ActionResult{OK: true, Message: fmt.Sprintf("revoked %s from %s", role, user), Next: &view}, nil

	case "create_database":
		name := firstNonEmpty(req.Payload["name"], req.Payload["key"], req.Payload["database"])
		owner := req.Payload["owner"]
		encoding := req.Payload["encoding"]
		if encoding == "" {
			encoding = "UTF8"
		}
		if name == "" {
			return pluginrpc.ActionResult{OK: false, Message: "database name required"}, nil
		}
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		if err := s.client.CreateDatabase(name, owner, encoding); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewDatabases)
		return pluginrpc.ActionResult{OK: true, Message: "created database " + name, Next: &view}, nil

	case "drop_database":
		name := strings.TrimSuffix(firstNonEmpty(req.Payload["name"], req.Payload["key"]), " *")
		if name == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no database selected"}, nil
		}
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		if err := s.client.DropDatabase(name); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewDatabases)
		return pluginrpc.ActionResult{OK: true, Message: "dropped database " + name, Next: &view}, nil

	case "select_database", "select_db":
		name := strings.TrimSuffix(firstNonEmpty(req.Payload["database"], req.Payload["name"], req.Payload["key"], req.Payload["db"]), " *")
		if name == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no database selected"}, nil
		}
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		if err := s.client.SwitchDatabase(name); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		s.database = name
		view, _ := s.buildViewLocked(viewTables)
		return pluginrpc.ActionResult{
			OK:      true,
			Message: "switched to database " + name,
			Next:    &view,
		}, nil

	case "install_extension":
		name := firstNonEmpty(req.Payload["name"], req.Payload["key"])
		if name == "" {
			return pluginrpc.ActionResult{OK: false, Message: "extension name required"}, nil
		}
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		if err := s.client.CreateExtension(name); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewExtensions)
		return pluginrpc.ActionResult{OK: true, Message: "installed extension " + name, Next: &view}, nil

	case "drop_extension":
		name := firstNonEmpty(req.Payload["name"], req.Payload["key"])
		if name == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no extension selected"}, nil
		}
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		if err := s.client.DropExtension(name); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewExtensions)
		return pluginrpc.ActionResult{OK: true, Message: "dropped extension " + name, Next: &view}, nil

	case "kill_connection", "terminate_connection":
		pidStr := firstNonEmpty(req.Payload["pid"], req.Payload["key"])
		pid, err := strconv.Atoi(pidStr)
		if err != nil || pid <= 0 {
			return pluginrpc.ActionResult{OK: false, Message: "no connection selected"}, nil
		}
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		if err := s.client.TerminateConnection(pid); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewConnections)
		return pluginrpc.ActionResult{OK: true, Message: fmt.Sprintf("terminated PID %d", pid), Next: &view}, nil

	case "cancel_query":
		pidStr := firstNonEmpty(req.Payload["pid"], req.Payload["key"])
		pid, err := strconv.Atoi(pidStr)
		if err != nil || pid <= 0 {
			return pluginrpc.ActionResult{OK: false, Message: "no connection selected"}, nil
		}
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		if err := s.client.CancelQuery(pid); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewConnections)
		return pluginrpc.ActionResult{OK: true, Message: fmt.Sprintf("cancelled query on PID %d", pid), Next: &view}, nil

	case "edit_config":
		name := firstNonEmpty(req.Payload["name"], req.Payload["key"], req.Payload["parameter"])
		value := req.Payload["value"]
		if name == "" {
			return pluginrpc.ActionResult{OK: false, Message: "parameter name required"}, nil
		}
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		if err := s.client.AlterConfig(name, value); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewConfig)
		return pluginrpc.ActionResult{OK: true, Message: "updated " + name, Next: &view}, nil

	case "view_columns":
		schema := firstNonEmpty(req.Payload["schema"], req.Payload["col0"])
		table := firstNonEmpty(req.Payload["table"], req.Payload["col1"], req.Payload["key"])
		if schema == "" || table == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no table selected"}, nil
		}
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		cols, err := s.client.GetTableColumns(schema, table)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("Table: %s.%s\n\n", schema, table))
		for _, c := range cols {
			b.WriteString(strings.Join(c, " | "))
			b.WriteString("\n")
		}
		return pluginrpc.ActionResult{OK: true, ModalTitle: fmt.Sprintf("Columns: %s.%s", schema, table), ModalBody: b.String()}, nil

	case "view_index":
		schema := firstNonEmpty(req.Payload["schema"], req.Payload["col0"])
		table := firstNonEmpty(req.Payload["table"], req.Payload["col1"])
		name := firstNonEmpty(req.Payload["name"], req.Payload["col2"], req.Payload["key"])
		if name == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no index selected"}, nil
		}
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		indexes, err := s.client.GetIndexes()
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		for _, idx := range indexes {
			if idx.Name == name && (schema == "" || idx.Schema == schema) && (table == "" || idx.Table == table) {
				return pluginrpc.ActionResult{
					OK:         true,
					ModalTitle: "Index: " + name,
					ModalBody:  idx.IndexDef,
				}, nil
			}
		}
		return pluginrpc.ActionResult{OK: false, Message: "index not found"}, nil

	default:
		return pluginrpc.ActionResult{OK: false, Message: "unknown action: " + action}, nil
	}
}

func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client != nil && s.client.IsConnected() {
		return s.client.Disconnect()
	}
	return nil
}

func (s *Service) ensureConnectedLocked() error {
	if s.client != nil && s.client.IsConnected() {
		return nil
	}
	if s.host == "" {
		return fmt.Errorf("not configured (host did not call Configure)")
	}
	pluginrpc.RPCLog("ensureConnected: dialing %s:%d user=%s …", s.host, s.port, s.username)
	start := time.Now()
	err := s.client.Connect(s.host, s.port, s.username, s.password, s.database, s.sslmode)
	pluginrpc.RPCLog("ensureConnected: dial done err=%v dur=%s", err, time.Since(start))
	return err
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

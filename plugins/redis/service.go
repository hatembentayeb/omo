package redis

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"omo/pkg/pluginapi"
	"omo/pkg/pluginrpc"
)

// Service is the RPC-facing redis backend (no tview).
// Connection settings are pushed from the host via Configure so GetView
// never calls back into the host secrets broker (avoids net/rpc deadlocks).
type Service struct {
	mu          sync.Mutex
	client      *RedisClient
	host        string
	port        string
	username    string
	password    string
	database    int
	name        string
	currentView string
}

// NewService creates a redis RPC service.
func NewService() *Service {
	return &Service{
		client:      NewRedisClient(),
		port:        "6379",
		database:    0,
		currentView: viewKeys,
	}
}

func (s *Service) GetMetadata() (pluginapi.PluginMetadata, error) {
	return pluginapi.PluginMetadata{
		Name:        "redis",
		Version:     "1.0.0",
		Description: "Redis management plugin",
		Author:      "Redis Plugin Team",
		License:     "MIT",
		Tags:        []string{"database", "cache", "nosql"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "https://github.com/hatembentayeb/omo/plugins/redis",
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
	s.port = req.Settings["port"]
	if s.port == "" {
		s.port = "6379"
	}
	s.username = req.Settings["username"]
	s.password = req.Settings["password"]
	s.name = req.Settings["name"]
	if db := req.Settings["database"]; db != "" {
		if n, err := strconv.Atoi(db); err == nil {
			s.database = n
		}
	}
	if s.host == "" {
		return fmt.Errorf("host is required")
	}

	pluginrpc.RPCLog("Service.Configure host=%s port=%s user=%s db=%d", s.host, s.port, s.username, s.database)

	if s.client != nil && s.client.IsConnected() {
		_ = s.client.Disconnect()
	}
	return nil
}

func (s *Service) GetView(req pluginrpc.ViewRequest) (pluginrpc.ViewData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if req.View == pluginrpc.DashboardView {
		return s.viewDashboardLocked()
	}
	viewID := req.View
	if viewID == "" {
		viewID = s.currentView
	}
	if viewID == "" {
		viewID = viewKeys
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

	case "delete":
		key := req.Payload["key"]
		if key == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no key selected"}, nil
		}
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		if err := s.client.DeleteKey(key); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewKeys)
		return pluginrpc.ActionResult{OK: true, Message: fmt.Sprintf("deleted %s", key), Next: &view}, nil

	case "flush":
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		if err := s.client.FlushDB(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewKeys)
		return pluginrpc.ActionResult{OK: true, Message: "flushed DB", Next: &view}, nil

	case "create_key":
		key := req.Payload["key"]
		value := req.Payload["value"]
		if key == "" {
			return pluginrpc.ActionResult{OK: false, Message: "key required"}, nil
		}
		ttl := int64(-1)
		if t := req.Payload["ttl"]; t != "" {
			if n, err := strconv.ParseInt(t, 10, 64); err == nil {
				ttl = n
			}
		}
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		if err := s.client.SetKey(key, value, ttl); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewKeys)
		return pluginrpc.ActionResult{OK: true, Message: fmt.Sprintf("set %s", key), Next: &view}, nil

	case "view_key":
		key := req.Payload["key"]
		if key == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no key selected"}, nil
		}
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		content, err := s.client.GetKeyContent(key)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{
			OK:         true,
			ModalTitle: "Key: " + key,
			ModalBody:  content,
		}, nil

	case "select_db":
		dbStr := req.Payload["db"]
		if dbStr == "" {
			dbStr = req.Payload["key"] // allow typed value via key field
		}
		dbStr = strings.TrimPrefix(strings.TrimSpace(dbStr), "db")
		dbStr = strings.TrimSuffix(dbStr, " *")
		db, err := strconv.Atoi(dbStr)
		if err != nil || db < 0 || db > 15 {
			return pluginrpc.ActionResult{OK: false, Message: "db must be 0-15"}, nil
		}
		if err := s.reconnectDBLocked(db); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewKeys)
		return pluginrpc.ActionResult{OK: true, Message: fmt.Sprintf("selected db %d", db), Next: &view}, nil

	case "memory_doctor":
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		report, err := s.client.GetMemoryDoctor()
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{
			OK:         true,
			ModalTitle: "Memory Doctor",
			ModalBody:  report,
		}, nil

	case "publish":
		channel := req.Payload["channel"]
		if channel == "" {
			channel = req.Payload["key"]
		}
		message := req.Payload["message"]
		if channel == "" || message == "" {
			return pluginrpc.ActionResult{OK: false, Message: "channel and message required"}, nil
		}
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		if err := s.client.PublishMessage(channel, message); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewPubSub)
		return pluginrpc.ActionResult{OK: true, Message: fmt.Sprintf("published to %s", channel), Next: &view}, nil

	case "subscribe":
		channel := req.Payload["channel"]
		if channel == "" {
			channel = req.Payload["key"]
		}
		if channel == "" || channel == "-" || channel == "*" {
			return pluginrpc.ActionResult{OK: false, Message: "no channel selected"}, nil
		}
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		body, err := s.peekPubSubLocked(channel)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{
			OK:         true,
			ModalTitle: "PubSub: " + channel,
			ModalBody:  body,
		}, nil

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
	pluginrpc.RPCLog("ensureConnected: dialing %s:%s user=%s …", s.host, s.port, s.username)
	start := time.Now()
	err := s.client.Connect(s.host, s.port, s.username, s.password, s.database)
	pluginrpc.RPCLog("ensureConnected: dial done err=%v dur=%s", err, time.Since(start))
	return err
}

func (s *Service) reconnectDBLocked(db int) error {
	s.database = db
	if s.client != nil && s.client.IsConnected() {
		_ = s.client.Disconnect()
	}
	return s.ensureConnectedLocked()
}

func (s *Service) loadKeysLocked(limit int) ([][]string, error) {
	var (
		cursor uint64
		done   bool
		keys   []string
	)
	for len(keys) < limit && !done {
		batch, next, err := s.client.ScanKeys("*", cursor, int64(limit))
		if err != nil {
			return nil, err
		}
		cursor = next
		if next == 0 {
			done = true
		}
		keys = append(keys, batch...)
	}
	if len(keys) > limit {
		keys = keys[:limit]
	}

	rows := make([][]string, 0, len(keys))
	for _, key := range keys {
		info, err := s.client.GetKeyInfo(key)
		if err != nil {
			rows = append(rows, []string{key, "?", "?", "?"})
			continue
		}
		rows = append(rows, []string{
			key,
			info["type"],
			info["ttl"],
			info["size"],
		})
	}
	return rows, nil
}

package ssh

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"omo/pkg/pluginapi"
	"omo/pkg/pluginrpc"
)

// Service is the RPC-facing SSH backend (no tview).
// View IDs live in ssh_view_nav.go (shared with native UI).
type Service struct {
	mu          sync.Mutex
	client      *SSHClient
	server      SSHServer
	currentView string
	configured  bool
}

// NewService creates an SSH RPC service.
func NewService() *Service {
	return &Service{
		currentView: viewServers,
	}
}

func (s *Service) GetMetadata() (pluginapi.PluginMetadata, error) {
	return pluginapi.PluginMetadata{
		Name:        "ssh",
		Version:     "1.0.0",
		Description: "SSH server management and remote execution",
		Author:      "ohmyops",
		License:     "MIT",
		Tags:        []string{"ssh", "remote", "server", "devops"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "https://github.com/hatembentayeb/omo/plugins/ssh",
	}, nil
}

func (s *Service) Configure(req pluginrpc.ConfigureRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	pluginrpc.RPCLog("Service.Configure begin")

	if req.Settings == nil {
		return fmt.Errorf("missing settings")
	}

	srv := SSHServer{
		Name:         req.Settings["name"],
		Description:  req.Settings["notes"],
		Host:         firstNonEmpty(req.Settings["host"], req.Settings["url"]),
		User:         req.Settings["username"],
		Password:     req.Settings["password"],
		Port:         22,
		AuthMethod:   "auto",
		KeepAlive:    30,
		Env:          make(map[string]string),
		PrivateKey:   req.Settings["private_key"],
		KeyPath:      req.Settings["key_path"],
		Passphrase:   req.Settings["passphrase"],
		ProxyCommand: req.Settings["proxy_command"],
		JumpHost:     req.Settings["jump_host"],
		JumpKey:      req.Settings["jump_key"],
		JumpKeyPath:  req.Settings["jump_key_path"],
		Fingerprint:  req.Settings["fingerprint"],
		StartupCmd:   req.Settings["startup_cmd"],
	}
	if p := req.Settings["port"]; p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			srv.Port = n
		}
	}
	if am := req.Settings["auth_method"]; am != "" {
		srv.AuthMethod = am
	}
	if ka := req.Settings["keep_alive"]; ka != "" {
		if n, err := strconv.Atoi(ka); err == nil {
			srv.KeepAlive = n
		}
	}
	if tags := req.Settings["tags"]; tags != "" {
		for _, t := range strings.Split(tags, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				srv.Tags = append(srv.Tags, t)
			}
		}
	}
	for k, v := range req.Settings {
		if strings.HasPrefix(k, "env_") {
			srv.Env[strings.TrimPrefix(k, "env_")] = v
		}
	}

	if srv.Host == "" {
		return fmt.Errorf("host is required")
	}

	if s.client != nil {
		s.client.Disconnect()
		s.client = nil
	}
	s.server = srv
	s.configured = true
	pluginrpc.RPCLog("Service.Configure host=%s user=%s port=%d", srv.Host, srv.User, srv.Port)
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
		viewID = viewServers
	}
	pluginrpc.RPCLog("Service.GetView begin view=%s", viewID)
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

	case "connect":
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewOverview)
		return pluginrpc.ActionResult{OK: true, Message: "connected to " + s.server.Host, Next: &view}, nil

	case "shell", "ssh_shell", "quick_ssh":
		return pluginrpc.ActionResult{
			OK:      true,
			Message: "use native ssh for shell",
		}, nil

	case "execute":
		cmd := firstNonEmpty(req.Payload["command"], req.Payload["key"], req.Payload["name"])
		if cmd == "" {
			return pluginrpc.ActionResult{OK: false, Message: "command required"}, nil
		}
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		res, err := s.client.Execute(cmd)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		body := fmt.Sprintf("exit=%d duration=%s\n\n--- stdout ---\n%s\n--- stderr ---\n%s",
			res.ExitCode, res.Duration, res.Stdout, res.Stderr)
		return pluginrpc.ActionResult{OK: true, ModalTitle: "Exec: " + cmd, ModalBody: body}, nil

	case "server_info":
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		info, err := s.client.GetHostInfo()
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		body := fmt.Sprintf("Hostname: %s\nOS: %s\nKernel: %s\nUptime: %s\nCPUs: %s\nMem: %s / %s avail\nDisk /: %s\nLoad: %s\nIPs: %s\nLast login: %s",
			info.Hostname, info.OS, info.Kernel, info.Uptime, info.CPUCount,
			info.MemTotal, info.MemAvail, info.DiskUsage, info.LoadAvg,
			strings.Join(info.IPAddresses, ", "), info.LastLogin)
		return pluginrpc.ActionResult{OK: true, ModalTitle: "Server Info", ModalBody: body}, nil

	default:
		return pluginrpc.ActionResult{OK: false, Message: "unknown action: " + action}, nil
	}
}

func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client != nil {
		s.client.Disconnect()
		s.client = nil
	}
	return nil
}

func (s *Service) ensureConnectedLocked() error {
	if s.client != nil && s.client.IsConnected() {
		return nil
	}
	if !s.configured || s.server.Host == "" {
		return fmt.Errorf("not configured (host did not call Configure)")
	}
	pluginrpc.RPCLog("ensureConnected: dialing %s@%s:%d …", s.server.User, s.server.Host, s.server.Port)
	if s.client != nil {
		s.client.Disconnect()
	}
	s.client = NewSSHClient(s.server)
	start := time.Now()
	err := s.client.Connect()
	pluginrpc.RPCLog("ensureConnected: dial done err=%v dur=%s", err, time.Since(start))
	return err
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

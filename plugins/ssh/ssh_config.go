package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"omo/pkg/pluginapi"
)

// SSHServer is built entirely from a KeePass entry at runtime.
//
// KeePass Entry Schema (path: ssh/<environment>/<name>):
//
//	Title    → server display name
//	URL      → hostname or IP address
//	UserName → SSH login username
//	Password → SSH password (fallback when no key is set)
//	Notes    → description / notes
//
//	Custom Attributes:
//	  port          → SSH port (default: 22)
//	  auth_method   → "key", "password", or "auto" (default: auto)
//	  private_key   → PEM-encoded private key content
//	  key_path      → path to private key file (e.g. ~/.ssh/id_ed25519)
//	  passphrase    → private key passphrase
//	  proxy_command → ProxyCommand (e.g. "ssh -W %h:%p bastion")
//	  jump_host     → jump/bastion host (user@host:port)
//	  jump_key      → PEM key for jump host
//	  jump_key_path → path to key file for jump host
//	  fingerprint   → expected host key fingerprint
//	  tags          → comma-separated tags (e.g. "web,nginx,prod")
//	  env_*         → environment variables (e.g. env_TERM=xterm-256color)
//	  startup_cmd   → command to run after connecting
//	  keep_alive    → keepalive interval in seconds (default: 30)
type SSHServer struct {
	Name         string
	Description  string
	Environment  string
	Host         string
	Port         int
	User         string
	Password     string
	AuthMethod   string
	PrivateKey   string
	KeyPath      string
	Passphrase   string
	ProxyCommand string
	JumpHost     string
	JumpKey      string
	JumpKeyPath  string
	Fingerprint  string
	Tags         []string
	Env          map[string]string
	StartupCmd   string
	KeepAlive    int
}

type SSHUIConfig struct {
	RefreshInterval int
}

func DefaultSSHUIConfig() SSHUIConfig {
	return SSHUIConfig{
		RefreshInterval: 10,
	}
}

// DiscoverServers reads KeePass groups under "ssh/" and builds SSHServer
// objects from the entries.
func DiscoverServers() ([]SSHServer, error) {
	if !pluginapi.HasSecrets() {
		return nil, fmt.Errorf("secrets provider not available")
	}

	if err := pluginapi.Secrets().Reload(); err != nil {
		return nil, fmt.Errorf("reload secrets: %w", err)
	}

	paths, err := pluginapi.ListNonReferenceSecrets("ssh")
	if err != nil {
		return nil, fmt.Errorf("list ssh secrets: %w", err)
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("no SSH entries in KeePass (create entries under ssh/<environment>/<name>)")
	}

	var servers []SSHServer
	for _, path := range paths {
		env := extractEnvironment(path)
		if env == "" {
			continue
		}

		entry, err := pluginapi.Secrets().Get(path)
		if err != nil {
			continue
		}

		srv := entryToServer(entry, env)
		servers = append(servers, srv)
	}

	sort.Slice(servers, func(i, j int) bool {
		if servers[i].Environment != servers[j].Environment {
			return servers[i].Environment < servers[j].Environment
		}
		return servers[i].Name < servers[j].Name
	})

	return servers, nil
}

func extractEnvironment(path string) string {
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 3 {
		return ""
	}
	return parts[1]
}

func entryToServer(entry *pluginapi.SecretEntry, env string) SSHServer {
	srv := SSHServer{
		Name:        entry.Title,
		Description: entry.Notes,
		Environment: env,
		Host:        entry.URL,
		User:        entry.UserName,
		Password:    entry.Password,
		Port:        22,
		AuthMethod:  "auto",
		KeepAlive:   30,
		Env:         make(map[string]string),
	}

	if entry.CustomAttributes == nil {
		return srv
	}

	ca := entry.CustomAttributes

	if v, ok := ca["port"]; ok {
		if p, err := strconv.Atoi(v); err == nil {
			srv.Port = p
		}
	}
	if v, ok := ca["auth_method"]; ok {
		srv.AuthMethod = v
	}
	if v, ok := ca["private_key"]; ok {
		srv.PrivateKey = v
	}
	if v, ok := ca["key_path"]; ok {
		srv.KeyPath = v
	}
	if v, ok := ca["passphrase"]; ok {
		srv.Passphrase = v
	}
	if v, ok := ca["proxy_command"]; ok {
		srv.ProxyCommand = v
	}
	if v, ok := ca["jump_host"]; ok {
		srv.JumpHost = v
	}
	if v, ok := ca["jump_key"]; ok {
		srv.JumpKey = v
	}
	if v, ok := ca["jump_key_path"]; ok {
		srv.JumpKeyPath = v
	}
	if v, ok := ca["fingerprint"]; ok {
		srv.Fingerprint = v
	}
	if v, ok := ca["tags"]; ok {
		for _, t := range strings.Split(v, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				srv.Tags = append(srv.Tags, t)
			}
		}
	}
	if v, ok := ca["startup_cmd"]; ok {
		srv.StartupCmd = v
	}
	if v, ok := ca["keep_alive"]; ok {
		if ka, err := strconv.Atoi(v); err == nil {
			srv.KeepAlive = ka
		}
	}

	for k, v := range ca {
		if strings.HasPrefix(k, "env_") {
			srv.Env[strings.TrimPrefix(k, "env_")] = v
		}
	}

	return srv
}

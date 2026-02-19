package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"omo/pkg/pluginapi"

	"gopkg.in/yaml.v3"
)

const sshConfigHeader = `# SSH Plugin Configuration
# Path: ~/.omo/configs/ssh/ssh.yaml
#
# All connection details are stored in KeePass under ssh/<environment>/<name>.
# This file only controls which environments are enabled and UI settings.
#
# KeePass Entry Schema (unified attribute names):
#   Title    → server display name
#   URL      → hostname or IP address
#   UserName → SSH login username
#   Password → SSH password (fallback when no key is set)
#   Notes    → description / notes
#
#   Custom Attributes (set in KeePass "Advanced" tab):
#     port           → SSH port (default: 22)
#     auth_method    → "key", "password", or "auto" (default: auto)
#     private_key    → PEM-encoded private key content
#     key_path       → path to private key file (e.g. ~/.ssh/id_ed25519)
#     passphrase     → private key passphrase
#     proxy_command  → ProxyCommand (e.g. "ssh -W %h:%p bastion")
#     jump_host      → jump/bastion host (user@host:port)
#     jump_key       → PEM key for jump host
#     jump_key_path  → path to key file for jump host
#     fingerprint    → expected host key fingerprint
#     tags           → comma-separated tags (e.g. "web,nginx,prod")
#     env_*          → environment variables (e.g. env_TERM=xterm-256color)
#     startup_cmd    → command to run after connecting
#     keep_alive     → keepalive interval in seconds (default: 30)
#
# Example KeePass structure:
#   ssh/
#     development/
#       local-vm       (Title=local-vm, URL=192.168.56.10, UserName=dev ...)
#       docker-host    (Title=docker-host, URL=192.168.56.20 ...)
#     production/
#       web-01         (Title=web-01, URL=10.0.1.10, jump_host=bastion@10.0.0.1 ...)
#       db-master      (Title=db-master, URL=10.0.2.10, proxy_command=... ...)
#     staging/
#       app-01         (...)
#     sandbox/
#       test-01        (...)
`

// SSHConfig is the YAML config. It only controls enable/disable and UI.
type SSHConfig struct {
	Environments []SSHEnvToggle `yaml:"environments"`
	UI           SSHUIConfig    `yaml:"ui"`
}

// SSHEnvToggle enables or disables a KeePass environment group.
type SSHEnvToggle struct {
	Name    string `yaml:"name"`
	Enabled bool   `yaml:"enabled"`
}

type SSHUIConfig struct {
	RefreshInterval int `yaml:"refresh_interval"`
}

// SSHServer is built entirely from a KeePass entry at runtime.
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

func DefaultSSHConfig() *SSHConfig {
	return &SSHConfig{
		Environments: []SSHEnvToggle{
			{Name: "development", Enabled: true},
			{Name: "production", Enabled: true},
			{Name: "staging", Enabled: true},
			{Name: "sandbox", Enabled: true},
		},
		UI: SSHUIConfig{
			RefreshInterval: 10,
		},
	}
}

func LoadSSHConfig(configPath string) (*SSHConfig, error) {
	if configPath == "" {
		configPath = pluginapi.PluginConfigPath("ssh")
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		_ = writeSSHDefaultConfig(configPath, sshConfigHeader, DefaultSSHConfig())
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("error reading config: %v", err)
	}

	config := DefaultSSHConfig()
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("error parsing config: %v", err)
	}

	return config, nil
}

// DiscoverServers reads KeePass groups under "ssh/" and builds SSHServer
// objects from the entries. Only enabled environments from the YAML config
// are included.
func DiscoverServers() ([]SSHServer, error) {
	if !pluginapi.HasSecrets() {
		return nil, fmt.Errorf("secrets provider not available")
	}

	if err := pluginapi.Secrets().Reload(); err != nil {
		return nil, fmt.Errorf("reload secrets: %w", err)
	}

	config, err := LoadSSHConfig("")
	if err != nil {
		return nil, err
	}

	enabled := buildEnabledSet(config)

	ensureSSHKeePassGroups(config)

	paths, err := pluginapi.Secrets().List("ssh")
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
		if _, ok := enabled[env]; !ok {
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

func buildEnabledSet(config *SSHConfig) map[string]struct{} {
	m := make(map[string]struct{})
	for _, e := range config.Environments {
		if e.Enabled {
			m[e.Name] = struct{}{}
		}
	}
	return m
}

// extractEnvironment returns the environment portion from "ssh/env/name".
func extractEnvironment(path string) string {
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 3 {
		return ""
	}
	return parts[1]
}

// entryToServer maps a KeePass SecretEntry to an SSHServer using unified
// attribute names.
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

// ensureSSHKeePassGroups creates environment groups in KeePass
// for each enabled environment. It puts a placeholder entry in each
// group that doesn't already have any entries, so the folder structure
// is visible in KeePassXC.
func ensureSSHKeePassGroups(config *SSHConfig) {
	if !pluginapi.HasSecrets() {
		return
	}
	for _, env := range config.Environments {
		if !env.Enabled {
			continue
		}
		prefix := fmt.Sprintf("ssh/%s", env.Name)
		existing, err := pluginapi.Secrets().List(prefix)
		if err == nil && len(existing) > 0 {
			continue
		}
		path := fmt.Sprintf("ssh/%s/example-server", env.Name)
		_ = pluginapi.Secrets().Put(path, &pluginapi.SecretEntry{
			Title:    "example-server",
			UserName: "root",
			Password: "",
			URL:      "192.168.1.100",
			Notes:    fmt.Sprintf("Placeholder for %s. Replace with real server details.", env.Name),
			CustomAttributes: map[string]string{
				"port":        "22",
				"auth_method": "auto",
				"tags":        env.Name,
			},
		})
	}
}

func writeSSHDefaultConfig(configPath, header string, cfg interface{}) error {
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, []byte(header+"\n"+string(data)), 0644)
}

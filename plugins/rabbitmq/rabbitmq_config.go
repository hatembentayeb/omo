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

const rabbitmqConfigHeader = `# RabbitMQ Plugin Configuration
# Path: ~/.omo/configs/rabbitmq/rabbitmq.yaml
#
# All connection details are stored in KeePass under rabbitmq/<environment>/<name>.
# This file only controls which environments are enabled and UI settings.
#
# KeePass Entry Schema (unified attribute names):
#   Title    → instance display name
#   URL      → hostname or IP address
#   UserName → RabbitMQ username
#   Password → RabbitMQ password
#   Notes    → description / notes
#
#   Custom Attributes (set in KeePass "Advanced" tab):
#     amqp_port   → AMQP port (default: 5672)
#     mgmt_port   → Management API port (default: 15672)
#     vhost       → virtual host (default: "/")
#     use_tls     → "true" or "false" (default: false)
#     tags        → comma-separated tags (e.g. "prod,cluster-a")
#
# Example KeePass structure:
#   rabbitmq/
#     development/
#       local-rabbit   (Title=local-rabbit, URL=localhost, UserName=guest ...)
#     production/
#       rabbit-cluster (Title=rabbit-cluster, URL=10.0.1.50, UserName=admin ...)
#     staging/
#       staging-rabbit (...)
`

// RabbitMQConfig is the YAML config. It only controls enable/disable and UI.
type RabbitMQConfig struct {
	Environments []RabbitMQEnvToggle `yaml:"environments"`
	UI           RabbitMQUIConfig    `yaml:"ui"`
}

// RabbitMQEnvToggle enables or disables a KeePass environment group.
type RabbitMQEnvToggle struct {
	Name    string `yaml:"name"`
	Enabled bool   `yaml:"enabled"`
}

// RabbitMQUIConfig represents UI configuration options
type RabbitMQUIConfig struct {
	RefreshInterval   int    `yaml:"refresh_interval"`
	MaxQueuesDisplay  int    `yaml:"max_queues_display"`
	EnableConnections bool   `yaml:"enable_connections"`
	EnableChannels    bool   `yaml:"enable_channels"`
	DefaultView       string `yaml:"default_view"`
}

// RabbitMQInstance is built entirely from a KeePass entry at runtime.
type RabbitMQInstance struct {
	Name        string
	Description string
	Environment string
	Host        string
	AMQPPort    int
	MgmtPort    int
	Username    string
	Password    string
	VHost       string
	UseTLS      bool
	Tags        []string
}

func DefaultRabbitMQConfig() *RabbitMQConfig {
	return &RabbitMQConfig{
		Environments: []RabbitMQEnvToggle{
			{Name: "development", Enabled: true},
			{Name: "production", Enabled: true},
			{Name: "staging", Enabled: true},
			{Name: "sandbox", Enabled: true},
		},
		UI: RabbitMQUIConfig{
			RefreshInterval:   10,
			MaxQueuesDisplay:  200,
			EnableConnections: true,
			EnableChannels:    true,
			DefaultView:       "overview",
		},
	}
}

func LoadRabbitMQConfig(configPath string) (*RabbitMQConfig, error) {
	if configPath == "" {
		configPath = pluginapi.PluginConfigPath("rabbitmq")
	}

	needsWrite := false

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		needsWrite = true
	} else {
		data, err := os.ReadFile(configPath)
		if err == nil && isOldRabbitMQConfig(data) {
			needsWrite = true
		}
	}

	if needsWrite {
		_ = writeDefaultRabbitMQConfig(configPath, rabbitmqConfigHeader, DefaultRabbitMQConfig())
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %v", err)
	}

	config := DefaultRabbitMQConfig()
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("error parsing config file: %v", err)
	}

	return config, nil
}

// isOldRabbitMQConfig detects the legacy format that had instances defined in YAML.
func isOldRabbitMQConfig(data []byte) bool {
	var probe struct {
		Instances    interface{} `yaml:"instances"`
		Environments interface{} `yaml:"environments"`
	}
	if err := yaml.Unmarshal(data, &probe); err != nil {
		return false
	}
	return probe.Instances != nil && probe.Environments == nil
}

// DiscoverInstances reads KeePass groups under "rabbitmq/" and builds
// RabbitMQInstance objects from the entries. Only enabled environments
// from the YAML config are included.
func DiscoverInstances() ([]RabbitMQInstance, error) {
	if !pluginapi.HasSecrets() {
		return nil, fmt.Errorf("secrets provider not available")
	}

	if err := pluginapi.Secrets().Reload(); err != nil {
		return nil, fmt.Errorf("reload secrets: %w", err)
	}

	config, err := LoadRabbitMQConfig("")
	if err != nil {
		return nil, err
	}

	enabled := buildRabbitMQEnabledSet(config)

	ensureRabbitMQKeePassGroups(config)

	paths, err := pluginapi.Secrets().List("rabbitmq")
	if err != nil {
		return nil, fmt.Errorf("list rabbitmq secrets: %w", err)
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("no RabbitMQ entries in KeePass (create entries under rabbitmq/<environment>/<name>)")
	}

	var instances []RabbitMQInstance
	for _, path := range paths {
		env := extractRabbitMQEnvironment(path)
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

		inst := entryToRabbitMQInstance(entry, env)
		instances = append(instances, inst)
	}

	sort.Slice(instances, func(i, j int) bool {
		if instances[i].Environment != instances[j].Environment {
			return instances[i].Environment < instances[j].Environment
		}
		return instances[i].Name < instances[j].Name
	})

	return instances, nil
}

func buildRabbitMQEnabledSet(config *RabbitMQConfig) map[string]struct{} {
	m := make(map[string]struct{})
	for _, e := range config.Environments {
		if e.Enabled {
			m[e.Name] = struct{}{}
		}
	}
	return m
}

func extractRabbitMQEnvironment(path string) string {
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 3 {
		return ""
	}
	return parts[1]
}

func entryToRabbitMQInstance(entry *pluginapi.SecretEntry, env string) RabbitMQInstance {
	inst := RabbitMQInstance{
		Name:        entry.Title,
		Description: entry.Notes,
		Environment: env,
		Host:        entry.URL,
		Username:    entry.UserName,
		Password:    entry.Password,
		AMQPPort:    5672,
		MgmtPort:    15672,
		VHost:       "/",
	}

	if entry.CustomAttributes == nil {
		return inst
	}

	ca := entry.CustomAttributes

	if v, ok := ca["amqp_port"]; ok {
		if p, err := strconv.Atoi(v); err == nil {
			inst.AMQPPort = p
		}
	}
	if v, ok := ca["mgmt_port"]; ok {
		if p, err := strconv.Atoi(v); err == nil {
			inst.MgmtPort = p
		}
	}
	if v, ok := ca["vhost"]; ok && v != "" {
		inst.VHost = v
	}
	if v, ok := ca["use_tls"]; ok {
		inst.UseTLS = v == "true" || v == "1" || v == "yes"
	}
	if v, ok := ca["tags"]; ok {
		for _, t := range strings.Split(v, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				inst.Tags = append(inst.Tags, t)
			}
		}
	}

	return inst
}

// ensureRabbitMQKeePassGroups creates environment groups in KeePass
// for each enabled environment. It puts a placeholder entry in each
// group that doesn't already have any entries, so the folder structure
// is visible in KeePassXC.
func ensureRabbitMQKeePassGroups(config *RabbitMQConfig) {
	if !pluginapi.HasSecrets() {
		return
	}
	for _, env := range config.Environments {
		if !env.Enabled {
			continue
		}
		prefix := fmt.Sprintf("rabbitmq/%s", env.Name)
		existing, err := pluginapi.Secrets().List(prefix)
		if err == nil && len(existing) > 0 {
			continue
		}
		path := fmt.Sprintf("rabbitmq/%s/example", env.Name)
		_ = pluginapi.Secrets().Put(path, &pluginapi.SecretEntry{
			Title:    "example",
			UserName: "guest",
			Password: "guest",
			URL:      "localhost",
			Notes:    fmt.Sprintf("Placeholder for %s. Replace with real RabbitMQ details.", env.Name),
			CustomAttributes: map[string]string{
				"vhost":     "/",
				"amqp_port": "5672",
				"mgmt_port": "15672",
			},
		})
	}
}

// GetRabbitMQUIConfig returns the UI configuration
func GetRabbitMQUIConfig() (RabbitMQUIConfig, error) {
	config, err := LoadRabbitMQConfig("")
	if err != nil {
		return RabbitMQUIConfig{}, err
	}
	return config.UI, nil
}

func writeDefaultRabbitMQConfig(configPath, header string, cfg interface{}) error {
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, []byte(header+"\n"+string(data)), 0644)
}

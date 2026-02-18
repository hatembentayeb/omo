package main

import (
	"fmt"
	"os"
	"path/filepath"

	"omo/pkg/pluginapi"

	"gopkg.in/yaml.v3"
)

// rabbitmqConfigHeader is prepended to the auto-generated config YAML.
const rabbitmqConfigHeader = `# RabbitMQ Plugin Configuration
# Path: ~/.omo/configs/rabbitmq/rabbitmq.yaml
#
# KeePass Secret Schema (secret path: rabbitmq/<environment>/<name>):
#   Title    → instance name
#   URL      → host (e.g. "localhost")
#   UserName → RabbitMQ username
#   Password → RabbitMQ password
#   Custom Attributes:
#     vhost       → virtual host (default "/")
#     amqp_port   → AMQP port (default "5672")
#     mgmt_port   → Management API port (default "15672")
#
# When "secret" is set, connection fields are resolved from KeePass.
# YAML values take precedence over KeePass values (override only blanks).
`

// RabbitMQConfig represents the configuration for the RabbitMQ plugin
type RabbitMQConfig struct {
	Instances []RabbitMQInstance `yaml:"instances"`
	UI        RabbitMQUIConfig   `yaml:"ui"`
}

// RabbitMQInstance represents a configured RabbitMQ server.
type RabbitMQInstance struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Secret      string `yaml:"secret,omitempty"`
	Host        string `yaml:"host"`
	AMQPPort    int    `yaml:"amqp_port"`
	MgmtPort    int    `yaml:"mgmt_port"`
	Username    string `yaml:"username"`
	Password    string `yaml:"password"`
	VHost       string `yaml:"vhost"`
	UseTLS      bool   `yaml:"use_tls"`
}

// RabbitMQUIConfig represents UI configuration options
type RabbitMQUIConfig struct {
	RefreshInterval   int    `yaml:"refresh_interval"`
	MaxQueuesDisplay  int    `yaml:"max_queues_display"`
	EnableConnections bool   `yaml:"enable_connections"`
	EnableChannels    bool   `yaml:"enable_channels"`
	DefaultView       string `yaml:"default_view"`
}

// DefaultRabbitMQConfig returns the default configuration
func DefaultRabbitMQConfig() *RabbitMQConfig {
	return &RabbitMQConfig{
		Instances: []RabbitMQInstance{},
		UI: RabbitMQUIConfig{
			RefreshInterval:   10,
			MaxQueuesDisplay:  200,
			EnableConnections: true,
			EnableChannels:    true,
			DefaultView:       "overview",
		},
	}
}

// LoadRabbitMQConfig loads the RabbitMQ configuration from the specified file.
func LoadRabbitMQConfig(configPath string) (*RabbitMQConfig, error) {
	if configPath == "" {
		configPath = pluginapi.PluginConfigPath("rabbitmq")
	}

	// Auto-create default config if missing
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		_ = writeDefaultRabbitMQConfig(configPath, rabbitmqConfigHeader, DefaultRabbitMQConfig())
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %v", err)
	}

	config := DefaultRabbitMQConfig()
	err = yaml.Unmarshal(data, config)
	if err != nil {
		return nil, fmt.Errorf("error parsing config file: %v", err)
	}

	initRabbitMQKeePass()

	if err := resolveRabbitMQSecrets(config); err != nil {
		return nil, fmt.Errorf("error resolving secrets: %v", err)
	}

	return config, nil
}

// initRabbitMQKeePass seeds placeholder KeePass entries if none exist.
func initRabbitMQKeePass() {
	if !pluginapi.HasSecrets() {
		return
	}
	entries, err := pluginapi.Secrets().List("rabbitmq")
	if err != nil || len(entries) > 0 {
		return
	}
	_ = pluginapi.Secrets().Put("rabbitmq/default/example", &pluginapi.SecretEntry{
		Title:    "example",
		UserName: "guest",
		Password: "guest",
		URL:      "localhost",
		Notes:    "RabbitMQ placeholder. Set URL (host), UserName, Password.",
		CustomAttributes: map[string]string{
			"vhost":     "/",
			"amqp_port": "5672",
			"mgmt_port": "15672",
		},
	})
}

// resolveRabbitMQSecrets iterates over instances and populates connection
// fields from the secrets provider when a secret path is defined.
func resolveRabbitMQSecrets(config *RabbitMQConfig) error {
	if !pluginapi.HasSecrets() {
		return nil
	}

	for i := range config.Instances {
		inst := &config.Instances[i]
		if inst.Secret == "" {
			continue
		}

		entry, err := pluginapi.ResolveSecret(inst.Secret)
		if err != nil {
			return fmt.Errorf("instance %q: %w", inst.Name, err)
		}

		if inst.Host == "" && entry.URL != "" {
			inst.Host = entry.URL
		}
		if inst.Username == "" && entry.UserName != "" {
			inst.Username = entry.UserName
		}
		if inst.Password == "" && entry.Password != "" {
			inst.Password = entry.Password
		}
		if inst.Name == "" && entry.Title != "" {
			inst.Name = entry.Title
		}
		if inst.VHost == "" {
			if v, ok := entry.CustomAttributes["vhost"]; ok && v != "" {
				inst.VHost = v
			}
		}
	}
	return nil
}

// GetAvailableRabbitMQInstances returns the list of configured instances
func GetAvailableRabbitMQInstances() ([]RabbitMQInstance, error) {
	config, err := LoadRabbitMQConfig("")
	if err != nil {
		return nil, err
	}
	return config.Instances, nil
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

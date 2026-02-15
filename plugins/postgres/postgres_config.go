package main

import (
	"fmt"
	"os"
	"path/filepath"

	"omo/pkg/pluginapi"

	"gopkg.in/yaml.v3"
)

// postgresConfigHeader is prepended to the auto-generated config YAML.
const postgresConfigHeader = `# PostgreSQL Plugin Configuration
# Path: ~/.omo/configs/postgres/postgres.yaml
#
# KeePass Secret Schema (secret path: postgres/<environment>/<name>):
#   Title    → instance name
#   URL      → host (e.g. "localhost", "pg.example.com")
#   UserName → PostgreSQL username
#   Password → PostgreSQL password
#
# When "secret" is set, connection fields are resolved from KeePass.
# YAML values take precedence over KeePass values (override only blanks).
`

// PostgresConfig represents the configuration for the PostgreSQL plugin
type PostgresConfig struct {
	Instances       []PostgresInstance `yaml:"instances"`
	UI              UIConfig           `yaml:"ui"`
	DefaultInstance string             `yaml:"default_instance,omitempty"`
}

// PostgresInstance represents a configured PostgreSQL server instance.
type PostgresInstance struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Secret      string `yaml:"secret,omitempty"`
	Host        string `yaml:"host"`
	Port        int    `yaml:"port"`
	Username    string `yaml:"username"`
	Password    string `yaml:"password"`
	Database    string `yaml:"database"`
	SSLMode     string `yaml:"sslmode"`
}

// UIConfig represents UI configuration options
type UIConfig struct {
	RefreshInterval int  `yaml:"refresh_interval"`
	MaxRowsDisplay  int  `yaml:"max_rows_display"`
	EnableLogs      bool `yaml:"enable_logs"`
	EnableStats     bool `yaml:"enable_stats"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *PostgresConfig {
	return &PostgresConfig{
		Instances: []PostgresInstance{},
		UI: UIConfig{
			RefreshInterval: 10,
			MaxRowsDisplay:  1000,
			EnableLogs:      true,
			EnableStats:     true,
		},
	}
}

// LoadConfig loads the PostgreSQL configuration from the specified file.
func LoadConfig(configPath string) (*PostgresConfig, error) {
	if configPath == "" {
		configPath = pluginapi.PluginConfigPath("postgres")
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		_ = writeDefaultConfig(configPath, postgresConfigHeader, DefaultConfig())
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %v", err)
	}

	config := DefaultConfig()
	err = yaml.Unmarshal(data, config)
	if err != nil {
		return nil, fmt.Errorf("error parsing config file: %v", err)
	}

	initPostgresKeePass()

	if err := resolvePostgresSecrets(config); err != nil {
		return nil, fmt.Errorf("error resolving secrets: %v", err)
	}

	return config, nil
}

// initPostgresKeePass seeds placeholder KeePass entries for PostgreSQL if none exist.
func initPostgresKeePass() {
	if !pluginapi.HasSecrets() {
		return
	}
	entries, err := pluginapi.Secrets().List("postgres")
	if err != nil || len(entries) > 0 {
		return
	}
	_ = pluginapi.Secrets().Put("postgres/default/example", &pluginapi.SecretEntry{
		Title:    "example",
		UserName: "postgres",
		Password: "",
		URL:      "localhost",
		Notes:    "PostgreSQL placeholder. Set URL (host), UserName, Password.",
	})
}

// resolvePostgresSecrets iterates over instances and populates connection
// fields from the secrets provider when a secret path is defined.
func resolvePostgresSecrets(config *PostgresConfig) error {
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
	}
	return nil
}

// GetAvailableInstances returns the list of configured PostgreSQL instances
func GetAvailableInstances() ([]PostgresInstance, error) {
	config, err := LoadConfig("")
	if err != nil {
		return nil, err
	}
	return config.Instances, nil
}

// GetUIConfig returns the UI configuration
func GetUIConfig() (UIConfig, error) {
	config, err := LoadConfig("")
	if err != nil {
		return UIConfig{}, err
	}
	return config.UI, nil
}

// writeDefaultConfig marshals the default config struct to YAML and writes it to disk.
func writeDefaultConfig(configPath, header string, cfg interface{}) error {
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, []byte(header+"\n"+string(data)), 0644)
}

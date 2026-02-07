package main

import (
	"fmt"
	"os"

	"omo/pkg/pluginapi"

	"gopkg.in/yaml.v3"
)

// RedisConfig represents the configuration for the Redis plugin
type RedisConfig struct {
	Instances []RedisInstance `yaml:"instances"`
	UI        UIConfig       `yaml:"ui"`
}

// RedisInstance represents a configured Redis server instance.
// When the Secret field is set (e.g. "redis/production/main-cache"),
// it references a KeePass entry whose fields override Host, Username,
// Password, etc. at load time.
type RedisInstance struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Secret      string `yaml:"secret,omitempty"` // KeePass path: pluginName/env/entryName
	Host        string `yaml:"host"`
	Port        int    `yaml:"port"`
	Username    string `yaml:"username"`           // Redis ACL username (Redis 6+)
	Password    string `yaml:"password"`
	Database    int    `yaml:"database"`
}

// UIConfig represents UI configuration options
type UIConfig struct {
	RefreshInterval  int  `yaml:"refresh_interval"`
	MaxKeysDisplay   int  `yaml:"max_keys_display"`
	EnableSlowLog    bool `yaml:"enable_slowlog"`
	EnableServerInfo bool `yaml:"enable_server_info"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *RedisConfig {
	return &RedisConfig{
		Instances: []RedisInstance{},
		UI: UIConfig{
			RefreshInterval:  5,
			MaxKeysDisplay:   1000,
			EnableSlowLog:    true,
			EnableServerInfo: true,
		},
	}
}

// LoadConfig loads the Redis configuration from the specified file.
// Default path: ~/.omo/configs/redis/redis.yaml
//
// After unmarshalling, any instance with a non-empty Secret field will
// have its connection fields resolved from the KeePass secrets provider.
func LoadConfig(configPath string) (*RedisConfig, error) {
	if configPath == "" {
		configPath = pluginapi.PluginConfigPath("redis")
	}

	// Check if the file exists
	_, err := os.Stat(configPath)
	if os.IsNotExist(err) {
		// File doesn't exist, return default config
		return DefaultConfig(), nil
	}

	// Read the configuration file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %v", err)
	}

	// Unmarshal the configuration
	config := DefaultConfig()
	err = yaml.Unmarshal(data, config)
	if err != nil {
		return nil, fmt.Errorf("error parsing config file: %v", err)
	}

	// Resolve secrets for instances that reference KeePass entries.
	if err := resolveRedisSecrets(config); err != nil {
		return nil, fmt.Errorf("error resolving secrets: %v", err)
	}

	return config, nil
}

// resolveRedisSecrets iterates over instances and populates connection
// fields from the secrets provider when a secret path is defined.
func resolveRedisSecrets(config *RedisConfig) error {
	if !pluginapi.HasSecrets() {
		return nil // no provider — skip silently
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

		// Override only blank fields so YAML values take precedence.
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

// GetAvailableInstances returns the list of configured Redis instances
func GetAvailableInstances() ([]RedisInstance, error) {
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

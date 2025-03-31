package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// RedisConfig represents the configuration for the Redis plugin
type RedisConfig struct {
	Instances []RedisInstance `yaml:"instances"`
	UI        UIConfig        `yaml:"ui"`
}

// RedisInstance represents a configured Redis server instance
type RedisInstance struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Host        string `yaml:"host"`
	Port        int    `yaml:"port"`
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

// LoadConfig loads the Redis configuration from the specified file
func LoadConfig(configPath string) (*RedisConfig, error) {
	// If no path is specified, use the default config path
	if configPath == "" {
		configPath = filepath.Join("config", "redis.yaml")
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

	return config, nil
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

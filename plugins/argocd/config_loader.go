package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

// ArgocdConfig represents the configuration structure for ArgoCD instances
type ArgocdConfig struct {
	Instances       []ArgocdInstance `yaml:"instances"`
	DefaultInstance string           `yaml:"default_instance"`
	Debug           DebugConfig      `yaml:"debug"`
}

// DebugConfig represents debug settings
type DebugConfig struct {
	Enabled               bool `yaml:"enabled"`
	LogAPICalls           bool `yaml:"log_api_calls"`
	LogResponses          bool `yaml:"log_responses"`
	RequestTimeoutSeconds int  `yaml:"request_timeout_seconds"`
}

// ArgocdInstance represents a single ArgoCD server instance
type ArgocdInstance struct {
	Name     string `yaml:"name"`
	URL      string `yaml:"url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// LoadArgocdConfig loads the ArgoCD configuration from the config file
func LoadArgocdConfig() (*ArgocdConfig, error) {
	// Determine config file path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %v", err)
	}

	// Try to find config file in different locations
	configPaths := []string{
		"config/argocd.yml", // Project directory
		filepath.Join(homeDir, ".config/omo/argocd.yml"), // User config directory
		filepath.Join(homeDir, ".omo/argocd.yml"),        // User hidden directory
	}

	var configData []byte
	var configFilePath string

	// Try each path until we find a valid config file
	for _, path := range configPaths {
		data, err := os.ReadFile(path)
		if err == nil {
			configData = data
			configFilePath = path
			break
		}
	}

	if configData == nil {
		return nil, fmt.Errorf("no ArgoCD configuration file found")
	}

	// Parse the YAML
	var config ArgocdConfig
	if err := yaml.Unmarshal(configData, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %v", configFilePath, err)
	}

	// Check if config has at least one instance
	if len(config.Instances) == 0 {
		return nil, fmt.Errorf("no ArgoCD instances defined in config")
	}

	return &config, nil
}

// FindInstanceByName looks up an instance by name in the configuration
func FindInstanceByName(config *ArgocdConfig, name string) *ArgocdInstance {
	for _, instance := range config.Instances {
		if instance.Name == name {
			return &instance
		}
	}
	return nil
}

package main

import (
	"fmt"
	"os"

	"omo/pkg/pluginapi"

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

// LoadArgocdConfig loads the ArgoCD configuration.
// Default path: ~/.omo/configs/argocd/argocd.yaml
func LoadArgocdConfig() (*ArgocdConfig, error) {
	configPath := pluginapi.PluginConfigPath("argocd")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("no ArgoCD config found at %s: %v", configPath, err)
	}

	var config ArgocdConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config %s: %v", configPath, err)
	}

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

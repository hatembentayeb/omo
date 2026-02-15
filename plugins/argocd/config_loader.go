package main

import (
	"fmt"
	"os"
	"path/filepath"

	"omo/pkg/pluginapi"

	"gopkg.in/yaml.v2"
)

// argocdConfigHeader is prepended to the auto-generated config YAML.
const argocdConfigHeader = `# ArgoCD Plugin Configuration
# Path: ~/.omo/configs/argocd/argocd.yaml
#
# KeePass Secret Schema (secret path: argocd/<environment>/<name>):
#   Title    → instance name
#   URL      → ArgoCD server URL (e.g. "https://argocd.example.com")
#   UserName → ArgoCD username
#   Password → ArgoCD password
#
# When "secret" is set, connection fields are resolved from KeePass.
# YAML values take precedence over KeePass values (override only blanks).
`

// DefaultArgocdConfig returns the default configuration
func DefaultArgocdConfig() *ArgocdConfig {
	return &ArgocdConfig{
		Instances: []ArgocdInstance{},
		Debug: DebugConfig{
			Enabled:               false,
			LogAPICalls:           false,
			LogResponses:          false,
			RequestTimeoutSeconds: 30,
		},
	}
}

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

// ArgocdInstance represents a single ArgoCD server instance.
// When the Secret field is set (e.g. "argocd/production/main-cluster"),
// it references a KeePass entry whose fields override URL, Username,
// Password, etc. at load time.
type ArgocdInstance struct {
	Name     string `yaml:"name"`
	Secret   string `yaml:"secret,omitempty"` // KeePass path: pluginName/env/entryName
	URL      string `yaml:"url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// LoadArgocdConfig loads the ArgoCD configuration.
// Default path: ~/.omo/configs/argocd/argocd.yaml
//
// After unmarshalling, any instance with a non-empty Secret field will
// have its connection fields resolved from the KeePass secrets provider.
func LoadArgocdConfig() (*ArgocdConfig, error) {
	configPath := pluginapi.PluginConfigPath("argocd")

	// Auto-create default config if missing
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		_ = writeDefaultConfig(configPath, argocdConfigHeader, DefaultArgocdConfig())
	}

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

	// Init KeePass placeholder if no entries exist yet
	initArgocdKeePass()

	// Resolve secrets for instances that reference KeePass entries.
	if err := resolveArgocdSecrets(&config); err != nil {
		return nil, fmt.Errorf("error resolving secrets: %v", err)
	}

	return &config, nil
}

// initArgocdKeePass seeds placeholder KeePass entries for ArgoCD if none exist.
func initArgocdKeePass() {
	if !pluginapi.HasSecrets() {
		return
	}
	entries, err := pluginapi.Secrets().List("argocd")
	if err != nil || len(entries) > 0 {
		return
	}
	_ = pluginapi.Secrets().Put("argocd/default/example", &pluginapi.SecretEntry{
		Title:    "example",
		UserName: "admin",
		Password: "",
		URL:      "https://localhost:8080",
		Notes:    "ArgoCD placeholder. Set URL (server), UserName, Password.",
	})
}

// resolveArgocdSecrets iterates over instances and populates connection
// fields from the secrets provider when a secret path is defined.
func resolveArgocdSecrets(config *ArgocdConfig) error {
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

		// Override only blank fields so YAML values take precedence.
		if inst.URL == "" && entry.URL != "" {
			inst.URL = entry.URL
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

// FindInstanceByName looks up an instance by name in the configuration
func FindInstanceByName(config *ArgocdConfig, name string) *ArgocdInstance {
	for _, instance := range config.Instances {
		if instance.Name == name {
			return &instance
		}
	}
	return nil
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

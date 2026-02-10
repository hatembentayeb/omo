package main

import (
	"fmt"
	"os"
	"path/filepath"

	"omo/pkg/pluginapi"

	"gopkg.in/yaml.v3"
)

// k8sUserConfigHeader is prepended to the auto-generated config YAML.
const k8sUserConfigHeader = `# K8sUser Plugin Configuration
# Path: ~/.omo/configs/k8suser/k8suser.yaml
#
# Clusters are loaded from two sources:
#   1. This config file (with optional KeePass secret resolution)
#   2. Host machine's ~/.kube/config (kubectl contexts)
#
# KeePass Secret Schema (secret path: k8suser/<environment>/<name>):
#   Title    → cluster name
#   URL      → Kubernetes API server URL (e.g. "https://k8s.example.com:6443")
#   Password → bearer token for authentication
#   Custom Attributes:
#     kubeconfig → path to kubeconfig file
#     context    → kubectl context name
#     ca_cert    → path to CA certificate
#
# When "secret" is set, connection fields are resolved from KeePass.
# YAML values take precedence over KeePass values (override only blanks).
`

// K8sUserConfig represents the configuration for the K8s User plugin
type K8sUserConfig struct {
	Clusters []K8sCluster     `yaml:"clusters"`
	UI       K8sUserUIConfig  `yaml:"ui"`
}

// K8sCluster represents a configured Kubernetes cluster.
// When the Secret field is set (e.g. "k8suser/production/main-cluster"),
// it references a KeePass entry whose fields override Kubeconfig, Token, etc.
type K8sCluster struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Secret      string `yaml:"secret,omitempty"` // KeePass path: pluginName/env/entryName
	Kubeconfig  string `yaml:"kubeconfig,omitempty"`
	Context     string `yaml:"context,omitempty"`
	Server      string `yaml:"server,omitempty"`
	Token       string `yaml:"token,omitempty"`
	CACert      string `yaml:"ca_cert,omitempty"`
}

// K8sUserUIConfig represents UI configuration options
type K8sUserUIConfig struct {
	RefreshInterval     int    `yaml:"refresh_interval"`
	DefaultNamespace    string `yaml:"default_namespace"`
	CertValidityDays    int    `yaml:"cert_validity_days"`
	EnableRoleManager   bool   `yaml:"enable_role_manager"`
	EnableAccessTesting bool   `yaml:"enable_access_testing"`
	EnableExport        bool   `yaml:"enable_export"`
}

// DefaultK8sUserConfig returns the default configuration
func DefaultK8sUserConfig() *K8sUserConfig {
	return &K8sUserConfig{
		Clusters: []K8sCluster{},
		UI: K8sUserUIConfig{
			RefreshInterval:     30,
			DefaultNamespace:    "default",
			CertValidityDays:    365,
			EnableRoleManager:   true,
			EnableAccessTesting: true,
			EnableExport:        true,
		},
	}
}

// LoadK8sUserConfig loads the K8s User configuration from the specified file.
// Default path: ~/.omo/configs/k8suser/k8suser.yaml
//
// After unmarshalling, any cluster with a non-empty Secret field will
// have its connection fields resolved from the KeePass secrets provider.
func LoadK8sUserConfig(configPath string) (*K8sUserConfig, error) {
	if configPath == "" {
		configPath = pluginapi.PluginConfigPath("k8suser")
	}

	// Auto-create default config if missing
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		_ = writeDefaultConfig(configPath, k8sUserConfigHeader, DefaultK8sUserConfig())
	}

	// Read the configuration file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %v", err)
	}

	// Unmarshal the configuration
	config := DefaultK8sUserConfig()
	err = yaml.Unmarshal(data, config)
	if err != nil {
		return nil, fmt.Errorf("error parsing config file: %v", err)
	}

	// Init KeePass placeholder if no entries exist yet
	initK8sUserKeePass()

	// Resolve secrets for clusters that reference KeePass entries.
	if err := resolveK8sUserSecrets(config); err != nil {
		return nil, fmt.Errorf("error resolving secrets: %v", err)
	}

	return config, nil
}

// initK8sUserKeePass seeds placeholder KeePass entries for K8sUser if none exist.
func initK8sUserKeePass() {
	if !pluginapi.HasSecrets() {
		return
	}
	entries, err := pluginapi.Secrets().List("k8suser")
	if err != nil || len(entries) > 0 {
		return
	}
	_ = pluginapi.Secrets().Put("k8suser/default/example", &pluginapi.SecretEntry{
		Title:    "example",
		UserName: "",
		Password: "",
		URL:      "",
		Notes:    "K8sUser placeholder. Set URL (API server), Password (bearer token).",
		CustomAttributes: map[string]string{
			"kubeconfig": "~/.kube/config",
			"context":    "",
			"ca_cert":    "",
		},
	})
}

// resolveK8sUserSecrets iterates over clusters and populates connection
// fields from the secrets provider when a secret path is defined.
func resolveK8sUserSecrets(config *K8sUserConfig) error {
	if !pluginapi.HasSecrets() {
		return nil // no provider — skip silently
	}

	for i := range config.Clusters {
		cluster := &config.Clusters[i]
		if cluster.Secret == "" {
			continue
		}

		entry, err := pluginapi.ResolveSecret(cluster.Secret)
		if err != nil {
			return fmt.Errorf("cluster %q: %w", cluster.Name, err)
		}

		// Override only blank fields so YAML values take precedence.
		if cluster.Server == "" && entry.URL != "" {
			cluster.Server = entry.URL
		}
		if cluster.Token == "" && entry.Password != "" {
			cluster.Token = entry.Password
		}
		if cluster.Name == "" && entry.Title != "" {
			cluster.Name = entry.Title
		}
		// Custom attributes
		if cluster.Kubeconfig == "" {
			if v, ok := entry.CustomAttributes["kubeconfig"]; ok {
				cluster.Kubeconfig = v
			}
		}
		if cluster.CACert == "" {
			if v, ok := entry.CustomAttributes["ca_cert"]; ok {
				cluster.CACert = v
			}
		}
		if cluster.Context == "" {
			if v, ok := entry.CustomAttributes["context"]; ok {
				cluster.Context = v
			}
		}
	}
	return nil
}

// GetAvailableK8sClusters returns the list of configured K8s clusters
func GetAvailableK8sClusters() ([]K8sCluster, error) {
	config, err := LoadK8sUserConfig("")
	if err != nil {
		return nil, err
	}
	return config.Clusters, nil
}

// GetK8sUserUIConfig returns the UI configuration
func GetK8sUserUIConfig() (K8sUserUIConfig, error) {
	config, err := LoadK8sUserConfig("")
	if err != nil {
		return K8sUserUIConfig{}, err
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

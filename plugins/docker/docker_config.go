package main

import (
	"fmt"
	"os"
	"path/filepath"

	"omo/pkg/pluginapi"

	"gopkg.in/yaml.v3"
)

// dockerConfigHeader is prepended to the auto-generated config YAML.
const dockerConfigHeader = `# Docker Plugin Configuration
# Path: ~/.omo/configs/docker/docker.yaml
#
# KeePass Secret Schema (secret path: docker/<environment>/<name>):
#   Title    → host name
#   URL      → docker host URL (e.g. "unix:///var/run/docker.sock", "tcp://host:2376")
#   Custom Attributes:
#     cert_path → path to TLS certificates directory
#
# When "secret" is set, connection fields are resolved from KeePass.
# YAML values take precedence over KeePass values (override only blanks).
`

// DockerConfig represents the configuration for the Docker plugin
type DockerConfig struct {
	Hosts []DockerHost `yaml:"hosts"`
	UI    UIConfig     `yaml:"ui"`
}

// DockerHost represents a configured Docker host.
// When the Secret field is set (e.g. "docker/production/remote-server"),
// it references a KeePass entry whose fields override Host, CertPath, etc.
type DockerHost struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Secret      string `yaml:"secret,omitempty"` // KeePass path: pluginName/env/entryName
	Host        string `yaml:"host"`
	TLS         bool   `yaml:"tls"`
	TLSVerify   bool   `yaml:"tls_verify"`
	CertPath    string `yaml:"cert_path"`
}

// UIConfig represents UI configuration options
type UIConfig struct {
	RefreshInterval      int  `yaml:"refresh_interval"`
	MaxContainersDisplay int  `yaml:"max_containers_display"`
	MaxImagesDisplay     int  `yaml:"max_images_display"`
	ShowAllContainers    bool `yaml:"show_all_containers"`
	LogTailLines         int  `yaml:"log_tail_lines"`
	EnableStats          bool `yaml:"enable_stats"`
	EnableCompose        bool `yaml:"enable_compose"`
}

// DefaultDockerConfig returns the default configuration
func DefaultDockerConfig() *DockerConfig {
	return &DockerConfig{
		Hosts: []DockerHost{
			{
				Name:        "local",
				Description: "Local Docker Daemon",
				Host:        "unix:///var/run/docker.sock",
				TLS:         false,
				TLSVerify:   false,
				CertPath:    "",
			},
		},
		UI: UIConfig{
			RefreshInterval:      5,
			MaxContainersDisplay: 100,
			MaxImagesDisplay:     100,
			ShowAllContainers:    true,
			LogTailLines:         500,
			EnableStats:          true,
			EnableCompose:        true,
		},
	}
}

// LoadDockerConfig loads the Docker configuration from the specified file.
//
// After unmarshalling, any host with a non-empty Secret field will have
// its connection fields resolved from the KeePass secrets provider.
func LoadDockerConfig(configPath string) (*DockerConfig, error) {
	// If no path is specified, use the default config path
	if configPath == "" {
		configPath = pluginapi.PluginConfigPath("docker")
	}

	// Auto-create default config if missing
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		_ = writeDefaultConfig(configPath, dockerConfigHeader, DefaultDockerConfig())
	}

	// Read the configuration file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %v", err)
	}

	// Unmarshal the configuration
	config := DefaultDockerConfig()
	err = yaml.Unmarshal(data, config)
	if err != nil {
		return nil, fmt.Errorf("error parsing config file: %v", err)
	}

	// Init KeePass placeholder if no entries exist yet
	initDockerKeePass()

	// Resolve secrets for hosts that reference KeePass entries.
	if err := resolveDockerSecrets(config); err != nil {
		return nil, fmt.Errorf("error resolving secrets: %v", err)
	}

	return config, nil
}

// initDockerKeePass seeds placeholder KeePass entries for Docker if none exist.
func initDockerKeePass() {
	if !pluginapi.HasSecrets() {
		return
	}
	entries, err := pluginapi.Secrets().List("docker")
	if err != nil || len(entries) > 0 {
		return
	}
	_ = pluginapi.Secrets().Put("docker/default/example", &pluginapi.SecretEntry{
		Title: "example",
		URL:   "unix:///var/run/docker.sock",
		Notes: "Docker placeholder. Set URL (docker host), cert_path (TLS certs dir).",
		CustomAttributes: map[string]string{
			"cert_path": "",
		},
	})
}

// resolveDockerSecrets iterates over hosts and populates connection
// fields from the secrets provider when a secret path is defined.
func resolveDockerSecrets(config *DockerConfig) error {
	if !pluginapi.HasSecrets() {
		return nil
	}

	for i := range config.Hosts {
		h := &config.Hosts[i]
		if h.Secret == "" {
			continue
		}

		entry, err := pluginapi.ResolveSecret(h.Secret)
		if err != nil {
			return fmt.Errorf("host %q: %w", h.Name, err)
		}

		// Override only blank fields so YAML values take precedence.
		if h.Host == "" && entry.URL != "" {
			h.Host = entry.URL
		}
		if h.Name == "" && entry.Title != "" {
			h.Name = entry.Title
		}
		// Custom attributes: tls_cert_path
		if h.CertPath == "" {
			if cp, ok := entry.CustomAttributes["cert_path"]; ok {
				h.CertPath = cp
			}
		}
	}
	return nil
}

// GetAvailableHosts returns the list of configured Docker hosts
func GetAvailableHosts() ([]DockerHost, error) {
	config, err := LoadDockerConfig("")
	if err != nil {
		return nil, err
	}
	return config.Hosts, nil
}

// GetDockerUIConfig returns the UI configuration
func GetDockerUIConfig() (UIConfig, error) {
	config, err := LoadDockerConfig("")
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

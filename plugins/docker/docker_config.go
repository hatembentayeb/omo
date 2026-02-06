package main

import (
	"fmt"
	"os"

	"omo/pkg/pluginapi"

	"gopkg.in/yaml.v3"
)

// DockerConfig represents the configuration for the Docker plugin
type DockerConfig struct {
	Hosts []DockerHost `yaml:"hosts"`
	UI    UIConfig     `yaml:"ui"`
}

// DockerHost represents a configured Docker host
type DockerHost struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Host        string `yaml:"host"`
	TLS         bool   `yaml:"tls"`
	TLSVerify   bool   `yaml:"tls_verify"`
	CertPath    string `yaml:"cert_path"`
}

// UIConfig represents UI configuration options
type UIConfig struct {
	RefreshInterval       int  `yaml:"refresh_interval"`
	MaxContainersDisplay  int  `yaml:"max_containers_display"`
	MaxImagesDisplay      int  `yaml:"max_images_display"`
	ShowAllContainers     bool `yaml:"show_all_containers"`
	LogTailLines          int  `yaml:"log_tail_lines"`
	EnableStats           bool `yaml:"enable_stats"`
	EnableCompose         bool `yaml:"enable_compose"`
}

// DefaultConfig returns the default configuration
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
			RefreshInterval:       5,
			MaxContainersDisplay:  100,
			MaxImagesDisplay:      100,
			ShowAllContainers:     true,
			LogTailLines:          500,
			EnableStats:           true,
			EnableCompose:         true,
		},
	}
}

// LoadDockerConfig loads the Docker configuration from the specified file
func LoadDockerConfig(configPath string) (*DockerConfig, error) {
	// If no path is specified, use the default config path
	if configPath == "" {
		configPath = pluginapi.PluginConfigPath("docker")
	}

	// Check if the file exists
	_, err := os.Stat(configPath)
	if os.IsNotExist(err) {
		// File doesn't exist, return default config
		return DefaultDockerConfig(), nil
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

	return config, nil
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

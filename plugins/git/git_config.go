package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// GitConfig represents the configuration for the Git plugin
type GitConfig struct {
	SearchPaths []string  `yaml:"search_paths"`
	UI          UIConfig  `yaml:"ui"`
	DefaultRepo string    `yaml:"default_repo"`
}

// UIConfig represents UI configuration options
type UIConfig struct {
	RefreshInterval    int  `yaml:"refresh_interval"`
	MaxReposDisplay    int  `yaml:"max_repos_display"`
	MaxCommitsDisplay  int  `yaml:"max_commits_display"`
	MaxSearchDepth     int  `yaml:"max_search_depth"`
	ShowRemoteBranches bool `yaml:"show_remote_branches"`
	ShowStash          bool `yaml:"show_stash"`
	ShowTags           bool `yaml:"show_tags"`
}

// DefaultGitConfig returns the default configuration
func DefaultGitConfig() *GitConfig {
	homedir, _ := os.UserHomeDir()
	return &GitConfig{
		SearchPaths: []string{
			filepath.Join(homedir, "projects"),
			filepath.Join(homedir, "work"),
			filepath.Join(homedir, "go/src"),
		},
		UI: UIConfig{
			RefreshInterval:    30,
			MaxReposDisplay:    100,
			MaxCommitsDisplay:  50,
			MaxSearchDepth:     3,
			ShowRemoteBranches: true,
			ShowStash:          true,
			ShowTags:           true,
		},
		DefaultRepo: "",
	}
}

// LoadGitConfig loads the Git configuration from the specified file
func LoadGitConfig(configPath string) (*GitConfig, error) {
	// If no path is specified, use the default config path
	if configPath == "" {
		configPath = filepath.Join("config", "git.yaml")
	}

	// Check if the file exists
	_, err := os.Stat(configPath)
	if os.IsNotExist(err) {
		// File doesn't exist, return default config
		return DefaultGitConfig(), nil
	}

	// Read the configuration file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %v", err)
	}

	// Unmarshal the configuration
	config := DefaultGitConfig()
	err = yaml.Unmarshal(data, config)
	if err != nil {
		return nil, fmt.Errorf("error parsing config file: %v", err)
	}

	// Expand home directory in paths
	homedir, _ := os.UserHomeDir()
	for i, path := range config.SearchPaths {
		if strings.HasPrefix(path, "~") {
			config.SearchPaths[i] = strings.Replace(path, "~", homedir, 1)
		}
	}

	return config, nil
}

// GetSearchPaths returns the configured search paths
func GetSearchPaths() ([]string, error) {
	config, err := LoadGitConfig("")
	if err != nil {
		return nil, err
	}
	return config.SearchPaths, nil
}

// GetGitUIConfig returns the UI configuration
func GetGitUIConfig() (UIConfig, error) {
	config, err := LoadGitConfig("")
	if err != nil {
		return UIConfig{}, err
	}
	return config.UI, nil
}

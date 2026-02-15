package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"omo/pkg/pluginapi"

	"gopkg.in/yaml.v3"
)

// gitConfigHeader is prepended to the auto-generated config YAML.
const gitConfigHeader = `# Git Plugin Configuration
# Path: ~/.omo/configs/git/git.yaml
#
# KeePass Secret Schema (secret path: git/<environment>/<name>):
#   Title    → repository name
#   URL      → remote URL (e.g. "https://github.com/org/repo.git")
#   UserName → git username
#   Password → auth token / personal access token
#
# When "secret" is set, auth fields are resolved from KeePass.
# YAML values take precedence over KeePass values (override only blanks).
# The plugin also discovers repositories from search_paths on the host.
`

// GitConfig represents the configuration for the Git plugin
type GitConfig struct {
	SearchPaths  []string      `yaml:"search_paths"`
	Repositories []GitRepoConfig `yaml:"repositories"`
	UI           UIConfig      `yaml:"ui"`
	DefaultRepo  string        `yaml:"default_repo"`
}

// GitRepoConfig represents a configured Git repository.
// When the Secret field is set (e.g. "git/github/main-repo"),
// it references a KeePass entry whose fields can provide auth tokens.
type GitRepoConfig struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Secret      string `yaml:"secret,omitempty"` // KeePass path: pluginName/env/entryName
	Path        string `yaml:"path"`
	RemoteURL   string `yaml:"remote_url,omitempty"`
	Username    string `yaml:"username,omitempty"`
	Token       string `yaml:"token,omitempty"`
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
		configPath = pluginapi.PluginConfigPath("git")
	}

	// Auto-create default config if missing
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		_ = writeDefaultConfig(configPath, gitConfigHeader, DefaultGitConfig())
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

	// Init KeePass placeholder if no entries exist yet
	initGitKeePass()

	// Resolve secrets for repositories that reference KeePass entries.
	if err := resolveGitSecrets(config); err != nil {
		return nil, fmt.Errorf("error resolving secrets: %v", err)
	}

	return config, nil
}

// initGitKeePass seeds placeholder KeePass entries for Git if none exist.
func initGitKeePass() {
	if !pluginapi.HasSecrets() {
		return
	}
	entries, err := pluginapi.Secrets().List("git")
	if err != nil || len(entries) > 0 {
		return
	}
	_ = pluginapi.Secrets().Put("git/default/example", &pluginapi.SecretEntry{
		Title:    "example",
		UserName: "",
		Password: "",
		URL:      "",
		Notes:    "Git placeholder. Set URL (remote URL), UserName (git user), Password (PAT/token).",
	})
}

// resolveGitSecrets iterates over repositories and populates auth
// fields from the secrets provider when a secret path is defined.
func resolveGitSecrets(config *GitConfig) error {
	if !pluginapi.HasSecrets() {
		return nil // no provider — skip silently
	}

	for i := range config.Repositories {
		repo := &config.Repositories[i]
		if repo.Secret == "" {
			continue
		}

		entry, err := pluginapi.ResolveSecret(repo.Secret)
		if err != nil {
			return fmt.Errorf("repository %q: %w", repo.Name, err)
		}

		// Override only blank fields so YAML values take precedence.
		if repo.Username == "" && entry.UserName != "" {
			repo.Username = entry.UserName
		}
		if repo.Token == "" && entry.Password != "" {
			repo.Token = entry.Password
		}
		if repo.RemoteURL == "" && entry.URL != "" {
			repo.RemoteURL = entry.URL
		}
		if repo.Name == "" && entry.Title != "" {
			repo.Name = entry.Title
		}
	}
	return nil
}

// GetConfiguredRepositories returns the list of configured repositories
func GetConfiguredRepositories() ([]GitRepoConfig, error) {
	config, err := LoadGitConfig("")
	if err != nil {
		return nil, err
	}
	return config.Repositories, nil
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

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"omo/pkg/pluginapi"
)

var defaultGitEnvironments = []string{
	"development",
	"production",
	"staging",
	"sandbox",
	"local",
	"test",
}

// GitRepoConfig is built entirely from a KeePass entry at runtime.
//
// KeePass Entry Schema (path: git/<environment>/<name>):
//
//	Title    → repository display name
//	URL      → remote URL (e.g. "https://github.com/org/repo.git")
//	UserName → git username
//	Password → auth token / personal access token
//	Notes    → description / notes
//
//	Custom Attributes:
//	  path → local path to the repository
//	  tags → comma-separated tags
type GitRepoConfig struct {
	Name        string
	Description string
	Environment string
	Path        string
	RemoteURL   string
	Username    string
	Token       string
	Tags        []string
}

// UIConfig holds hardcoded UI defaults for the Git plugin.
type UIConfig struct {
	RefreshInterval    int
	MaxReposDisplay    int
	MaxCommitsDisplay  int
	MaxSearchDepth     int
	ShowRemoteBranches bool
	ShowStash          bool
	ShowTags           bool
}

func DefaultGitUIConfig() UIConfig {
	return UIConfig{
		RefreshInterval:    30,
		MaxReposDisplay:    100,
		MaxCommitsDisplay:  50,
		MaxSearchDepth:     3,
		ShowRemoteBranches: true,
		ShowStash:          true,
		ShowTags:           true,
	}
}

// DiscoverRepositories reads KeePass groups under "git/" and builds
// GitRepoConfig objects from the entries.
func DiscoverRepositories() ([]GitRepoConfig, error) {
	if !pluginapi.HasSecrets() {
		return nil, fmt.Errorf("secrets provider not available")
	}

	if err := pluginapi.Secrets().Reload(); err != nil {
		return nil, fmt.Errorf("reload secrets: %w", err)
	}

	ensureGitKeePassGroups()

	paths, err := pluginapi.Secrets().List("git")
	if err != nil {
		return nil, fmt.Errorf("list git secrets: %w", err)
	}

	var repos []GitRepoConfig
	for _, path := range paths {
		env := extractEnvironment(path)
		if env == "" {
			continue
		}

		entry, err := pluginapi.Secrets().Get(path)
		if err != nil {
			continue
		}

		repo := entryToGitRepo(entry, env)
		repos = append(repos, repo)
	}

	sort.Slice(repos, func(i, j int) bool {
		if repos[i].Environment != repos[j].Environment {
			return repos[i].Environment < repos[j].Environment
		}
		return repos[i].Name < repos[j].Name
	})

	return repos, nil
}

func extractEnvironment(path string) string {
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 3 {
		return ""
	}
	return parts[1]
}

func entryToGitRepo(entry *pluginapi.SecretEntry, env string) GitRepoConfig {
	repo := GitRepoConfig{
		Name:        entry.Title,
		Description: entry.Notes,
		Environment: env,
		RemoteURL:   entry.URL,
		Username:    entry.UserName,
		Token:       entry.Password,
	}

	if entry.CustomAttributes == nil {
		return repo
	}

	ca := entry.CustomAttributes

	if v, ok := ca["path"]; ok {
		repo.Path = v
	}
	if v, ok := ca["tags"]; ok {
		for _, t := range strings.Split(v, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				repo.Tags = append(repo.Tags, t)
			}
		}
	}

	return repo
}

func ensureGitKeePassGroups() {
	if !pluginapi.HasSecrets() {
		return
	}

	requiredAttrs := map[string]string{
		"path": "",
		"tags": "",
	}

	for _, env := range defaultGitEnvironments {
		prefix := fmt.Sprintf("git/%s", env)
		existing, err := pluginapi.Secrets().List(prefix)
		if err == nil && len(existing) > 0 {
			backfillAttributes(existing, requiredAttrs)
			continue
		}
		path := fmt.Sprintf("git/%s/example", env)
		_ = pluginapi.Secrets().Put(path, &pluginapi.SecretEntry{
			Title:            "example",
			UserName:         "",
			Password:         "",
			URL:              "",
			Notes:            fmt.Sprintf("Git %s placeholder. Set URL (remote URL), UserName (git user), Password (PAT/token).", env),
			CustomAttributes: requiredAttrs,
		})
	}
}

func backfillAttributes(entryPaths []string, required map[string]string) {
	for _, entryPath := range entryPaths {
		entry, err := pluginapi.Secrets().Get(entryPath)
		if err != nil || entry == nil {
			continue
		}
		if entry.CustomAttributes == nil {
			entry.CustomAttributes = make(map[string]string)
		}
		updated := false
		for attr, defaultVal := range required {
			if _, exists := entry.CustomAttributes[attr]; !exists {
				entry.CustomAttributes[attr] = defaultVal
				updated = true
			}
		}
		if updated {
			_ = pluginapi.Secrets().Put(entryPath, entry)
		}
	}
}

// GetSearchPaths returns default search paths for auto-discovering
// Git repositories on the local filesystem.
func GetSearchPaths() ([]string, error) {
	homedir, _ := os.UserHomeDir()
	return []string{
		filepath.Join(homedir, "projects"),
		filepath.Join(homedir, "work"),
		filepath.Join(homedir, "go/src"),
	}, nil
}

// GetConfiguredRepositories returns all discovered Git repositories from KeePass.
func GetConfiguredRepositories() ([]GitRepoConfig, error) {
	return DiscoverRepositories()
}

// GetGitUIConfig returns the hardcoded UI configuration.
func GetGitUIConfig() (UIConfig, error) {
	return DefaultGitUIConfig(), nil
}

package main

import (
	"fmt"
	"sort"
	"strings"

	"omo/pkg/pluginapi"
)

var defaultGitHubEnvironments = []string{
	"personal",
	"work",
	"organization",
}

// GitHubAccount represents a GitHub account/token stored in KeePass.
// One token gives access to all repos for the authenticated user.
//
// KeePass Entry Schema (path: github/<environment>/<name>):
//
//	Title    → account display name (e.g. "personal", "work-org")
//	UserName → org name (only required when type=org, leave empty for personal accounts)
//	Password → GitHub personal access token (PAT) — this is all you need for auth
//	URL      → GitHub API base URL (empty = https://api.github.com, or GitHub Enterprise URL)
//	Notes    → description / notes
//
//	Custom Attributes:
//	  type → "user" or "org" (default: "user")
//	         "user"  = list repos for the authenticated user (UserName not needed)
//	         "org"   = list repos for the org in UserName
type GitHubAccount struct {
	Name        string
	Owner       string // only used for type=org; for type=user this is resolved from the API
	Token       string
	APIURL      string
	Description string
	Environment string
	AccountType string // "user" or "org"
}

// GitHubRepo is a repository discovered from the API at runtime, not stored in KeePass.
type GitHubRepo struct {
	Name          string
	FullName      string
	Owner         string
	Description   string
	DefaultBranch string
	Private       bool
	Fork          bool
	Archived      bool
	Stars         int
	Language      string
	UpdatedAt     string
}

type UIConfig struct {
	RefreshInterval int
	MaxPRsDisplay   int
	MaxRunsDisplay  int
	ShowDraftPRs    bool
	DefaultPRState  string
}

func DefaultUIConfig() UIConfig {
	return UIConfig{
		RefreshInterval: 30,
		MaxPRsDisplay:   100,
		MaxRunsDisplay:  50,
		ShowDraftPRs:    true,
		DefaultPRState:  "open",
	}
}

func DiscoverAccounts() ([]GitHubAccount, error) {
	if !pluginapi.HasSecrets() {
		return nil, fmt.Errorf("secrets provider not available")
	}

	if err := pluginapi.Secrets().Reload(); err != nil {
		return nil, fmt.Errorf("reload secrets: %w", err)
	}

	ensureGitHubKeePassGroups()

	paths, err := pluginapi.Secrets().List("github")
	if err != nil {
		return nil, fmt.Errorf("list github secrets: %w", err)
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("no GitHub entries in KeePass (create entries under github/<environment>/<name>)")
	}

	var accounts []GitHubAccount
	for _, path := range paths {
		env := extractEnvironment(path)
		if env == "" {
			continue
		}

		entry, err := pluginapi.Secrets().Get(path)
		if err != nil {
			continue
		}

		acct := entryToGitHubAccount(entry, env)
		if acct.Token == "" {
			continue
		}
		if acct.AccountType == "org" && acct.Owner == "" {
			continue
		}
		accounts = append(accounts, acct)
	}

	sort.Slice(accounts, func(i, j int) bool {
		if accounts[i].Environment != accounts[j].Environment {
			return accounts[i].Environment < accounts[j].Environment
		}
		return accounts[i].Name < accounts[j].Name
	})

	return accounts, nil
}

func extractEnvironment(path string) string {
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 3 {
		return ""
	}
	return parts[1]
}

func entryToGitHubAccount(entry *pluginapi.SecretEntry, env string) GitHubAccount {
	acct := GitHubAccount{
		Name:        entry.Title,
		Owner:       entry.UserName,
		Token:       entry.Password,
		Description: entry.Notes,
		Environment: env,
		AccountType: "user",
	}

	if entry.URL != "" {
		acct.APIURL = entry.URL
	}

	if entry.CustomAttributes == nil {
		return acct
	}

	ca := entry.CustomAttributes

	if v, ok := ca["type"]; ok && v != "" {
		acct.AccountType = v
	}

	return acct
}

func ensureGitHubKeePassGroups() {
	if !pluginapi.HasSecrets() {
		return
	}

	requiredAttrs := map[string]string{
		"type": "user",
	}

	for _, env := range defaultGitHubEnvironments {
		prefix := fmt.Sprintf("github/%s", env)
		existing, err := pluginapi.Secrets().List(prefix)
		if err == nil && len(existing) > 0 {
			backfillAttributes(existing, requiredAttrs)
			continue
		}
		path := fmt.Sprintf("github/%s/example", env)
		_ = pluginapi.Secrets().Put(path, &pluginapi.SecretEntry{
			Title:            "example",
			UserName:         "",
			Password:         "ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			URL:              "",
			Notes:            fmt.Sprintf("GitHub %s account. Set Password (PAT token). For orgs: set type=org and UserName to the org name.", env),
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

func GetAvailableAccounts() ([]GitHubAccount, error) {
	return DiscoverAccounts()
}

func GetGitHubUIConfig() UIConfig {
	return DefaultUIConfig()
}

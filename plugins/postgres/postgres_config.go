package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"omo/pkg/pluginapi"
)

var defaultPostgresEnvironments = []string{
	"development",
	"production",
	"staging",
	"sandbox",
	"local",
	"test",
}

// PostgresInstance is built entirely from a KeePass entry at runtime.
//
// KeePass Entry Schema (path: postgres/<environment>/<name>):
//
//	Title    → instance display name
//	URL      → host (e.g. "localhost", "pg.example.com")
//	UserName → PostgreSQL username
//	Password → PostgreSQL password
//	Notes    → description / notes
//
//	Custom Attributes:
//	  port     → PostgreSQL port (default: 5432)
//	  database → database name (default: "postgres")
//	  sslmode  → SSL mode (default: "disable")
//	  tags     → comma-separated tags
type PostgresInstance struct {
	Name        string
	Description string
	Environment string
	Host        string
	Port        int
	Username    string
	Password    string
	Database    string
	SSLMode     string
	Tags        []string
}

// UIConfig holds hardcoded UI defaults for the PostgreSQL plugin.
type UIConfig struct {
	RefreshInterval int
	MaxRowsDisplay  int
	EnableLogs      bool
	EnableStats     bool
}

func DefaultUIConfig() UIConfig {
	return UIConfig{
		RefreshInterval: 10,
		MaxRowsDisplay:  1000,
		EnableLogs:      true,
		EnableStats:     true,
	}
}

// DiscoverInstances reads KeePass groups under "postgres/" and builds
// PostgresInstance objects from the entries.
func DiscoverInstances() ([]PostgresInstance, error) {
	if !pluginapi.HasSecrets() {
		return nil, fmt.Errorf("secrets provider not available")
	}

	if err := pluginapi.Secrets().Reload(); err != nil {
		return nil, fmt.Errorf("reload secrets: %w", err)
	}

	ensurePostgresKeePassGroups()

	paths, err := pluginapi.Secrets().List("postgres")
	if err != nil {
		return nil, fmt.Errorf("list postgres secrets: %w", err)
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("no PostgreSQL entries in KeePass (create entries under postgres/<environment>/<name>)")
	}

	var instances []PostgresInstance
	for _, path := range paths {
		env := extractEnvironment(path)
		if env == "" {
			continue
		}

		entry, err := pluginapi.Secrets().Get(path)
		if err != nil {
			continue
		}

		inst := entryToPostgresInstance(entry, env)
		instances = append(instances, inst)
	}

	sort.Slice(instances, func(i, j int) bool {
		if instances[i].Environment != instances[j].Environment {
			return instances[i].Environment < instances[j].Environment
		}
		return instances[i].Name < instances[j].Name
	})

	return instances, nil
}

func extractEnvironment(path string) string {
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 3 {
		return ""
	}
	return parts[1]
}

func entryToPostgresInstance(entry *pluginapi.SecretEntry, env string) PostgresInstance {
	inst := PostgresInstance{
		Name:        entry.Title,
		Description: entry.Notes,
		Environment: env,
		Host:        entry.URL,
		Username:    entry.UserName,
		Password:    entry.Password,
		Port:        5432,
		Database:    "postgres",
		SSLMode:     "disable",
	}

	if entry.CustomAttributes == nil {
		return inst
	}

	ca := entry.CustomAttributes

	if v, ok := ca["port"]; ok {
		if p, err := strconv.Atoi(v); err == nil {
			inst.Port = p
		}
	}
	if v, ok := ca["database"]; ok && v != "" {
		inst.Database = v
	}
	if v, ok := ca["sslmode"]; ok && v != "" {
		inst.SSLMode = v
	}
	if v, ok := ca["tags"]; ok {
		for _, t := range strings.Split(v, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				inst.Tags = append(inst.Tags, t)
			}
		}
	}

	return inst
}

func ensurePostgresKeePassGroups() {
	if !pluginapi.HasSecrets() {
		return
	}

	requiredAttrs := map[string]string{
		"port":     "5432",
		"database": "postgres",
		"sslmode":  "disable",
		"tags":     "",
	}

	for _, env := range defaultPostgresEnvironments {
		prefix := fmt.Sprintf("postgres/%s", env)
		existing, err := pluginapi.Secrets().List(prefix)
		if err == nil && len(existing) > 0 {
			backfillAttributes(existing, requiredAttrs)
			continue
		}
		path := fmt.Sprintf("postgres/%s/example", env)
		_ = pluginapi.Secrets().Put(path, &pluginapi.SecretEntry{
			Title:            "example",
			UserName:         "postgres",
			Password:         "",
			URL:              "localhost",
			Notes:            fmt.Sprintf("PostgreSQL %s placeholder. Set URL (host), UserName, Password.", env),
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

// GetAvailableInstances returns all discovered PostgreSQL instances.
func GetAvailableInstances() ([]PostgresInstance, error) {
	return DiscoverInstances()
}

// GetUIConfig returns the hardcoded UI configuration.
func GetUIConfig() (UIConfig, error) {
	return DefaultUIConfig(), nil
}

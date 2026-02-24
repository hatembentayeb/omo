package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"omo/pkg/pluginapi"
)

var defaultRedisEnvironments = []string{
	"development",
	"production",
	"staging",
	"sandbox",
	"local",
	"test",
}

// RedisInstance is built entirely from a KeePass entry at runtime.
//
// KeePass Entry Schema (path: redis/<environment>/<name>):
//
//	Title    → instance display name
//	URL      → host (e.g. "localhost", "redis.example.com")
//	UserName → Redis ACL username (Redis 6+)
//	Password → Redis password
//	Notes    → description / notes
//
//	Custom Attributes:
//	  port     → Redis port (default: 6379)
//	  database → Redis database index (default: 0)
//	  tags     → comma-separated tags
type RedisInstance struct {
	Name        string
	Description string
	Environment string
	Host        string
	Port        int
	Username    string
	Password    string
	Database    int
	Tags        []string
}

// UIConfig holds hardcoded UI defaults for the Redis plugin.
type UIConfig struct {
	RefreshInterval  int
	MaxKeysDisplay   int
	EnableSlowLog    bool
	EnableServerInfo bool
}

func DefaultUIConfig() UIConfig {
	return UIConfig{
		RefreshInterval:  5,
		MaxKeysDisplay:   1000,
		EnableSlowLog:    true,
		EnableServerInfo: true,
	}
}

// DiscoverInstances reads KeePass groups under "redis/" and builds
// RedisInstance objects from the entries.
func DiscoverInstances() ([]RedisInstance, error) {
	if !pluginapi.HasSecrets() {
		return nil, fmt.Errorf("secrets provider not available")
	}

	if err := pluginapi.Secrets().Reload(); err != nil {
		return nil, fmt.Errorf("reload secrets: %w", err)
	}

	ensureRedisKeePassGroups()

	paths, err := pluginapi.Secrets().List("redis")
	if err != nil {
		return nil, fmt.Errorf("list redis secrets: %w", err)
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("no Redis entries in KeePass (create entries under redis/<environment>/<name>)")
	}

	var instances []RedisInstance
	for _, path := range paths {
		env := extractEnvironment(path)
		if env == "" {
			continue
		}

		entry, err := pluginapi.Secrets().Get(path)
		if err != nil {
			continue
		}

		inst := entryToRedisInstance(entry, env)
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

func entryToRedisInstance(entry *pluginapi.SecretEntry, env string) RedisInstance {
	inst := RedisInstance{
		Name:        entry.Title,
		Description: entry.Notes,
		Environment: env,
		Host:        entry.URL,
		Username:    entry.UserName,
		Password:    entry.Password,
		Port:        6379,
		Database:    0,
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
	if v, ok := ca["database"]; ok {
		if d, err := strconv.Atoi(v); err == nil {
			inst.Database = d
		}
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

// ensureRedisKeePassGroups creates environment groups in KeePass
// with placeholder entries so the folder structure is visible in KeePassXC.
func ensureRedisKeePassGroups() {
	if !pluginapi.HasSecrets() {
		return
	}

	requiredAttrs := map[string]string{
		"port":     "6379",
		"database": "0",
		"tags":     "",
	}

	for _, env := range defaultRedisEnvironments {
		prefix := fmt.Sprintf("redis/%s", env)
		existing, err := pluginapi.Secrets().List(prefix)
		if err == nil && len(existing) > 0 {
			for _, entryPath := range existing {
				backfillAttributes(entryPath, requiredAttrs)
			}
			continue
		}
		path := fmt.Sprintf("redis/%s/example", env)
		_ = pluginapi.Secrets().Put(path, &pluginapi.SecretEntry{
			Title:            "example",
			UserName:         "",
			Password:         "",
			URL:              "localhost",
			Notes:            fmt.Sprintf("Redis %s placeholder. Set URL (host), UserName (ACL user), Password.", env),
			CustomAttributes: requiredAttrs,
		})
	}
}

func backfillAttributes(entryPath string, required map[string]string) {
	entry, err := pluginapi.Secrets().Get(entryPath)
	if err != nil || entry == nil {
		return
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

// GetAvailableInstances returns all discovered Redis instances.
func GetAvailableInstances() ([]RedisInstance, error) {
	return DiscoverInstances()
}

// GetUIConfig returns the hardcoded UI configuration.
func GetUIConfig() (UIConfig, error) {
	return DefaultUIConfig(), nil
}

package main

import (
	"fmt"
	"sort"
	"strings"

	"omo/pkg/pluginapi"
)

var defaultDockerEnvironments = []string{
	"development",
	"production",
	"staging",
	"sandbox",
	"local",
	"test",
}

// DockerHost is built entirely from a KeePass entry at runtime.
//
// KeePass Entry Schema (path: docker/<environment>/<name>):
//
//	Title    → host display name
//	URL      → docker host URL (e.g. "unix:///var/run/docker.sock", "tcp://host:2376")
//	Notes    → description / notes
//
//	Custom Attributes:
//	  cert_path  → path to TLS certificates directory
//	  tls        → "true" to enable TLS (default: false)
//	  tls_verify → "true" to enable TLS verification (default: false)
//	  tags       → comma-separated tags
type DockerHost struct {
	Name        string
	Description string
	Environment string
	Host        string
	TLS         bool
	TLSVerify   bool
	CertPath    string
	Tags        []string
}

// UIConfig holds hardcoded UI defaults for the Docker plugin.
type UIConfig struct {
	RefreshInterval      int
	MaxContainersDisplay int
	MaxImagesDisplay     int
	ShowAllContainers    bool
	LogTailLines         int
	EnableStats          bool
	EnableCompose        bool
}

func DefaultUIConfig() UIConfig {
	return UIConfig{
		RefreshInterval:      5,
		MaxContainersDisplay: 100,
		MaxImagesDisplay:     100,
		ShowAllContainers:    true,
		LogTailLines:         500,
		EnableStats:          true,
		EnableCompose:        true,
	}
}

// DiscoverHosts reads KeePass groups under "docker/" and builds
// DockerHost objects from the entries.
func DiscoverHosts() ([]DockerHost, error) {
	if !pluginapi.HasSecrets() {
		return nil, fmt.Errorf("secrets provider not available")
	}

	if err := pluginapi.Secrets().Reload(); err != nil {
		return nil, fmt.Errorf("reload secrets: %w", err)
	}

	ensureDockerKeePassGroups()

	paths, err := pluginapi.Secrets().List("docker")
	if err != nil {
		return nil, fmt.Errorf("list docker secrets: %w", err)
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("no Docker entries in KeePass (create entries under docker/<environment>/<name>)")
	}

	var hosts []DockerHost
	for _, path := range paths {
		env := extractEnvironment(path)
		if env == "" {
			continue
		}

		entry, err := pluginapi.Secrets().Get(path)
		if err != nil {
			continue
		}

		host := entryToDockerHost(entry, env)
		hosts = append(hosts, host)
	}

	sort.Slice(hosts, func(i, j int) bool {
		if hosts[i].Environment != hosts[j].Environment {
			return hosts[i].Environment < hosts[j].Environment
		}
		return hosts[i].Name < hosts[j].Name
	})

	return hosts, nil
}

func extractEnvironment(path string) string {
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 3 {
		return ""
	}
	return parts[1]
}

func entryToDockerHost(entry *pluginapi.SecretEntry, env string) DockerHost {
	host := DockerHost{
		Name:        entry.Title,
		Description: entry.Notes,
		Environment: env,
		Host:        entry.URL,
	}

	if entry.CustomAttributes == nil {
		return host
	}

	ca := entry.CustomAttributes

	if v, ok := ca["cert_path"]; ok {
		host.CertPath = v
	}
	if v, ok := ca["tls"]; ok {
		host.TLS = v == "true" || v == "1" || v == "yes"
	}
	if v, ok := ca["tls_verify"]; ok {
		host.TLSVerify = v == "true" || v == "1" || v == "yes"
	}
	if v, ok := ca["tags"]; ok {
		for _, t := range strings.Split(v, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				host.Tags = append(host.Tags, t)
			}
		}
	}

	return host
}

func ensureDockerKeePassGroups() {
	if !pluginapi.HasSecrets() {
		return
	}

	requiredAttrs := map[string]string{
		"cert_path":  "",
		"tls":        "false",
		"tls_verify": "false",
		"tags":       "",
	}

	for _, env := range defaultDockerEnvironments {
		prefix := fmt.Sprintf("docker/%s", env)
		existing, err := pluginapi.Secrets().List(prefix)
		if err == nil && len(existing) > 0 {
			backfillAttributes(existing, requiredAttrs)
			continue
		}
		path := fmt.Sprintf("docker/%s/example", env)
		_ = pluginapi.Secrets().Put(path, &pluginapi.SecretEntry{
			Title:            "example",
			URL:              "unix:///var/run/docker.sock",
			Notes:            fmt.Sprintf("Docker %s placeholder. Set URL (docker host), cert_path (TLS certs dir).", env),
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

// GetAvailableHosts returns all discovered Docker hosts.
func GetAvailableHosts() ([]DockerHost, error) {
	return DiscoverHosts()
}

// GetDockerUIConfig returns the hardcoded UI configuration.
func GetDockerUIConfig() (UIConfig, error) {
	return DefaultUIConfig(), nil
}

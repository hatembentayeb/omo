package main

import (
	"fmt"
	"sort"
	"strings"

	"omo/pkg/pluginapi"
)

var defaultK8sEnvironments = []string{
	"development",
	"production",
	"staging",
	"sandbox",
	"local",
	"test",
}

// K8sCluster is built entirely from a KeePass entry at runtime.
//
// KeePass Entry Schema (path: k8suser/<environment>/<name>):
//
//	Title    → cluster display name
//	URL      → Kubernetes API server URL (e.g. "https://k8s.example.com:6443")
//	Password → bearer token for authentication
//	Notes    → description / notes
//
//	Custom Attributes:
//	  kubeconfig → path to kubeconfig file (e.g. "~/.kube/config")
//	  context    → kubectl context name
//	  ca_cert    → path to CA certificate
//	  tags       → comma-separated tags
type K8sCluster struct {
	Name        string
	Description string
	Environment string
	Server      string
	Token       string
	Kubeconfig  string
	Context     string
	CACert      string
	Tags        []string
}

// K8sUserUIConfig holds hardcoded UI defaults for the K8sUser plugin.
type K8sUserUIConfig struct {
	RefreshInterval     int
	DefaultNamespace    string
	CertValidityDays    int
	EnableRoleManager   bool
	EnableAccessTesting bool
	EnableExport        bool
}

func DefaultK8sUserUIConfig() K8sUserUIConfig {
	return K8sUserUIConfig{
		RefreshInterval:     30,
		DefaultNamespace:    "default",
		CertValidityDays:    365,
		EnableRoleManager:   true,
		EnableAccessTesting: true,
		EnableExport:        true,
	}
}

// DiscoverClusters reads KeePass groups under "k8suser/" and builds
// K8sCluster objects from the entries.
func DiscoverClusters() ([]K8sCluster, error) {
	if !pluginapi.HasSecrets() {
		return nil, fmt.Errorf("secrets provider not available")
	}

	if err := pluginapi.Secrets().Reload(); err != nil {
		return nil, fmt.Errorf("reload secrets: %w", err)
	}

	ensureK8sUserKeePassGroups()

	paths, err := pluginapi.Secrets().List("k8suser")
	if err != nil {
		return nil, fmt.Errorf("list k8suser secrets: %w", err)
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("no K8sUser entries in KeePass (create entries under k8suser/<environment>/<name>)")
	}

	var clusters []K8sCluster
	for _, path := range paths {
		env := extractEnvironment(path)
		if env == "" {
			continue
		}

		entry, err := pluginapi.Secrets().Get(path)
		if err != nil {
			continue
		}

		cluster := entryToK8sCluster(entry, env)
		clusters = append(clusters, cluster)
	}

	sort.Slice(clusters, func(i, j int) bool {
		if clusters[i].Environment != clusters[j].Environment {
			return clusters[i].Environment < clusters[j].Environment
		}
		return clusters[i].Name < clusters[j].Name
	})

	return clusters, nil
}

func extractEnvironment(path string) string {
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 3 {
		return ""
	}
	return parts[1]
}

func entryToK8sCluster(entry *pluginapi.SecretEntry, env string) K8sCluster {
	cluster := K8sCluster{
		Name:        entry.Title,
		Description: entry.Notes,
		Environment: env,
		Server:      entry.URL,
		Token:       entry.Password,
	}

	if entry.CustomAttributes == nil {
		return cluster
	}

	ca := entry.CustomAttributes

	if v, ok := ca["kubeconfig"]; ok {
		cluster.Kubeconfig = v
	}
	if v, ok := ca["context"]; ok {
		cluster.Context = v
	}
	if v, ok := ca["ca_cert"]; ok {
		cluster.CACert = v
	}
	if v, ok := ca["tags"]; ok {
		for _, t := range strings.Split(v, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				cluster.Tags = append(cluster.Tags, t)
			}
		}
	}

	return cluster
}

func ensureK8sUserKeePassGroups() {
	if !pluginapi.HasSecrets() {
		return
	}

	requiredAttrs := map[string]string{
		"kubeconfig": "~/.kube/config",
		"context":    "",
		"ca_cert":    "",
		"tags":       "",
	}

	for _, env := range defaultK8sEnvironments {
		prefix := fmt.Sprintf("k8suser/%s", env)
		existing, err := pluginapi.Secrets().List(prefix)
		if err == nil && len(existing) > 0 {
			backfillAttributes(existing, requiredAttrs)
			continue
		}
		path := fmt.Sprintf("k8suser/%s/example", env)
		_ = pluginapi.Secrets().Put(path, &pluginapi.SecretEntry{
			Title:            "example",
			UserName:         "",
			Password:         "",
			URL:              "",
			Notes:            fmt.Sprintf("K8sUser %s placeholder. Set URL (API server), Password (bearer token).", env),
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

// GetAvailableK8sClusters returns all discovered K8s clusters.
func GetAvailableK8sClusters() ([]K8sCluster, error) {
	return DiscoverClusters()
}

// GetK8sUserUIConfig returns the hardcoded UI configuration.
func GetK8sUserUIConfig() (K8sUserUIConfig, error) {
	return DefaultK8sUserUIConfig(), nil
}

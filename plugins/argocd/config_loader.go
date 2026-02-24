package main

import (
	"fmt"
	"sort"
	"strings"

	"omo/pkg/pluginapi"
)

var defaultArgocdEnvironments = []string{
	"development",
	"production",
	"staging",
	"sandbox",
	"local",
	"test",
}

// ArgocdConfig holds non-secret settings for the ArgoCD plugin.
// These are hardcoded defaults (no YAML file needed).
type ArgocdConfig struct {
	Debug DebugConfig
}

type DebugConfig struct {
	Enabled               bool
	LogAPICalls           bool
	LogResponses          bool
	RequestTimeoutSeconds int
}

// ArgocdInstance is built entirely from a KeePass entry at runtime.
//
// KeePass Entry Schema (path: argocd/<environment>/<name>):
//
//	Title    → instance display name
//	URL      → ArgoCD server URL (e.g. "https://argocd.example.com")
//	UserName → ArgoCD username (for user/pass auth)
//	Password → ArgoCD password (for user/pass auth)
//	Notes    → description / notes
//
//	Custom Attributes:
//	  auth_token      → bearer token (alternative to user/pass)
//	  insecure        → "true" to skip TLS verification (default: false)
//	  kubeconfig      → inline kubeconfig YAML (for RBAC management via k8s API)
//	  kubeconfig_path → path to kubeconfig file (alternative to inline)
//	  namespace       → ArgoCD namespace in k8s (default: "argocd")
//	  tags            → comma-separated tags (e.g. "prod,cluster-a")
type ArgocdInstance struct {
	Name           string
	Description    string
	Environment    string
	URL            string
	Username       string
	Password       string
	AuthToken      string
	Insecure       bool
	Kubeconfig     string
	KubeconfigPath string
	Namespace      string
	Tags           []string
}

func DefaultArgocdConfig() *ArgocdConfig {
	return &ArgocdConfig{
		Debug: DebugConfig{
			Enabled:               false,
			LogAPICalls:           false,
			LogResponses:          false,
			RequestTimeoutSeconds: 30,
		},
	}
}

// LoadArgocdConfig returns the default ArgoCD config.
func LoadArgocdConfig() (*ArgocdConfig, error) {
	return DefaultArgocdConfig(), nil
}

// DiscoverArgoInstances reads KeePass groups under "argocd/" and builds
// ArgocdInstance objects from the entries.
func DiscoverArgoInstances() ([]ArgocdInstance, error) {
	if !pluginapi.HasSecrets() {
		return nil, fmt.Errorf("secrets provider not available")
	}

	if err := pluginapi.Secrets().Reload(); err != nil {
		return nil, fmt.Errorf("reload secrets: %w", err)
	}

	ensureArgocdKeePassGroups()

	paths, err := pluginapi.Secrets().List("argocd")
	if err != nil {
		return nil, fmt.Errorf("list argocd secrets: %w", err)
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("no ArgoCD entries in KeePass (create entries under argocd/<environment>/<name>)")
	}

	var instances []ArgocdInstance
	for _, path := range paths {
		env := extractArgoEnvironment(path)
		if env == "" {
			continue
		}

		entry, err := pluginapi.Secrets().Get(path)
		if err != nil {
			continue
		}

		inst := entryToArgoInstance(entry, env)
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

func extractArgoEnvironment(path string) string {
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 3 {
		return ""
	}
	return parts[1]
}

func entryToArgoInstance(entry *pluginapi.SecretEntry, env string) ArgocdInstance {
	inst := ArgocdInstance{
		Name:        entry.Title,
		Description: entry.Notes,
		Environment: env,
		URL:         entry.URL,
		Username:    entry.UserName,
		Password:    entry.Password,
		Namespace:   "argocd",
	}

	if entry.CustomAttributes == nil {
		return inst
	}

	ca := entry.CustomAttributes

	if v, ok := ca["auth_token"]; ok {
		inst.AuthToken = v
	}
	if v, ok := ca["insecure"]; ok {
		inst.Insecure = v == "true" || v == "1" || v == "yes"
	}
	if v, ok := ca["kubeconfig"]; ok {
		inst.Kubeconfig = v
	}
	if v, ok := ca["kubeconfig_path"]; ok {
		inst.KubeconfigPath = v
	}
	if v, ok := ca["namespace"]; ok && v != "" {
		inst.Namespace = v
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

// ensureArgocdKeePassGroups creates environment groups in KeePass
// with placeholder entries and ensures all known custom attributes
// exist on every entry so users can see what fields are available.
func ensureArgocdKeePassGroups() {
	if !pluginapi.HasSecrets() {
		return
	}

	requiredAttrs := map[string]string{
		"auth_token":      "",
		"insecure":        "false",
		"kubeconfig":      "",
		"kubeconfig_path": "",
		"namespace":       "argocd",
		"tags":            "",
	}

	for _, env := range defaultArgocdEnvironments {
		prefix := fmt.Sprintf("argocd/%s", env)
		existing, err := pluginapi.Secrets().List(prefix)
		if err == nil && len(existing) > 0 {
			backfillAttributes(existing, requiredAttrs)
			continue
		}
		path := fmt.Sprintf("argocd/%s/example", env)
		_ = pluginapi.Secrets().Put(path, &pluginapi.SecretEntry{
			Title:            "example",
			UserName:         "admin",
			Password:         "",
			URL:              "https://argocd.example.com",
			Notes:            fmt.Sprintf("ArgoCD %s placeholder. Fill in your ArgoCD details and rename this entry.", env),
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

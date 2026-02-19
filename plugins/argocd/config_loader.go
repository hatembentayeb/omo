package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"omo/pkg/pluginapi"

	"gopkg.in/yaml.v2"
)

const argocdConfigHeader = `# ArgoCD Plugin Configuration
# Path: ~/.omo/configs/argocd/argocd.yaml
#
# All connection details are stored in KeePass under argocd/<environment>/<name>.
# This file only controls which environments are enabled and debug settings.
#
# KeePass Entry Schema (unified attribute names):
#   Title    → instance display name
#   URL      → ArgoCD server URL (e.g. "https://argocd.example.com")
#   UserName → ArgoCD username (for user/pass auth)
#   Password → ArgoCD password (for user/pass auth)
#   Notes    → description / notes
#
#   Custom Attributes (set in KeePass "Advanced" tab):
#     auth_token  → bearer token (alternative to user/pass)
#     insecure        → "true" to skip TLS verification (default: false)
#     kubeconfig      → inline kubeconfig YAML (for RBAC management via k8s API)
#     kubeconfig_path → path to kubeconfig file (alternative to inline)
#     namespace       → ArgoCD namespace in k8s (default: "argocd")
#     tags            → comma-separated tags (e.g. "prod,cluster-a")
#
# Example KeePass structure:
#   argocd/
#     development/
#       local-argo      (Title=local-argo, URL=https://localhost:8080, UserName=admin ...)
#     production/
#       prod-cluster    (Title=prod-cluster, URL=https://argocd.prod.com, auth_token=ey... ...)
#     staging/
#       staging-argo    (...)
`

// ArgocdConfig is the YAML config. It only controls enable/disable and debug.
type ArgocdConfig struct {
	Environments []ArgocdEnvToggle `yaml:"environments"`
	Debug        DebugConfig       `yaml:"debug"`
}

// ArgocdEnvToggle enables or disables a KeePass environment group.
type ArgocdEnvToggle struct {
	Name    string `yaml:"name"`
	Enabled bool   `yaml:"enabled"`
}

// DebugConfig represents debug settings
type DebugConfig struct {
	Enabled               bool `yaml:"enabled"`
	LogAPICalls           bool `yaml:"log_api_calls"`
	LogResponses          bool `yaml:"log_responses"`
	RequestTimeoutSeconds int  `yaml:"request_timeout_seconds"`
}

// ArgocdInstance is built entirely from a KeePass entry at runtime.
type ArgocdInstance struct {
	Name           string
	Description    string
	Environment    string
	URL            string
	Username       string
	Password       string
	AuthToken      string
	Insecure       bool
	Kubeconfig     string // inline kubeconfig YAML for RBAC management
	KubeconfigPath string // path to kubeconfig file
	Namespace      string // ArgoCD k8s namespace (default "argocd")
	Tags           []string
}

func DefaultArgocdConfig() *ArgocdConfig {
	return &ArgocdConfig{
		Environments: []ArgocdEnvToggle{
			{Name: "development", Enabled: true},
			{Name: "production", Enabled: true},
			{Name: "staging", Enabled: true},
			{Name: "sandbox", Enabled: true},
		},
		Debug: DebugConfig{
			Enabled:               false,
			LogAPICalls:           false,
			LogResponses:          false,
			RequestTimeoutSeconds: 30,
		},
	}
}

func LoadArgocdConfig() (*ArgocdConfig, error) {
	configPath := pluginapi.PluginConfigPath("argocd")

	needsWrite := false

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		needsWrite = true
	} else {
		data, err := os.ReadFile(configPath)
		if err == nil && isOldArgocdConfig(data) {
			needsWrite = true
		}
	}

	if needsWrite {
		_ = writeDefaultConfig(configPath, argocdConfigHeader, DefaultArgocdConfig())
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("no ArgoCD config found at %s: %v", configPath, err)
	}

	config := DefaultArgocdConfig()
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config %s: %v", configPath, err)
	}

	return config, nil
}

// isOldArgocdConfig detects the legacy format that had instances defined in YAML.
func isOldArgocdConfig(data []byte) bool {
	var probe struct {
		Instances    interface{} `yaml:"instances"`
		Environments interface{} `yaml:"environments"`
	}
	if err := yaml.Unmarshal(data, &probe); err != nil {
		return false
	}
	return probe.Instances != nil && probe.Environments == nil
}

// DiscoverInstances reads KeePass groups under "argocd/" and builds
// ArgocdInstance objects from the entries. Only enabled environments
// from the YAML config are included.
func DiscoverArgoInstances() ([]ArgocdInstance, error) {
	if !pluginapi.HasSecrets() {
		return nil, fmt.Errorf("secrets provider not available")
	}

	if err := pluginapi.Secrets().Reload(); err != nil {
		return nil, fmt.Errorf("reload secrets: %w", err)
	}

	config, err := LoadArgocdConfig()
	if err != nil {
		return nil, err
	}

	enabled := buildArgoEnabledSet(config)

	ensureArgocdKeePassGroups(config)

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
		if _, ok := enabled[env]; !ok {
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

func buildArgoEnabledSet(config *ArgocdConfig) map[string]struct{} {
	m := make(map[string]struct{})
	for _, e := range config.Environments {
		if e.Enabled {
			m[e.Name] = struct{}{}
		}
	}
	return m
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
// for each enabled environment and ensures all known custom attributes
// exist on every entry so users can see what fields are available.
func ensureArgocdKeePassGroups(config *ArgocdConfig) {
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

	for _, env := range config.Environments {
		if !env.Enabled {
			continue
		}
		prefix := fmt.Sprintf("argocd/%s", env.Name)
		existing, err := pluginapi.Secrets().List(prefix)
		if err != nil || len(existing) == 0 {
			// No entries — create a placeholder with all attributes
			path := fmt.Sprintf("argocd/%s/example", env.Name)
			_ = pluginapi.Secrets().Put(path, &pluginapi.SecretEntry{
				Title:            "example",
				UserName:         "admin",
				Password:         "",
				URL:              "https://argocd.example.com",
				Notes:            fmt.Sprintf("Placeholder for %s. Fill in your ArgoCD details and rename this entry.", env.Name),
				CustomAttributes: requiredAttrs,
			})
			continue
		}

		// Backfill missing custom attributes on existing entries
		for _, entryPath := range existing {
			entry, err := pluginapi.Secrets().Get(entryPath)
			if err != nil || entry == nil {
				continue
			}
			if entry.CustomAttributes == nil {
				entry.CustomAttributes = make(map[string]string)
			}
			updated := false
			for attr, defaultVal := range requiredAttrs {
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
}

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

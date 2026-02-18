package main

import (
	"fmt"
	"os"
	"path/filepath"

	"omo/pkg/pluginapi"

	"gopkg.in/yaml.v3"
)

// s3ConfigHeader is prepended to the auto-generated config YAML.
const s3ConfigHeader = `# S3 Plugin Configuration
# Path: ~/.omo/configs/s3/s3.yaml
#
# Profiles are loaded from two sources:
#   1. This config file (with optional KeePass secret resolution)
#   2. Host machine's ~/.aws/credentials and ~/.aws/config
#
# KeePass Secret Schema (secret path: s3/<environment>/<name>):
#   Title    → profile name
#   URL      → S3-compatible endpoint (e.g. "https://s3.amazonaws.com")
#   UserName → AWS access key ID
#   Password → AWS secret access key
#   Custom Attributes:
#     region   → AWS region (e.g. "us-east-1")
#     role_arn → IAM role ARN for assume-role
#
# When "secret" is set, credential fields are resolved from KeePass.
# YAML values take precedence over KeePass values (override only blanks).
`

// S3Config represents the configuration for the S3 plugin
type S3Config struct {
	Profiles []S3Profile `yaml:"profiles"`
	UI       S3UIConfig  `yaml:"ui"`
}

// S3Profile represents a configured AWS profile for S3.
// When the Secret field is set (e.g. "s3/production/storage"),
// it references a KeePass entry whose fields override AccessKey,
// SecretKey, etc. at load time.
type S3Profile struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Secret      string `yaml:"secret,omitempty"` // KeePass path: pluginName/env/entryName
	Region      string `yaml:"region"`
	AccessKey   string `yaml:"access_key,omitempty"`
	SecretKey   string `yaml:"secret_key,omitempty"`
	Endpoint    string `yaml:"endpoint,omitempty"` // Custom S3-compatible endpoint
	RoleARN     string `yaml:"role_arn,omitempty"`
}

// S3UIConfig represents UI configuration options
type S3UIConfig struct {
	RefreshInterval     int    `yaml:"refresh_interval"`
	DefaultRegion       string `yaml:"default_region"`
	MaxBucketsDisplay   int    `yaml:"max_buckets_display"`
	MaxObjectsDisplay   int    `yaml:"max_objects_display"`
	EnableMetrics       bool   `yaml:"enable_metrics"`
	EnableObjectBrowser bool   `yaml:"enable_object_browser"`
	EnablePolicies      bool   `yaml:"enable_policies_viewer"`
	PageSize            int    `yaml:"page_size"`
}

// DefaultS3Config returns the default configuration
func DefaultS3Config() *S3Config {
	return &S3Config{
		Profiles: []S3Profile{},
		UI: S3UIConfig{
			RefreshInterval:     30,
			DefaultRegion:       "us-east-1",
			MaxBucketsDisplay:   100,
			MaxObjectsDisplay:   1000,
			EnableMetrics:       true,
			EnableObjectBrowser: true,
			EnablePolicies:      true,
			PageSize:            50,
		},
	}
}

// LoadS3Config loads the S3 configuration from the specified file.
// Default path: ~/.omo/configs/s3/s3.yaml
//
// After unmarshalling, any profile with a non-empty Secret field will
// have its credential fields resolved from the KeePass secrets provider.
func LoadS3Config(configPath string) (*S3Config, error) {
	if configPath == "" {
		configPath = pluginapi.PluginConfigPath("s3")
	}

	// Auto-create default config if missing
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		_ = writeDefaultConfig(configPath, s3ConfigHeader, DefaultS3Config())
	}

	// Read the configuration file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %v", err)
	}

	// Unmarshal the configuration
	config := DefaultS3Config()
	err = yaml.Unmarshal(data, config)
	if err != nil {
		return nil, fmt.Errorf("error parsing config file: %v", err)
	}

	// Init KeePass placeholder if no entries exist yet
	initS3KeePass()

	// Resolve secrets for profiles that reference KeePass entries.
	if err := resolveS3Secrets(config); err != nil {
		return nil, fmt.Errorf("error resolving secrets: %v", err)
	}

	return config, nil
}

// initS3KeePass seeds placeholder KeePass entries for S3 if none exist.
func initS3KeePass() {
	if !pluginapi.HasSecrets() {
		return
	}
	entries, err := pluginapi.Secrets().List("s3")
	if err != nil || len(entries) > 0 {
		return
	}
	_ = pluginapi.Secrets().Put("s3/default/example", &pluginapi.SecretEntry{
		Title:    "example",
		UserName: "",
		Password: "",
		URL:      "",
		Notes:    "S3 placeholder. Set UserName (Access Key ID), Password (Secret Key), URL (endpoint).",
		CustomAttributes: map[string]string{
			"region":   "us-east-1",
			"role_arn": "",
		},
	})
}

func resolveS3ProfileSecret(prof *S3Profile, entry *pluginapi.SecretEntry) {
	if prof.AccessKey == "" && entry.UserName != "" {
		prof.AccessKey = entry.UserName
	}
	if prof.SecretKey == "" && entry.Password != "" {
		prof.SecretKey = entry.Password
	}
	if prof.Name == "" && entry.Title != "" {
		prof.Name = entry.Title
	}
	if prof.Endpoint == "" && entry.URL != "" {
		prof.Endpoint = entry.URL
	}
	for attr, field := range map[string]*string{
		"region":   &prof.Region,
		"role_arn": &prof.RoleARN,
	} {
		if *field == "" {
			if v, ok := entry.CustomAttributes[attr]; ok {
				*field = v
			}
		}
	}
}

// resolveS3Secrets iterates over profiles and populates credential
// fields from the secrets provider when a secret path is defined.
func resolveS3Secrets(config *S3Config) error {
	if !pluginapi.HasSecrets() {
		return nil
	}

	for i := range config.Profiles {
		prof := &config.Profiles[i]
		if prof.Secret == "" {
			continue
		}

		entry, err := pluginapi.ResolveSecret(prof.Secret)
		if err != nil {
			return fmt.Errorf("profile %q: %w", prof.Name, err)
		}

		resolveS3ProfileSecret(prof, entry)
	}
	return nil
}

// GetAvailableS3Profiles returns the list of configured S3 profiles
func GetAvailableS3Profiles() ([]S3Profile, error) {
	config, err := LoadS3Config("")
	if err != nil {
		return nil, err
	}
	return config.Profiles, nil
}

// GetS3UIConfig returns the UI configuration
func GetS3UIConfig() (S3UIConfig, error) {
	config, err := LoadS3Config("")
	if err != nil {
		return S3UIConfig{}, err
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

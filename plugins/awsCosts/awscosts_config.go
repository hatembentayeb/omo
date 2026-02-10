package main

import (
	"fmt"
	"os"
	"path/filepath"

	"omo/pkg/pluginapi"

	"gopkg.in/yaml.v3"
)

// awsCostsConfigHeader is prepended to the auto-generated config YAML.
const awsCostsConfigHeader = `# AWS Costs Plugin Configuration
# Path: ~/.omo/configs/awsCosts/awsCosts.yaml
#
# Profiles are loaded from two sources:
#   1. This config file (with optional KeePass secret resolution)
#   2. Host machine's ~/.aws/credentials and ~/.aws/config
#
# KeePass Secret Schema (secret path: awsCosts/<environment>/<name>):
#   Title    → profile name
#   URL      → AWS region (e.g. "us-east-1")
#   UserName → AWS access key ID
#   Password → AWS secret access key
#   Custom Attributes:
#     role_arn → IAM role ARN for assume-role
#
# When "secret" is set, credential fields are resolved from KeePass.
# YAML values take precedence over KeePass values (override only blanks).
`

// AWSCostsConfig represents the configuration for the AWS Costs plugin
type AWSCostsConfig struct {
	Profiles []AWSProfile       `yaml:"profiles"`
	UI       AWSCostsUIConfig   `yaml:"ui"`
}

// AWSProfile represents a configured AWS profile.
// When the Secret field is set (e.g. "awsCosts/production/billing"),
// it references a KeePass entry whose fields override AccessKey, SecretKey, etc.
type AWSProfile struct {
	Name       string `yaml:"name"`
	Description string `yaml:"description"`
	Secret     string `yaml:"secret,omitempty"` // KeePass path: pluginName/env/entryName
	Region     string `yaml:"region"`
	AccessKey  string `yaml:"access_key,omitempty"`
	SecretKey  string `yaml:"secret_key,omitempty"`
	RoleARN    string `yaml:"role_arn,omitempty"`
}

// AWSCostsUIConfig represents UI configuration options
type AWSCostsUIConfig struct {
	RefreshInterval int    `yaml:"refresh_interval"`
	DefaultTimeRange string `yaml:"default_time_range"`
	DefaultGranularity string `yaml:"default_granularity"`
	DefaultRegion   string `yaml:"default_region"`
	EnableBudgets   bool   `yaml:"enable_budgets"`
	EnableForecasts bool   `yaml:"enable_forecasts"`
}

// DefaultAWSCostsConfig returns the default configuration
func DefaultAWSCostsConfig() *AWSCostsConfig {
	return &AWSCostsConfig{
		Profiles: []AWSProfile{},
		UI: AWSCostsUIConfig{
			RefreshInterval:    300,
			DefaultTimeRange:   "LAST_30_DAYS",
			DefaultGranularity: "DAILY",
			DefaultRegion:      "us-east-1",
			EnableBudgets:      true,
			EnableForecasts:    true,
		},
	}
}

// LoadAWSCostsConfig loads the AWS Costs configuration from the specified file.
// Default path: ~/.omo/configs/awsCosts/awsCosts.yaml
//
// After unmarshalling, any profile with a non-empty Secret field will
// have its credential fields resolved from the KeePass secrets provider.
func LoadAWSCostsConfig(configPath string) (*AWSCostsConfig, error) {
	if configPath == "" {
		configPath = pluginapi.PluginConfigPath("awsCosts")
	}

	// Auto-create default config if missing
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		_ = writeDefaultConfig(configPath, awsCostsConfigHeader, DefaultAWSCostsConfig())
	}

	// Read the configuration file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %v", err)
	}

	// Unmarshal the configuration
	config := DefaultAWSCostsConfig()
	err = yaml.Unmarshal(data, config)
	if err != nil {
		return nil, fmt.Errorf("error parsing config file: %v", err)
	}

	// Init KeePass placeholder if no entries exist yet
	initAWSCostsKeePass()

	// Resolve secrets for profiles that reference KeePass entries.
	if err := resolveAWSCostsSecrets(config); err != nil {
		return nil, fmt.Errorf("error resolving secrets: %v", err)
	}

	return config, nil
}

// initAWSCostsKeePass seeds placeholder KeePass entries for AWS Costs if none exist.
func initAWSCostsKeePass() {
	if !pluginapi.HasSecrets() {
		return
	}
	entries, err := pluginapi.Secrets().List("awsCosts")
	if err != nil || len(entries) > 0 {
		return
	}
	_ = pluginapi.Secrets().Put("awsCosts/default/example", &pluginapi.SecretEntry{
		Title:    "example",
		UserName: "",
		Password: "",
		URL:      "us-east-1",
		Notes:    "AWS Costs placeholder. Set UserName (Access Key ID), Password (Secret Key), URL (region).",
		CustomAttributes: map[string]string{
			"role_arn": "",
		},
	})
}

// resolveAWSCostsSecrets iterates over profiles and populates credential
// fields from the secrets provider when a secret path is defined.
func resolveAWSCostsSecrets(config *AWSCostsConfig) error {
	if !pluginapi.HasSecrets() {
		return nil // no provider — skip silently
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

		// Override only blank fields so YAML values take precedence.
		if prof.AccessKey == "" && entry.UserName != "" {
			prof.AccessKey = entry.UserName
		}
		if prof.SecretKey == "" && entry.Password != "" {
			prof.SecretKey = entry.Password
		}
		if prof.Name == "" && entry.Title != "" {
			prof.Name = entry.Title
		}
		if prof.Region == "" && entry.URL != "" {
			prof.Region = entry.URL
		}
		// Custom attributes
		if prof.RoleARN == "" {
			if v, ok := entry.CustomAttributes["role_arn"]; ok {
				prof.RoleARN = v
			}
		}
	}
	return nil
}

// GetAvailableAWSProfiles returns the list of configured AWS profiles
func GetAvailableAWSProfiles() ([]AWSProfile, error) {
	config, err := LoadAWSCostsConfig("")
	if err != nil {
		return nil, err
	}
	return config.Profiles, nil
}

// GetAWSCostsUIConfig returns the UI configuration
func GetAWSCostsUIConfig() (AWSCostsUIConfig, error) {
	config, err := LoadAWSCostsConfig("")
	if err != nil {
		return AWSCostsUIConfig{}, err
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

package main

import (
	"fmt"
	"sort"
	"strings"

	"omo/pkg/pluginapi"
)

var defaultAWSEnvironments = []string{
	"development",
	"production",
	"staging",
	"sandbox",
	"local",
	"test",
}

// AWSProfile is built entirely from a KeePass entry at runtime.
//
// KeePass Entry Schema (path: awsCosts/<environment>/<name>):
//
//	Title    → profile display name
//	URL      → AWS region (e.g. "us-east-1")
//	UserName → AWS access key ID
//	Password → AWS secret access key
//	Notes    → description / notes
//
//	Custom Attributes:
//	  role_arn → IAM role ARN for assume-role
//	  tags    → comma-separated tags
type AWSProfile struct {
	Name        string
	Description string
	Environment string
	Region      string
	AccessKey   string
	SecretKey   string
	RoleARN     string
	Tags        []string
}

// AWSCostsUIConfig holds hardcoded UI defaults for the AWS Costs plugin.
type AWSCostsUIConfig struct {
	RefreshInterval    int
	DefaultTimeRange   string
	DefaultGranularity string
	DefaultRegion      string
	EnableBudgets      bool
	EnableForecasts    bool
}

func DefaultAWSCostsUIConfig() AWSCostsUIConfig {
	return AWSCostsUIConfig{
		RefreshInterval:    300,
		DefaultTimeRange:   "LAST_30_DAYS",
		DefaultGranularity: "DAILY",
		DefaultRegion:      "us-east-1",
		EnableBudgets:      true,
		EnableForecasts:    true,
	}
}

// DiscoverProfiles reads KeePass groups under "awsCosts/" and builds
// AWSProfile objects from the entries.
func DiscoverProfiles() ([]AWSProfile, error) {
	if !pluginapi.HasSecrets() {
		return nil, fmt.Errorf("secrets provider not available")
	}

	if err := pluginapi.Secrets().Reload(); err != nil {
		return nil, fmt.Errorf("reload secrets: %w", err)
	}

	ensureAWSCostsKeePassGroups()

	paths, err := pluginapi.Secrets().List("awsCosts")
	if err != nil {
		return nil, fmt.Errorf("list awsCosts secrets: %w", err)
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("no AWS Costs entries in KeePass (create entries under awsCosts/<environment>/<name>)")
	}

	var profiles []AWSProfile
	for _, path := range paths {
		env := extractEnvironment(path)
		if env == "" {
			continue
		}

		entry, err := pluginapi.Secrets().Get(path)
		if err != nil {
			continue
		}

		prof := entryToAWSProfile(entry, env)
		profiles = append(profiles, prof)
	}

	sort.Slice(profiles, func(i, j int) bool {
		if profiles[i].Environment != profiles[j].Environment {
			return profiles[i].Environment < profiles[j].Environment
		}
		return profiles[i].Name < profiles[j].Name
	})

	return profiles, nil
}

func extractEnvironment(path string) string {
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 3 {
		return ""
	}
	return parts[1]
}

func entryToAWSProfile(entry *pluginapi.SecretEntry, env string) AWSProfile {
	prof := AWSProfile{
		Name:        entry.Title,
		Description: entry.Notes,
		Environment: env,
		AccessKey:   entry.UserName,
		SecretKey:   entry.Password,
		Region:      entry.URL,
	}

	if entry.CustomAttributes == nil {
		return prof
	}

	ca := entry.CustomAttributes

	if v, ok := ca["role_arn"]; ok {
		prof.RoleARN = v
	}
	if v, ok := ca["tags"]; ok {
		for _, t := range strings.Split(v, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				prof.Tags = append(prof.Tags, t)
			}
		}
	}

	return prof
}

func ensureAWSCostsKeePassGroups() {
	if !pluginapi.HasSecrets() {
		return
	}

	requiredAttrs := map[string]string{
		"role_arn": "",
		"tags":     "",
	}

	for _, env := range defaultAWSEnvironments {
		prefix := fmt.Sprintf("awsCosts/%s", env)
		existing, err := pluginapi.Secrets().List(prefix)
		if err == nil && len(existing) > 0 {
			backfillAttributes(existing, requiredAttrs)
			continue
		}
		path := fmt.Sprintf("awsCosts/%s/example", env)
		_ = pluginapi.Secrets().Put(path, &pluginapi.SecretEntry{
			Title:            "example",
			UserName:         "",
			Password:         "",
			URL:              "us-east-1",
			Notes:            fmt.Sprintf("AWS Costs %s placeholder. Set UserName (Access Key ID), Password (Secret Key), URL (region).", env),
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

// GetAvailableAWSProfiles returns all discovered AWS profiles.
func GetAvailableAWSProfiles() ([]AWSProfile, error) {
	return DiscoverProfiles()
}

// GetAWSCostsUIConfig returns the hardcoded UI configuration.
func GetAWSCostsUIConfig() (AWSCostsUIConfig, error) {
	return DefaultAWSCostsUIConfig(), nil
}

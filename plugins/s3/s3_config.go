package main

import (
	"fmt"
	"sort"
	"strings"

	"omo/pkg/pluginapi"
)

var defaultS3Environments = []string{
	"development",
	"production",
	"staging",
	"sandbox",
	"local",
	"test",
}

// S3Profile is built entirely from a KeePass entry at runtime.
//
// KeePass Entry Schema (path: s3/<environment>/<name>):
//
//	Title    → profile display name
//	URL      → S3-compatible endpoint (e.g. "https://s3.amazonaws.com")
//	UserName → AWS access key ID
//	Password → AWS secret access key
//	Notes    → description / notes
//
//	Custom Attributes:
//	  region   → AWS region (e.g. "us-east-1")
//	  role_arn → IAM role ARN for assume-role
//	  tags     → comma-separated tags
type S3Profile struct {
	Name        string
	Description string
	Environment string
	Region      string
	AccessKey   string
	SecretKey   string
	Endpoint    string
	RoleARN     string
	Tags        []string
}

// S3UIConfig holds hardcoded UI defaults for the S3 plugin.
type S3UIConfig struct {
	RefreshInterval     int
	DefaultRegion       string
	MaxBucketsDisplay   int
	MaxObjectsDisplay   int
	EnableMetrics       bool
	EnableObjectBrowser bool
	EnablePolicies      bool
	PageSize            int
}

func DefaultS3UIConfig() S3UIConfig {
	return S3UIConfig{
		RefreshInterval:     30,
		DefaultRegion:       "us-east-1",
		MaxBucketsDisplay:   100,
		MaxObjectsDisplay:   1000,
		EnableMetrics:       true,
		EnableObjectBrowser: true,
		EnablePolicies:      true,
		PageSize:            50,
	}
}

// DiscoverProfiles reads KeePass groups under "s3/" and builds
// S3Profile objects from the entries.
func DiscoverProfiles() ([]S3Profile, error) {
	if !pluginapi.HasSecrets() {
		return nil, fmt.Errorf("secrets provider not available")
	}

	if err := pluginapi.Secrets().Reload(); err != nil {
		return nil, fmt.Errorf("reload secrets: %w", err)
	}

	ensureS3KeePassGroups()

	paths, err := pluginapi.Secrets().List("s3")
	if err != nil {
		return nil, fmt.Errorf("list s3 secrets: %w", err)
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("no S3 entries in KeePass (create entries under s3/<environment>/<name>)")
	}

	var profiles []S3Profile
	for _, path := range paths {
		env := extractEnvironment(path)
		if env == "" {
			continue
		}

		entry, err := pluginapi.Secrets().Get(path)
		if err != nil {
			continue
		}

		prof := entryToS3Profile(entry, env)
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

func entryToS3Profile(entry *pluginapi.SecretEntry, env string) S3Profile {
	prof := S3Profile{
		Name:        entry.Title,
		Description: entry.Notes,
		Environment: env,
		AccessKey:   entry.UserName,
		SecretKey:   entry.Password,
		Endpoint:    entry.URL,
	}

	if entry.CustomAttributes == nil {
		return prof
	}

	ca := entry.CustomAttributes

	if v, ok := ca["region"]; ok && v != "" {
		prof.Region = v
	}
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

func ensureS3KeePassGroups() {
	if !pluginapi.HasSecrets() {
		return
	}

	requiredAttrs := map[string]string{
		"region":   "us-east-1",
		"role_arn": "",
		"tags":     "",
	}

	for _, env := range defaultS3Environments {
		prefix := fmt.Sprintf("s3/%s", env)
		existing, err := pluginapi.Secrets().List(prefix)
		if err == nil && len(existing) > 0 {
			backfillAttributes(existing, requiredAttrs)
			continue
		}
		path := fmt.Sprintf("s3/%s/example", env)
		_ = pluginapi.Secrets().Put(path, &pluginapi.SecretEntry{
			Title:            "example",
			UserName:         "",
			Password:         "",
			URL:              "",
			Notes:            fmt.Sprintf("S3 %s placeholder. Set UserName (Access Key ID), Password (Secret Key), URL (endpoint).", env),
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

// GetAvailableS3Profiles returns all discovered S3 profiles.
func GetAvailableS3Profiles() ([]S3Profile, error) {
	return DiscoverProfiles()
}

// GetS3UIConfig returns the hardcoded UI configuration.
func GetS3UIConfig() (S3UIConfig, error) {
	return DefaultS3UIConfig(), nil
}

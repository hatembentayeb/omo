package main

import (
	"time"
)

// PluginMetadata defines metadata for OhmyopsPlugin
type PluginMetadata struct {
	Name        string    // Name of the plugin
	Version     string    // Version of the plugin
	Description string    // Short description of the plugin
	Author      string    // Author of the plugin
	License     string    // License of the plugin
	Tags        []string  // Tags for categorizing the plugin
	Arch        []string  // Supported architectures
	LastUpdated time.Time // Last update time
	URL         string    // URL to the plugin repository or documentation
}

// PluginInfoProvider extends the OhmyopsPlugin interface to include metadata
type PluginInfoProvider interface {
	GetMetadata() PluginMetadata
}

// GlobalPluginRegistry stores information about all installed plugins
var GlobalPluginRegistry map[string]PluginMetadata

func init() {
	GlobalPluginRegistry = make(map[string]PluginMetadata)

	// Initialize with demo metadata for existing plugins
	now := time.Now()

	// Kafka plugin metadata
	kafkaMetadata := PluginMetadata{
		Name:        "kafka",
		Version:     "1.5.0",
		Description: "Kafka management plugin",
		Author:      "HATMAN",
		License:     "MIT",
		Tags:        []string{"messaging", "streaming", "broker"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: now.AddDate(0, -1, 0), // 1 month ago
		URL:         "https://github.com/hatembentayeb/ohmyops-v2/plugins/kafka",
	}
	GlobalPluginRegistry["kafka"] = kafkaMetadata

	// S3 plugin metadata
	s3Metadata := PluginMetadata{
		Name:        "s3",
		Version:     "1.2.0",
		Description: "AWS S3 management plugin",
		Author:      "HATMAN",
		License:     "Apache-2.0",
		Tags:        []string{"storage", "cloud", "aws"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: now.AddDate(0, -2, 0), // 2 months ago
		URL:         "https://github.com/hatembentayeb/ohmyops-v2/plugins/s3",
	}
	GlobalPluginRegistry["s3"] = s3Metadata
}

// RegisterPlugin adds a plugin to the global registry
func RegisterPlugin(name string, metadata PluginMetadata) {
	GlobalPluginRegistry[name] = metadata
}

// GetPluginMetadata retrieves plugin metadata from the registry
func GetPluginMetadata(name string) (PluginMetadata, bool) {
	metadata, exists := GlobalPluginRegistry[name]
	return metadata, exists
}

// GetAllPluginsMetadata returns all registered plugins metadata
func GetAllPluginsMetadata() []PluginMetadata {
	plugins := make([]PluginMetadata, 0, len(GlobalPluginRegistry))
	for _, metadata := range GlobalPluginRegistry {
		plugins = append(plugins, metadata)
	}
	return plugins
}

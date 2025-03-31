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
	// The registry will be populated when plugins are loaded
	// No hardcoded metadata here - it will come from the plugins themselves
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

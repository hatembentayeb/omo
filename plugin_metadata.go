package main

import (
	"time"
)

// PluginMetadata defines metadata for OhmyopsPlugin
// This struct contains all information needed to identify and describe a plugin
// in the OMO (Oh My Ops) system.
type PluginMetadata struct {
	Name        string    // Name of the plugin, used as a unique identifier
	Version     string    // Version of the plugin in semver format
	Description string    // Short description explaining the plugin's functionality
	Author      string    // Author or organization that created the plugin
	License     string    // License under which the plugin is distributed
	Tags        []string  // Tags for categorizing and filtering plugins
	Arch        []string  // Supported CPU architectures (e.g., "amd64", "arm64")
	LastUpdated time.Time // Last update timestamp of the plugin
	URL         string    // URL to the plugin repository or documentation
}

// PluginInfoProvider extends the OhmyopsPlugin interface to include metadata
// This interface allows plugins to self-describe by providing their metadata
// to the main application.
type PluginInfoProvider interface {
	GetMetadata() PluginMetadata
}

// GlobalPluginRegistry stores information about all installed plugins
// This map uses the plugin name as the key and stores corresponding metadata
var GlobalPluginRegistry map[string]PluginMetadata

// init initializes the plugin registry at application startup
// Creates an empty map that will be populated when plugins are loaded
func init() {
	GlobalPluginRegistry = make(map[string]PluginMetadata)
	// The registry will be populated when plugins are loaded
	// No hardcoded metadata here - it will come from the plugins themselves
}

// RegisterPlugin adds a plugin to the global registry
// This function is called during plugin loading to register metadata
// Parameters:
//   - name: unique identifier for the plugin
//   - metadata: structured information about the plugin
func RegisterPlugin(name string, metadata PluginMetadata) {
	GlobalPluginRegistry[name] = metadata
}

// GetPluginMetadata retrieves plugin metadata from the registry
// Parameters:
//   - name: unique identifier of the plugin to retrieve
//
// Returns:
//   - metadata: the plugin's metadata structure
//   - exists: boolean indicating whether the plugin was found
func GetPluginMetadata(name string) (PluginMetadata, bool) {
	metadata, exists := GlobalPluginRegistry[name]
	return metadata, exists
}

// GetAllPluginsMetadata returns all registered plugins metadata
// Returns:
//   - plugins: slice containing metadata for all registered plugins
func GetAllPluginsMetadata() []PluginMetadata {
	plugins := make([]PluginMetadata, 0, len(GlobalPluginRegistry))
	for _, metadata := range GlobalPluginRegistry {
		plugins = append(plugins, metadata)
	}
	return plugins
}

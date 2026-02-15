package registry

import (
	"omo/pkg/pluginapi"
)

// GlobalPluginRegistry stores information about all installed plugins
// This map uses the plugin name as the key and stores corresponding metadata
var GlobalPluginRegistry map[string]pluginapi.PluginMetadata

// init initializes the plugin registry at application startup
// Creates an empty map that will be populated when plugins are loaded
func init() {
	GlobalPluginRegistry = make(map[string]pluginapi.PluginMetadata)
	// The registry will be populated when plugins are loaded
	// No hardcoded metadata here - it will come from the plugins themselves
}

// RegisterPlugin adds a plugin to the global registry
// This function is called during plugin loading to register metadata
// Parameters:
//   - name: unique identifier for the plugin
//   - metadata: structured information about the plugin
func RegisterPlugin(name string, metadata pluginapi.PluginMetadata) {
	GlobalPluginRegistry[name] = metadata
}

// GetPluginMetadata retrieves plugin metadata from the registry
// Parameters:
//   - name: unique identifier of the plugin to retrieve
//
// Returns:
//   - metadata: the plugin's metadata structure
//   - exists: boolean indicating whether the plugin was found
func GetPluginMetadata(name string) (pluginapi.PluginMetadata, bool) {
	metadata, exists := GlobalPluginRegistry[name]
	return metadata, exists
}

// GetAllPluginsMetadata returns all registered plugins metadata
// Returns:
//   - plugins: slice containing metadata for all registered plugins
func GetAllPluginsMetadata() []pluginapi.PluginMetadata {
	plugins := make([]pluginapi.PluginMetadata, 0, len(GlobalPluginRegistry))
	for _, metadata := range GlobalPluginRegistry {
		plugins = append(plugins, metadata)
	}
	return plugins
}

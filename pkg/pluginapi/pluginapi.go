package pluginapi

import (
	"time"

	"github.com/rivo/tview"
)

// PluginMetadata defines metadata for OhMyOps plugins.
// This struct is shared between the host and plugins.
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

// Plugin is the minimal interface every plugin must implement.
type Plugin interface {
	Start(*tview.Application) tview.Primitive
	GetMetadata() PluginMetadata
}

// Stoppable is an optional lifecycle hook for plugins that need cleanup.
type Stoppable interface {
	Stop()
}

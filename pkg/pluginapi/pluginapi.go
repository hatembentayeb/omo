package pluginapi

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/rivo/tview"
)

// OmoHome is the root directory for all omo data under the user's home.
// Layout:
//
//	~/.omo/
//	├── plugins/<name>/<name>.so     ← compiled plugin shared libraries
//	├── configs/<name>/<name>.yaml   ← per-plugin configuration files
//	├── secrets/omo.kdbx             ← KeePass secrets database
//	└── keys/omo.key                 ← KeePass key file (auto-generated)
const OmoHome = ".omo"

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

// OmoDir returns the absolute path to ~/.omo.
// It panics if the user home directory cannot be resolved, which should
// never happen on a properly configured system.
func OmoDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		panic("cannot resolve user home directory: " + err.Error())
	}
	return filepath.Join(home, OmoHome)
}

// PluginsDir returns the absolute path to ~/.omo/plugins.
func PluginsDir() string {
	return filepath.Join(OmoDir(), "plugins")
}

// ConfigsDir returns the absolute path to ~/.omo/configs.
func ConfigsDir() string {
	return filepath.Join(OmoDir(), "configs")
}

// PluginConfigPath returns the config file path for a given plugin name.
// e.g. PluginConfigPath("redis") → ~/.omo/configs/redis/redis.yaml
func PluginConfigPath(pluginName string) string {
	return filepath.Join(ConfigsDir(), pluginName, pluginName+".yaml")
}

// PluginSOPath returns the shared library path for a given plugin name.
// e.g. PluginSOPath("redis") → ~/.omo/plugins/redis/redis.so
func PluginSOPath(pluginName string) string {
	return filepath.Join(PluginsDir(), pluginName, pluginName+".so")
}

// NewHTTPClient returns an http.Client that forces IPv4 connections.
// Some environments (notably Termux on Android) advertise IPv6 but fail
// to route it, causing "dial tcp [::1]:443: connect: connection refused"
// errors when contacting GitHub. Forcing "tcp4" avoids this.
func NewHTTPClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.DialContext(ctx, "tcp4", addr)
		},
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}

// EnsurePluginDirs creates the plugin and config directories for a given plugin.
func EnsurePluginDirs(pluginName string) error {
	dirs := []string{
		filepath.Join(PluginsDir(), pluginName),
		filepath.Join(ConfigsDir(), pluginName),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}

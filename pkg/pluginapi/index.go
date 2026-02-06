package pluginapi

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// DefaultIndexURL is the URL to the official OhMyOps plugin index.
const DefaultIndexURL = "https://raw.githubusercontent.com/ohmyops/omo/main/index.yaml"

// IndexEntry represents a single plugin in the index.
type IndexEntry struct {
	Name        string   `yaml:"name"`
	Version     string   `yaml:"version"`
	Description string   `yaml:"description"`
	Author      string   `yaml:"author"`
	License     string   `yaml:"license"`
	URL         string   `yaml:"url"`
	Tags        []string `yaml:"tags"`
	Arch        []string `yaml:"arch"`
}

// PluginIndex represents the full index file.
type PluginIndex struct {
	APIVersion          string       `yaml:"api_version"`
	DownloadURLTemplate string       `yaml:"download_url_template"`
	Plugins             []IndexEntry `yaml:"plugins"`
}

// DownloadURL returns the download URL for a specific plugin entry,
// resolved for the current OS and architecture.
func (e *IndexEntry) DownloadURL(template string) string {
	r := strings.NewReplacer(
		"{{name}}", e.Name,
		"{{version}}", e.Version,
		"{{os}}", runtime.GOOS,
		"{{arch}}", runtime.GOARCH,
	)
	return r.Replace(template)
}

// FetchIndex downloads and parses the plugin index from the given URL.
// If url is empty, uses DefaultIndexURL.
func FetchIndex(url string) (*PluginIndex, error) {
	if url == "" {
		url = DefaultIndexURL
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch index: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("index returned HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read index body: %w", err)
	}

	return ParseIndex(data)
}

// ParseIndex parses raw YAML bytes into a PluginIndex.
func ParseIndex(data []byte) (*PluginIndex, error) {
	var idx PluginIndex
	if err := yaml.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("failed to parse index: %w", err)
	}
	return &idx, nil
}

// LoadLocalIndex reads the cached index from ~/.omo/index.yaml.
// Returns nil (no error) if the file doesn't exist yet.
func LoadLocalIndex() (*PluginIndex, error) {
	path := filepath.Join(OmoDir(), "index.yaml")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return ParseIndex(data)
}

// SaveLocalIndex caches the index to ~/.omo/index.yaml.
func SaveLocalIndex(idx *PluginIndex) error {
	data, err := yaml.Marshal(idx)
	if err != nil {
		return err
	}
	dir := OmoDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "index.yaml"), data, 0644)
}

// IsInstalled checks if a plugin is installed locally.
func IsInstalled(pluginName string) bool {
	_, err := os.Stat(PluginSOPath(pluginName))
	return err == nil
}

// InstalledVersion attempts to read the version of an installed plugin
// by loading its .so and calling GetMetadata(). Returns empty string if
// not installed or version cannot be determined.
func InstalledVersion(pluginName string) string {
	// We can't load .so here without the plugin package (circular),
	// so we'll store installed versions in a manifest.
	manifest := loadManifest()
	if v, ok := manifest[pluginName]; ok {
		return v
	}
	return ""
}

// RecordInstalledVersion saves the version of an installed plugin.
func RecordInstalledVersion(pluginName, version string) error {
	manifest := loadManifest()
	manifest[pluginName] = version
	return saveManifest(manifest)
}

// RemoveInstalledRecord removes a plugin from the installed manifest.
func RemoveInstalledRecord(pluginName string) error {
	manifest := loadManifest()
	delete(manifest, pluginName)
	return saveManifest(manifest)
}

// manifest is a simple name→version map stored as YAML.
func manifestPath() string {
	return filepath.Join(OmoDir(), "installed.yaml")
}

func loadManifest() map[string]string {
	m := make(map[string]string)
	data, err := os.ReadFile(manifestPath())
	if err != nil {
		return m
	}
	yaml.Unmarshal(data, &m)
	return m
}

func saveManifest(m map[string]string) error {
	data, err := yaml.Marshal(m)
	if err != nil {
		return err
	}
	return os.WriteFile(manifestPath(), data, 0644)
}

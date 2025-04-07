package main

import (
	"os"
	"path/filepath"
	"plugin"
	"strconv"
	"time"

	"omo/ui"

	"github.com/rivo/tview"
)

// NewPackageManager creates and returns a configured package manager UI component
func NewPackageManager(app *tview.Application, pages *tview.Pages) *ui.Cores {
	// Create a new Cores UI component
	core := ui.NewCores(app, "Package Manager")

	// Set up table headers and data
	core.SetTableHeaders([]string{"Name", "Version", "Latest", "Status", "Description"})
	core.SetSelectionKey("Name")

	// Add key bindings - using uppercase letters and Title case descriptions
	core.AddKeyBinding("I", "Install", nil)
	core.AddKeyBinding("U", "Update", nil)
	core.AddKeyBinding("R", "Remove", nil)
	core.AddKeyBinding("Z", "Updateall", nil)
	core.AddKeyBinding("Q", "Back", nil)

	// Set refresh callback to load plugin data
	core.SetRefreshCallback(func() ([][]string, error) {
		// First, make sure we load all available plugins from the plugins directory
		loadAllPluginsMetadata()

		// Get actual plugin metadata from the global registry
		plugins := GetAllPluginsMetadata()
		pluginData := make([][]string, 0, len(plugins))

		// Add metadata for each plugin
		for _, metadata := range plugins {
			// Determine status (installed or not)
			status := "Not Installed"
			_, err := os.Stat(filepath.Join(pluginsDir, metadata.Name))
			if err == nil {
				status = "Installed"
			}

			// Add the plugin data
			pluginData = append(pluginData, []string{
				metadata.Name,
				metadata.Version,
				metadata.Version, // Use same version for "latest" for now
				status,
				metadata.Description,
			})
		}

		// Update info panel with stats
		updateInfoPanel(core, pluginData)

		return pluginData, nil
	})

	// Set action callback to handle user actions
	core.SetActionCallback(func(action string, payload map[string]interface{}) error {
		// Handle different actions
		switch action {
		case "rowSelected":
			// Log which plugin was selected
			if namedData, ok := payload["namedData"].(map[string]string); ok {
				pluginName := namedData["Name"]
				core.Log("Selected plugin: " + pluginName)
			}

		case "keypress":
			// Handle specific key presses
			if key, ok := payload["key"].(string); ok {
				switch key {
				case "I":
					// Install logic
					if rowData := core.GetSelectedRowData(); rowData != nil {
						pluginName := rowData[0]
						status := rowData[3]
						if status == "Not Installed" {
							// Simulate installation
							core.Log("Installing plugin: " + pluginName + "...")
							time.Sleep(500 * time.Millisecond)
							core.Log("[green]Plugin installed successfully: " + pluginName)
							core.RefreshData()
						} else {
							core.Log("[yellow]Plugin already installed: " + pluginName)
						}
					} else {
						core.Log("[red]No plugin selected")
					}

				case "U":
					// Update logic
					if rowData := core.GetSelectedRowData(); rowData != nil {
						pluginName := rowData[0]
						currentVer := rowData[1]
						latestVer := rowData[2]
						status := rowData[3]

						if status == "Installed" && currentVer != latestVer {
							// Simulate update
							core.Log("Updating plugin: " + pluginName + " from " + currentVer + " to " + latestVer + "...")
							time.Sleep(500 * time.Millisecond)
							core.Log("[green]Plugin updated successfully: " + pluginName)
							core.RefreshData()
						} else if status != "Installed" {
							core.Log("[yellow]Plugin not installed: " + pluginName)
						} else {
							core.Log("[yellow]Plugin already up to date: " + pluginName)
						}
					} else {
						core.Log("[red]No plugin selected")
					}

				case "R":
					// Remove logic
					if rowData := core.GetSelectedRowData(); rowData != nil {
						pluginName := rowData[0]
						status := rowData[3]

						if status == "Installed" {
							// Simulate removal
							core.Log("Removing plugin: " + pluginName + "...")
							time.Sleep(500 * time.Millisecond)
							core.Log("[green]Plugin removed successfully: " + pluginName)
							core.RefreshData()
						} else {
							core.Log("[yellow]Plugin not installed: " + pluginName)
						}
					} else {
						core.Log("[red]No plugin selected")
					}

				case "Z":
					// Update all plugins
					core.Log("Updating all plugins...")
					time.Sleep(1 * time.Second)
					core.Log("[green]All plugins updated successfully")
					core.RefreshData()

				case "Q":
					// Quit/back to main UI
					core.Log("Exiting package manager...")
					// Return to the main screen
					core.UnregisterHandlers() // Remove key handlers
					core.StopAutoRefresh()    // Stop background refresh
					pages.SwitchToPage("main")
				}
			}
		}

		return nil
	})

	// Initialize UI by triggering a refresh
	core.RefreshData()

	// Register key handlers
	core.RegisterHandlers()

	// Start auto-refresh
	core.StartAutoRefresh(30 * time.Second)

	return core
}

// updateInfoPanel updates the info panel with current stats
func updateInfoPanel(core *ui.Cores, data [][]string) {
	// Count stats
	total := len(data)
	installed := 0
	updates := 0

	for _, plugin := range data {
		if plugin[3] == "Installed" {
			installed++
			if plugin[1] != plugin[2] {
				updates++
			}
		}
	}

	available := total - installed

	// Format the info panel text
	infoText :=
		"[aqua::b]Total Plugins:[white::b] " +
			strconv.Itoa(total) + "\n" +
			"[aqua::b]Installed:[white::b] " +
			strconv.Itoa(installed) + "\n" +
			"[aqua::b]Available:[white::b] " +
			strconv.Itoa(available) + "\n" +
			"[aqua::b]Updates:[yellow::b] " +
			strconv.Itoa(updates) + "\n" +
			"[aqua::b]Last Check:[white::b] " +
			time.Now().Format("15:04:05")

	core.SetInfoText(infoText)
}

// loadAllPluginsMetadata scans the plugins directory and loads metadata from all plugin files
// This function:
// 1. Reads all files in the plugins directory
// 2. Attempts to load each file as a Go plugin
// 3. Extracts metadata from plugins using either the GetMetadata function or OhmyopsPlugin interface
// 4. Registers valid plugins in the GlobalPluginRegistry
func loadAllPluginsMetadata() {
	files, err := os.ReadDir(pluginsDir)
	if err != nil {
		return
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		pluginName := file.Name()
		pluginPath := filepath.Join(pluginsDir, pluginName)

		// Skip if already loaded
		if _, exists := GetPluginMetadata(pluginName); exists {
			continue
		}

		// Try to load the plugin
		p, err := plugin.Open(pluginPath)
		if err != nil {
			continue
		}

		// Try to get metadata directly via GetMetadata function
		if metadataFunc, err := p.Lookup("GetMetadata"); err == nil {
			if getter, ok := metadataFunc.(func() interface{}); ok {
				if rawMetadata := getter(); rawMetadata != nil {
					// Try to extract fields from the metadata
					if m, ok := rawMetadata.(map[string]interface{}); ok {
						metadata := PluginMetadata{
							Name: pluginName,
						}

						// Extract all available fields from map to metadata struct
						if name, ok := m["Name"].(string); ok {
							metadata.Name = name
						}
						if version, ok := m["Version"].(string); ok {
							metadata.Version = version
						}
						if description, ok := m["Description"].(string); ok {
							metadata.Description = description
						}
						if author, ok := m["Author"].(string); ok {
							metadata.Author = author
						}
						if license, ok := m["License"].(string); ok {
							metadata.License = license
						}
						if tags, ok := m["Tags"].([]string); ok {
							metadata.Tags = tags
						}
						if arch, ok := m["Arch"].([]string); ok {
							metadata.Arch = arch
						}
						if url, ok := m["URL"].(string); ok {
							metadata.URL = url
						}
						if lastUpdated, ok := m["LastUpdated"].(time.Time); ok {
							metadata.LastUpdated = lastUpdated
						} else {
							metadata.LastUpdated = time.Now() // Default to now
						}

						// Register the plugin metadata
						RegisterPlugin(pluginName, metadata)
					}
				}
			}
			continue
		}

		// Alternative: try to get via the OhmyopsPlugin interface
		if pluginSymbol, err := p.Lookup("OhmyopsPlugin"); err == nil {
			if ohmyopsPlugin, ok := pluginSymbol.(OhmyopsPlugin); ok {
				metadata := ohmyopsPlugin.GetMetadata()
				RegisterPlugin(pluginName, metadata)
			}
		}
	}
}

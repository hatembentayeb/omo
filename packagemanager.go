package main

import (
	"os"
	"path/filepath"
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
		// Get actual plugin metadata from the global registry
		plugins := GetAllPluginsMetadata()
		pluginData := make([][]string, 0, len(plugins))

		// If we have real metadata available, use it
		if len(plugins) > 0 {
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
		} else {
			// Fallback to sample data if no real metadata is available
			pluginData = [][]string{
				{"redis", "1.0.0", "1.2.0", "Installed", "Redis management plugin"},
				{"postgres", "0.5.0", "0.5.0", "Not Installed", "PostgreSQL management plugin"},
				{"mongodb", "1.2.0", "1.2.0", "Installed", "MongoDB management plugin"},
				{"elasticsearch", "0.8.0", "1.0.0", "Not Installed", "Elasticsearch management plugin"},
				{"mysql", "2.1.0", "2.1.0", "Installed", "MySQL management plugin"},
				{"kafka", "1.5.0", "1.8.0", "Installed", "Kafka management plugin"},
				{"rabbitmq", "1.0.0", "1.0.0", "Not Installed", "RabbitMQ management plugin"},
				{"nginx", "1.2.0", "1.2.0", "Installed", "Nginx management plugin"},
				{"haproxy", "0.9.0", "1.1.0", "Installed", "HAProxy management plugin"},
				{"prometheus", "1.0.0", "1.0.0", "Not Installed", "Prometheus management plugin"},
				{"grafana", "2.0.0", "2.0.0", "Installed", "Grafana management plugin"},
				{"jenkins", "1.5.0", "1.5.0", "Not Installed", "Jenkins management plugin"},
				{"docker", "1.8.0", "1.8.0", "Installed", "Docker management plugin"},
				{"kubernetes", "1.0.0", "1.2.0", "Installed", "Kubernetes management plugin"},
				{"vault", "0.8.0", "0.8.0", "Not Installed", "Vault management plugin"},
			}
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
			string('0'+total) + "\n" +
			"[aqua::b]Installed:[white::b] " +
			string('0'+installed) + "\n" +
			"[aqua::b]Available:[white::b] " +
			string('0'+available) + "\n" +
			"[aqua::b]Updates:[yellow::b] " +
			string('0'+updates) + "\n" +
			"[aqua::b]Last Check:[white::b] " +
			time.Now().Format("15:04:05")

	core.SetInfoText(infoText)
}

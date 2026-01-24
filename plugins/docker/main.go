package main

import (
	"time"

	"omo/pkg/pluginapi"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// DockerPlugin represents the Docker management plugin
type DockerPlugin struct {
	Name        string
	Description string
	dockerView  *DockerView
}

// Start initializes and starts the Docker plugin UI
func (d *DockerPlugin) Start(app *tview.Application) tview.Primitive {
	// Initialize the Docker view
	pages := tview.NewPages()
	dockerView := NewDockerView(app, pages)
	d.dockerView = dockerView

	// Get the main UI component
	mainUI := dockerView.GetMainUI()

	// Add keyboard handling to the pages
	pages.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Check for specific Docker shortcuts
		if event.Key() == tcell.KeyCtrlD {
			// Special Docker keyboard shortcut
			if d.dockerView != nil {
				go func() {
					// Run in goroutine with delay to prevent UI freeze
					time.Sleep(50 * time.Millisecond)
					d.dockerView.RefreshAll()
				}()
			}
			return nil // Consume the event
		}
		// Pass all other keys through
		return event
	})

	pages.AddPage("docker", mainUI, true, true)

	// Set initial focus to the table explicitly
	app.SetFocus(d.dockerView.cores.GetTable())

	// Show a detailed welcome message and instructions with better formatting
	d.dockerView.cores.Log("[blue]Docker plugin initialized")
	d.dockerView.cores.Log("[yellow]⏳ Connecting to Docker daemon...")

	// Run initial refresh in a goroutine to prevent UI freeze on startup
	go func() {
		// Small delay for UI to render first
		time.Sleep(100 * time.Millisecond)

		d.dockerView.cores.Log("[yellow]⏳ Loading Docker resources...")
		d.dockerView.RefreshAll()

		// Add usage help
		d.dockerView.cores.Log("[green]✓ Docker plugin ready")
		d.dockerView.cores.Log("[aqua]Navigation Keys:")
		d.dockerView.cores.Log("   [yellow]C[white] - View Containers")
		d.dockerView.cores.Log("   [yellow]I[white] - View Images")
		d.dockerView.cores.Log("   [yellow]?[white] - Show help screen")
	}()

	return pages
}

// Stop cleans up resources used by the Docker plugin
func (d *DockerPlugin) Stop() {
	if d.dockerView != nil {
		// Clean up any resources
		if d.dockerView.refreshTimer != nil {
			d.dockerView.refreshTimer.Stop()
		}
	}
}

// GetMetadata returns plugin metadata
func (d *DockerPlugin) GetMetadata() pluginapi.PluginMetadata {
	return pluginapi.PluginMetadata{
		Name:        "docker",
		Version:     "1.0.0",
		Description: "Docker container and image management plugin",
		Author:      "Docker Plugin Team",
		License:     "MIT",
		Tags:        []string{"containers", "docker", "devops", "infrastructure"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "https://github.com/hatembentayeb/omo/plugins/docker",
	}
}

// OhmyopsPlugin is exported as a variable to be loaded by the main application
var OhmyopsPlugin DockerPlugin

func init() {
	OhmyopsPlugin.Name = "Docker Manager"
	OhmyopsPlugin.Description = "Manage Docker containers, images, networks and volumes"
}

// GetMetadata is exported for legacy loaders.
func GetMetadata() pluginapi.PluginMetadata {
	return pluginapi.PluginMetadata{
		Name:        "docker",
		Version:     "1.0.0",
		Description: "Docker container and image management plugin",
		Author:      "Docker Plugin Team",
		License:     "MIT",
		Tags:        []string{"containers", "docker", "devops", "infrastructure"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "https://github.com/hatembentayeb/omo/plugins/docker",
	}
}

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
		// Check for Ctrl+T to open host selector
		if event.Key() == tcell.KeyCtrlT {
			if d.dockerView != nil {
				d.dockerView.ShowHostSelector()
			}
			return nil
		}

		// Check for Ctrl+D for refresh
		if event.Key() == tcell.KeyCtrlD {
			if d.dockerView != nil {
				go func() {
					time.Sleep(50 * time.Millisecond)
					d.dockerView.refresh()
				}()
			}
			return nil
		}

		// Pass all other keys through
		return event
	})

	pages.AddPage("docker", mainUI, true, true)

	// Set initial focus to the table explicitly
	app.SetFocus(d.dockerView.containersView.GetTable())

	// Show a detailed welcome message and instructions
	d.dockerView.containersView.Log("[blue]Docker plugin initialized")
	d.dockerView.containersView.Log("[yellow]Connecting to Docker daemon...")

	// Run initial connection in a goroutine
	go func() {
		time.Sleep(100 * time.Millisecond)
		d.dockerView.AutoConnectToDefaultHost()

		d.dockerView.containersView.Log("[green]Docker plugin ready")
		d.dockerView.containersView.Log("[aqua]Navigation Keys:")
		d.dockerView.containersView.Log("   [yellow]C[white] - Containers")
		d.dockerView.containersView.Log("   [yellow]I[white] - Images")
		d.dockerView.containersView.Log("   [yellow]N[white] - Networks")
		d.dockerView.containersView.Log("   [yellow]V[white] - Volumes")
		d.dockerView.containersView.Log("   [yellow]T[white] - Stats")
		d.dockerView.containersView.Log("   [yellow]O[white] - Compose")
		d.dockerView.containersView.Log("   [yellow]Y[white] - System")
		d.dockerView.containersView.Log("   [yellow]?[white] - Help")
	}()

	return pages
}

// Stop cleans up resources used by the Docker plugin
func (d *DockerPlugin) Stop() {
	if d.dockerView != nil {
		d.dockerView.Stop()
	}
}

// GetMetadata returns plugin metadata
func (d *DockerPlugin) GetMetadata() pluginapi.PluginMetadata {
	return pluginapi.PluginMetadata{
		Name:        "docker",
		Version:     "2.0.0",
		Description: "Docker container, image, network, volume, and compose management",
		Author:      "OhMyOps Team",
		License:     "MIT",
		Tags:        []string{"containers", "docker", "devops", "infrastructure", "compose"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "https://github.com/hatembentayeb/omo/plugins/docker",
	}
}

// OhmyopsPlugin is exported as a variable to be loaded by the main application
var OhmyopsPlugin DockerPlugin

func init() {
	OhmyopsPlugin.Name = "Docker Manager"
	OhmyopsPlugin.Description = "Manage Docker containers, images, networks, volumes, and compose projects"
}

// GetMetadata is exported for legacy loaders.
func GetMetadata() pluginapi.PluginMetadata {
	return pluginapi.PluginMetadata{
		Name:        "docker",
		Version:     "2.0.0",
		Description: "Docker container, image, network, volume, and compose management",
		Author:      "OhMyOps Team",
		License:     "MIT",
		Tags:        []string{"containers", "docker", "devops", "infrastructure", "compose"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "https://github.com/hatembentayeb/omo/plugins/docker",
	}
}

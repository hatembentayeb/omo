package main

import (
	"fmt"
	"time"

	"github.com/rivo/tview"

	"omo/pkg/ui"
)

// DockerView manages the UI for interacting with Docker
type DockerView struct {
	app             *tview.Application
	pages           *tview.Pages
	viewPages       *tview.Pages
	containersView  *ui.CoreView
	imagesView      *ui.CoreView
	networksView    *ui.CoreView
	volumesView     *ui.CoreView
	statsView       *ui.CoreView
	composeView     *ui.CoreView
	systemView      *ui.CoreView
	logsView        *ui.CoreView
	inspectView     *ui.CoreView
	dockerClient    *DockerClient
	currentHost     *DockerHost
	currentViewName string
	refreshTimer    *time.Timer
	refreshInterval time.Duration
}

// NewDockerView creates a new Docker view
func NewDockerView(app *tview.Application, pages *tview.Pages) *DockerView {
	dv := &DockerView{
		app:             app,
		pages:           pages,
		viewPages:       tview.NewPages(),
		refreshInterval: 10 * time.Second,
	}

	// Initialize Docker client
	dv.dockerClient = NewDockerClient()
	dv.dockerClient.SetLogger(func(message string) {
		if dv.containersView != nil {
			dv.containersView.Log(message)
		}
	})

	// Create all views
	dv.containersView = dv.newContainersView()
	dv.imagesView = dv.newImagesView()
	dv.networksView = dv.newNetworksView()
	dv.volumesView = dv.newVolumesView()
	dv.statsView = dv.newStatsView()
	dv.composeView = dv.newComposeView()
	dv.systemView = dv.newSystemView()
	dv.logsView = dv.newLogsView()

	// Set modal pages for all views
	views := []*ui.CoreView{
		dv.containersView,
		dv.imagesView,
		dv.networksView,
		dv.volumesView,
		dv.statsView,
		dv.composeView,
		dv.systemView,
		dv.logsView,
	}
	for _, view := range views {
		if view != nil {
			view.SetModalPages(dv.pages)
		}
	}

	// Add all view pages
	dv.viewPages.AddPage("docker-containers", dv.containersView.GetLayout(), true, true)
	dv.viewPages.AddPage("docker-images", dv.imagesView.GetLayout(), true, false)
	dv.viewPages.AddPage("docker-networks", dv.networksView.GetLayout(), true, false)
	dv.viewPages.AddPage("docker-volumes", dv.volumesView.GetLayout(), true, false)
	dv.viewPages.AddPage("docker-stats", dv.statsView.GetLayout(), true, false)
	dv.viewPages.AddPage("docker-compose", dv.composeView.GetLayout(), true, false)
	dv.viewPages.AddPage("docker-system", dv.systemView.GetLayout(), true, false)
	dv.viewPages.AddPage("docker-logs", dv.logsView.GetLayout(), true, false)

	// Set current view
	dv.currentViewName = viewContainers
	dv.setViewStack(dv.containersView, viewContainers)
	dv.setViewStack(dv.imagesView, viewImages)
	dv.setViewStack(dv.networksView, viewNetworks)
	dv.setViewStack(dv.volumesView, viewVolumes)
	dv.setViewStack(dv.statsView, viewStats)
	dv.setViewStack(dv.composeView, viewCompose)
	dv.setViewStack(dv.systemView, viewSystem)
	dv.setViewStack(dv.logsView, viewLogs)

	// Set initial status
	dv.containersView.SetInfoText("[yellow]Docker Manager[white]\nStatus: Connecting...\nUse [green]Ctrl+T[white] to select host")

	// Start auto-refresh timer
	dv.startAutoRefresh()

	return dv
}

// GetMainUI returns the main UI component
func (dv *DockerView) GetMainUI() tview.Primitive {
	return dv.viewPages
}

// Stop cleans up resources when the view is no longer used
func (dv *DockerView) Stop() {
	if dv.refreshTimer != nil {
		dv.refreshTimer.Stop()
	}

	views := []*ui.CoreView{
		dv.containersView,
		dv.imagesView,
		dv.networksView,
		dv.volumesView,
		dv.statsView,
		dv.composeView,
		dv.systemView,
		dv.logsView,
	}
	for _, view := range views {
		if view != nil {
			view.StopAutoRefresh()
			view.UnregisterHandlers()
		}
	}
}

// ShowHostSelector displays the host selector form
func (dv *DockerView) ShowHostSelector() {
	current := dv.currentCores()
	if current != nil {
		current.Log("[blue]Opening host selector...")
	}

	// Get available Docker hosts from config
	hosts, err := GetAvailableHosts()
	if err != nil {
		if current != nil {
			current.Log(fmt.Sprintf("[red]Failed to load Docker hosts: %v", err))
		}
		return
	}

	if len(hosts) == 0 {
		if current != nil {
			current.Log("[yellow]No Docker hosts configured, using local daemon")
		}
		dv.connectToLocalDocker()
		return
	}

	// Create a list of host items for the selector
	items := make([][]string, len(hosts))
	for i, host := range hosts {
		items[i] = []string{
			host.Name,
			fmt.Sprintf("%s - %s", host.Host, host.Description),
		}
	}

	// Show selection modal
	ui.ShowStandardListSelectorModal(
		dv.pages,
		dv.app,
		"Select Docker Host",
		items,
		func(index int, name string, cancelled bool) {
			if !cancelled && index >= 0 && index < len(hosts) {
				dv.connectToDockerHost(&hosts[index])
			} else {
				if current != nil {
					current.Log("[blue]Host selection cancelled")
				}
			}

			// Return focus to the table
			if current != nil {
				dv.app.SetFocus(current.GetTable())
			}
		},
	)
}

// connectToDockerHost connects to a specific Docker host
func (dv *DockerView) connectToDockerHost(host *DockerHost) {
	current := dv.currentCores()
	if current != nil {
		current.Log(fmt.Sprintf("[yellow]Connecting to %s...", host.Name))
	}

	err := dv.dockerClient.ConnectToHost(*host)
	if err != nil {
		if current != nil {
			current.Log(fmt.Sprintf("[red]Failed to connect: %v", err))
		}
		return
	}

	dv.currentHost = host

	if current != nil {
		current.Log(fmt.Sprintf("[green]Connected to %s", host.Name))
		current.SetInfoText(fmt.Sprintf("[green]Docker Manager[white]\nHost: %s\nStatus: Connected",
			host.Name))
	}

	dv.refresh()
}

// connectToLocalDocker connects to the local Docker daemon
func (dv *DockerView) connectToLocalDocker() {
	localHost := &DockerHost{
		Name:        "local",
		Description: "Local Docker Daemon",
		Host:        "",
	}
	dv.connectToDockerHost(localHost)
}

// refresh refreshes the current view
func (dv *DockerView) refresh() {
	currentView := dv.currentCores()
	if currentView != nil {
		currentView.RefreshData()
	}
}

// handleAction handles actions triggered by the UI
func (dv *DockerView) handleAction(action string, payload map[string]interface{}) error {
	switch action {
	case "refresh":
		dv.refresh()
		return nil
	case "keypress":
		if key, ok := payload["key"].(string); ok {
			// View-specific actions first (before global keys)
			
			// Container actions (only in containers view)
			if dv.currentViewName == viewContainers {
				switch key {
				case "S":
					dv.startSelectedContainer()
					return nil
				case "X":
					dv.stopSelectedContainer()
					return nil
				case "D":
					dv.removeSelectedContainer()
					return nil
				case "L":
					dv.viewSelectedContainerLogs()
					return nil
				case "E":
					dv.execInSelectedContainer()
					return nil
				case "R":
					dv.restartSelectedContainer()
					return nil
				case "P":
					dv.pauseSelectedContainer()
					return nil
				case "U":
					dv.unpauseSelectedContainer()
					return nil
				case "K":
					dv.killSelectedContainer()
					return nil
				}
			}

			// Image actions (only in images view)
			if dv.currentViewName == viewImages {
				switch key {
				case "D":
					dv.removeSelectedImage()
					return nil
				case "P":
					dv.pullImage()
					return nil
				case "H":
					dv.showImageHistory()
					return nil
				}
			}

			// Network actions (only in networks view)
			if dv.currentViewName == viewNetworks {
				switch key {
				case "D":
					dv.removeSelectedNetwork()
					return nil
				case "A":
					dv.createNetwork()
					return nil
				}
			}

			// Volume actions (only in volumes view)
			if dv.currentViewName == viewVolumes {
				switch key {
				case "D":
					dv.removeSelectedVolume()
					return nil
				case "A":
					dv.createVolume()
					return nil
				case "P":
					dv.pruneVolumes()
					return nil
				}
			}

			// Compose actions (only in compose view)
			if dv.currentViewName == viewCompose {
				switch key {
				case "U":
					dv.composeUp()
					return nil
				case "D":
					dv.composeDown()
					return nil
				case "S":
					dv.composeStop()
					return nil
				case "R":
					dv.composeRestart()
					return nil
				case "L":
					dv.composeLogs()
					return nil
				}
			}

			// Navigation keys (available in all views)
			switch key {
			case "C":
				dv.showContainers()
				return nil
			case "I":
				dv.showImages()
				return nil
			case "N":
				dv.showNetworks()
				return nil
			case "V":
				dv.showVolumes()
				return nil
			case "T":
				dv.showStats()
				return nil
			case "O":
				dv.showCompose()
				return nil
			case "Y":
				dv.showSystem()
				return nil
			case "?":
				dv.showHelp()
				return nil
			case "R":
				dv.refresh()
				return nil
			}
		}
	case "navigate_back":
		if view, ok := payload["current_view"].(string); ok {
			if view == viewRoot {
				dv.switchToView(viewContainers)
				return nil
			}
			dv.switchToView(view)
			return nil
		}
	}
	return fmt.Errorf("unhandled")
}

// showHelp displays Docker plugin help
func (dv *DockerView) showHelp() {
	helpText := `
[yellow]Docker Manager Help[white]

[green]Navigation Views:[white]
C       - Containers view (main)
I       - Images view
N       - Networks view
V       - Volumes view
T       - Stats view (CPU/Memory)
O       - Compose projects view
Y       - System info view
Ctrl+T  - Select Docker host

[green]Container Actions (C view):[white]
S       - Start container
X       - Stop container
R       - Restart container
D       - Remove container
L       - View container logs
E       - Open interactive shell in container
P       - Pause container
U       - Unpause container
K       - Kill container (force)
Enter   - Inspect container details

[green]Image Actions (I view):[white]
P       - Pull new image
D       - Remove image
H       - View image history
R       - Run image as container
Enter   - Inspect image details

[green]Network Actions (N view):[white]
A       - Create new network
D       - Remove network
Enter   - Inspect network details

[green]Volume Actions (V view):[white]
A       - Create new volume
D       - Remove volume
P       - Prune unused volumes
Enter   - Inspect volume details

[green]Compose Actions (O view):[white]
U       - Compose up (start)
D       - Compose down (stop & remove)
S       - Compose stop
R       - Compose restart
L       - View compose logs

[green]System Actions (Y view):[white]
P       - Prune all unused resources
D       - Show disk usage
E       - Show recent events

[green]General:[white]
?       - Show this help
R       - Refresh current view
/       - Filter table
Esc     - Close modal / Cancel
`

	ui.ShowInfoModal(
		dv.pages,
		dv.app,
		"Docker Help",
		helpText,
		func() {
			current := dv.currentCores()
			if current != nil {
				dv.app.SetFocus(current.GetTable())
			}
		},
	)
}

// startAutoRefresh sets up and starts the auto-refresh timer
func (dv *DockerView) startAutoRefresh() {
	// Load the refresh interval from config
	if uiConfig, err := GetDockerUIConfig(); err == nil {
		dv.refreshInterval = time.Duration(uiConfig.RefreshInterval) * time.Second
	}

	// Cancel any existing timer
	if dv.refreshTimer != nil {
		dv.refreshTimer.Stop()
	}

	// Create a new timer
	dv.refreshTimer = time.AfterFunc(dv.refreshInterval, func() {
		if dv.dockerClient != nil && dv.dockerClient.IsConnected() {
			dv.app.QueueUpdate(func() {
				dv.refresh()
				dv.startAutoRefresh()
			})
		} else {
			dv.startAutoRefresh()
		}
	})
}

// AutoConnectToDefaultHost automatically connects to the default Docker host
func (dv *DockerView) AutoConnectToDefaultHost() {
	hosts, err := GetAvailableHosts()
	if err != nil {
		dv.containersView.Log(fmt.Sprintf("[yellow]Failed to load Docker hosts: %v", err))
		dv.connectToLocalDocker()
		return
	}

	if len(hosts) == 0 {
		dv.containersView.Log("[yellow]No Docker hosts configured, using local daemon")
		dv.connectToLocalDocker()
		return
	}

	// Connect to the first host in the list
	dv.containersView.Log(fmt.Sprintf("[blue]Auto-connecting to Docker host: %s", hosts[0].Name))
	dv.connectToDockerHost(&hosts[0])
}

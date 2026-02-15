package main

import (
	"fmt"
	"strings"

	"omo/pkg/ui"
)

func (dv *DockerView) newContainersView() *ui.CoreView {
	cores := ui.NewCoreView(dv.app, "Docker Containers")
	cores.SetTableHeaders([]string{"ID", "Name", "Image", "State", "Status", "Ports"})
	cores.SetRefreshCallback(dv.refreshContainersData)
	cores.SetSelectionKey("ID")

	// Navigation key bindings
	cores.AddKeyBinding("C", "Containers", dv.showContainers)
	cores.AddKeyBinding("I", "Images", dv.showImages)
	cores.AddKeyBinding("N", "Networks", dv.showNetworks)
	cores.AddKeyBinding("V", "Volumes", dv.showVolumes)
	cores.AddKeyBinding("T", "Stats", dv.showStats)
	cores.AddKeyBinding("O", "Compose", dv.showCompose)
	cores.AddKeyBinding("Y", "System", dv.showSystem)

	// Container action key bindings
	cores.AddKeyBinding("S", "Start", dv.startSelectedContainer)
	cores.AddKeyBinding("X", "Stop", dv.stopSelectedContainer)
	cores.AddKeyBinding("D", "Delete", dv.removeSelectedContainer)
	cores.AddKeyBinding("L", "Logs", dv.viewSelectedContainerLogs)
	cores.AddKeyBinding("E", "Exec", dv.execInSelectedContainer)
	cores.AddKeyBinding("R", "Restart", dv.restartSelectedContainer)
	cores.AddKeyBinding("P", "Pause", dv.pauseSelectedContainer)
	cores.AddKeyBinding("U", "Unpause", dv.unpauseSelectedContainer)
	cores.AddKeyBinding("K", "Kill", dv.killSelectedContainer)
	cores.AddKeyBinding("?", "Help", dv.showHelp)

	cores.SetActionCallback(dv.handleAction)

	// Set row selection callback
	cores.SetRowSelectedCallback(func(row int) {
		tableData := cores.GetTableData()
		if row >= 0 && row < len(tableData) && len(tableData[row]) > 0 {
			cores.Log(fmt.Sprintf("[blue]Selected container: %s (%s)", tableData[row][1], tableData[row][0]))
		}
	})

	// Set Enter key to show container details
	cores.GetTable().SetSelectedFunc(func(row, column int) {
		if row <= 0 {
			return
		}
		dv.showContainerDetails()
	})

	cores.RegisterHandlers()
	return cores
}

func (dv *DockerView) refreshContainersData() ([][]string, error) {
	if dv.dockerClient == nil {
		return [][]string{}, fmt.Errorf("docker client not initialized")
	}

	containers, err := dv.dockerClient.ListContainers()
	if err != nil {
		return [][]string{}, err
	}

	rows := make([][]string, len(containers))
	for i, container := range containers {
		rows[i] = container.GetTableRow()
	}

	if dv.currentHost != nil {
		dv.containersView.SetInfoText(fmt.Sprintf("[green]Docker Manager[white]\nHost: %s\nContainers: %d\nStatus: Connected",
			dv.currentHost.Name, len(containers)))
	}

	return rows, nil
}

func (dv *DockerView) getSelectedContainerID() (string, bool) {
	row := dv.containersView.GetSelectedRowData()
	if len(row) == 0 {
		return "", false
	}
	return row[0], true
}

func (dv *DockerView) getSelectedContainerName() string {
	row := dv.containersView.GetSelectedRowData()
	if len(row) < 2 {
		return ""
	}
	return row[1]
}

func (dv *DockerView) startSelectedContainer() {
	id, ok := dv.getSelectedContainerID()
	if !ok {
		dv.containersView.Log("[yellow]No container selected")
		return
	}

	name := dv.getSelectedContainerName()
	dv.containersView.Log(fmt.Sprintf("[yellow]Starting container %s...", name))

	if err := dv.dockerClient.StartContainer(id); err != nil {
		dv.containersView.Log(fmt.Sprintf("[red]Failed to start container: %v", err))
		return
	}

	dv.containersView.Log(fmt.Sprintf("[green]Container %s started", name))
	dv.refresh()
}

func (dv *DockerView) stopSelectedContainer() {
	id, ok := dv.getSelectedContainerID()
	if !ok {
		dv.containersView.Log("[yellow]No container selected")
		return
	}

	name := dv.getSelectedContainerName()
	dv.containersView.Log(fmt.Sprintf("[yellow]Stopping container %s...", name))

	if err := dv.dockerClient.StopContainer(id); err != nil {
		dv.containersView.Log(fmt.Sprintf("[red]Failed to stop container: %v", err))
		return
	}

	dv.containersView.Log(fmt.Sprintf("[green]Container %s stopped", name))
	dv.refresh()
}

func (dv *DockerView) restartSelectedContainer() {
	id, ok := dv.getSelectedContainerID()
	if !ok {
		dv.containersView.Log("[yellow]No container selected")
		return
	}

	name := dv.getSelectedContainerName()
	dv.containersView.Log(fmt.Sprintf("[yellow]Restarting container %s...", name))

	if err := dv.dockerClient.RestartContainer(id); err != nil {
		dv.containersView.Log(fmt.Sprintf("[red]Failed to restart container: %v", err))
		return
	}

	dv.containersView.Log(fmt.Sprintf("[green]Container %s restarted", name))
	dv.refresh()
}

func (dv *DockerView) pauseSelectedContainer() {
	id, ok := dv.getSelectedContainerID()
	if !ok {
		dv.containersView.Log("[yellow]No container selected")
		return
	}

	name := dv.getSelectedContainerName()
	dv.containersView.Log(fmt.Sprintf("[yellow]Pausing container %s...", name))

	if err := dv.dockerClient.PauseContainer(id); err != nil {
		dv.containersView.Log(fmt.Sprintf("[red]Failed to pause container: %v", err))
		return
	}

	dv.containersView.Log(fmt.Sprintf("[green]Container %s paused", name))
	dv.refresh()
}

func (dv *DockerView) unpauseSelectedContainer() {
	id, ok := dv.getSelectedContainerID()
	if !ok {
		dv.containersView.Log("[yellow]No container selected")
		return
	}

	name := dv.getSelectedContainerName()
	dv.containersView.Log(fmt.Sprintf("[yellow]Unpausing container %s...", name))

	if err := dv.dockerClient.UnpauseContainer(id); err != nil {
		dv.containersView.Log(fmt.Sprintf("[red]Failed to unpause container: %v", err))
		return
	}

	dv.containersView.Log(fmt.Sprintf("[green]Container %s unpaused", name))
	dv.refresh()
}

func (dv *DockerView) killSelectedContainer() {
	id, ok := dv.getSelectedContainerID()
	if !ok {
		dv.containersView.Log("[yellow]No container selected")
		return
	}

	name := dv.getSelectedContainerName()

	ui.ShowStandardConfirmationModal(
		dv.pages,
		dv.app,
		"Kill Container",
		fmt.Sprintf("Are you sure you want to kill container [red]%s[white]?\nThis will forcefully terminate the container!", name),
		func(confirmed bool) {
			if confirmed {
				dv.containersView.Log(fmt.Sprintf("[yellow]Killing container %s...", name))
				if err := dv.dockerClient.KillContainer(id); err != nil {
					dv.containersView.Log(fmt.Sprintf("[red]Failed to kill container: %v", err))
				} else {
					dv.containersView.Log(fmt.Sprintf("[red]Container %s killed", name))
					dv.refresh()
				}
			}
			dv.app.SetFocus(dv.containersView.GetTable())
		},
	)
}

func (dv *DockerView) removeSelectedContainer() {
	id, ok := dv.getSelectedContainerID()
	if !ok {
		dv.containersView.Log("[yellow]No container selected")
		return
	}

	name := dv.getSelectedContainerName()

	ui.ShowStandardConfirmationModal(
		dv.pages,
		dv.app,
		"Remove Container",
		fmt.Sprintf("Are you sure you want to remove container [red]%s[white]?", name),
		func(confirmed bool) {
			if confirmed {
				dv.containersView.Log(fmt.Sprintf("[yellow]Removing container %s...", name))
				if err := dv.dockerClient.RemoveContainer(id); err != nil {
					dv.containersView.Log(fmt.Sprintf("[red]Failed to remove container: %v", err))
				} else {
					dv.containersView.Log(fmt.Sprintf("[yellow]Container %s removed", name))
					dv.refresh()
				}
			}
			dv.app.SetFocus(dv.containersView.GetTable())
		},
	)
}

func (dv *DockerView) viewSelectedContainerLogs() {
	id, ok := dv.getSelectedContainerID()
	if !ok {
		dv.containersView.Log("[yellow]No container selected")
		return
	}

	name := dv.getSelectedContainerName()
	dv.containersView.Log(fmt.Sprintf("[yellow]Fetching logs for %s...", name))

	logs, err := dv.dockerClient.GetContainerLogs(id)
	if err != nil {
		dv.containersView.Log(fmt.Sprintf("[red]Failed to get logs: %v", err))
		return
	}

	ui.ShowInfoModal(
		dv.pages,
		dv.app,
		fmt.Sprintf("Logs: %s", name),
		logs,
		func() {
			dv.app.SetFocus(dv.containersView.GetTable())
		},
	)
}

func (dv *DockerView) execInSelectedContainer() {
	id, ok := dv.getSelectedContainerID()
	if !ok {
		dv.containersView.Log("[yellow]No container selected")
		return
	}

	name := dv.getSelectedContainerName()

	// Check if container is running
	row := dv.containersView.GetSelectedRowData()
	if len(row) < 4 || row[3] != "running" {
		dv.containersView.Log("[red]Container must be running to exec into it")
		return
	}

	ui.ShowCompactStyledInputModal(
		dv.pages,
		dv.app,
		"Shell",
		"Shell",
		"sh",
		30,
		nil,
		func(shell string, cancelled bool) {
			if cancelled || shell == "" {
				dv.app.SetFocus(dv.containersView.GetTable())
				return
			}

			dv.containersView.Log(fmt.Sprintf("[yellow]Opening shell '%s' in %s... (type 'exit' to return)", shell, name))

			// Suspend the TUI app
			dv.app.Suspend(func() {
				// Clear screen before exec
				fmt.Print("\033[H\033[2J")
				fmt.Printf("Connecting to container %s with %s...\n", name, shell)
				fmt.Println("Type 'exit' to return to the app.")

				// Run interactive shell
				err := dv.dockerClient.ExecInteractiveShell(id, shell)
				if err != nil {
					fmt.Printf("\nShell exited with error: %v\n", err)
					fmt.Println("Press Enter to return to the app...")
					fmt.Scanln()
				}
			})

			// After returning from shell, refresh and restore focus
			dv.containersView.Log(fmt.Sprintf("[green]Returned from shell in %s", name))
			dv.app.SetFocus(dv.containersView.GetTable())
		},
	)
}

func (dv *DockerView) showContainerDetails() {
	id, ok := dv.getSelectedContainerID()
	if !ok {
		return
	}

	name := dv.getSelectedContainerName()
	dv.containersView.Log(fmt.Sprintf("[yellow]Inspecting container %s...", name))

	inspect, err := dv.dockerClient.InspectContainer(id)
	if err != nil {
		dv.containersView.Log(fmt.Sprintf("[red]Failed to inspect container: %v", err))
		return
	}

	// Format the inspection output
	var details strings.Builder
	details.WriteString(fmt.Sprintf("[yellow]Container: %s[white]\n\n", name))
	details.WriteString(fmt.Sprintf("[green]ID:[white] %s\n", inspect.ID))
	details.WriteString(fmt.Sprintf("[green]Image:[white] %s\n", inspect.Image))
	details.WriteString(fmt.Sprintf("[green]Created:[white] %s\n", inspect.Created))
	details.WriteString(fmt.Sprintf("[green]State:[white] %s\n", inspect.State))
	details.WriteString(fmt.Sprintf("[green]Status:[white] %s\n", inspect.Status))
	details.WriteString(fmt.Sprintf("[green]Platform:[white] %s\n", inspect.Platform))
	details.WriteString(fmt.Sprintf("[green]Restart Count:[white] %d\n", inspect.RestartCount))

	if len(inspect.Ports) > 0 {
		details.WriteString(fmt.Sprintf("\n[yellow]Ports:[white]\n"))
		for _, port := range inspect.Ports {
			details.WriteString(fmt.Sprintf("  %s\n", port))
		}
	}

	if len(inspect.Mounts) > 0 {
		details.WriteString(fmt.Sprintf("\n[yellow]Mounts:[white]\n"))
		for _, mount := range inspect.Mounts {
			details.WriteString(fmt.Sprintf("  %s\n", mount))
		}
	}

	if len(inspect.Networks) > 0 {
		details.WriteString(fmt.Sprintf("\n[yellow]Networks:[white]\n"))
		for _, network := range inspect.Networks {
			details.WriteString(fmt.Sprintf("  %s\n", network))
		}
	}

	if len(inspect.Env) > 0 {
		details.WriteString(fmt.Sprintf("\n[yellow]Environment:[white]\n"))
		for _, env := range inspect.Env {
			details.WriteString(fmt.Sprintf("  %s\n", env))
		}
	}

	ui.ShowInfoModal(
		dv.pages,
		dv.app,
		fmt.Sprintf("Container: %s", name),
		details.String(),
		func() {
			dv.app.SetFocus(dv.containersView.GetTable())
		},
	)
}

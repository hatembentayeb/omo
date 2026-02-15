package main

import (
	"fmt"
	"strings"

	"omo/pkg/ui"
)

func (dv *DockerView) newVolumesView() *ui.CoreView {
	cores := ui.NewCoreView(dv.app, "Docker Volumes")
	cores.SetTableHeaders([]string{"Name", "Driver", "Mountpoint", "Scope", "Created"})
	cores.SetRefreshCallback(dv.refreshVolumesData)
	cores.SetSelectionKey("Name")

	// Navigation key bindings
	cores.AddKeyBinding("C", "Containers", dv.showContainers)
	cores.AddKeyBinding("I", "Images", dv.showImages)
	cores.AddKeyBinding("N", "Networks", dv.showNetworks)
	cores.AddKeyBinding("V", "Volumes", dv.showVolumes)
	cores.AddKeyBinding("T", "Stats", dv.showStats)
	cores.AddKeyBinding("O", "Compose", dv.showCompose)
	cores.AddKeyBinding("Y", "System", dv.showSystem)

	// Volume action key bindings
	cores.AddKeyBinding("D", "Delete", dv.removeSelectedVolume)
	cores.AddKeyBinding("A", "Create", dv.createVolume)
	cores.AddKeyBinding("P", "Prune", dv.pruneVolumes)
	cores.AddKeyBinding("?", "Help", dv.showHelp)

	cores.SetActionCallback(dv.handleAction)

	// Set row selection callback
	cores.SetRowSelectedCallback(func(row int) {
		tableData := cores.GetTableData()
		if row >= 0 && row < len(tableData) && len(tableData[row]) > 0 {
			cores.Log(fmt.Sprintf("[blue]Selected volume: %s", tableData[row][0]))
		}
	})

	// Set Enter key to show volume details
	cores.GetTable().SetSelectedFunc(func(row, column int) {
		if row <= 0 {
			return
		}
		dv.showVolumeDetails()
	})

	cores.RegisterHandlers()
	return cores
}

func (dv *DockerView) refreshVolumesData() ([][]string, error) {
	if dv.dockerClient == nil {
		return [][]string{}, fmt.Errorf("docker client not initialized")
	}

	volumes, err := dv.dockerClient.ListVolumes()
	if err != nil {
		return [][]string{}, err
	}

	rows := make([][]string, len(volumes))
	for i, vol := range volumes {
		// Truncate mountpoint for display
		mountpoint := vol.Mountpoint
		if len(mountpoint) > 40 {
			mountpoint = "..." + mountpoint[len(mountpoint)-37:]
		}

		createdAt := vol.CreatedAt
		if len(createdAt) > 19 {
			createdAt = createdAt[:19]
		}

		rows[i] = []string{
			vol.Name,
			vol.Driver,
			mountpoint,
			vol.Scope,
			createdAt,
		}
	}

	if dv.currentHost != nil {
		dv.volumesView.SetInfoText(fmt.Sprintf("[green]Docker Volumes[white]\nHost: %s\nVolumes: %d\nStatus: Connected",
			dv.currentHost.Name, len(volumes)))
	}

	return rows, nil
}

func (dv *DockerView) getSelectedVolumeName() (string, bool) {
	row := dv.volumesView.GetSelectedRowData()
	if len(row) == 0 {
		return "", false
	}
	return row[0], true
}

func (dv *DockerView) removeSelectedVolume() {
	name, ok := dv.getSelectedVolumeName()
	if !ok {
		dv.volumesView.Log("[yellow]No volume selected")
		return
	}

	ui.ShowStandardConfirmationModal(
		dv.pages,
		dv.app,
		"Remove Volume",
		fmt.Sprintf("Are you sure you want to remove volume [red]%s[white]?\nThis action cannot be undone!", name),
		func(confirmed bool) {
			if confirmed {
				dv.volumesView.Log(fmt.Sprintf("[yellow]Removing volume %s...", name))
				if err := dv.dockerClient.RemoveVolume(name); err != nil {
					dv.volumesView.Log(fmt.Sprintf("[red]Failed to remove volume: %v", err))
				} else {
					dv.volumesView.Log(fmt.Sprintf("[yellow]Volume %s removed", name))
					dv.refresh()
				}
			}
			dv.app.SetFocus(dv.volumesView.GetTable())
		},
	)
}

func (dv *DockerView) createVolume() {
	ui.ShowCompactStyledInputModal(
		dv.pages,
		dv.app,
		"Create Volume",
		"Volume Name",
		"",
		30,
		nil,
		func(volumeName string, cancelled bool) {
			if cancelled || volumeName == "" {
				dv.app.SetFocus(dv.volumesView.GetTable())
				return
			}

			dv.volumesView.Log(fmt.Sprintf("[yellow]Creating volume %s...", volumeName))

			if err := dv.dockerClient.CreateVolume(volumeName); err != nil {
				dv.volumesView.Log(fmt.Sprintf("[red]Failed to create volume: %v", err))
			} else {
				dv.volumesView.Log(fmt.Sprintf("[green]Volume %s created", volumeName))
				dv.refresh()
			}

			dv.app.SetFocus(dv.volumesView.GetTable())
		},
	)
}

func (dv *DockerView) pruneVolumes() {
	ui.ShowStandardConfirmationModal(
		dv.pages,
		dv.app,
		"Prune Volumes",
		"Are you sure you want to remove all [red]unused[white] volumes?\nThis action cannot be undone!",
		func(confirmed bool) {
			if confirmed {
				dv.volumesView.Log("[yellow]Pruning unused volumes...")
				report, err := dv.dockerClient.PruneVolumes()
				if err != nil {
					dv.volumesView.Log(fmt.Sprintf("[red]Failed to prune volumes: %v", err))
				} else {
					dv.volumesView.Log(fmt.Sprintf("[green]%s", report))
					dv.refresh()
				}
			}
			dv.app.SetFocus(dv.volumesView.GetTable())
		},
	)
}

func (dv *DockerView) showVolumeDetails() {
	name, ok := dv.getSelectedVolumeName()
	if !ok {
		return
	}

	dv.volumesView.Log(fmt.Sprintf("[yellow]Inspecting volume %s...", name))

	inspect, err := dv.dockerClient.InspectVolume(name)
	if err != nil {
		dv.volumesView.Log(fmt.Sprintf("[red]Failed to inspect volume: %v", err))
		return
	}

	var details strings.Builder
	details.WriteString(fmt.Sprintf("[yellow]Volume: %s[white]\n\n", name))
	details.WriteString(fmt.Sprintf("[green]Driver:[white] %s\n", inspect.Driver))
	details.WriteString(fmt.Sprintf("[green]Mountpoint:[white] %s\n", inspect.Mountpoint))
	details.WriteString(fmt.Sprintf("[green]Scope:[white] %s\n", inspect.Scope))
	details.WriteString(fmt.Sprintf("[green]Created:[white] %s\n", inspect.CreatedAt))

	if len(inspect.Labels) > 0 {
		details.WriteString(fmt.Sprintf("\n[yellow]Labels:[white]\n"))
		for key, value := range inspect.Labels {
			details.WriteString(fmt.Sprintf("  %s: %s\n", key, value))
		}
	}

	if len(inspect.Options) > 0 {
		details.WriteString(fmt.Sprintf("\n[yellow]Options:[white]\n"))
		for key, value := range inspect.Options {
			details.WriteString(fmt.Sprintf("  %s: %s\n", key, value))
		}
	}

	if inspect.UsageData != nil {
		details.WriteString(fmt.Sprintf("\n[yellow]Usage:[white]\n"))
		details.WriteString(fmt.Sprintf("  Size: %s\n", inspect.UsageData.Size))
		details.WriteString(fmt.Sprintf("  Ref Count: %d\n", inspect.UsageData.RefCount))
	}

	ui.ShowInfoModal(
		dv.pages,
		dv.app,
		fmt.Sprintf("Volume: %s", name),
		details.String(),
		func() {
			dv.app.SetFocus(dv.volumesView.GetTable())
		},
	)
}

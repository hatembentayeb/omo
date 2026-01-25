package main

import (
	"fmt"
	"strings"

	"omo/pkg/ui"
)

func (dv *DockerView) newSystemView() *ui.Cores {
	cores := ui.NewCores(dv.app, "Docker System")
	cores.SetTableHeaders([]string{"Property", "Value"})
	cores.SetRefreshCallback(dv.refreshSystemData)
	cores.SetSelectionKey("Property")

	// Navigation key bindings
	cores.AddKeyBinding("C", "Containers", dv.showContainers)
	cores.AddKeyBinding("I", "Images", dv.showImages)
	cores.AddKeyBinding("N", "Networks", dv.showNetworks)
	cores.AddKeyBinding("V", "Volumes", dv.showVolumes)
	cores.AddKeyBinding("T", "Stats", dv.showStats)
	cores.AddKeyBinding("O", "Compose", dv.showCompose)
	cores.AddKeyBinding("Y", "System", dv.showSystem)

	// System action key bindings
	cores.AddKeyBinding("P", "Prune All", dv.pruneSystem)
	cores.AddKeyBinding("D", "Disk Usage", dv.showDiskUsage)
	cores.AddKeyBinding("E", "Events", dv.showEvents)
	cores.AddKeyBinding("?", "Help", dv.showHelp)

	cores.SetActionCallback(dv.handleAction)

	cores.RegisterHandlers()
	return cores
}

func (dv *DockerView) refreshSystemData() ([][]string, error) {
	if dv.dockerClient == nil {
		return [][]string{{"Status", "Not Connected"}}, nil
	}

	info, err := dv.dockerClient.GetSystemInfo()
	if err != nil {
		return [][]string{{"Error", err.Error()}}, err
	}

	rows := [][]string{
		{"Docker Version", info.ServerVersion},
		{"API Version", info.APIVersion},
		{"OS", info.OperatingSystem},
		{"Architecture", info.Architecture},
		{"Kernel Version", info.KernelVersion},
		{"Total Memory", info.MemTotal},
		{"CPUs", fmt.Sprintf("%d", info.NCPU)},
		{"Containers", fmt.Sprintf("%d", info.Containers)},
		{"Running", fmt.Sprintf("%d", info.ContainersRunning)},
		{"Paused", fmt.Sprintf("%d", info.ContainersPaused)},
		{"Stopped", fmt.Sprintf("%d", info.ContainersStopped)},
		{"Images", fmt.Sprintf("%d", info.Images)},
		{"Storage Driver", info.Driver},
		{"Logging Driver", info.LoggingDriver},
		{"Cgroup Driver", info.CgroupDriver},
		{"Cgroup Version", info.CgroupVersion},
		{"Docker Root Dir", info.DockerRootDir},
		{"Swarm Status", info.SwarmStatus},
	}

	if dv.currentHost != nil {
		dv.systemView.SetInfoText(fmt.Sprintf("[green]Docker System[white]\nHost: %s\nVersion: %s\nContainers: %d",
			dv.currentHost.Name, info.ServerVersion, info.Containers))
	}

	return rows, nil
}

func (dv *DockerView) pruneSystem() {
	ui.ShowStandardConfirmationModal(
		dv.pages,
		dv.app,
		"Prune Docker System",
		"Are you sure you want to prune [red]ALL[white] unused Docker resources?\n\nThis will remove:\n- All stopped containers\n- All unused networks\n- All dangling images\n- All unused volumes\n- Build cache",
		func(confirmed bool) {
			if confirmed {
				dv.systemView.Log("[yellow]Pruning Docker system...")

				go func() {
					report, err := dv.dockerClient.PruneSystem()
					dv.app.QueueUpdateDraw(func() {
						if err != nil {
							dv.systemView.Log(fmt.Sprintf("[red]Failed to prune system: %v", err))
						} else {
							dv.systemView.Log(fmt.Sprintf("[green]System pruned:\n%s", report))
							dv.refresh()
						}
					})
				}()
			}
			dv.app.SetFocus(dv.systemView.GetTable())
		},
	)
}

func (dv *DockerView) showDiskUsage() {
	dv.systemView.Log("[yellow]Getting disk usage...")

	usage, err := dv.dockerClient.GetDiskUsage()
	if err != nil {
		dv.systemView.Log(fmt.Sprintf("[red]Failed to get disk usage: %v", err))
		return
	}

	var details strings.Builder
	details.WriteString("[yellow]Docker Disk Usage[white]\n\n")
	details.WriteString(fmt.Sprintf("[green]Images:[white]\n"))
	details.WriteString(fmt.Sprintf("  Total: %d\n", usage.ImagesCount))
	details.WriteString(fmt.Sprintf("  Size: %s\n", usage.ImagesSize))
	details.WriteString(fmt.Sprintf("  Reclaimable: %s\n\n", usage.ImagesReclaimable))

	details.WriteString(fmt.Sprintf("[green]Containers:[white]\n"))
	details.WriteString(fmt.Sprintf("  Total: %d\n", usage.ContainersCount))
	details.WriteString(fmt.Sprintf("  Size: %s\n", usage.ContainersSize))
	details.WriteString(fmt.Sprintf("  Reclaimable: %s\n\n", usage.ContainersReclaimable))

	details.WriteString(fmt.Sprintf("[green]Volumes:[white]\n"))
	details.WriteString(fmt.Sprintf("  Total: %d\n", usage.VolumesCount))
	details.WriteString(fmt.Sprintf("  Size: %s\n", usage.VolumesSize))
	details.WriteString(fmt.Sprintf("  Reclaimable: %s\n\n", usage.VolumesReclaimable))

	details.WriteString(fmt.Sprintf("[green]Build Cache:[white]\n"))
	details.WriteString(fmt.Sprintf("  Total: %d\n", usage.BuildCacheCount))
	details.WriteString(fmt.Sprintf("  Size: %s\n", usage.BuildCacheSize))
	details.WriteString(fmt.Sprintf("  Reclaimable: %s\n\n", usage.BuildCacheReclaimable))

	details.WriteString(fmt.Sprintf("[yellow]Total Reclaimable:[white] %s\n", usage.TotalReclaimable))

	ui.ShowInfoModal(
		dv.pages,
		dv.app,
		"Docker Disk Usage",
		details.String(),
		func() {
			dv.app.SetFocus(dv.systemView.GetTable())
		},
	)
}

func (dv *DockerView) showEvents() {
	dv.systemView.Log("[yellow]Getting recent events...")

	events, err := dv.dockerClient.GetRecentEvents()
	if err != nil {
		dv.systemView.Log(fmt.Sprintf("[red]Failed to get events: %v", err))
		return
	}

	if events == "" {
		events = "No recent events"
	}

	ui.ShowInfoModal(
		dv.pages,
		dv.app,
		"Docker Events",
		events,
		func() {
			dv.app.SetFocus(dv.systemView.GetTable())
		},
	)
}

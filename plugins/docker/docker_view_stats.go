package main

import (
	"fmt"

	"omo/pkg/ui"
)

func (dv *DockerView) newStatsView() *ui.Cores {
	cores := ui.NewCores(dv.app, "Container Stats")
	cores.SetTableHeaders([]string{"Container", "CPU %", "Memory", "Mem %", "Net I/O", "Block I/O", "PIDs"})
	cores.SetRefreshCallback(dv.refreshStatsData)
	cores.SetSelectionKey("Container")

	// Navigation key bindings
	cores.AddKeyBinding("C", "Containers", dv.showContainers)
	cores.AddKeyBinding("I", "Images", dv.showImages)
	cores.AddKeyBinding("N", "Networks", dv.showNetworks)
	cores.AddKeyBinding("V", "Volumes", dv.showVolumes)
	cores.AddKeyBinding("T", "Stats", dv.showStats)
	cores.AddKeyBinding("O", "Compose", dv.showCompose)
	cores.AddKeyBinding("Y", "System", dv.showSystem)

	// Stats action key bindings
	cores.AddKeyBinding("R", "Refresh", dv.refresh)
	cores.AddKeyBinding("?", "Help", dv.showHelp)

	cores.SetActionCallback(dv.handleAction)

	// Set row selection callback
	cores.SetRowSelectedCallback(func(row int) {
		tableData := cores.GetTableData()
		if row >= 0 && row < len(tableData) && len(tableData[row]) > 0 {
			cores.Log(fmt.Sprintf("[blue]Container: %s - CPU: %s, Memory: %s",
				tableData[row][0], tableData[row][1], tableData[row][2]))
		}
	})

	cores.RegisterHandlers()
	return cores
}

func (dv *DockerView) refreshStatsData() ([][]string, error) {
	if dv.dockerClient == nil {
		return [][]string{}, fmt.Errorf("docker client not initialized")
	}

	stats, err := dv.dockerClient.GetContainerStats()
	if err != nil {
		return [][]string{}, err
	}

	rows := make([][]string, len(stats))
	for i, stat := range stats {
		rows[i] = []string{
			stat.Name,
			stat.CPUPercent,
			stat.MemoryUsage,
			stat.MemoryPercent,
			stat.NetIO,
			stat.BlockIO,
			stat.PIDs,
		}
	}

	if dv.currentHost != nil {
		dv.statsView.SetInfoText(fmt.Sprintf("[green]Container Stats[white]\nHost: %s\nRunning: %d\nStatus: Connected",
			dv.currentHost.Name, len(stats)))
	}

	return rows, nil
}

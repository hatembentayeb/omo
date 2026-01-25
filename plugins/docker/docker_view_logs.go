package main

import (
	"fmt"

	"omo/pkg/ui"
)

func (dv *DockerView) newLogsView() *ui.Cores {
	cores := ui.NewCores(dv.app, "Container Logs")
	cores.SetTableHeaders([]string{"Container", "Status", "Image", "Logs Available"})
	cores.SetRefreshCallback(dv.refreshLogsViewData)
	cores.SetSelectionKey("Container")

	// Navigation key bindings
	cores.AddKeyBinding("C", "Containers", dv.showContainers)
	cores.AddKeyBinding("I", "Images", dv.showImages)
	cores.AddKeyBinding("N", "Networks", dv.showNetworks)
	cores.AddKeyBinding("V", "Volumes", dv.showVolumes)
	cores.AddKeyBinding("T", "Stats", dv.showStats)
	cores.AddKeyBinding("O", "Compose", dv.showCompose)
	cores.AddKeyBinding("Y", "System", dv.showSystem)

	// Logs action key bindings
	cores.AddKeyBinding("L", "View Logs", dv.viewLogsFromLogView)
	cores.AddKeyBinding("F", "Follow", dv.followLogs)
	cores.AddKeyBinding("?", "Help", dv.showHelp)

	cores.SetActionCallback(dv.handleAction)

	// Set Enter key to view logs
	cores.GetTable().SetSelectedFunc(func(row, column int) {
		if row <= 0 {
			return
		}
		dv.viewLogsFromLogView()
	})

	cores.RegisterHandlers()
	return cores
}

func (dv *DockerView) refreshLogsViewData() ([][]string, error) {
	if dv.dockerClient == nil {
		return [][]string{}, fmt.Errorf("docker client not initialized")
	}

	containers, err := dv.dockerClient.ListContainers()
	if err != nil {
		return [][]string{}, err
	}

	rows := make([][]string, len(containers))
	for i, container := range containers {
		logsAvailable := "Yes"
		if container.State != "running" {
			logsAvailable = "Historic"
		}

		rows[i] = []string{
			container.Name,
			container.State,
			container.Image,
			logsAvailable,
		}
	}

	if dv.currentHost != nil {
		dv.logsView.SetInfoText(fmt.Sprintf("[green]Container Logs[white]\nHost: %s\nContainers: %d",
			dv.currentHost.Name, len(containers)))
	}

	return rows, nil
}

func (dv *DockerView) getSelectedContainerFromLogsView() (string, bool) {
	row := dv.logsView.GetSelectedRowData()
	if len(row) == 0 {
		return "", false
	}
	return row[0], true
}

func (dv *DockerView) viewLogsFromLogView() {
	name, ok := dv.getSelectedContainerFromLogsView()
	if !ok {
		dv.logsView.Log("[yellow]No container selected")
		return
	}

	dv.logsView.Log(fmt.Sprintf("[yellow]Fetching logs for %s...", name))

	// Find container ID by name
	containers, err := dv.dockerClient.ListContainers()
	if err != nil {
		dv.logsView.Log(fmt.Sprintf("[red]Failed to list containers: %v", err))
		return
	}

	var containerID string
	for _, c := range containers {
		if c.Name == name {
			containerID = c.ID
			break
		}
	}

	if containerID == "" {
		dv.logsView.Log(fmt.Sprintf("[red]Container %s not found", name))
		return
	}

	logs, err := dv.dockerClient.GetContainerLogs(containerID)
	if err != nil {
		dv.logsView.Log(fmt.Sprintf("[red]Failed to get logs: %v", err))
		return
	}

	ui.ShowInfoModal(
		dv.pages,
		dv.app,
		fmt.Sprintf("Logs: %s", name),
		logs,
		func() {
			dv.app.SetFocus(dv.logsView.GetTable())
		},
	)
}

func (dv *DockerView) followLogs() {
	name, ok := dv.getSelectedContainerFromLogsView()
	if !ok {
		dv.logsView.Log("[yellow]No container selected")
		return
	}

	dv.logsView.Log(fmt.Sprintf("[yellow]Following logs for %s... (logs will appear in terminal)", name))

	// Find container ID by name
	containers, err := dv.dockerClient.ListContainers()
	if err != nil {
		dv.logsView.Log(fmt.Sprintf("[red]Failed to list containers: %v", err))
		return
	}

	var containerID string
	for _, c := range containers {
		if c.Name == name {
			containerID = c.ID
			break
		}
	}

	if containerID == "" {
		dv.logsView.Log(fmt.Sprintf("[red]Container %s not found", name))
		return
	}

	// For follow mode, we show streaming logs
	go func() {
		logs, err := dv.dockerClient.GetContainerLogsStream(containerID, 100)
		if err != nil {
			dv.app.QueueUpdateDraw(func() {
				dv.logsView.Log(fmt.Sprintf("[red]Failed to get logs: %v", err))
			})
			return
		}

		dv.app.QueueUpdateDraw(func() {
			ui.ShowInfoModal(
				dv.pages,
				dv.app,
				fmt.Sprintf("Live Logs: %s", name),
				logs,
				func() {
					dv.app.SetFocus(dv.logsView.GetTable())
				},
			)
		})
	}()
}

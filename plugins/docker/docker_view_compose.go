package main

import (
	"fmt"

	"omo/pkg/ui"
)

func (dv *DockerView) newComposeView() *ui.CoreView {
	cores := ui.NewCoreView(dv.app, "Docker Compose")
	cores.SetTableHeaders([]string{"Project", "Status", "Services", "Running", "Config"})
	cores.SetRefreshCallback(dv.refreshComposeData)
	cores.SetSelectionKey("Project")

	// Navigation key bindings
	cores.AddKeyBinding("C", "Containers", dv.showContainers)
	cores.AddKeyBinding("I", "Images", dv.showImages)
	cores.AddKeyBinding("N", "Networks", dv.showNetworks)
	cores.AddKeyBinding("V", "Volumes", dv.showVolumes)
	cores.AddKeyBinding("T", "Stats", dv.showStats)
	cores.AddKeyBinding("O", "Compose", dv.showCompose)
	cores.AddKeyBinding("Y", "System", dv.showSystem)

	// Compose action key bindings
	cores.AddKeyBinding("U", "Up", dv.composeUp)
	cores.AddKeyBinding("D", "Down", dv.composeDown)
	cores.AddKeyBinding("R", "Restart", dv.composeRestart)
	cores.AddKeyBinding("L", "Logs", dv.composeLogs)
	cores.AddKeyBinding("S", "Stop", dv.composeStop)
	cores.AddKeyBinding("?", "Help", dv.showHelp)

	cores.SetActionCallback(dv.handleAction)

	// Set row selection callback
	cores.SetRowSelectedCallback(func(row int) {
		tableData := cores.GetTableData()
		if row >= 0 && row < len(tableData) && len(tableData[row]) > 0 {
			cores.Log(fmt.Sprintf("[blue]Selected project: %s", tableData[row][0]))
		}
	})

	cores.RegisterHandlers()
	return cores
}

func (dv *DockerView) refreshComposeData() ([][]string, error) {
	if dv.dockerClient == nil {
		return [][]string{}, fmt.Errorf("docker client not initialized")
	}

	projects, err := dv.dockerClient.ListComposeProjects()
	if err != nil {
		return [][]string{}, err
	}

	rows := make([][]string, len(projects))
	for i, project := range projects {
		rows[i] = []string{
			project.Name,
			project.Status,
			fmt.Sprintf("%d", project.ServiceCount),
			fmt.Sprintf("%d", project.RunningCount),
			project.ConfigFile,
		}
	}

	if dv.currentHost != nil {
		dv.composeView.SetInfoText(fmt.Sprintf("[green]Docker Compose[white]\nHost: %s\nProjects: %d\nStatus: Connected",
			dv.currentHost.Name, len(projects)))
	}

	return rows, nil
}

func (dv *DockerView) getSelectedProjectName() (string, bool) {
	row := dv.composeView.GetSelectedRowData()
	if len(row) == 0 {
		return "", false
	}
	return row[0], true
}

func (dv *DockerView) composeUp() {
	name, ok := dv.getSelectedProjectName()
	if !ok {
		dv.composeView.Log("[yellow]No project selected")
		return
	}

	dv.composeView.Log(fmt.Sprintf("[yellow]Starting project %s...", name))

	go func() {
		if err := dv.dockerClient.ComposeUp(name); err != nil {
			dv.app.QueueUpdateDraw(func() {
				dv.composeView.Log(fmt.Sprintf("[red]Failed to start project: %v", err))
			})
		} else {
			dv.app.QueueUpdateDraw(func() {
				dv.composeView.Log(fmt.Sprintf("[green]Project %s started", name))
				dv.refresh()
			})
		}
	}()
}

func (dv *DockerView) composeDown() {
	name, ok := dv.getSelectedProjectName()
	if !ok {
		dv.composeView.Log("[yellow]No project selected")
		return
	}

	ui.ShowStandardConfirmationModal(
		dv.pages,
		dv.app,
		"Stop Project",
		fmt.Sprintf("Are you sure you want to stop project [red]%s[white]?\nThis will stop and remove all containers.", name),
		func(confirmed bool) {
			if confirmed {
				dv.composeView.Log(fmt.Sprintf("[yellow]Stopping project %s...", name))

				go func() {
					if err := dv.dockerClient.ComposeDown(name); err != nil {
						dv.app.QueueUpdateDraw(func() {
							dv.composeView.Log(fmt.Sprintf("[red]Failed to stop project: %v", err))
						})
					} else {
						dv.app.QueueUpdateDraw(func() {
							dv.composeView.Log(fmt.Sprintf("[green]Project %s stopped", name))
							dv.refresh()
						})
					}
				}()
			}
			dv.app.SetFocus(dv.composeView.GetTable())
		},
	)
}

func (dv *DockerView) composeStop() {
	name, ok := dv.getSelectedProjectName()
	if !ok {
		dv.composeView.Log("[yellow]No project selected")
		return
	}

	dv.composeView.Log(fmt.Sprintf("[yellow]Stopping project %s...", name))

	go func() {
		if err := dv.dockerClient.ComposeStop(name); err != nil {
			dv.app.QueueUpdateDraw(func() {
				dv.composeView.Log(fmt.Sprintf("[red]Failed to stop project: %v", err))
			})
		} else {
			dv.app.QueueUpdateDraw(func() {
				dv.composeView.Log(fmt.Sprintf("[green]Project %s stopped", name))
				dv.refresh()
			})
		}
	}()
}

func (dv *DockerView) composeRestart() {
	name, ok := dv.getSelectedProjectName()
	if !ok {
		dv.composeView.Log("[yellow]No project selected")
		return
	}

	dv.composeView.Log(fmt.Sprintf("[yellow]Restarting project %s...", name))

	go func() {
		if err := dv.dockerClient.ComposeRestart(name); err != nil {
			dv.app.QueueUpdateDraw(func() {
				dv.composeView.Log(fmt.Sprintf("[red]Failed to restart project: %v", err))
			})
		} else {
			dv.app.QueueUpdateDraw(func() {
				dv.composeView.Log(fmt.Sprintf("[green]Project %s restarted", name))
				dv.refresh()
			})
		}
	}()
}

func (dv *DockerView) composeLogs() {
	name, ok := dv.getSelectedProjectName()
	if !ok {
		dv.composeView.Log("[yellow]No project selected")
		return
	}

	dv.composeView.Log(fmt.Sprintf("[yellow]Fetching logs for %s...", name))

	logs, err := dv.dockerClient.ComposeLogs(name)
	if err != nil {
		dv.composeView.Log(fmt.Sprintf("[red]Failed to get logs: %v", err))
		return
	}

	ui.ShowInfoModal(
		dv.pages,
		dv.app,
		fmt.Sprintf("Logs: %s", name),
		logs,
		func() {
			dv.app.SetFocus(dv.composeView.GetTable())
		},
	)
}

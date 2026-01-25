package main

import (
	"fmt"
	"strings"

	"omo/pkg/ui"
)

func (dv *DockerView) newNetworksView() *ui.Cores {
	cores := ui.NewCores(dv.app, "Docker Networks")
	cores.SetTableHeaders([]string{"ID", "Name", "Driver", "Scope", "Subnet", "Gateway"})
	cores.SetRefreshCallback(dv.refreshNetworksData)
	cores.SetSelectionKey("ID")

	// Navigation key bindings
	cores.AddKeyBinding("C", "Containers", dv.showContainers)
	cores.AddKeyBinding("I", "Images", dv.showImages)
	cores.AddKeyBinding("N", "Networks", dv.showNetworks)
	cores.AddKeyBinding("V", "Volumes", dv.showVolumes)
	cores.AddKeyBinding("T", "Stats", dv.showStats)
	cores.AddKeyBinding("O", "Compose", dv.showCompose)
	cores.AddKeyBinding("Y", "System", dv.showSystem)

	// Network action key bindings
	cores.AddKeyBinding("D", "Delete", dv.removeSelectedNetwork)
	cores.AddKeyBinding("A", "Create", dv.createNetwork)
	cores.AddKeyBinding("?", "Help", dv.showHelp)

	cores.SetActionCallback(dv.handleAction)

	// Set row selection callback
	cores.SetRowSelectedCallback(func(row int) {
		tableData := cores.GetTableData()
		if row >= 0 && row < len(tableData) && len(tableData[row]) > 0 {
			cores.Log(fmt.Sprintf("[blue]Selected network: %s", tableData[row][1]))
		}
	})

	// Set Enter key to show network details
	cores.GetTable().SetSelectedFunc(func(row, column int) {
		if row <= 0 {
			return
		}
		dv.showNetworkDetails()
	})

	cores.RegisterHandlers()
	return cores
}

func (dv *DockerView) refreshNetworksData() ([][]string, error) {
	if dv.dockerClient == nil {
		return [][]string{}, fmt.Errorf("docker client not initialized")
	}

	networks, err := dv.dockerClient.ListNetworks()
	if err != nil {
		return [][]string{}, err
	}

	rows := make([][]string, len(networks))
	for i, network := range networks {
		shortID := network.ID
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}
		rows[i] = []string{
			shortID,
			network.Name,
			network.Driver,
			network.Scope,
			network.Subnet,
			network.Gateway,
		}
	}

	if dv.currentHost != nil {
		dv.networksView.SetInfoText(fmt.Sprintf("[green]Docker Networks[white]\nHost: %s\nNetworks: %d\nStatus: Connected",
			dv.currentHost.Name, len(networks)))
	}

	return rows, nil
}

func (dv *DockerView) getSelectedNetworkID() (string, bool) {
	row := dv.networksView.GetSelectedRowData()
	if len(row) == 0 {
		return "", false
	}
	return row[0], true
}

func (dv *DockerView) getSelectedNetworkName() string {
	row := dv.networksView.GetSelectedRowData()
	if len(row) < 2 {
		return ""
	}
	return row[1]
}

func (dv *DockerView) removeSelectedNetwork() {
	id, ok := dv.getSelectedNetworkID()
	if !ok {
		dv.networksView.Log("[yellow]No network selected")
		return
	}

	name := dv.getSelectedNetworkName()

	// Check if it's a built-in network
	if name == "bridge" || name == "host" || name == "none" {
		dv.networksView.Log("[red]Cannot remove built-in network")
		return
	}

	ui.ShowStandardConfirmationModal(
		dv.pages,
		dv.app,
		"Remove Network",
		fmt.Sprintf("Are you sure you want to remove network [red]%s[white]?", name),
		func(confirmed bool) {
			if confirmed {
				dv.networksView.Log(fmt.Sprintf("[yellow]Removing network %s...", name))
				if err := dv.dockerClient.RemoveNetwork(id); err != nil {
					dv.networksView.Log(fmt.Sprintf("[red]Failed to remove network: %v", err))
				} else {
					dv.networksView.Log(fmt.Sprintf("[yellow]Network %s removed", name))
					dv.refresh()
				}
			}
			dv.app.SetFocus(dv.networksView.GetTable())
		},
	)
}

func (dv *DockerView) createNetwork() {
	ui.ShowCompactStyledInputModal(
		dv.pages,
		dv.app,
		"Create Network",
		"Network Name",
		"",
		30,
		nil,
		func(networkName string, cancelled bool) {
			if cancelled || networkName == "" {
				dv.app.SetFocus(dv.networksView.GetTable())
				return
			}

			dv.networksView.Log(fmt.Sprintf("[yellow]Creating network %s...", networkName))

			if err := dv.dockerClient.CreateNetwork(networkName, "bridge"); err != nil {
				dv.networksView.Log(fmt.Sprintf("[red]Failed to create network: %v", err))
			} else {
				dv.networksView.Log(fmt.Sprintf("[green]Network %s created", networkName))
				dv.refresh()
			}

			dv.app.SetFocus(dv.networksView.GetTable())
		},
	)
}

func (dv *DockerView) showNetworkDetails() {
	id, ok := dv.getSelectedNetworkID()
	if !ok {
		return
	}

	name := dv.getSelectedNetworkName()
	dv.networksView.Log(fmt.Sprintf("[yellow]Inspecting network %s...", name))

	inspect, err := dv.dockerClient.InspectNetwork(id)
	if err != nil {
		dv.networksView.Log(fmt.Sprintf("[red]Failed to inspect network: %v", err))
		return
	}

	var details strings.Builder
	details.WriteString(fmt.Sprintf("[yellow]Network: %s[white]\n\n", name))
	details.WriteString(fmt.Sprintf("[green]ID:[white] %s\n", inspect.ID))
	details.WriteString(fmt.Sprintf("[green]Driver:[white] %s\n", inspect.Driver))
	details.WriteString(fmt.Sprintf("[green]Scope:[white] %s\n", inspect.Scope))
	details.WriteString(fmt.Sprintf("[green]Internal:[white] %v\n", inspect.Internal))
	details.WriteString(fmt.Sprintf("[green]Attachable:[white] %v\n", inspect.Attachable))
	details.WriteString(fmt.Sprintf("[green]IPv6 Enabled:[white] %v\n", inspect.EnableIPv6))

	if inspect.Subnet != "" {
		details.WriteString(fmt.Sprintf("[green]Subnet:[white] %s\n", inspect.Subnet))
	}
	if inspect.Gateway != "" {
		details.WriteString(fmt.Sprintf("[green]Gateway:[white] %s\n", inspect.Gateway))
	}

	if len(inspect.Containers) > 0 {
		details.WriteString(fmt.Sprintf("\n[yellow]Connected Containers:[white]\n"))
		for _, container := range inspect.Containers {
			details.WriteString(fmt.Sprintf("  %s\n", container))
		}
	}

	if len(inspect.Labels) > 0 {
		details.WriteString(fmt.Sprintf("\n[yellow]Labels:[white]\n"))
		for key, value := range inspect.Labels {
			details.WriteString(fmt.Sprintf("  %s: %s\n", key, value))
		}
	}

	ui.ShowInfoModal(
		dv.pages,
		dv.app,
		fmt.Sprintf("Network: %s", name),
		details.String(),
		func() {
			dv.app.SetFocus(dv.networksView.GetTable())
		},
	)
}

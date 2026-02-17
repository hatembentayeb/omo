package main

import (
	"fmt"
	"strconv"
	"time"

	"omo/pkg/ui"
)

// newNodesView creates the nodes CoreView
func (rv *RabbitMQView) newNodesView() *ui.CoreView {
	view := ui.NewCoreView(rv.app, "RabbitMQ Nodes")

	view.SetTableHeaders([]string{"Name", "Type", "Running", "Memory", "Disk Free", "FD Used", "Sockets", "Uptime"})

	view.SetRefreshCallback(func() ([][]string, error) {
		return rv.refreshNodes()
	})

	view.AddKeyBinding("R", "Refresh", rv.refresh)
	view.AddKeyBinding("?", "Help", rv.showHelp)
	view.AddKeyBinding("I", "Info", nil)
	view.AddKeyBinding("O", "Overview", nil)
	view.AddKeyBinding("Q", "Queues", nil)

	view.SetActionCallback(rv.handleAction)

	view.SetRowSelectedCallback(func(row int) {
		tableData := view.GetTableData()
		if row >= 0 && row < len(tableData) && len(tableData[row]) > 0 {
			view.Log(fmt.Sprintf("[blue]Selected node: %s", tableData[row][0]))
		}
	})

	view.RegisterHandlers()

	return view
}

func (rv *RabbitMQView) refreshNodes() ([][]string, error) {
	if rv.rmqClient == nil || !rv.rmqClient.IsConnected() {
		return [][]string{{"Not connected", "", "", "", "", "", "", ""}}, nil
	}

	nodes, err := rv.rmqClient.GetNodes()
	if err != nil {
		return [][]string{{"Error: " + err.Error(), "", "", "", "", "", "", ""}}, nil
	}

	if len(nodes) == 0 {
		return [][]string{{"No nodes found", "", "", "", "", "", "", ""}}, nil
	}

	tableData := make([][]string, len(nodes))
	for i, n := range nodes {
		runningStr := boolStr(n.Running)
		uptime := formatDuration(time.Duration(n.Uptime) * time.Millisecond)
		fdUsed := fmt.Sprintf("%d/%d", n.FDUsed, n.FDTotal)
		sockUsed := fmt.Sprintf("%d/%d", n.SocketsUsed, n.SocketsTotal)

		tableData[i] = []string{
			n.Name,
			n.Type,
			runningStr,
			formatBytes(n.MemUsed),
			formatBytes(n.DiskFree),
			fdUsed,
			sockUsed,
			uptime,
		}
	}

	name := rv.rmqClient.GetClusterName()
	rv.nodesView.SetInfoText(fmt.Sprintf("[green]RabbitMQ Manager[white]\nInstance: %s\nNodes: %d", name, len(nodes)))

	return tableData, nil
}

func (rv *RabbitMQView) showNodeInfo() {
	selectedRow := rv.nodesView.GetSelectedRow()
	tableData := rv.nodesView.GetTableData()
	if selectedRow < 0 || selectedRow >= len(tableData) {
		rv.nodesView.Log("[red]No node selected")
		return
	}

	nodeName := tableData[selectedRow][0]

	nodes, err := rv.rmqClient.GetNodes()
	if err != nil {
		rv.nodesView.Log(fmt.Sprintf("[red]Failed to fetch node info: %v", err))
		return
	}

	var node *NodeInfo
	for i := range nodes {
		if nodes[i].Name == nodeName {
			node = &nodes[i]
			break
		}
	}

	if node == nil {
		rv.nodesView.Log(fmt.Sprintf("[red]Node %s not found", nodeName))
		return
	}

	memPercent := float64(0)
	if node.MemLimit > 0 {
		memPercent = float64(node.MemUsed) / float64(node.MemLimit) * 100
	}

	infoText := fmt.Sprintf(`[yellow]Node Details[white]

[aqua]Name:[white]            %s
[aqua]Type:[white]            %s
[aqua]Running:[white]         %s
[aqua]Uptime:[white]          %s
[aqua]Rates Mode:[white]      %s

[yellow]Memory[white]
[aqua]Used:[white]            %s
[aqua]Limit:[white]           %s
[aqua]Usage:[white]           %.1f%%

[yellow]Disk[white]
[aqua]Free:[white]            %s
[aqua]Limit:[white]           %s

[yellow]File Descriptors[white]
[aqua]Used:[white]            %d
[aqua]Total:[white]           %d

[yellow]Sockets[white]
[aqua]Used:[white]            %d
[aqua]Total:[white]           %d

[yellow]Processes[white]
[aqua]Used:[white]            %d
[aqua]Total:[white]           %d`,
		node.Name, node.Type, boolStr(node.Running),
		formatDuration(time.Duration(node.Uptime)*time.Millisecond),
		node.RatesMode,
		formatBytes(node.MemUsed), formatBytes(node.MemLimit), memPercent,
		formatBytes(node.DiskFree), formatBytes(node.DiskFreeLimit),
		node.FDUsed, node.FDTotal,
		node.SocketsUsed, node.SocketsTotal,
		node.ProcUsed, node.ProcTotal)

	ui.ShowInfoModal(
		rv.pages,
		rv.app,
		fmt.Sprintf("Node: %s", nodeName),
		infoText,
		func() { rv.app.SetFocus(rv.nodesView.GetTable()) },
	)
}

func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return strconv.Itoa(minutes) + "m"
}

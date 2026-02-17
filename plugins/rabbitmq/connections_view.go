package main

import (
	"fmt"
	"strconv"
	"time"

	"omo/pkg/ui"
)

// newConnectionsView creates the connections CoreView
func (rv *RabbitMQView) newConnectionsView() *ui.CoreView {
	view := ui.NewCoreView(rv.app, "RabbitMQ Connections")

	view.SetTableHeaders([]string{"Name", "User", "VHost", "State", "Channels", "Peer", "SSL"})

	view.SetRefreshCallback(func() ([][]string, error) {
		return rv.refreshConnections()
	})

	view.AddKeyBinding("R", "Refresh", rv.refresh)
	view.AddKeyBinding("?", "Help", rv.showHelp)
	view.AddKeyBinding("I", "Info", nil)
	view.AddKeyBinding("D", "Close Conn", nil)
	view.AddKeyBinding("O", "Overview", nil)
	view.AddKeyBinding("Q", "Queues", nil)
	view.AddKeyBinding("H", "Channels", nil)

	view.SetActionCallback(rv.handleAction)

	view.SetRowSelectedCallback(func(row int) {
		tableData := view.GetTableData()
		if row >= 0 && row < len(tableData) && len(tableData[row]) > 0 {
			view.Log(fmt.Sprintf("[blue]Selected connection: %s", tableData[row][0]))
		}
	})

	view.RegisterHandlers()

	return view
}

func (rv *RabbitMQView) refreshConnections() ([][]string, error) {
	if rv.rmqClient == nil || !rv.rmqClient.IsConnected() {
		return [][]string{{"Not connected", "", "", "", "", "", ""}}, nil
	}

	connections, err := rv.rmqClient.GetConnections()
	if err != nil {
		return [][]string{{"Error: " + err.Error(), "", "", "", "", "", ""}}, nil
	}

	if len(connections) == 0 {
		return [][]string{{"No active connections", "", "", "", "", "", ""}}, nil
	}

	tableData := make([][]string, len(connections))
	for i, c := range connections {
		sslStr := boolStr(c.SSL)
		peer := fmt.Sprintf("%s:%d", c.PeerHost, c.PeerPort)

		tableData[i] = []string{
			c.Name,
			c.User,
			c.VHost,
			c.State,
			strconv.Itoa(c.Channels),
			peer,
			sslStr,
		}
	}

	name := rv.rmqClient.GetClusterName()
	rv.connectionsView.SetInfoText(fmt.Sprintf("[green]RabbitMQ Manager[white]\nInstance: %s\nConnections: %d", name, len(connections)))

	return tableData, nil
}

func (rv *RabbitMQView) showConnectionInfo() {
	selectedRow := rv.connectionsView.GetSelectedRow()
	tableData := rv.connectionsView.GetTableData()
	if selectedRow < 0 || selectedRow >= len(tableData) {
		rv.connectionsView.Log("[red]No connection selected")
		return
	}

	connName := tableData[selectedRow][0]

	connections, err := rv.rmqClient.GetConnections()
	if err != nil {
		rv.connectionsView.Log(fmt.Sprintf("[red]Failed to fetch connection info: %v", err))
		return
	}

	var conn *ConnectionInfo
	for i := range connections {
		if connections[i].Name == connName {
			conn = &connections[i]
			break
		}
	}

	if conn == nil {
		rv.connectionsView.Log(fmt.Sprintf("[red]Connection %s not found", connName))
		return
	}

	connectedAt := "Unknown"
	if conn.Connected > 0 {
		connectedAt = time.Unix(conn.Connected/1000, 0).Format("2006-01-02 15:04:05")
	}

	infoText := fmt.Sprintf(`[yellow]Connection Details[white]

[aqua]Name:[white]      %s
[aqua]Node:[white]      %s
[aqua]User:[white]      %s
[aqua]VHost:[white]     %s
[aqua]State:[white]     %s
[aqua]Protocol:[white]  %s
[aqua]Peer:[white]      %s:%d
[aqua]Channels:[white]  %d
[aqua]SSL:[white]       %s
[aqua]Connected:[white] %s

[yellow]Traffic[white]
[aqua]Received:[white]  %s
[aqua]Sent:[white]      %s`,
		conn.Name, conn.Node, conn.User, conn.VHost,
		conn.State, conn.Protocol,
		conn.PeerHost, conn.PeerPort,
		conn.Channels, boolStr(conn.SSL),
		connectedAt,
		formatBytes(conn.RecvOct), formatBytes(conn.SendOct))

	ui.ShowInfoModal(
		rv.pages,
		rv.app,
		"Connection Info",
		infoText,
		func() { rv.app.SetFocus(rv.connectionsView.GetTable()) },
	)
}

func (rv *RabbitMQView) closeSelectedConnection() {
	selectedRow := rv.connectionsView.GetSelectedRow()
	tableData := rv.connectionsView.GetTableData()
	if selectedRow < 0 || selectedRow >= len(tableData) {
		rv.connectionsView.Log("[red]No connection selected")
		return
	}

	connName := tableData[selectedRow][0]

	ui.ShowStandardConfirmationModal(
		rv.pages,
		rv.app,
		"Close Connection",
		fmt.Sprintf("Force close connection '[red]%s[white]'?", connName),
		func(confirmed bool) {
			if confirmed {
				if err := rv.rmqClient.CloseConnection(connName); err != nil {
					rv.connectionsView.Log(fmt.Sprintf("[red]Failed to close connection: %v", err))
				} else {
					rv.connectionsView.Log(fmt.Sprintf("[yellow]Closed connection: %s", connName))
					rv.refresh()
				}
			}
			rv.app.SetFocus(rv.connectionsView.GetTable())
		},
	)
}

package main

import (
	"fmt"
	"strconv"

	"omo/pkg/ui"
)

func (pv *PostgresView) newConnectionsView() *ui.CoreView {
	cores := ui.NewCoreView(pv.app, "PostgreSQL Active Connections")
	cores.SetTableHeaders([]string{"PID", "User", "Database", "Client", "State", "Backend", "Wait", "Duration", "Query"})
	cores.SetRefreshCallback(pv.refreshConnections)
	cores.AddKeyBinding("R", "Refresh", pv.refresh)
	cores.AddKeyBinding("?", "Help", pv.showHelp)
	cores.AddKeyBinding("K", "Kill Conn", pv.showTerminateConnectionConfirmation)
	cores.AddKeyBinding("C", "Cancel Query", pv.showCancelQueryConfirmation)
	pv.addCommonBindings(cores)
	cores.SetActionCallback(pv.handleAction)
	cores.RegisterHandlers()
	return cores
}

func (pv *PostgresView) refreshConnections() ([][]string, error) {
	if pv.pgClient == nil || !pv.pgClient.IsConnected() {
		return [][]string{{"Status", "Not Connected", "", "", "", "", "", "", ""}}, nil
	}

	conns, err := pv.pgClient.GetActiveConnections()
	if err != nil {
		return [][]string{}, err
	}

	data := make([][]string, 0, len(conns))
	for _, conn := range conns {
		query := conn.Query
		if len(query) > 80 {
			query = query[:80] + "..."
		}
		data = append(data, []string{
			fmt.Sprintf("%d", conn.PID),
			conn.User,
			conn.Database,
			conn.ClientAddr,
			conn.State,
			conn.BackendType,
			conn.WaitEvent,
			conn.Duration,
			query,
		})
	}

	if len(data) == 0 {
		data = append(data, []string{"-", "-", "-", "-", "-", "-", "-", "-", "No active connections"})
	}

	return data, nil
}

func (pv *PostgresView) selectedConnectionPID() (int, bool) {
	row := pv.connectionsView.GetSelectedRowData()
	if len(row) == 0 {
		return 0, false
	}
	pid, err := strconv.Atoi(row[0])
	if err != nil {
		return 0, false
	}
	return pid, true
}

func (pv *PostgresView) showTerminateConnectionConfirmation() {
	pid, ok := pv.selectedConnectionPID()
	if !ok {
		pv.connectionsView.Log("[yellow]No connection selected")
		return
	}

	ui.ShowStandardConfirmationModal(
		pv.pages, pv.app, "Terminate Connection",
		fmt.Sprintf("Are you sure you want to [red]terminate[white] connection PID [red]%d[white]?", pid),
		func(confirmed bool) {
			if confirmed {
				if err := pv.pgClient.TerminateConnection(pid); err != nil {
					pv.connectionsView.Log(fmt.Sprintf("[red]Failed to terminate: %v", err))
				} else {
					pv.connectionsView.Log(fmt.Sprintf("[yellow]Terminated PID: %d", pid))
					pv.refresh()
				}
			}
			pv.app.SetFocus(pv.connectionsView.GetTable())
		},
	)
}

func (pv *PostgresView) showCancelQueryConfirmation() {
	pid, ok := pv.selectedConnectionPID()
	if !ok {
		pv.connectionsView.Log("[yellow]No connection selected")
		return
	}

	ui.ShowStandardConfirmationModal(
		pv.pages, pv.app, "Cancel Query",
		fmt.Sprintf("Cancel the running query on PID [yellow]%d[white]?", pid),
		func(confirmed bool) {
			if confirmed {
				if err := pv.pgClient.CancelQuery(pid); err != nil {
					pv.connectionsView.Log(fmt.Sprintf("[red]Failed to cancel: %v", err))
				} else {
					pv.connectionsView.Log(fmt.Sprintf("[yellow]Cancelled query on PID: %d", pid))
					pv.refresh()
				}
			}
			pv.app.SetFocus(pv.connectionsView.GetTable())
		},
	)
}

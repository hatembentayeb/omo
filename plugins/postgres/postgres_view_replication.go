package main

import (
	"fmt"

	"omo/pkg/ui"
)

func (pv *PostgresView) newReplicationView() *ui.CoreView {
	cores := ui.NewCoreView(pv.app, "PostgreSQL Replication")
	cores.SetTableHeaders([]string{"PID", "Application", "Client", "State", "Sent LSN", "Write LSN", "Flush LSN", "Replay LSN", "Write Lag", "Flush Lag", "Replay Lag"})
	cores.SetRefreshCallback(pv.refreshReplication)
	cores.AddKeyBinding("R", "Refresh", pv.refresh)
	cores.AddKeyBinding("?", "Help", pv.showHelp)
	pv.addCommonBindings(cores)
	cores.SetActionCallback(pv.handleAction)
	cores.RegisterHandlers()
	return cores
}

func (pv *PostgresView) refreshReplication() ([][]string, error) {
	if pv.pgClient == nil || !pv.pgClient.IsConnected() {
		return [][]string{{"Status", "", "Not Connected", "", "", "", "", "", "", "", ""}}, nil
	}

	replicas, err := pv.pgClient.GetReplicationStatus()
	if err != nil {
		return [][]string{}, err
	}

	data := make([][]string, 0, len(replicas))
	for _, r := range replicas {
		data = append(data, []string{
			fmt.Sprintf("%d", r.PID),
			r.Application,
			r.ClientAddr,
			r.State,
			r.SentLSN,
			r.WriteLSN,
			r.FlushLSN,
			r.ReplayLSN,
			r.WriteLag,
			r.FlushLag,
			r.ReplayLag,
		})
	}

	if len(data) == 0 {
		data = append(data, []string{"-", "-", "-", "No replicas", "-", "-", "-", "-", "-", "-", "-"})
	}

	return data, nil
}

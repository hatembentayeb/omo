package main

import (
	"omo/pkg/ui"
)

func (pv *PostgresView) newDbStatsView() *ui.CoreView {
	cores := ui.NewCoreView(pv.app, "PostgreSQL Database Stats")
	cores.SetTableHeaders([]string{
		"Database", "Backends", "Commits", "Rollbacks",
		"Blks Read", "Blks Hit", "Cache Hit%",
		"Returned", "Fetched", "Inserted", "Updated", "Deleted",
		"Conflicts", "Deadlocks",
	})
	cores.SetRefreshCallback(pv.refreshDbStats)
	cores.AddKeyBinding("R", "Refresh", pv.refresh)
	cores.AddKeyBinding("?", "Help", pv.showHelp)
	pv.addCommonBindings(cores)
	cores.SetActionCallback(pv.handleAction)
	cores.RegisterHandlers()
	return cores
}

func (pv *PostgresView) refreshDbStats() ([][]string, error) {
	if pv.pgClient == nil || !pv.pgClient.IsConnected() {
		empty := make([]string, 14)
		empty[0] = "Not Connected"
		return [][]string{empty}, nil
	}

	data, err := pv.pgClient.GetDatabaseStats()
	if err != nil {
		return [][]string{}, err
	}

	if len(data) == 0 {
		empty := make([]string, 14)
		empty[0] = "No data"
		data = append(data, empty)
	}

	return data, nil
}

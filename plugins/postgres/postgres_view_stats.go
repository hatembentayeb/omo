package main

import (
	"omo/pkg/ui"
)

func (pv *PostgresView) newStatsView() *ui.CoreView {
	cores := ui.NewCoreView(pv.app, "PostgreSQL Server Stats")
	cores.SetTableHeaders([]string{"Property", "Value"})
	cores.SetRefreshCallback(pv.refreshStats)
	cores.AddKeyBinding("R", "Refresh", pv.refresh)
	cores.AddKeyBinding("?", "Help", pv.showHelp)
	pv.addCommonBindings(cores)
	cores.SetActionCallback(pv.handleAction)
	cores.RegisterHandlers()
	return cores
}

func (pv *PostgresView) refreshStats() ([][]string, error) {
	if pv.pgClient == nil || !pv.pgClient.IsConnected() {
		return [][]string{{"Status", "Not Connected"}}, nil
	}

	stats, err := pv.pgClient.GetServerInfo()
	if err != nil {
		return [][]string{}, err
	}

	data := make([][]string, 0, len(stats))
	for _, s := range stats {
		data = append(data, []string{s.Key, s.Value})
	}

	return data, nil
}

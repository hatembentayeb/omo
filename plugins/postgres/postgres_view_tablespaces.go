package main

import (
	"omo/pkg/ui"
)

func (pv *PostgresView) newTablespacesView() *ui.CoreView {
	cores := ui.NewCoreView(pv.app, "PostgreSQL Tablespaces")
	cores.SetTableHeaders([]string{"Name", "Owner", "Location", "Size"})
	cores.SetRefreshCallback(pv.refreshTablespaces)
	cores.AddKeyBinding("R", "Refresh", pv.refresh)
	cores.AddKeyBinding("?", "Help", pv.showHelp)
	pv.addCommonBindings(cores)
	cores.SetActionCallback(pv.handleAction)
	cores.RegisterHandlers()
	return cores
}

func (pv *PostgresView) refreshTablespaces() ([][]string, error) {
	if pv.pgClient == nil || !pv.pgClient.IsConnected() {
		return [][]string{{"Status", "Not Connected", "", ""}}, nil
	}

	tablespaces, err := pv.pgClient.GetTablespaces()
	if err != nil {
		return [][]string{}, err
	}

	data := make([][]string, 0, len(tablespaces))
	for _, ts := range tablespaces {
		data = append(data, []string{ts.Name, ts.Owner, ts.Location, ts.Size})
	}

	if len(data) == 0 {
		data = append(data, []string{"-", "-", "-", "No tablespaces"})
	}

	return data, nil
}

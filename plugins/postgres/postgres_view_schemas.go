package main

import (
	"omo/pkg/ui"
)

func (pv *PostgresView) newSchemasView() *ui.CoreView {
	cores := ui.NewCoreView(pv.app, "PostgreSQL Schemas")
	cores.SetTableHeaders([]string{"Schema", "Owner"})
	cores.SetRefreshCallback(pv.refreshSchemas)
	cores.AddKeyBinding("R", "Refresh", pv.refresh)
	cores.AddKeyBinding("?", "Help", pv.showHelp)
	pv.addCommonBindings(cores)
	cores.SetActionCallback(pv.handleAction)
	cores.RegisterHandlers()
	return cores
}

func (pv *PostgresView) refreshSchemas() ([][]string, error) {
	if pv.pgClient == nil || !pv.pgClient.IsConnected() {
		return [][]string{{"Status", "Not Connected"}}, nil
	}

	schemas, err := pv.pgClient.GetSchemas()
	if err != nil {
		return [][]string{}, err
	}

	data := make([][]string, 0, len(schemas))
	for _, s := range schemas {
		data = append(data, []string{s.Name, s.Owner})
	}

	if len(data) == 0 {
		data = append(data, []string{"-", "No schemas"})
	}

	return data, nil
}

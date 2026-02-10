package main

import (
	"fmt"

	"omo/pkg/ui"
)

func (pv *PostgresView) newConfigView() *ui.CoreView {
	cores := ui.NewCoreView(pv.app, "PostgreSQL Configuration")
	cores.SetTableHeaders([]string{"Parameter", "Value", "Unit", "Category", "Source", "Boot Value", "Restart?"})
	cores.SetRefreshCallback(pv.refreshConfig)
	cores.AddKeyBinding("R", "Refresh", pv.refresh)
	cores.AddKeyBinding("?", "Help", pv.showHelp)
	cores.AddKeyBinding("E", "Edit Param", pv.showEditConfigForm)
	pv.addCommonBindings(cores)
	cores.SetActionCallback(pv.handleAction)
	cores.RegisterHandlers()
	return cores
}

func (pv *PostgresView) refreshConfig() ([][]string, error) {
	if pv.pgClient == nil || !pv.pgClient.IsConnected() {
		return [][]string{{"Status", "", "", "Not Connected", "", "", ""}}, nil
	}

	configs, err := pv.pgClient.GetConfig("%")
	if err != nil {
		return [][]string{}, err
	}

	data := make([][]string, 0, len(configs))
	for _, cfg := range configs {
		restart := "[green]No"
		if cfg.PendRestart {
			restart = "[red]Yes"
		}
		data = append(data, []string{
			cfg.Name,
			cfg.Setting,
			cfg.Unit,
			cfg.Category,
			cfg.Source,
			cfg.BootVal,
			restart,
		})
	}

	if len(data) == 0 {
		data = append(data, []string{"-", "-", "-", "-", "-", "-", "No settings"})
	}

	return data, nil
}

func (pv *PostgresView) showEditConfigForm() {
	row := pv.configView.GetSelectedRowData()
	if len(row) < 2 {
		pv.configView.Log("[yellow]No parameter selected")
		return
	}
	paramName := row[0]
	currentValue := row[1]

	ui.ShowCompactStyledInputModal(
		pv.pages, pv.app,
		fmt.Sprintf("Edit: %s", paramName),
		"Value",
		currentValue,
		30,
		nil,
		func(value string, cancelled bool) {
			if cancelled {
				pv.app.SetFocus(pv.configView.GetTable())
				return
			}
			if err := pv.pgClient.AlterConfig(paramName, value); err != nil {
				pv.configView.Log(fmt.Sprintf("[red]Failed to set %s: %v", paramName, err))
			} else {
				pv.configView.Log(fmt.Sprintf("[green]Set %s = %s (reloaded)", paramName, value))
				pv.refresh()
			}
			pv.app.SetFocus(pv.configView.GetTable())
		},
	)
}

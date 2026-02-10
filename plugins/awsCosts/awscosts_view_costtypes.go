package main

import (
	"omo/pkg/ui"
)

func (av *AWSCostsView) newCostTypesView() *ui.CoreView {
	cores := ui.NewCoreView(av.app, "Cost Type Settings")
	cores.SetTableHeaders([]string{"Setting", "Status", "Description"})
	cores.SetRefreshCallback(av.refreshCostTypesData)

	// Key bindings
	cores.AddKeyBinding("M", "Main", av.showMainView)
	cores.AddKeyBinding("S", "Services", av.showServicesView)
	cores.AddKeyBinding("B", "Budgets", av.showBudgetsView)
	cores.AddKeyBinding("T", "Cost Types", av.showCostTypesView)
	cores.AddKeyBinding("F", "Forecast", av.showForecastView)
	cores.AddKeyBinding("R", "Refresh", func() { av.refresh() })
	cores.AddKeyBinding("?", "Help", av.showHelp)

	cores.SetActionCallback(av.handleAction)
	cores.RegisterHandlers()
	return cores
}

func (av *AWSCostsView) refreshCostTypesData() ([][]string, error) {
	result := [][]string{
		{"Include Tax", "Enabled", "Include tax amounts in cost calculation"},
		{"Include Credits", "Enabled", "Include AWS credits and refunds"},
		{"Include Upfront", "Enabled", "Include upfront costs for reserved instances"},
		{"Include Recurring", "Enabled", "Include recurring costs for reserved instances"},
		{"Include Other Subscriptions", "Disabled", "Include other subscription costs"},
		{"Use Blended Costs", "Enabled", "Use blended costs for organizations"},
		{"Include Support", "Enabled", "Include AWS support costs"},
	}

	av.costTypesView.SetInfoText("[green]Cost Type Configuration[white]\nConfigure which cost types to include in calculations")

	return result, nil
}

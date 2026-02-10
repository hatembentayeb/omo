package main

import (
	"fmt"

	"omo/pkg/ui"
)

func (av *AWSCostsView) newServicesView() *ui.CoreView {
	cores := ui.NewCoreView(av.app, "Service Breakdown")
	cores.SetTableHeaders([]string{"Service", "Cost", "Percentage", "Distribution"})
	cores.SetRefreshCallback(av.refreshServicesData)

	// Key bindings
	cores.AddKeyBinding("M", "Main", av.showMainView)
	cores.AddKeyBinding("S", "Services", av.showServicesView)
	cores.AddKeyBinding("B", "Budgets", av.showBudgetsView)
	cores.AddKeyBinding("T", "Cost Types", av.showCostTypesView)
	cores.AddKeyBinding("F", "Forecast", av.showForecastView)
	cores.AddKeyBinding("P", "Time Period", av.showTimePeriodSelector)
	cores.AddKeyBinding("G", "Granularity", av.toggleGranularity)
	cores.AddKeyBinding("R", "Refresh", func() { av.refreshCostData() })
	cores.AddKeyBinding("?", "Help", av.showHelp)

	cores.SetActionCallback(av.handleAction)
	cores.RegisterHandlers()
	return cores
}

func (av *AWSCostsView) refreshServicesData() ([][]string, error) {
	if av.costData == nil || len(av.costData) == 0 {
		return [][]string{{"No data", "", "", ""}}, nil
	}

	totalCost := 0.0
	for _, cost := range av.costData {
		totalCost += cost.cost
	}

	result := make([][]string, 0, len(av.costData)+1)
	for _, cost := range av.costData {
		percentage := 0.0
		if totalCost > 0 {
			percentage = (cost.cost / totalCost) * 100
		}

		bars := int(percentage / 5)
		if bars > 20 {
			bars = 20
		}

		chart := ""
		for i := 0; i < bars; i++ {
			chart += "█"
		}

		result = append(result, []string{
			cost.service,
			fmt.Sprintf("$%.2f", cost.cost),
			fmt.Sprintf("%.1f%%", percentage),
			chart,
		})
	}

	result = append(result, []string{
		"TOTAL",
		fmt.Sprintf("$%.2f", totalCost),
		"100%",
		"",
	})

	av.servicesView.SetInfoText(fmt.Sprintf("[green]Service Breakdown[white]\nProfile: %s | %s | %s\nTotal: $%.2f",
		av.profile, av.timeRange, av.granularity, totalCost))

	return result, nil
}

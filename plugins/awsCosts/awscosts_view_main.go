package main

import (
	"fmt"

	"omo/pkg/ui"
)

func (av *AWSCostsView) newMainView() *ui.CoreView {
	cores := ui.NewCoreView(av.app, "AWS Cost Explorer")
	cores.SetTableHeaders([]string{"Service", "Cost", "Trend", "Chart", "Forecast", "Budget Status"})
	cores.SetRefreshCallback(av.refreshMainData)

	// Key bindings - view navigation
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

func (av *AWSCostsView) refreshMainData() ([][]string, error) {
	if av.costData == nil || len(av.costData) == 0 {
		return [][]string{{"No data loaded", "Use Ctrl+T to select profile", "", "", "", ""}}, nil
	}

	result := make([][]string, 0, len(av.costData))

	for _, cost := range av.costData {
		chartBar := av.createBarChart(cost.cost, cost.budget)

		trendFormatted := fmt.Sprintf("%.1f%%", cost.trend)
		if cost.trend > 0 {
			trendFormatted = "↑ " + trendFormatted
		} else if cost.trend < 0 {
			trendFormatted = "↓ " + trendFormatted
		}

		budgetStatus := "N/A"
		if cost.budget > 0 {
			percentage := (cost.cost / cost.budget) * 100
			if percentage >= 100 {
				budgetStatus = fmt.Sprintf("⚠️ %.1f%%", percentage)
			} else if percentage >= 80 {
				budgetStatus = fmt.Sprintf("⚠ %.1f%%", percentage)
			} else {
				budgetStatus = fmt.Sprintf("✓ %.1f%%", percentage)
			}
		}

		result = append(result, []string{
			cost.service,
			fmt.Sprintf("$%.2f %s", cost.cost, cost.unit),
			trendFormatted,
			chartBar,
			fmt.Sprintf("$%.2f", cost.forecast),
			budgetStatus,
		})
	}

	av.mainView.SetInfoText(fmt.Sprintf("[green]AWS Cost Explorer[white]\nProfile: %s\nRegion: %s\n%s | %s",
		av.profile, av.region, av.timeRange, av.granularity))

	return result, nil
}

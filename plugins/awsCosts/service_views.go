package awscosts

import (
	"fmt"
	"time"

	"omo/pkg/pluginrpc"

	"github.com/aws/aws-sdk-go/aws"
)

func navBindings() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "M", Label: "Main", Action: "goto_main"},
		{Key: "S", Label: "Services", Action: "goto_services"},
		{Key: "B", Label: "Budgets", Action: "goto_budgets"},
		{Key: "T", Label: "Cost Types", Action: "goto_costtypes"},
		{Key: "F", Label: "Forecast", Action: "goto_forecast"},
	}
}

func withNav(extra ...pluginrpc.KeyBinding) []pluginrpc.KeyBinding {
	out := make([]pluginrpc.KeyBinding, 0, len(extra)+len(navBindings())+1)
	out = append(out, pluginrpc.KeyBinding{Key: "R", Label: "Refresh", Action: "refresh"})
	out = append(out, extra...)
	out = append(out, navBindings()...)
	return out
}

func (s *Service) baseInfo(extra string) string {
	msg := fmt.Sprintf("[green]AWS Cost Explorer[white]\nProfile: %s\nRegion: %s\n%s | %s\nView: %s",
		s.profile, s.region, s.timeRange, s.granularity, s.currentView)
	if extra != "" {
		msg += "\n" + extra
	}
	return msg
}

func (s *Service) buildViewLocked(viewID string) (pluginrpc.ViewData, error) {
	if viewID == "" {
		viewID = awsViewMain
	}
	s.currentView = viewID

	if err := s.ensureClientLocked(); err != nil {
		return pluginrpc.ViewData{
			View:    viewID,
			Title:   "AWS Cost Explorer",
			Info:    "[yellow]AWS Cost Explorer[white]\nStatus: Not Connected\n" + err.Error(),
			Status:  "not connected",
			Headers: []string{"Status", "Detail"},
			Rows:    [][]string{{"error", err.Error()}},
			KeyBindings: []pluginrpc.KeyBinding{
				{Key: "R", Label: "Refresh", Action: "refresh"},
			},
		}, nil
	}

	switch viewID {
	case awsViewServices:
		return s.viewServicesLocked()
	case awsViewBudgets:
		return s.viewBudgetsLocked()
	case awsViewCostTypes:
		return s.viewCostTypesLocked()
	case awsViewForecast:
		return s.viewForecastLocked()
	default:
		return s.viewMainLocked()
	}
}

func (s *Service) viewMainLocked() (pluginrpc.ViewData, error) {
	if err := s.loadCostDataLocked(); err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(s.costData))
	for _, cost := range s.costData {
		chartBar := createBarChart(cost.cost, cost.budget, s.costData)
		trendFormatted := fmt.Sprintf("%.1f%%", cost.trend)
		if cost.trend > 0 {
			trendFormatted = "↑ " + trendFormatted
		} else if cost.trend < 0 {
			trendFormatted = "↓ " + trendFormatted
		}
		budgetStatus := "N/A"
		if cost.budget > 0 {
			percentage := (cost.cost / cost.budget) * 100
			budgetStatus = fmt.Sprintf("%.1f%%", percentage)
		}
		rows = append(rows, []string{
			cost.service,
			fmt.Sprintf("$%.2f %s", cost.cost, cost.unit),
			trendFormatted,
			chartBar,
			fmt.Sprintf("$%.2f", cost.forecast),
			budgetStatus,
		})
	}
	if len(rows) == 0 {
		rows = [][]string{{"No data loaded", "", "", "", "", ""}}
	}
	return pluginrpc.ViewData{
		View:         awsViewMain,
		Title:        "AWS Cost Explorer",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"Service", "Cost", "Trend", "Chart", "Forecast", "Budget Status"},
		Rows:         rows,
		SelectionKey: "Service",
		KeyBindings: withNav(
			pluginrpc.KeyBinding{Key: "P", Label: "Time Period", Action: "set_time_period"},
			pluginrpc.KeyBinding{Key: "G", Label: "Granularity", Action: "toggle_granularity"},
		),
	}, nil
}

func (s *Service) viewServicesLocked() (pluginrpc.ViewData, error) {
	if err := s.loadCostDataLocked(); err != nil {
		return pluginrpc.ViewData{}, err
	}
	totalCost := 0.0
	for _, cost := range s.costData {
		totalCost += cost.cost
	}
	rows := make([][]string, 0, len(s.costData)+1)
	for _, cost := range s.costData {
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
		rows = append(rows, []string{
			cost.service,
			fmt.Sprintf("$%.2f", cost.cost),
			fmt.Sprintf("%.1f%%", percentage),
			chart,
		})
	}
	rows = append(rows, []string{"TOTAL", fmt.Sprintf("$%.2f", totalCost), "100%", ""})
	return pluginrpc.ViewData{
		View:         awsViewServices,
		Title:        "Service Breakdown",
		Info:         s.baseInfo(fmt.Sprintf("Total: $%.2f", totalCost)),
		Status:       "connected",
		Headers:      []string{"Service", "Cost", "Percentage", "Distribution"},
		Rows:         rows,
		SelectionKey: "Service",
		KeyBindings: withNav(
			pluginrpc.KeyBinding{Key: "P", Label: "Time Period", Action: "set_time_period"},
			pluginrpc.KeyBinding{Key: "G", Label: "Granularity", Action: "toggle_granularity"},
		),
	}, nil
}

func (s *Service) viewBudgetsLocked() (pluginrpc.ViewData, error) {
	if err := s.ensureClientLocked(); err != nil {
		return pluginrpc.ViewData{}, err
	}
	result, err := s.client.GetBudgets()
	if err != nil {
		return pluginrpc.ViewData{
			View:         awsViewBudgets,
			Title:        "AWS Budgets",
			Info:         s.baseInfo(""),
			Status:       "connected",
			Headers:      []string{"Name", "Amount", "Period", "Used", "Remaining", "Status"},
			Rows:         [][]string{{"Error fetching budgets", err.Error(), "", "", "", ""}},
			SelectionKey: "Name",
			KeyBindings:  withNav(),
		}, nil
	}
	rows := make([][]string, 0, len(result.Budgets))
	for _, b := range result.Budgets {
		rows = append(rows, formatBudgetRow(b))
	}
	if len(rows) == 0 {
		rows = [][]string{{"No budgets found", "Configure budgets in AWS Console", "", "", "", ""}}
	}
	return pluginrpc.ViewData{
		View:         awsViewBudgets,
		Title:        "AWS Budgets",
		Info:         s.baseInfo(fmt.Sprintf("Budgets: %d", len(result.Budgets))),
		Status:       "connected",
		Headers:      []string{"Name", "Amount", "Period", "Used", "Remaining", "Status"},
		Rows:         rows,
		SelectionKey: "Name",
		KeyBindings:  withNav(),
	}, nil
}

func (s *Service) viewCostTypesLocked() (pluginrpc.ViewData, error) {
	rows := [][]string{
		{"Include Tax", "Enabled", "Include tax amounts in cost calculation"},
		{"Include Credits", "Enabled", "Include AWS credits and refunds"},
		{"Include Upfront", "Enabled", "Include upfront costs for reserved instances"},
		{"Include Recurring", "Enabled", "Include recurring costs for reserved instances"},
		{"Include Other Subscriptions", "Disabled", "Include other subscription costs"},
		{"Use Blended Costs", "Enabled", "Use blended costs for organizations"},
		{"Include Support", "Enabled", "Include AWS support costs"},
	}
	return pluginrpc.ViewData{
		View:         awsViewCostTypes,
		Title:        "Cost Type Settings",
		Info:         "[green]Cost Type Configuration[white]\nConfigure which cost types to include in calculations",
		Status:       "connected",
		Headers:      []string{"Setting", "Status", "Description"},
		Rows:         rows,
		SelectionKey: "Setting",
		KeyBindings:  withNav(),
	}, nil
}

func (s *Service) viewForecastLocked() (pluginrpc.ViewData, error) {
	if err := s.ensureClientLocked(); err != nil {
		return pluginrpc.ViewData{}, err
	}
	now := time.Now()
	result, err := s.client.GetCostForecast(now.AddDate(0, 0, 1), now.AddDate(0, 3, 0), "MONTHLY")
	if err != nil {
		return pluginrpc.ViewData{
			View:         awsViewForecast,
			Title:        "Cost Forecast",
			Info:         s.baseInfo(""),
			Status:       "connected",
			Headers:      []string{"Period", "Actual", "Forecast", "Lower Bound", "Upper Bound"},
			Rows:         [][]string{{"Error fetching forecast", err.Error(), "", "", ""}},
			SelectionKey: "Period",
			KeyBindings:  withNav(),
		}, nil
	}

	ce := s.client.CostExplorer()
	tableData := [][]string{
		{now.Format("Jan 2006") + " (current)", getCurrentMonthActual(ce, now), "", "", ""},
	}
	if result.Total != nil && result.Total.Amount != nil {
		tableData = append(tableData, []string{
			"Total Forecast", "", fmt.Sprintf("$%s", aws.StringValue(result.Total.Amount)), "", "",
		})
	}
	for _, fc := range result.ForecastResultsByTime {
		tableData = append(tableData, formatForecastRow(fc))
	}
	if len(tableData) == 0 {
		tableData = [][]string{{"No forecast data available", "", "", "", ""}}
	}
	return pluginrpc.ViewData{
		View:         awsViewForecast,
		Title:        "Cost Forecast",
		Info:         s.baseInfo("3-month forecast (MONTHLY)"),
		Status:       "connected",
		Headers:      []string{"Period", "Actual", "Forecast", "Lower Bound", "Upper Bound"},
		Rows:         tableData,
		SelectionKey: "Period",
		KeyBindings:  withNav(),
	}, nil
}

func createBarChart(cost, budget float64, all []*CostData) string {
	maxBars := 20
	bars := 0
	if budget > 0 {
		bars = int((cost / budget) * float64(maxBars))
		if bars > maxBars {
			bars = maxBars
		}
	} else {
		maxCost := 0.0
		for _, c := range all {
			if c.cost > maxCost {
				maxCost = c.cost
			}
		}
		if maxCost > 0 {
			bars = int((cost / maxCost) * float64(maxBars))
		}
	}
	chart := ""
	for i := 0; i < bars; i++ {
		if i < maxBars*7/10 {
			chart += "█"
		} else if i < maxBars*9/10 {
			chart += "▓"
		} else {
			chart += "▒"
		}
	}
	if budget > 0 {
		return fmt.Sprintf("%s (%.0f%%)", chart, (cost/budget)*100)
	}
	return chart
}

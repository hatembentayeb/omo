package main

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/costexplorer"

	"omo/pkg/ui"
)

func (av *AWSCostsView) newForecastView() *ui.CoreView {
	cores := ui.NewCoreView(av.app, "Cost Forecast")
	cores.SetTableHeaders([]string{"Period", "Actual", "Forecast", "Lower Bound", "Upper Bound"})
	cores.SetRefreshCallback(av.refreshForecastData)

	// Key bindings
	cores.AddKeyBinding("M", "Main", av.showMainView)
	cores.AddKeyBinding("S", "Services", av.showServicesView)
	cores.AddKeyBinding("B", "Budgets", av.showBudgetsView)
	cores.AddKeyBinding("T", "Cost Types", av.showCostTypesView)
	cores.AddKeyBinding("F", "Forecast", av.showForecastView)
	cores.AddKeyBinding("R", "Refresh", func() { av.refreshCostData() })
	cores.AddKeyBinding("?", "Help", av.showHelp)

	cores.SetActionCallback(av.handleAction)
	cores.RegisterHandlers()
	return cores
}

func formatForecastRow(fc *costexplorer.ForecastResult) []string {
	period := ""
	if fc.TimePeriod != nil && fc.TimePeriod.Start != nil {
		t, err := time.Parse("2006-01-02", aws.StringValue(fc.TimePeriod.Start))
		if err == nil {
			period = t.Format("Jan 2006")
		} else {
			period = aws.StringValue(fc.TimePeriod.Start)
		}
	}

	mean := "N/A"
	if fc.MeanValue != nil {
		mean = fmt.Sprintf("$%s", aws.StringValue(fc.MeanValue))
	}

	lower := "N/A"
	if fc.PredictionIntervalLowerBound != nil {
		lower = fmt.Sprintf("$%s", aws.StringValue(fc.PredictionIntervalLowerBound))
	}
	upper := "N/A"
	if fc.PredictionIntervalUpperBound != nil {
		upper = fmt.Sprintf("$%s", aws.StringValue(fc.PredictionIntervalUpperBound))
	}

	return []string{period, "", mean, lower, upper}
}

func getCurrentMonthActual(ce *costexplorer.CostExplorer, now time.Time) string {
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	actualInput := &costexplorer.GetCostAndUsageInput{
		TimePeriod: &costexplorer.DateInterval{
			Start: aws.String(monthStart.Format("2006-01-02")),
			End:   aws.String(now.Format("2006-01-02")),
		},
		Granularity: aws.String("MONTHLY"),
		Metrics:     []*string{aws.String("BlendedCost")},
	}

	actualResult, _ := ce.GetCostAndUsage(actualInput)
	if actualResult != nil {
		for _, r := range actualResult.ResultsByTime {
			if total, ok := r.Total["BlendedCost"]; ok && total.Amount != nil {
				return fmt.Sprintf("$%s", aws.StringValue(total.Amount))
			}
		}
	}
	return "N/A"
}

func (av *AWSCostsView) refreshForecastData() ([][]string, error) {
	if av.profile == "" {
		return [][]string{{"No profile selected", "", "", "", ""}}, nil
	}

	sess, err := session.NewSessionWithOptions(session.Options{
		Profile: av.profile,
		Config:  *aws.NewConfig().WithRegion(av.region),
	})
	if err != nil {
		return [][]string{{"Error creating session", err.Error(), "", "", ""}}, nil
	}

	ce := costexplorer.New(sess)
	now := time.Now()

	input := &costexplorer.GetCostForecastInput{
		TimePeriod: &costexplorer.DateInterval{
			Start: aws.String(now.AddDate(0, 0, 1).Format("2006-01-02")),
			End:   aws.String(now.AddDate(0, 3, 0).Format("2006-01-02")),
		},
		Granularity: aws.String("MONTHLY"),
		Metric:      aws.String("BLENDED_COST"),
	}

	result, err := ce.GetCostForecast(input)
	if err != nil {
		return [][]string{{"Error fetching forecast", err.Error(), "", "", ""}}, nil
	}

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

	av.forecastView.SetInfoText(fmt.Sprintf("[green]Cost Forecast[white]\nProfile: %s | Region: %s\n3-month forecast (MONTHLY granularity)", av.profile, av.region))

	return tableData, nil
}

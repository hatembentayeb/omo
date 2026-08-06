package awscosts

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/budgets"
	"github.com/aws/aws-sdk-go/service/costexplorer"
)

func formatBudgetRow(b *budgets.Budget) []string {
	name := aws.StringValue(b.BudgetName)
	period := aws.StringValue(b.TimeUnit)

	limitAmount := "N/A"
	if b.BudgetLimit != nil && b.BudgetLimit.Amount != nil {
		limitAmount = fmt.Sprintf("$%s %s", aws.StringValue(b.BudgetLimit.Amount), aws.StringValue(b.BudgetLimit.Unit))
	}

	actualSpend := "N/A"
	remaining := "N/A"
	status := "N/A"

	if b.CalculatedSpend != nil && b.CalculatedSpend.ActualSpend != nil && b.CalculatedSpend.ActualSpend.Amount != nil {
		actualSpend = fmt.Sprintf("$%s", aws.StringValue(b.CalculatedSpend.ActualSpend.Amount))
	}

	if b.BudgetLimit != nil && b.BudgetLimit.Amount != nil && actualSpend != "N/A" {
		var limit, actual float64
		fmt.Sscanf(aws.StringValue(b.BudgetLimit.Amount), "%f", &limit)
		fmt.Sscanf(aws.StringValue(b.CalculatedSpend.ActualSpend.Amount), "%f", &actual)

		remaining = fmt.Sprintf("$%.2f", limit-actual)

		if limit > 0 {
			pct := (actual / limit) * 100
			switch {
			case pct >= 100:
				status = fmt.Sprintf("OVER %.1f%%", pct)
			case pct >= 80:
				status = fmt.Sprintf("WARNING %.1f%%", pct)
			default:
				status = fmt.Sprintf("OK %.1f%%", pct)
			}
		}
	}

	return []string{name, limitAmount, period, actualSpend, remaining, status}
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

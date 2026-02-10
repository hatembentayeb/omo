package main

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/budgets"

	"omo/pkg/ui"
)

func (av *AWSCostsView) newBudgetsView() *ui.CoreView {
	cores := ui.NewCoreView(av.app, "AWS Budgets")
	cores.SetTableHeaders([]string{"Name", "Amount", "Period", "Used", "Remaining", "Status"})
	cores.SetRefreshCallback(av.refreshBudgetsData)

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

func (av *AWSCostsView) refreshBudgetsData() ([][]string, error) {
	if av.profile == "" {
		return [][]string{{"No profile selected", "", "", "", "", ""}}, nil
	}

	sess, err := session.NewSessionWithOptions(session.Options{
		Profile: av.profile,
		Config:  *aws.NewConfig().WithRegion(av.region),
	})
	if err != nil {
		return [][]string{{"Error creating session", err.Error(), "", "", "", ""}}, nil
	}

	budgetsSvc := budgets.New(sess)

	// AWS requires the account ID; use "self" is not valid — we need to fetch it.
	// The Budgets API requires the account ID. We get it from STS.
	accountID, err := getAWSAccountID(sess)
	if err != nil {
		return [][]string{{"Could not determine account ID", err.Error(), "", "", "", ""}}, nil
	}

	input := &budgets.DescribeBudgetsInput{
		AccountId:  aws.String(accountID),
		MaxResults: aws.Int64(100),
	}

	result, err := budgetsSvc.DescribeBudgets(input)
	if err != nil {
		return [][]string{{"Error fetching budgets", err.Error(), "", "", "", ""}}, nil
	}

	if len(result.Budgets) == 0 {
		av.budgetsView.SetInfoText("[yellow]AWS Budgets[white]\nNo budgets configured in this account")
		return [][]string{{"No budgets found", "Configure budgets in AWS Console", "", "", "", ""}}, nil
	}

	tableData := make([][]string, 0, len(result.Budgets))
	for _, b := range result.Budgets {
		name := aws.StringValue(b.BudgetName)
		period := aws.StringValue(b.TimeUnit)

		limitAmount := "N/A"
		if b.BudgetLimit != nil && b.BudgetLimit.Amount != nil {
			limitAmount = fmt.Sprintf("$%s %s", aws.StringValue(b.BudgetLimit.Amount), aws.StringValue(b.BudgetLimit.Unit))
		}

		actualSpend := "N/A"
		forecastedSpend := "N/A"
		remaining := "N/A"
		status := "N/A"

		if b.CalculatedSpend != nil {
			if b.CalculatedSpend.ActualSpend != nil && b.CalculatedSpend.ActualSpend.Amount != nil {
				actualSpend = fmt.Sprintf("$%s", aws.StringValue(b.CalculatedSpend.ActualSpend.Amount))
			}
			if b.CalculatedSpend.ForecastedSpend != nil && b.CalculatedSpend.ForecastedSpend.Amount != nil {
				forecastedSpend = fmt.Sprintf("$%s", aws.StringValue(b.CalculatedSpend.ForecastedSpend.Amount))
			}
		}

		// Calculate remaining and status
		if b.BudgetLimit != nil && b.BudgetLimit.Amount != nil && b.CalculatedSpend != nil && b.CalculatedSpend.ActualSpend != nil && b.CalculatedSpend.ActualSpend.Amount != nil {
			var limit, actual float64
			fmt.Sscanf(aws.StringValue(b.BudgetLimit.Amount), "%f", &limit)
			fmt.Sscanf(aws.StringValue(b.CalculatedSpend.ActualSpend.Amount), "%f", &actual)

			rem := limit - actual
			remaining = fmt.Sprintf("$%.2f", rem)

			if limit > 0 {
				pct := (actual / limit) * 100
				if pct >= 100 {
					status = fmt.Sprintf("OVER %.1f%%", pct)
				} else if pct >= 80 {
					status = fmt.Sprintf("WARNING %.1f%%", pct)
				} else {
					status = fmt.Sprintf("OK %.1f%%", pct)
				}
			}
		}

		_ = forecastedSpend // available for extended display

		tableData = append(tableData, []string{
			name,
			limitAmount,
			period,
			actualSpend,
			remaining,
			status,
		})
	}

	av.budgetsView.SetInfoText(fmt.Sprintf("[green]AWS Budgets[white]\nProfile: %s\nBudgets: %d", av.profile, len(result.Budgets)))

	return tableData, nil
}

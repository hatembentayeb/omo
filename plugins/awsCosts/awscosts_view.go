package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/costexplorer"
	"github.com/rivo/tview"

	"omo/pkg/ui"
)

// AWSCostsView manages the UI for interacting with AWS Cost Explorer
type AWSCostsView struct {
	app             *tview.Application
	pages           *tview.Pages
	viewPages       *tview.Pages
	cores           *ui.CoreView // alias to mainView
	mainView        *ui.CoreView
	servicesView    *ui.CoreView
	budgetsView     *ui.CoreView
	costTypesView   *ui.CoreView
	forecastView    *ui.CoreView
	client          *costexplorer.CostExplorer
	profile         string
	region          string
	costData        []*CostData
	granularity     string
	timeRange       string
	currentView     string
	refreshTimer    *time.Timer
	refreshInterval time.Duration
}

// NewAWSCostsView creates a new AWS Costs view
func NewAWSCostsView(app *tview.Application, pages *tview.Pages) *AWSCostsView {
	av := &AWSCostsView{
		app:             app,
		pages:           pages,
		viewPages:       tview.NewPages(),
		granularity:     "DAILY",
		timeRange:       "LAST_30_DAYS",
		refreshInterval: 300 * time.Second,
	}

	// Create the main costs view
	av.mainView = av.newMainView()
	av.cores = av.mainView

	// Create sub-views
	av.servicesView = av.newServicesView()
	av.budgetsView = av.newBudgetsView()
	av.costTypesView = av.newCostTypesView()
	av.forecastView = av.newForecastView()

	// Set modal pages for all views
	views := []*ui.CoreView{
		av.mainView,
		av.servicesView,
		av.budgetsView,
		av.costTypesView,
		av.forecastView,
	}
	for _, view := range views {
		if view != nil {
			view.SetModalPages(pages)
		}
	}

	// Register pages
	av.viewPages.AddPage("aws-main", av.mainView.GetLayout(), true, true)
	av.viewPages.AddPage("aws-services", av.servicesView.GetLayout(), true, false)
	av.viewPages.AddPage("aws-budgets", av.budgetsView.GetLayout(), true, false)
	av.viewPages.AddPage("aws-costtypes", av.costTypesView.GetLayout(), true, false)
	av.viewPages.AddPage("aws-forecast", av.forecastView.GetLayout(), true, false)

	// Set view stacks
	av.currentView = awsViewMain
	av.setViewStack(av.mainView, awsViewMain)
	av.setViewStack(av.servicesView, awsViewServices)
	av.setViewStack(av.budgetsView, awsViewBudgets)
	av.setViewStack(av.costTypesView, awsViewCostTypes)
	av.setViewStack(av.forecastView, awsViewForecast)

	// Set initial state
	av.cores.SetInfoText("[yellow]AWS Cost Explorer[white]\nStatus: Not Connected\nUse [green]Ctrl+T[white] to select profile")
	av.cores.SetTableData([][]string{
		{"Please select an AWS profile", "", "", "", "", ""},
	})

	// Start auto-refresh
	av.startAutoRefresh()

	return av
}

// GetMainUI returns the main UI component
func (av *AWSCostsView) GetMainUI() tview.Primitive {
	return av.viewPages
}

// Stop cleans up resources
func (av *AWSCostsView) Stop() {
	if av.refreshTimer != nil {
		av.refreshTimer.Stop()
	}

	views := []*ui.CoreView{
		av.mainView,
		av.servicesView,
		av.budgetsView,
		av.costTypesView,
		av.forecastView,
	}
	for _, view := range views {
		if view != nil {
			view.StopAutoRefresh()
			view.UnregisterHandlers()
		}
	}
}

// refresh refreshes the current view
func (av *AWSCostsView) refresh() {
	current := av.currentCores()
	if current != nil {
		current.RefreshData()
	}
}

// ShowProfileSelector displays the AWS profile selector
func (av *AWSCostsView) ShowProfileSelector() {
	selector := NewProfileSelector(av.app, av.pages, func(profileName, profileRegion string) {
		if profileName == av.profile && profileRegion == av.region {
			av.app.SetFocus(av.cores.GetTable())
			return
		}

		av.profile = profileName
		av.region = profileRegion
		av.cores.Log(fmt.Sprintf("[green]Selected profile: %s (region: %s)", av.profile, av.region))

		av.cores.SetTableData([][]string{
			{"Loading data...", "Please wait", "", "", "", ""},
		})

		av.refreshCostData()
		av.app.SetFocus(av.cores.GetTable())
	})

	selector.SetLogger(func(msg string) {
		av.cores.Log(msg)
	})

	selector.Show()
}

// refreshCostData retrieves fresh cost data from AWS
func (av *AWSCostsView) refreshCostData() {
	av.cores.Log("Refreshing AWS cost data...")

	pm := ui.ShowProgressModal(
		av.pages, av.app, "Loading AWS Cost Data", 100, true,
		nil, true,
	)

	safeGo(func() {
		sess, err := session.NewSessionWithOptions(session.Options{
			Profile: av.profile,
			Config: aws.Config{
				Region: aws.String(av.region),
			},
		})

		if err != nil {
			av.app.QueueUpdateDraw(func() {
				pm.Close()
				ui.ShowStandardErrorModal(
					av.pages, av.app, "AWS Session Error",
					fmt.Sprintf("Could not create AWS session: %v", err),
					nil,
				)
			})
			return
		}

		av.client = costexplorer.New(sess)

		timeRange := getCostTimeRange(av.timeRange)

		input := &costexplorer.GetCostAndUsageInput{
			TimePeriod: &costexplorer.DateInterval{
				Start: aws.String(timeRange.Start.Format("2006-01-02")),
				End:   aws.String(timeRange.End.Format("2006-01-02")),
			},
			Granularity: aws.String(av.granularity),
			Metrics:     []*string{aws.String("BlendedCost"), aws.String("UnblendedCost")},
			GroupBy: []*costexplorer.GroupDefinition{
				{
					Type: aws.String("DIMENSION"),
					Key:  aws.String("SERVICE"),
				},
			},
		}

		costData, err := av.client.GetCostAndUsage(input)
		if err != nil {
			av.app.QueueUpdateDraw(func() {
				pm.Close()
				ui.ShowStandardErrorModal(
					av.pages, av.app, "AWS Cost Explorer Error",
					fmt.Sprintf("Could not retrieve cost data: %v", err),
					nil,
				)
			})
			return
		}

		av.processAwsCostData(costData)
		av.fetchBudgetData()

		av.app.QueueUpdateDraw(func() {
			pm.Close()
			av.refresh()
		})
	})
}

// processAwsCostData processes the AWS cost data into our internal format
func (av *AWSCostsView) processAwsCostData(costData *costexplorer.GetCostAndUsageOutput) {
	av.costData = make([]*CostData, 0)

	for _, resultByTime := range costData.ResultsByTime {
		for _, group := range resultByTime.Groups {
			serviceName := *group.Keys[0]
			cost := 0.0
			unit := "USD"

			if amount, ok := group.Metrics["BlendedCost"]; ok && amount.Amount != nil {
				if parsedCost, err := strconv.ParseFloat(*amount.Amount, 64); err == nil {
					cost = parsedCost
				}
				if amount.Unit != nil {
					unit = *amount.Unit
				}
			}

			av.costData = append(av.costData, &CostData{
				service:   serviceName,
				cost:      cost,
				date:      *resultByTime.TimePeriod.Start,
				unit:      unit,
				trend:     0.0,
				forecast:  cost,
				budget:    0.0,
				region:    av.region,
				usageType: "Usage",
			})
		}
	}
}

// fetchBudgetData retrieves AWS budget information
func (av *AWSCostsView) fetchBudgetData() {
	// This would make actual API calls to the AWS Budgets API
}

// handleAction handles actions triggered by the UI
func (av *AWSCostsView) handleAction(action string, payload map[string]interface{}) error {
	switch action {
	case "refresh":
		av.refresh()
		return nil
	case "keypress":
		if key, ok := payload["key"].(string); ok {
			switch key {
			case "R":
				av.refreshCostData()
				return nil
			case "S":
				av.showServicesView()
				return nil
			case "B":
				av.showBudgetsView()
				return nil
			case "T":
				av.showCostTypesView()
				return nil
			case "F":
				av.showForecastView()
				return nil
			case "P":
				av.showTimePeriodSelector()
				return nil
			case "G":
				av.toggleGranularity()
				return nil
			case "M":
				av.showMainView()
				return nil
			case "?":
				av.showHelp()
				return nil
			}
		}
	case "navigate_back":
		if view, ok := payload["current_view"].(string); ok {
			if view == awsViewRoot {
				av.switchView(awsViewMain)
				return nil
			}
			av.switchView(view)
			return nil
		}
	}
	return fmt.Errorf("unhandled")
}

// showTimePeriodSelector displays a modal to select the time range
func (av *AWSCostsView) showTimePeriodSelector() {
	timeRanges := []string{
		"LAST_7_DAYS",
		"LAST_30_DAYS",
		"THIS_MONTH",
		"LAST_3_MONTHS",
		"LAST_6_MONTHS",
		"LAST_12_MONTHS",
	}

	items := make([][]string, 0, len(timeRanges))
	for _, tr := range timeRanges {
		display := tr
		if tr == av.timeRange {
			display += " (current)"
		}
		items = append(items, []string{display, ""})
	}

	ui.ShowStandardListSelectorModal(
		av.pages,
		av.app,
		"Select Time Period",
		items,
		func(index int, text string, cancelled bool) {
			if cancelled || index < 0 {
				return
			}

			selectedRange := timeRanges[index]
			if selectedRange != av.timeRange {
				av.timeRange = selectedRange
				av.cores.Log(fmt.Sprintf("Changed time range to: %s", av.timeRange))
				av.refreshCostData()
			}
		},
	)
}

// toggleGranularity switches between daily and monthly views
func (av *AWSCostsView) toggleGranularity() {
	if av.granularity == "DAILY" {
		av.granularity = "MONTHLY"
	} else {
		av.granularity = "DAILY"
	}

	av.cores.Log(fmt.Sprintf("Switched granularity to: %s", av.granularity))
	av.refreshCostData()
}

// showHelp displays help information
func (av *AWSCostsView) showHelp() {
	content := `[yellow]AWS Cost Explorer Help[white]

[green]Navigation[white]
  [aqua]↑/↓[white] - Navigate between services
  [aqua]Enter[white] - Select a service

[green]Views[white]
  [aqua]M[white] - Main cost overview
  [aqua]S[white] - Service breakdown
  [aqua]B[white] - View budgets
  [aqua]T[white] - Cost type settings
  [aqua]F[white] - Cost forecasts

[green]Actions[white]
  [aqua]R[white] - Refresh data
  [aqua]P[white] - Change time period
  [aqua]G[white] - Toggle daily/monthly granularity
  [aqua]Ctrl+T[white] - Switch AWS profile
  [aqua]?[white] - Show this help`

	ui.ShowInfoModal(
		av.pages,
		av.app,
		"AWS Cost Explorer Help",
		content,
		func() {
			current := av.currentCores()
			if current != nil {
				av.app.SetFocus(current.GetTable())
			}
		},
	)
}

// createBarChart generates a text-based bar chart for costs
func (av *AWSCostsView) createBarChart(cost float64, budget float64) string {
	maxBars := 20
	bars := 0

	if budget > 0 {
		bars = int((cost / budget) * float64(maxBars))
		if bars > maxBars {
			bars = maxBars
		}
	} else {
		maxCost := 0.0
		for _, c := range av.costData {
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
		percentage := (cost / budget) * 100
		return fmt.Sprintf("%s (%.0f%%)", chart, percentage)
	}

	return chart
}

// startAutoRefresh sets up and starts the auto-refresh timer
func (av *AWSCostsView) startAutoRefresh() {
	if uiConfig, err := GetAWSCostsUIConfig(); err == nil {
		av.refreshInterval = time.Duration(uiConfig.RefreshInterval) * time.Second
	}

	if av.refreshTimer != nil {
		av.refreshTimer.Stop()
	}

	av.refreshTimer = time.AfterFunc(av.refreshInterval, func() {
		if av.client != nil {
			av.app.QueueUpdate(func() {
				av.refresh()
				av.startAutoRefresh()
			})
		} else {
			av.startAutoRefresh()
		}
	})
}

// AutoConnect auto-connects to the first profile or shows the selector
func (av *AWSCostsView) AutoConnect() {
	av.ShowProfileSelector()
}

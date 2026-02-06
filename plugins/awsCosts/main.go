package main

import (
	"fmt"
	"strconv"
	"time"

	"omo/pkg/pluginapi"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/costexplorer"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"omo/pkg/ui"
)

// OhmyopsPlugin is expected by the main application
var OhmyopsPlugin AWSCostsPlugin

// AWSCostsPlugin represents the AWS Costs monitoring plugin
type AWSCostsPlugin struct {
	app         *tview.Application
	pages       *tview.Pages
	cores       *ui.CoreView
	currentView string
	client      *costexplorer.CostExplorer
	profile     string
	region      string
	costData    []*CostData
	granularity string // DAILY, MONTHLY
	timeRange   string // LAST_7_DAYS, LAST_30_DAYS, etc.
}

// CostData represents AWS cost information
type CostData struct {
	service   string
	cost      float64
	date      string
	unit      string
	region    string
	usageType string
	trend     float64 // percentage change
	forecast  float64 // forecasted amount
	budget    float64 // budget amount if available
}

// safeGo runs a function in a goroutine with panic recovery
func safeGo(f func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("Recovered from panic: %v\n", r)
			}
		}()
		f()
	}()
}

// Start initializes the plugin
func (p *AWSCostsPlugin) Start(app *tview.Application) tview.Primitive {
	p.app = app
	p.pages = tview.NewPages()
	p.currentView = "main"
	p.granularity = "DAILY"
	p.timeRange = "LAST_30_DAYS"

	// Initialize main view
	p.initializeMainView()

	// Add keyboard handling for profile selection (Ctrl+T)
	p.pages.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlT {
			p.showProfileSelector()
			return nil
		}
		return event
	})

	// Show empty data initially with instructions
	p.cores.SetTableData([][]string{
		{"Please select an AWS profile", "", "", "", "", ""},
	})

	// Update info text to indicate profile selection is needed
	p.cores.SetInfoText("AWS Cost Explorer | Please select a profile to begin")

	// Show profile selector immediately on startup
	p.cores.Log("[blue]Please select an AWS profile to continue...")

	// Show profile selector immediately when the app is ready
	p.showProfileSelector()

	return p.pages
}

// Stop cleans up resources when the plugin is unloaded.
func (p *AWSCostsPlugin) Stop() {
	if p.cores != nil {
		p.cores.StopAutoRefresh()
		p.cores.UnregisterHandlers()
	}

	if p.pages != nil {
		pageIDs := []string{
			"main",
			"confirmation-modal",
			"error-modal",
			"info-modal",
			"list-selector-modal",
			"progress-modal",
			"sort-modal",
			"compact-modal",
		}
		for _, pageID := range pageIDs {
			if p.pages.HasPage(pageID) {
				p.pages.RemovePage(pageID)
			}
		}
	}

	p.client = nil
	p.cores = nil
	p.pages = nil
	p.app = nil
}

// GetMetadata returns plugin metadata.
func (p *AWSCostsPlugin) GetMetadata() pluginapi.PluginMetadata {
	return pluginapi.PluginMetadata{
		Name:        "awsCosts",
		Version:     "1.0.0",
		Description: "AWS Cost Explorer and Budget Analyzer",
		Author:      "OhMyOps",
		License:     "MIT",
		Tags:        []string{"aws", "cost", "monitoring", "billing"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "",
	}
}

// initializeMainView creates the main costs list view
func (p *AWSCostsPlugin) initializeMainView() {
	p.cores = ui.NewCoreView(p.app, "AWS Cost Explorer")
	p.cores.SetModalPages(p.pages)
	p.cores.SetTableHeaders([]string{"Service", "Cost", "Trend", "Chart", "Forecast", "Budget Status"})
	p.cores.SetRefreshCallback(p.fetchCostData)
	p.cores.SetRowSelectedCallback(p.onServiceSelected)
	p.cores.SetInfoText(fmt.Sprintf("Monitor AWS costs: Profile: %s | Region: %s", p.profile, p.region))

	// Key bindings
	p.cores.AddKeyBinding("D", "Details", nil)
	p.cores.AddKeyBinding("S", "Services", nil)
	p.cores.AddKeyBinding("P", "Time Period", nil)
	p.cores.AddKeyBinding("G", "Granularity", nil)
	p.cores.AddKeyBinding("B", "Budget", nil)
	p.cores.AddKeyBinding("T", "Cost Types", nil)
	p.cores.AddKeyBinding("F", "Forecast", nil)
	p.cores.AddKeyBinding("^T", "Profile", nil)
	p.cores.AddKeyBinding("^B", "Back", nil)

	p.setupActionHandler()
	p.cores.RegisterHandlers()

	p.pages.AddPage("main", p.cores.GetLayout(), true, true)
	p.cores.PushView("AWS Costs")
	p.cores.Log("Plugin initialized")
}

// setupActionHandler configures the action handler for the plugin
func (p *AWSCostsPlugin) setupActionHandler() {
	p.cores.SetActionCallback(func(action string, payload map[string]interface{}) error {
		if action == "keypress" {
			if key, ok := payload["key"].(string); ok {
				switch key {
				case "R":
					p.refreshCostData()
				case "D":
					p.showCostDetails()
				case "S":
					p.showServiceBreakdown()
				case "P":
					p.showTimePeriodSelector()
				case "G":
					p.toggleGranularity()
				case "B":
					p.showBudgets()
				case "T":
					p.showCostTypeConfig()
				case "F":
					p.showForecast()
				case "?":
					p.showHelpModal()
				case "^B":
					p.returnToPreviousView()
				}
			}
		} else if action == "navigate_back" {
			currentView := p.cores.GetCurrentView()
			p.switchToView(currentView)
		}
		return nil
	})
}

// fetchCostData retrieves AWS cost data and formats it for display
func (p *AWSCostsPlugin) fetchCostData() ([][]string, error) {
	if p.costData == nil || len(p.costData) == 0 {
		return [][]string{{"Loading...", "Please wait", "", "", "", ""}}, nil
	}

	result := make([][]string, 0, len(p.costData))

	for _, cost := range p.costData {
		// Create ASCII/Unicode bar chart based on cost value
		chartBar := p.createBarChart(cost.cost, cost.budget)

		// Format trend with arrow
		trendFormatted := fmt.Sprintf("%.1f%%", cost.trend)
		if cost.trend > 0 {
			trendFormatted = "↑ " + trendFormatted
		} else if cost.trend < 0 {
			trendFormatted = "↓ " + trendFormatted
		}

		// Format budget status
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

		// Add row to results
		result = append(result, []string{
			cost.service,
			fmt.Sprintf("$%.2f %s", cost.cost, cost.unit),
			trendFormatted,
			chartBar,
			fmt.Sprintf("$%.2f", cost.forecast),
			budgetStatus,
		})
	}

	// Update the header text with current profile and region
	p.cores.SetInfoText(fmt.Sprintf("AWS Cost Explorer | Profile: %s | Region: %s | %s | %s",
		p.profile, p.region, p.timeRange, p.granularity))

	return result, nil
}

// refreshCostData retrieves fresh cost data from AWS
func (p *AWSCostsPlugin) refreshCostData() {
	p.cores.Log("Refreshing AWS cost data...")

	// Show progress modal
	pm := ui.ShowProgressModal(
		p.pages, p.app, "Loading AWS Cost Data", 100, true,
		nil, true,
	)

	safeGo(func() {
		// Initialize AWS session with selected profile
		sess, err := session.NewSessionWithOptions(session.Options{
			Profile: p.profile,
			Config: aws.Config{
				Region: aws.String(p.region),
			},
		})

		if err != nil {
			p.app.QueueUpdateDraw(func() {
				pm.Close()
				ui.ShowStandardErrorModal(
					p.pages, p.app, "AWS Session Error",
					fmt.Sprintf("Could not create AWS session: %v", err),
					nil,
				)
			})
			return
		}

		// Create Cost Explorer client
		p.client = costexplorer.New(sess)

		// Get time range for query
		timeRange := getCostTimeRange(p.timeRange)

		// Get cost by service
		input := &costexplorer.GetCostAndUsageInput{
			TimePeriod: &costexplorer.DateInterval{
				Start: aws.String(timeRange.Start.Format("2006-01-02")),
				End:   aws.String(timeRange.End.Format("2006-01-02")),
			},
			Granularity: aws.String(p.granularity),
			Metrics:     []*string{aws.String("BlendedCost"), aws.String("UnblendedCost")},
			GroupBy: []*costexplorer.GroupDefinition{
				{
					Type: aws.String("DIMENSION"),
					Key:  aws.String("SERVICE"),
				},
			},
		}

		costData, err := p.client.GetCostAndUsage(input)
		if err != nil {
			p.app.QueueUpdateDraw(func() {
				pm.Close()
				ui.ShowStandardErrorModal(
					p.pages, p.app, "AWS Cost Explorer Error",
					fmt.Sprintf("Could not retrieve cost data: %v", err),
					nil,
				)
			})
			return
		}

		// Process cost data
		p.processAwsCostData(costData)

		// Get budget data if available
		p.fetchBudgetData()

		// Update UI
		p.app.QueueUpdateDraw(func() {
			pm.Close()
			p.cores.RefreshData()
		})
	})
}

// processAwsCostData processes the AWS cost data into our internal format
func (p *AWSCostsPlugin) processAwsCostData(costData *costexplorer.GetCostAndUsageOutput) {
	// Clear existing data
	p.costData = make([]*CostData, 0)

	// Process each result by service
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

			// We'll get all these values from the actual API results
			// Calculate trend by comparing with previous period if available
			// For now, set trend to 0
			trend := 0.0

			// We'll use the actual cost for forecast and budget for now
			// In a complete implementation, these would be obtained from
			// the AWS Budgets and Forecasts APIs
			forecast := cost
			budget := 0.0 // We don't have budget data yet

			p.costData = append(p.costData, &CostData{
				service:   serviceName,
				cost:      cost,
				date:      *resultByTime.TimePeriod.Start,
				unit:      unit,
				trend:     trend,
				forecast:  forecast,
				budget:    budget,
				region:    p.region,
				usageType: "Usage",
			})
		}
	}
}

// createBarChart generates a text-based bar chart for costs
func (p *AWSCostsPlugin) createBarChart(cost float64, budget float64) string {
	maxBars := 20
	bars := 0

	if budget > 0 {
		// Scale bars relative to budget
		bars = int((cost / budget) * float64(maxBars))
		if bars > maxBars {
			bars = maxBars
		}
	} else {
		// Scale based on a reasonable default if no budget
		// Find max cost for scaling
		maxCost := 0.0
		for _, c := range p.costData {
			if c.cost > maxCost {
				maxCost = c.cost
			}
		}

		if maxCost > 0 {
			bars = int((cost / maxCost) * float64(maxBars))
		}
	}

	// Generate bar chart
	chart := ""
	for i := 0; i < bars; i++ {
		if i < maxBars*7/10 {
			chart += "█" // Green blocks for under 70%
		} else if i < maxBars*9/10 {
			chart += "▓" // Yellow blocks for 70-90%
		} else {
			chart += "▒" // Red blocks for over 90%
		}
	}

	// Show percentage of budget
	if budget > 0 {
		percentage := (cost / budget) * 100
		return fmt.Sprintf("%s (%.0f%%)", chart, percentage)
	}

	return chart
}

// showProfileSelector displays a modal to select the AWS profile
func (p *AWSCostsPlugin) showProfileSelector() {
	// Create profile selector instance
	selector := NewProfileSelector(p.app, p.pages, func(profileName, profileRegion string) {
		// Skip if same profile and region
		if profileName == p.profile && profileRegion == p.region {
			// Focus on the table even if profile didn't change
			p.app.SetFocus(p.cores.GetTable())
			return
		}

		// Update profile and region
		p.profile = profileName
		p.region = profileRegion
		p.cores.Log(fmt.Sprintf("[green]Selected profile: %s (region: %s)", p.profile, p.region))

		// Show loading indicator in the table while we connect
		p.cores.SetTableData([][]string{
			{"Loading data...", "Please wait", "", "", "", ""},
		})

		// Refresh data with the selected profile and region
		p.refreshCostData()

		// Ensure table gets focus after refresh
		p.app.SetFocus(p.cores.GetTable())
	})

	// Set the logger to use the cores logger with a wrapper function
	selector.SetLogger(func(msg string) {
		p.cores.Log(msg)
	})

	// Show the profile selector modal
	selector.Show()
}

// getCostTimeRange returns a time range struct based on the string description
func getCostTimeRange(timeRange string) struct{ Start, End time.Time } {
	now := time.Now()
	result := struct{ Start, End time.Time }{
		End: now,
	}

	switch timeRange {
	case "LAST_7_DAYS":
		result.Start = now.AddDate(0, 0, -7)
	case "LAST_30_DAYS":
		result.Start = now.AddDate(0, 0, -30)
	case "LAST_3_MONTHS":
		result.Start = now.AddDate(0, -3, 0)
	case "LAST_6_MONTHS":
		result.Start = now.AddDate(0, -6, 0)
	case "LAST_12_MONTHS":
		result.Start = now.AddDate(-1, 0, 0)
	case "THIS_MONTH":
		result.Start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	default:
		result.Start = now.AddDate(0, 0, -30) // Default to last 30 days
	}

	return result
}

// fetchBudgetData retrieves AWS budget information
func (p *AWSCostsPlugin) fetchBudgetData() {
	// This would make actual API calls to the AWS Budgets API
	// For now, we'll use the mock data already in our costData slice
}

// showCostDetails shows detailed information for the selected service
func (p *AWSCostsPlugin) showCostDetails() {
	selection := p.cores.GetSelectedRow()
	if selection < 0 || selection >= len(p.costData) {
		p.cores.Log("No service selected")
		return
	}

	selectedService := p.costData[selection]

	// Push view to navigation stack
	p.cores.PushView(fmt.Sprintf("Details: %s", selectedService.service))
	p.currentView = "details"

	// Update UI
	p.cores.SetTableHeaders([]string{"Metric", "Value"})
	p.cores.SetRefreshCallback(func() ([][]string, error) {
		// Create detailed view for the selected service
		data := [][]string{
			{"Service", selectedService.service},
			{"Cost", fmt.Sprintf("$%.2f %s", selectedService.cost, selectedService.unit)},
			{"Date", selectedService.date},
			{"Region", selectedService.region},
			{"Usage Type", selectedService.usageType},
			{"Trend", fmt.Sprintf("%.1f%%", selectedService.trend)},
			{"Forecast", fmt.Sprintf("$%.2f", selectedService.forecast)},
		}

		if selectedService.budget > 0 {
			percentage := (selectedService.cost / selectedService.budget) * 100
			data = append(data, []string{"Budget", fmt.Sprintf("$%.2f", selectedService.budget)})
			data = append(data, []string{"Budget Usage", fmt.Sprintf("%.1f%%", percentage)})
		}

		// Here we would add more details from AWS Cost Explorer API
		// such as usage by resource, pricing details, etc.

		return data, nil
	})

	p.cores.RefreshData()
}

// showServiceBreakdown displays cost breakdown by AWS service
func (p *AWSCostsPlugin) showServiceBreakdown() {
	// Push view to navigation stack
	p.cores.PushView("Service Breakdown")
	p.currentView = "services"

	// Update UI
	p.cores.SetTableHeaders([]string{"Service", "Cost", "Percentage", "Distribution"})
	p.cores.SetRefreshCallback(func() ([][]string, error) {
		// Calculate total cost and prepare data
		totalCost := 0.0
		for _, cost := range p.costData {
			totalCost += cost.cost
		}

		// Create data rows
		result := make([][]string, 0, len(p.costData))
		for _, cost := range p.costData {
			percentage := 0.0
			if totalCost > 0 {
				percentage = (cost.cost / totalCost) * 100
			}

			// Create distribution chart
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

		// Add a total row
		result = append(result, []string{
			"TOTAL",
			fmt.Sprintf("$%.2f", totalCost),
			"100%",
			"",
		})

		return result, nil
	})

	p.cores.RefreshData()
}

// showTimePeriodSelector displays a modal to select the time range
func (p *AWSCostsPlugin) showTimePeriodSelector() {
	timeRanges := []string{
		"LAST_7_DAYS",
		"LAST_30_DAYS",
		"THIS_MONTH",
		"LAST_3_MONTHS",
		"LAST_6_MONTHS",
		"LAST_12_MONTHS",
	}

	// Create list for selection
	items := make([][]string, 0, len(timeRanges))
	for _, tr := range timeRanges {
		display := tr
		if tr == p.timeRange {
			display += " (current)"
		}
		items = append(items, []string{display, ""})
	}

	ui.ShowStandardListSelectorModal(
		p.pages,
		p.app,
		"Select Time Period",
		items,
		func(index int, text string, cancelled bool) {
			// Skip if cancelled
			if cancelled || index < 0 {
				return
			}

			selectedRange := timeRanges[index]

			// If range changed, update and refresh
			if selectedRange != p.timeRange {
				p.timeRange = selectedRange
				p.cores.Log(fmt.Sprintf("Changed time range to: %s", p.timeRange))
				p.refreshCostData()
			}
		},
	)
}

// toggleGranularity switches between daily and monthly views
func (p *AWSCostsPlugin) toggleGranularity() {
	if p.granularity == "DAILY" {
		p.granularity = "MONTHLY"
	} else {
		p.granularity = "DAILY"
	}

	p.cores.Log(fmt.Sprintf("Switched granularity to: %s", p.granularity))
	p.refreshCostData()
}

// showBudgets displays the budgets and alerts view
func (p *AWSCostsPlugin) showBudgets() {
	// Push view to navigation stack
	p.cores.PushView("Budgets")
	p.currentView = "budgets"

	// Update UI
	p.cores.SetTableHeaders([]string{"Name", "Amount", "Period", "Used", "Remaining", "Status"})
	p.cores.SetRefreshCallback(func() ([][]string, error) {
		// This would call AWS Budgets API
		// Using mock data for now
		result := [][]string{
			{"EC2 Monthly", "$500.00", "Monthly", "$320.45", "$179.55", "✓ 64.1%"},
			{"S3 Storage", "$200.00", "Monthly", "$187.30", "$12.70", "⚠ 93.7%"},
			{"RDS Database", "$300.00", "Monthly", "$312.50", "-$12.50", "⚠️ 104.2%"},
			{"Lambda Functions", "$100.00", "Monthly", "$42.10", "$57.90", "✓ 42.1%"},
			{"Total AWS", "$1500.00", "Monthly", "$1123.75", "$376.25", "✓ 74.9%"},
		}

		return result, nil
	})

	p.cores.RefreshData()
}

// showCostTypeConfig displays and configures cost type settings
func (p *AWSCostsPlugin) showCostTypeConfig() {
	// Push view to navigation stack
	p.cores.PushView("Cost Types")
	p.currentView = "costtypes"

	// Update UI
	p.cores.SetTableHeaders([]string{"Setting", "Status", "Description"})
	p.cores.SetRefreshCallback(func() ([][]string, error) {
		result := [][]string{
			{"Include Tax", "Enabled", "Include tax amounts in cost calculation"},
			{"Include Credits", "Enabled", "Include AWS credits and refunds"},
			{"Include Upfront", "Enabled", "Include upfront costs for reserved instances"},
			{"Include Recurring", "Enabled", "Include recurring costs for reserved instances"},
			{"Include Other Subscriptions", "Disabled", "Include other subscription costs"},
			{"Use Blended Costs", "Enabled", "Use blended costs for organizations"},
			{"Include Support", "Enabled", "Include AWS support costs"},
		}

		return result, nil
	})

	p.cores.RefreshData()
}

// showForecast displays cost forecast view
func (p *AWSCostsPlugin) showForecast() {
	// Push view to navigation stack
	p.cores.PushView("Cost Forecast")
	p.currentView = "forecast"

	// Update UI
	p.cores.SetTableHeaders([]string{"Period", "Actual", "Forecast", "Trend", "Visualization"})
	p.cores.SetRefreshCallback(func() ([][]string, error) {
		// This would call AWS Cost Explorer forecast API
		// Using mock data for now
		now := time.Now()
		currentMonth := now.Format("Jan 2006")
		nextMonth := now.AddDate(0, 1, 0).Format("Jan 2006")
		twoMonths := now.AddDate(0, 2, 0).Format("Jan 2006")

		result := [][]string{
			{currentMonth, "$1123.75", "$1450.00", "↑ 12.5%", "█████████████▓▓▓▓▓▓▓"},
			{nextMonth, "N/A", "$1580.00", "↑ 9.0%", "█████████████████▓▓▓"},
			{twoMonths, "N/A", "$1640.00", "↑ 3.8%", "██████████████████▓▓"},
		}

		return result, nil
	})

	p.cores.RefreshData()
}

// onServiceSelected handles when a service is selected in the table
func (p *AWSCostsPlugin) onServiceSelected(row int) {
	// Show details when a row is double-clicked
	if row >= 0 && row < len(p.costData) {
		p.showCostDetails()
	}
}

// showHelpModal displays the help information
func (p *AWSCostsPlugin) showHelpModal() {
	// Create the help content with sections
	content := `[yellow]AWS Cost Explorer Help[white]

[green]Navigation[white]
  [aqua]↑/↓[white] - Navigate between services
  [aqua]Enter[white] - Select a service

[green]Actions[white]
  [aqua]R[white] - Refresh data
  [aqua]D[white] - View service details
  [aqua]S[white] - Service breakdown
  [aqua]P[white] - Change time period
  [aqua]G[white] - Toggle between daily/monthly view
  [aqua]B[white] - View budgets
  [aqua]T[white] - Configure cost type settings
  [aqua]F[white] - View cost forecasts
  [aqua]Ctrl+T[white] - Switch AWS profile
  [aqua]Esc[white] - Go back to previous view
  [aqua]Ctrl+B[white] - Go back to previous view

[green]AWS Profiles[white]
  AWS profiles are loaded from your ~/.aws/credentials file.
  Use Ctrl+T to switch between profiles if you have multiple configured.`

	// Show the info modal with a callback to return focus to the table
	ui.ShowInfoModal(
		p.pages,
		p.app,
		"AWS Cost Explorer Help",
		content,
		func() {
			// Return focus to the table when modal is closed
			p.app.SetFocus(p.cores.GetTable())
		},
	)
}

// switchToView updates the current view
func (p *AWSCostsPlugin) switchToView(viewName string) {
	p.currentView = viewName
}

// returnToPreviousView returns to the previous view
func (p *AWSCostsPlugin) returnToPreviousView() {
	lastView := p.cores.PopView()
	if lastView != "" {
		currentView := p.cores.GetCurrentView()
		p.switchToView(currentView)
	}
}

package main

import (
	"omo/pkg/ui"
)

const (
	awsViewRoot     = "awsCosts"
	awsViewMain     = "main"
	awsViewServices = "services"
	awsViewBudgets  = "budgets"
	awsViewCostTypes = "costtypes"
	awsViewForecast = "forecast"
)

func (av *AWSCostsView) currentCores() *ui.CoreView {
	switch av.currentView {
	case awsViewServices:
		return av.servicesView
	case awsViewBudgets:
		return av.budgetsView
	case awsViewCostTypes:
		return av.costTypesView
	case awsViewForecast:
		return av.forecastView
	default:
		return av.mainView
	}
}

func (av *AWSCostsView) setViewStack(cores *ui.CoreView, viewName string) {
	if cores == nil {
		return
	}

	stack := []string{awsViewRoot, awsViewMain}
	if viewName != awsViewMain {
		stack = append(stack, viewName)
	}
	cores.SetViewStack(stack)
}

func (av *AWSCostsView) switchView(viewName string) {
	pageName := "aws-" + viewName
	av.currentView = viewName
	av.viewPages.SwitchToPage(pageName)

	av.setViewStack(av.currentCores(), viewName)
	av.refresh()
	current := av.currentCores()
	if current != nil {
		av.app.SetFocus(current.GetTable())
	}
}

func (av *AWSCostsView) showMainView() {
	av.switchView(awsViewMain)
}

func (av *AWSCostsView) showServicesView() {
	av.switchView(awsViewServices)
}

func (av *AWSCostsView) showBudgetsView() {
	av.switchView(awsViewBudgets)
}

func (av *AWSCostsView) showCostTypesView() {
	av.switchView(awsViewCostTypes)
}

func (av *AWSCostsView) showForecastView() {
	av.switchView(awsViewForecast)
}

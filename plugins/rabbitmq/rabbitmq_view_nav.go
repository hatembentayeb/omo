package main

import (
	"omo/pkg/ui"
)

const (
	rmqViewRoot        = "rabbitmq"
	rmqViewOverview    = "overview"
	rmqViewQueues      = "queues"
	rmqViewExchanges   = "exchanges"
	rmqViewBindings    = "bindings"
	rmqViewConnections = "connections"
	rmqViewChannels    = "channels"
	rmqViewNodes       = "nodes"
)

func (rv *RabbitMQView) currentCores() *ui.CoreView {
	switch rv.currentView {
	case rmqViewQueues:
		return rv.queuesView
	case rmqViewExchanges:
		return rv.exchangesView
	case rmqViewBindings:
		return rv.bindingsView
	case rmqViewConnections:
		return rv.connectionsView
	case rmqViewChannels:
		return rv.channelsView
	case rmqViewNodes:
		return rv.nodesView
	default:
		return rv.overviewView
	}
}

func (rv *RabbitMQView) setViewStack(cores *ui.CoreView, viewName string) {
	if cores == nil {
		return
	}

	stack := []string{rmqViewRoot, rmqViewOverview}
	if viewName != rmqViewOverview {
		stack = append(stack, viewName)
	}
	cores.SetViewStack(stack)
}

func (rv *RabbitMQView) switchView(viewName string) {
	pageName := "rmq-" + viewName
	rv.currentView = viewName
	rv.viewPages.SwitchToPage(pageName)

	rv.setViewStack(rv.currentCores(), viewName)
	rv.refresh()
	current := rv.currentCores()
	if current != nil {
		rv.app.SetFocus(current.GetTable())
	}
}

func (rv *RabbitMQView) showOverview()    { rv.switchView(rmqViewOverview) }
func (rv *RabbitMQView) showQueues()      { rv.switchView(rmqViewQueues) }
func (rv *RabbitMQView) showExchanges()   { rv.switchView(rmqViewExchanges) }
func (rv *RabbitMQView) showBindings()    { rv.switchView(rmqViewBindings) }
func (rv *RabbitMQView) showConnections() { rv.switchView(rmqViewConnections) }
func (rv *RabbitMQView) showChannels()    { rv.switchView(rmqViewChannels) }
func (rv *RabbitMQView) showNodes()       { rv.switchView(rmqViewNodes) }

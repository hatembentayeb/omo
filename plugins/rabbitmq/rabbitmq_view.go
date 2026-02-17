package main

import (
	"fmt"
	"time"

	"github.com/rivo/tview"

	"omo/pkg/ui"
)

// RabbitMQView manages the UI for interacting with RabbitMQ
type RabbitMQView struct {
	app             *tview.Application
	pages           *tview.Pages
	viewPages       *tview.Pages
	overviewView    *ui.CoreView
	queuesView      *ui.CoreView
	exchangesView   *ui.CoreView
	bindingsView    *ui.CoreView
	connectionsView *ui.CoreView
	channelsView    *ui.CoreView
	nodesView       *ui.CoreView
	cores           *ui.CoreView
	rmqClient       *RabbitMQClient
	currentView     string
	refreshTimer    *time.Timer
	refreshInterval time.Duration
	instances       []RabbitMQInstance
}

// NewRabbitMQView creates a new RabbitMQ view
func NewRabbitMQView(app *tview.Application, pages *tview.Pages) *RabbitMQView {
	rv := &RabbitMQView{
		app:             app,
		pages:           pages,
		viewPages:       tview.NewPages(),
		refreshInterval: 10 * time.Second,
	}

	rv.rmqClient = NewRabbitMQClient()

	// Load config
	config, err := LoadRabbitMQConfig("")
	if err == nil {
		rv.instances = config.Instances
		if config.UI.RefreshInterval > 0 {
			rv.refreshInterval = time.Duration(config.UI.RefreshInterval) * time.Second
		}
	}

	// Create all sub-views
	rv.overviewView = rv.newOverviewView()
	rv.queuesView = rv.newQueuesView()
	rv.exchangesView = rv.newExchangesView()
	rv.bindingsView = rv.newBindingsView()
	rv.connectionsView = rv.newConnectionsView()
	rv.channelsView = rv.newChannelsView()
	rv.nodesView = rv.newNodesView()

	// Set cores alias to default view
	rv.cores = rv.overviewView

	// Set modal pages on all views
	views := []*ui.CoreView{
		rv.overviewView,
		rv.queuesView,
		rv.exchangesView,
		rv.bindingsView,
		rv.connectionsView,
		rv.channelsView,
		rv.nodesView,
	}
	for _, view := range views {
		if view != nil {
			view.SetModalPages(rv.pages)
		}
	}

	// Register view pages
	rv.viewPages.AddPage("rmq-overview", rv.overviewView.GetLayout(), true, true)
	rv.viewPages.AddPage("rmq-queues", rv.queuesView.GetLayout(), true, false)
	rv.viewPages.AddPage("rmq-exchanges", rv.exchangesView.GetLayout(), true, false)
	rv.viewPages.AddPage("rmq-bindings", rv.bindingsView.GetLayout(), true, false)
	rv.viewPages.AddPage("rmq-connections", rv.connectionsView.GetLayout(), true, false)
	rv.viewPages.AddPage("rmq-channels", rv.channelsView.GetLayout(), true, false)
	rv.viewPages.AddPage("rmq-nodes", rv.nodesView.GetLayout(), true, false)

	// Set initial view
	rv.currentView = rmqViewOverview
	rv.setViewStack(rv.overviewView, rmqViewOverview)
	rv.setViewStack(rv.queuesView, rmqViewQueues)
	rv.setViewStack(rv.exchangesView, rmqViewExchanges)
	rv.setViewStack(rv.bindingsView, rmqViewBindings)
	rv.setViewStack(rv.connectionsView, rmqViewConnections)
	rv.setViewStack(rv.channelsView, rmqViewChannels)
	rv.setViewStack(rv.nodesView, rmqViewNodes)

	// Set initial status
	rv.overviewView.SetInfoText("[yellow]RabbitMQ Manager[white]\nStatus: Not Connected\nUse [green]Ctrl+T[white] to select instance")

	// Auto-connect
	rv.autoConnect()

	// Start auto-refresh
	rv.startAutoRefresh()

	return rv
}

// GetMainUI returns the main UI component
func (rv *RabbitMQView) GetMainUI() tview.Primitive {
	return rv.viewPages
}

// Stop cleans up resources
func (rv *RabbitMQView) Stop() {
	if rv.refreshTimer != nil {
		rv.refreshTimer.Stop()
	}

	if rv.rmqClient != nil && rv.rmqClient.IsConnected() {
		rv.rmqClient.Disconnect()
	}

	views := []*ui.CoreView{
		rv.overviewView,
		rv.queuesView,
		rv.exchangesView,
		rv.bindingsView,
		rv.connectionsView,
		rv.channelsView,
		rv.nodesView,
	}
	for _, view := range views {
		if view != nil {
			view.StopAutoRefresh()
			view.UnregisterHandlers()
		}
	}
}

// refresh refreshes the current view
func (rv *RabbitMQView) refresh() {
	current := rv.currentCores()
	if current != nil {
		current.RefreshData()
	}
}

// ShowInstanceSelector displays the instance selection modal
func (rv *RabbitMQView) ShowInstanceSelector() {
	rv.cores.Log("[blue]Opening instance selector...")

	instances, err := GetAvailableRabbitMQInstances()
	if err != nil {
		rv.cores.Log(fmt.Sprintf("[red]Failed to load RabbitMQ instances: %v", err))
		return
	}

	if len(instances) == 0 {
		rv.cores.Log("[yellow]No RabbitMQ instances configured in ~/.omo/configs/rabbitmq/rabbitmq.yaml")
		return
	}

	items := make([][]string, len(instances))
	for i, inst := range instances {
		items[i] = []string{
			inst.Name,
			fmt.Sprintf("%s:%d - %s", inst.Host, inst.AMQPPort, inst.Description),
		}
	}

	ui.ShowStandardListSelectorModal(
		rv.pages,
		rv.app,
		"Select RabbitMQ Instance",
		items,
		func(index int, name string, cancelled bool) {
			if !cancelled && index >= 0 && index < len(instances) {
				rv.connectToInstance(instances[index])
			} else {
				rv.cores.Log("[blue]Instance selection cancelled")
			}
			rv.app.SetFocus(rv.currentCores().GetTable())
		},
	)
}

// connectToInstance connects to a RabbitMQ instance
func (rv *RabbitMQView) connectToInstance(instance RabbitMQInstance) {
	rv.cores.Log(fmt.Sprintf("[blue]Connecting to RabbitMQ: %s (%s:%d)...", instance.Name, instance.Host, instance.AMQPPort))

	err := rv.rmqClient.Connect(instance)
	if err != nil {
		rv.cores.Log(fmt.Sprintf("[red]Failed to connect: %v", err))
		rv.updateInfoNotConnected()
		return
	}

	rv.cores.Log(fmt.Sprintf("[green]Connected to RabbitMQ: %s", instance.Name))
	rv.updateInfoConnected()
	rv.refresh()
}

// autoConnect connects to the first available instance
func (rv *RabbitMQView) autoConnect() {
	if len(rv.instances) == 0 {
		return
	}

	inst := rv.instances[0]
	rv.cores.Log(fmt.Sprintf("[blue]Auto-connecting to RabbitMQ: %s", inst.Name))
	rv.connectToInstance(inst)
}

// updateInfoConnected updates info text on all views
func (rv *RabbitMQView) updateInfoConnected() {
	name := rv.rmqClient.GetClusterName()
	rv.overviewView.SetInfoText(fmt.Sprintf("[green]RabbitMQ Manager[white]\nInstance: %s\nView: Overview", name))
	rv.queuesView.SetInfoText(fmt.Sprintf("[green]RabbitMQ Manager[white]\nInstance: %s\nView: Queues", name))
	rv.exchangesView.SetInfoText(fmt.Sprintf("[green]RabbitMQ Manager[white]\nInstance: %s\nView: Exchanges", name))
	rv.bindingsView.SetInfoText(fmt.Sprintf("[green]RabbitMQ Manager[white]\nInstance: %s\nView: Bindings", name))
	rv.connectionsView.SetInfoText(fmt.Sprintf("[green]RabbitMQ Manager[white]\nInstance: %s\nView: Connections", name))
	rv.channelsView.SetInfoText(fmt.Sprintf("[green]RabbitMQ Manager[white]\nInstance: %s\nView: Channels", name))
	rv.nodesView.SetInfoText(fmt.Sprintf("[green]RabbitMQ Manager[white]\nInstance: %s\nView: Nodes", name))
}

// updateInfoNotConnected updates info text when not connected
func (rv *RabbitMQView) updateInfoNotConnected() {
	msg := "[yellow]RabbitMQ Manager[white]\nStatus: Not Connected\nUse [green]Ctrl+T[white] to select instance"
	rv.overviewView.SetInfoText(msg)
	rv.queuesView.SetInfoText(msg)
	rv.exchangesView.SetInfoText(msg)
	rv.bindingsView.SetInfoText(msg)
	rv.connectionsView.SetInfoText(msg)
	rv.channelsView.SetInfoText(msg)
	rv.nodesView.SetInfoText(msg)
}

// startAutoRefresh starts the auto-refresh timer
func (rv *RabbitMQView) startAutoRefresh() {
	if rv.refreshTimer != nil {
		rv.refreshTimer.Stop()
	}

	rv.refreshTimer = time.AfterFunc(rv.refreshInterval, func() {
		if rv.rmqClient != nil && rv.rmqClient.IsConnected() {
			rv.app.QueueUpdate(func() {
				rv.refresh()
				rv.startAutoRefresh()
			})
		} else {
			rv.startAutoRefresh()
		}
	})
}

// showHelp shows the help modal
func (rv *RabbitMQView) showHelp() {
	helpText := `[yellow]RabbitMQ Manager Help[white]

[green]Key Bindings:[white]
R       - Refresh current view
Ctrl+T  - Connect to RabbitMQ instance
?       - Show this help

[green]View Navigation:[white]
O       - Overview (cluster summary)
Q       - Queues view
E       - Exchanges view
B       - Bindings view
C       - Connections view
H       - Channels view
S       - Nodes view

[green]Queues View:[white]
I       - Show queue details
N       - Create new queue
D       - Delete queue
P       - Purge queue
M       - Browse messages
U       - Publish message

[green]Exchanges View:[white]
I       - Show exchange details
N       - Create new exchange
D       - Delete exchange

[green]Connections View:[white]
I       - Show connection details
D       - Force close connection

[green]Navigation:[white]
Arrow keys  - Navigate list
ESC         - Close modal dialogs
`

	ui.ShowInfoModal(
		rv.pages,
		rv.app,
		"RabbitMQ Help",
		helpText,
		func() {
			current := rv.currentCores()
			if current != nil {
				rv.app.SetFocus(current.GetTable())
			}
		},
	)
}

// handleAction handles actions triggered by the UI
func (rv *RabbitMQView) handleAction(action string, payload map[string]interface{}) error {
	switch action {
	case "refresh":
		rv.refresh()
		return nil
	case "keypress":
		if key, ok := payload["key"].(string); ok {
			switch key {
			case "O":
				rv.showOverview()
				return nil
			case "Q":
				rv.showQueues()
				return nil
			case "E":
				rv.showExchanges()
				return nil
			case "B":
				rv.showBindings()
				return nil
			case "C":
				rv.showConnections()
				return nil
			case "H":
				rv.showChannels()
				return nil
			case "S":
				rv.showNodes()
				return nil
			case "?":
				rv.showHelp()
				return nil
			case "R":
				rv.refresh()
				return nil
			case "I":
				rv.showInfoForCurrentView()
				return nil
			case "N":
				rv.handleCreate()
				return nil
			case "D":
				rv.handleDelete()
				return nil
			case "P":
				if rv.currentView == rmqViewQueues {
					rv.purgeSelectedQueue()
					return nil
				}
			case "M":
				if rv.currentView == rmqViewQueues {
					rv.browseSelectedQueueMessages()
					return nil
				}
			case "U":
				if rv.currentView == rmqViewQueues {
					rv.publishToSelectedQueue()
					return nil
				}
			}
		}
	case "navigate_back":
		if view, ok := payload["current_view"].(string); ok {
			if view == rmqViewRoot {
				rv.switchView(rmqViewOverview)
				return nil
			}
			rv.switchView(view)
			return nil
		}
	}
	return fmt.Errorf("unhandled")
}

// showInfoForCurrentView shows detail info based on current view
func (rv *RabbitMQView) showInfoForCurrentView() {
	switch rv.currentView {
	case rmqViewQueues:
		rv.showQueueInfo()
	case rmqViewExchanges:
		rv.showExchangeInfo()
	case rmqViewConnections:
		rv.showConnectionInfo()
	case rmqViewChannels:
		rv.showChannelInfo()
	case rmqViewNodes:
		rv.showNodeInfo()
	}
}

// handleCreate dispatches create actions based on current view
func (rv *RabbitMQView) handleCreate() {
	switch rv.currentView {
	case rmqViewQueues:
		rv.showCreateQueueForm()
	case rmqViewExchanges:
		rv.showCreateExchangeForm()
	}
}

// handleDelete dispatches delete actions based on current view
func (rv *RabbitMQView) handleDelete() {
	switch rv.currentView {
	case rmqViewQueues:
		rv.deleteSelectedQueue()
	case rmqViewExchanges:
		rv.deleteSelectedExchange()
	case rmqViewConnections:
		rv.closeSelectedConnection()
	}
}

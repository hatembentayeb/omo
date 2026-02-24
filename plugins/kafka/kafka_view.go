package main

import (
	"fmt"
	"time"

	"github.com/rivo/tview"

	"omo/pkg/ui"
)

// KafkaView manages the UI for interacting with Kafka clusters
type KafkaView struct {
	app             *tview.Application
	pages           *tview.Pages
	viewPages       *tview.Pages
	brokersView     *ui.CoreView
	topicsView      *ui.CoreView
	consumersView   *ui.CoreView
	partitionsView  *ui.CoreView
	messagesView    *ui.CoreView
	cores           *ui.CoreView // alias for the currently active CoreView
	kafkaClient     *KafkaClient
	currentCluster  string
	currentView     string
	selectedTopic   string // topic selected when navigating to partitions
	refreshTimer    *time.Timer
	refreshInterval time.Duration
	instances       []KafkaInstance
}

// NewKafkaView creates a new Kafka view
func NewKafkaView(app *tview.Application, pages *tview.Pages) *KafkaView {
	kv := &KafkaView{
		app:             app,
		pages:           pages,
		viewPages:       tview.NewPages(),
		refreshInterval: 10 * time.Second,
	}

	// Initialize Kafka client
	kv.kafkaClient = NewKafkaClient()

	// Discover instances from KeePass
	instances, err := DiscoverInstances()
	if err == nil {
		kv.instances = instances
	}
	if uiCfg, uiErr := GetKafkaUIConfig(); uiErr == nil && uiCfg.RefreshInterval > 0 {
		kv.refreshInterval = time.Duration(uiCfg.RefreshInterval) * time.Second
	}

	// Create all sub-views
	kv.brokersView = kv.newBrokersView()
	kv.topicsView = kv.newTopicsView()
	kv.consumersView = kv.newConsumersView()
	kv.partitionsView = kv.newPartitionsView()
	kv.messagesView = kv.newMessagesView()

	// Set cores alias to default view
	kv.cores = kv.brokersView

	// Set modal pages on all views
	views := []*ui.CoreView{
		kv.brokersView,
		kv.topicsView,
		kv.consumersView,
		kv.partitionsView,
		kv.messagesView,
	}
	for _, view := range views {
		if view != nil {
			view.SetModalPages(kv.pages)
		}
	}

	// Register viewPages
	kv.viewPages.AddPage("kafka-brokers", kv.brokersView.GetLayout(), true, true)
	kv.viewPages.AddPage("kafka-topics", kv.topicsView.GetLayout(), true, false)
	kv.viewPages.AddPage("kafka-consumers", kv.consumersView.GetLayout(), true, false)
	kv.viewPages.AddPage("kafka-partitions", kv.partitionsView.GetLayout(), true, false)
	kv.viewPages.AddPage("kafka-messages", kv.messagesView.GetLayout(), true, false)

	// Set initial view
	kv.currentView = kafkaViewBrokers
	kv.setViewStack(kv.brokersView, kafkaViewBrokers)
	kv.setViewStack(kv.topicsView, kafkaViewTopics)
	kv.setViewStack(kv.consumersView, kafkaViewConsumers)
	kv.setViewStack(kv.partitionsView, kafkaViewPartitions)
	kv.setViewStack(kv.messagesView, kafkaViewMessages)

	// Set initial status
	kv.brokersView.SetInfoText("[yellow]Kafka Manager[white]\nStatus: Not Connected\nUse [green]Ctrl+T[white] to select cluster")

	// Auto-connect to first instance if available
	kv.autoConnect()

	// Start auto-refresh timer
	kv.startAutoRefresh()

	return kv
}

// GetMainUI returns the main UI component
func (kv *KafkaView) GetMainUI() tview.Primitive {
	return kv.viewPages
}

// Stop cleans up resources
func (kv *KafkaView) Stop() {
	if kv.refreshTimer != nil {
		kv.refreshTimer.Stop()
	}

	if kv.kafkaClient != nil && kv.kafkaClient.IsConnected() {
		kv.kafkaClient.Disconnect()
	}

	views := []*ui.CoreView{
		kv.brokersView,
		kv.topicsView,
		kv.consumersView,
		kv.partitionsView,
		kv.messagesView,
	}
	for _, view := range views {
		if view != nil {
			view.StopAutoRefresh()
			view.UnregisterHandlers()
		}
	}
}

// refresh refreshes the current view
func (kv *KafkaView) refresh() {
	current := kv.currentCores()
	if current != nil {
		current.RefreshData()
	}
}

// ShowClusterSelector displays the cluster selection modal
func (kv *KafkaView) ShowClusterSelector() {
	kv.cores.Log("[blue]Opening cluster selector...")

	instances, err := GetAvailableKafkaInstances()
	if err != nil {
		kv.cores.Log(fmt.Sprintf("[red]Failed to load Kafka instances: %v", err))
		return
	}

	if len(instances) == 0 {
		kv.cores.Log("[yellow]No Kafka instances configured in KeePass (create entries under kafka/<environment>/<name>)")
		return
	}

	// Create list items
	items := make([][]string, len(instances))
	for i, inst := range instances {
		items[i] = []string{
			inst.Name,
			fmt.Sprintf("%s - %s", inst.BootstrapServers, inst.Description),
		}
	}

	ui.ShowStandardListSelectorModal(
		kv.pages,
		kv.app,
		"Select Kafka Cluster",
		items,
		func(index int, name string, cancelled bool) {
			if !cancelled && index >= 0 && index < len(instances) {
				kv.connectToInstance(instances[index])
			} else {
				kv.cores.Log("[blue]Cluster selection cancelled")
			}
			kv.app.SetFocus(kv.currentCores().GetTable())
		},
	)
}

// connectToInstance connects to a Kafka cluster instance
func (kv *KafkaView) connectToInstance(instance KafkaInstance) {
	kv.cores.Log(fmt.Sprintf("[blue]Connecting to Kafka cluster: %s (%s)...", instance.Name, instance.BootstrapServers))

	err := kv.kafkaClient.Connect(instance.Name, instance.BootstrapServers, &instance)
	if err != nil {
		kv.cores.Log(fmt.Sprintf("[red]Failed to connect: %v", err))
		kv.updateInfoNotConnected()
		return
	}

	kv.currentCluster = instance.Name
	kv.cores.Log(fmt.Sprintf("[green]Connected to Kafka cluster: %s", instance.Name))

	// Update all views info
	kv.updateInfoConnected()

	// Refresh current view
	kv.refresh()
}

// autoConnect connects to the first available instance
func (kv *KafkaView) autoConnect() {
	if len(kv.instances) == 0 {
		return
	}

	inst := kv.instances[0]
	kv.cores.Log(fmt.Sprintf("[blue]Auto-connecting to Kafka cluster: %s", inst.Name))
	kv.connectToInstance(inst)
}

// updateInfoConnected updates info text on all views when connected
func (kv *KafkaView) updateInfoConnected() {
	clusterName := kv.kafkaClient.GetCurrentCluster()
	kv.brokersView.SetInfoText(fmt.Sprintf("[green]Kafka Manager[white]\nCluster: %s\nView: Brokers", clusterName))
	kv.topicsView.SetInfoText(fmt.Sprintf("[green]Kafka Manager[white]\nCluster: %s\nView: Topics", clusterName))
	kv.consumersView.SetInfoText(fmt.Sprintf("[green]Kafka Manager[white]\nCluster: %s\nView: Consumers", clusterName))
	kv.partitionsView.SetInfoText(fmt.Sprintf("[green]Kafka Manager[white]\nCluster: %s\nView: Partitions", clusterName))
	kv.messagesView.SetInfoText(fmt.Sprintf("[green]Kafka Manager[white]\nCluster: %s\nView: Messages", clusterName))
}

// updateInfoNotConnected updates info text when not connected
func (kv *KafkaView) updateInfoNotConnected() {
	msg := "[yellow]Kafka Manager[white]\nStatus: Not Connected\nUse [green]Ctrl+T[white] to select cluster"
	kv.brokersView.SetInfoText(msg)
	kv.topicsView.SetInfoText(msg)
	kv.consumersView.SetInfoText(msg)
	kv.partitionsView.SetInfoText(msg)
	kv.messagesView.SetInfoText(msg)
}

// startAutoRefresh starts the auto-refresh timer
func (kv *KafkaView) startAutoRefresh() {
	if kv.refreshTimer != nil {
		kv.refreshTimer.Stop()
	}

	kv.refreshTimer = time.AfterFunc(kv.refreshInterval, func() {
		if kv.kafkaClient != nil && kv.kafkaClient.IsConnected() {
			kv.app.QueueUpdate(func() {
				kv.refresh()
				kv.startAutoRefresh()
			})
		} else {
			kv.startAutoRefresh()
		}
	})
}

// showHelp shows the help modal
func (kv *KafkaView) showHelp() {
	helpText := `[yellow]Kafka Manager Help[white]

[green]Key Bindings:[white]
R       - Refresh current view
Ctrl+T  - Connect to Kafka cluster
?       - Show this help

[green]View Navigation:[white]
B       - Brokers view
T       - Topics view
G       - Consumer Groups view
P       - Partitions (select a topic first)
M       - Messages (select a topic first)

[green]Broker View:[white]
I       - Show broker details

[green]Topics View:[white]
I       - Show topic details
P       - Show partitions for selected topic
M       - Browse messages for selected topic

[green]Messages View:[white]
I       - Show full message content
T       - Back to Topics

[green]Consumer Groups View:[white]
I       - Show consumer group details
O       - Show consumer group offsets

[green]Navigation:[white]
Arrow keys  - Navigate list
ESC         - Close modal dialogs
`

	ui.ShowInfoModal(
		kv.pages,
		kv.app,
		"Kafka Help",
		helpText,
		func() {
			current := kv.currentCores()
			if current != nil {
				kv.app.SetFocus(current.GetTable())
			}
		},
	)
}

// handleAction handles actions triggered by the UI
func (kv *KafkaView) handleAction(action string, payload map[string]interface{}) error {
	switch action {
	case "refresh":
		kv.refresh()
		return nil
	case "keypress":
		if key, ok := payload["key"].(string); ok {
			if kv.handleKafkaKeys(key) {
				return nil
			}
		}
	case "navigate_back":
		if view, ok := payload["current_view"].(string); ok {
			if view == kafkaViewRoot {
				kv.switchView(kafkaViewBrokers)
				return nil
			}
			kv.switchView(view)
			return nil
		}
	}
	return fmt.Errorf("unhandled")
}

func (kv *KafkaView) handleKafkaKeys(key string) bool {
	switch key {
	case "B":
		kv.showBrokers()
	case "T":
		kv.showTopics()
	case "G":
		kv.showConsumers()
	case "?":
		kv.showHelp()
	case "R":
		kv.refresh()
	case "I":
		kv.showInfoForCurrentView()
	case "M":
		if kv.currentView == kafkaViewTopics {
			kv.showMessagesForSelectedTopic()
			return true
		}
		return false
	case "P":
		if kv.currentView == kafkaViewTopics {
			kv.showPartitionsForSelectedTopic()
			return true
		}
		kv.showPartitions()
	case "O":
		if kv.currentView == kafkaViewConsumers {
			kv.showConsumerOffsets()
			return true
		}
		return false
	default:
		return false
	}
	return true
}

// showInfoForCurrentView shows detail info based on current view
func (kv *KafkaView) showInfoForCurrentView() {
	switch kv.currentView {
	case kafkaViewBrokers:
		kv.showBrokerInfo()
	case kafkaViewTopics:
		kv.showTopicInfo()
	case kafkaViewConsumers:
		kv.showConsumerGroupInfo()
	case kafkaViewPartitions:
		kv.showPartitionInfo()
	case kafkaViewMessages:
		kv.showMessageDetail()
	}
}

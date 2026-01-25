package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/rivo/tview"

	"omo/pkg/ui"
)

// BrokerView manages the UI for viewing Kafka brokers
type BrokerView struct {
	app            *tview.Application
	pages          *tview.Pages
	cores          *ui.Cores
	kafkaClient    *KafkaClient
	currentCluster string
	brokers        []BrokerInfo
	topicsView     *TopicsView
	consumersView  *ConsumersView
	partitionsView *PartitionsView
}

// BrokerInfo represents a Kafka broker's information
type BrokerInfo struct {
	ID             int
	Host           string
	Port           int
	Controller     bool
	Version        string
	Status         string
	PartitionCount int
}

// NewBrokerView creates a new brokers view
func NewBrokerView(app *tview.Application, pages *tview.Pages) *BrokerView {
	bv := &BrokerView{
		app:            app,
		pages:          pages,
		currentCluster: "",
		brokers:        []BrokerInfo{},
	}

	// Create Cores UI component
	bv.cores = ui.NewCores(app, "")
	bv.cores.SetModalPages(pages)

	// Initialize with root view
	bv.cores.PushView("kafka")

	// Set table headers
	bv.cores.SetTableHeaders([]string{"ID", "Host", "Port", "Controller", "Version", "Status", "Partitions"})

	// Set up refresh callback to make 'R' key work properly
	bv.cores.SetRefreshCallback(func() ([][]string, error) {
		return bv.refreshBrokers()
	})

	// Set action callback to handle keypresses
	bv.cores.SetActionCallback(func(action string, payload map[string]interface{}) error {
		if action == "keypress" {
			if key, ok := payload["key"].(string); ok {
				switch key {
				case "R":
					bv.refresh()
					return nil
				case "C":
					bv.showClusterSelector()
					return nil
				case "?":
					bv.showHelp()
					return nil
				case "I":
					bv.showBrokerInfo()
					return nil
				case "T":
					bv.showTopicsForBroker()
					return nil
				case "A":
					bv.showAllTopics()
					return nil
				case "G":
					bv.showAllConsumers()
					return nil
				}
			}
		} else if action == "navigate_back" {
			// Handle navigation events
			if viewName, ok := payload["current_view"].(string); ok {
				bv.showViewByName(viewName)
				return nil
			}
		}
		return nil
	})

	// Add key bindings
	bv.cores.AddKeyBinding("R", "Refresh", bv.refresh)
	bv.cores.AddKeyBinding("C", "Connect", bv.showClusterSelector)
	bv.cores.AddKeyBinding("?", "Help", bv.showHelp)
	bv.cores.AddKeyBinding("I", "Info", bv.showBrokerInfo)
	bv.cores.AddKeyBinding("T", "Topics", bv.showTopicsForBroker)
	bv.cores.AddKeyBinding("A", "Topics", bv.showAllTopics)
	bv.cores.AddKeyBinding("G", "Consumers", nil)

	// Set row selection callback for tracking selection
	bv.cores.SetRowSelectedCallback(func(row int) {
		if row >= 0 && row < len(bv.brokers) {
			bv.cores.Log(fmt.Sprintf("[blue]Selected broker: %d (%s:%d)",
				bv.brokers[row].ID, bv.brokers[row].Host, bv.brokers[row].Port))
		}
	})

	// Register the key handlers to actually handle the key events
	bv.cores.RegisterHandlers()

	// Initialize Kafka client
	bv.kafkaClient = NewKafkaClient()

	// Initialize the views with the local cluster name
	bv.currentCluster = "local"
	bv.topicsView = NewTopicsView(app, pages, bv.kafkaClient, bv.currentCluster, -1)
	bv.consumersView = NewConsumersView(app, pages, bv.kafkaClient, bv.currentCluster, "")
	bv.partitionsView = NewPartitionsView(app, pages, bv.kafkaClient, bv.currentCluster, "")
	// Add the views to the pages
	pages.AddPage("topics", bv.topicsView.GetMainUI(), true, false)
	pages.AddPage("consumers", bv.consumersView.GetMainUI(), true, false)

	// Automatically connect to the local cluster to show some data
	bv.cores.Log("[blue]Automatically connecting to local Kafka cluster")

	// Initial refresh to show data
	bv.refresh()

	return bv
}

// showViewByName displays the specified view based on its name
func (bv *BrokerView) showViewByName(viewName string) {
	switch viewName {
	case "brokers":
		bv.pages.SwitchToPage("main")
		bv.app.SetFocus(bv.cores.GetTable())
	case "topics":
		bv.pages.SwitchToPage("topics")
		bv.app.SetFocus(bv.topicsView.cores.GetTable())
	default:
		bv.pages.SwitchToPage("main")
		bv.app.SetFocus(bv.cores.GetTable())
	}
}

// GetMainUI returns the main UI component
func (bv *BrokerView) GetMainUI() tview.Primitive {
	// Ensure table gets focus when this view is shown
	bv.app.SetFocus(bv.cores.GetTable())
	return bv.cores.GetLayout()
}

// refreshBrokers refreshes the broker list
func (bv *BrokerView) refreshBrokers() ([][]string, error) {
	if bv.kafkaClient == nil || bv.currentCluster == "" {
		// No client or cluster, show empty data
		bv.brokers = []BrokerInfo{}
		bv.cores.SetTableData([][]string{})
		bv.cores.SetInfoText(fmt.Sprintf("[yellow]Kafka Brokers[white]\nStatus: Not Connected"))
		return [][]string{}, nil
	}

	// In a real implementation, this would fetch actual broker data
	// For now, let's simulate some sample data
	bv.brokers = []BrokerInfo{
		{ID: 1, Host: "localhost", Port: 9092, Controller: true, Version: "3.5.0", Status: "Online", PartitionCount: 24},
		{ID: 2, Host: "kafka-2", Port: 9092, Controller: false, Version: "3.5.0", Status: "Online", PartitionCount: 18},
		{ID: 3, Host: "kafka-3", Port: 9092, Controller: false, Version: "3.5.0", Status: "Online", PartitionCount: 20},
	}

	// Convert to table data
	tableData := make([][]string, len(bv.brokers))
	for i, broker := range bv.brokers {
		tableData[i] = []string{
			strconv.Itoa(broker.ID),
			broker.Host,
			strconv.Itoa(broker.Port),
			fmt.Sprintf("%t", broker.Controller),
			broker.Version,
			broker.Status,
			strconv.Itoa(broker.PartitionCount),
		}
	}

	// Update table data
	bv.cores.SetTableData(tableData)

	// Update info text
	bv.cores.SetInfoText(fmt.Sprintf("[yellow]Kafka Brokers[white]\nCluster: %s\nBrokers: %d",
		bv.currentCluster, len(bv.brokers)))

	return tableData, nil
}

// refresh manually refreshes the broker list
func (bv *BrokerView) refresh() {
	bv.cores.RefreshData()
}

// showClusterSelector shows the cluster selection modal
func (bv *BrokerView) showClusterSelector() {
	// Sample clusters for demonstration
	clusters := [][]string{
		{"local", "localhost:9092"},
		{"development", "kafka-1:9092"},
		{"staging", "kafka-2:9092"},
		{"production", "kafka-3:9092"},
	}

	// Show the list selector modal
	ui.ShowStandardListSelectorModal(
		bv.pages,
		bv.app,
		"Select Kafka Cluster",
		clusters,
		func(index int, name string, cancelled bool) {
			// Ensure table regains focus after modal is closed
			bv.app.SetFocus(bv.cores.GetTable())

			if !cancelled && index >= 0 {
				bv.currentCluster = name

				// If using the KafkaClient, connect to the cluster
				if bv.kafkaClient != nil {
					if err := bv.kafkaClient.Connect(name); err != nil {
						bv.cores.Log(fmt.Sprintf("[red]Error connecting to cluster: %v", err))
					}
				}

				bv.cores.Log(fmt.Sprintf("[blue]Connected to Kafka cluster: %s", name))
				bv.refresh()

				// Update the cluster for the other views
				if bv.topicsView != nil {
					bv.topicsView.currentCluster = name
				}
				if bv.consumersView != nil {
					bv.consumersView.currentCluster = name
				}
			}
		},
	)
}

// showHelp shows the help modal
func (bv *BrokerView) showHelp() {
	helpText := `[yellow]Kafka Broker View Help[white]

[aqua]Key Bindings:[white]
[green]R[white] - Refresh brokers list
[green]C[white] - Connect to a different Kafka cluster
[green]I[white] - Show detailed information about the selected broker
[green]T[white] - Show topics hosted on the selected broker
[green]A[white] - Show all topics across all brokers
[green]G[white] - Show all consumer groups
[green]?[white] - Show this help information
[green]ESC[white] - Navigate back to previous view

[aqua]Navigation:[white]
- Use ESC to go back to previous views
- The breadcrumb at the bottom shows your current location
- Select a broker by clicking on it or using arrow keys
`

	ui.ShowInfoModal(
		bv.pages,
		bv.app,
		"Help",
		helpText,
		func() {
			// Ensure table regains focus after modal is closed
			bv.app.SetFocus(bv.cores.GetTable())
		},
	)
}

// showBrokerInfo shows detailed information about the selected broker
func (bv *BrokerView) showBrokerInfo() {
	selectedRow := bv.cores.GetSelectedRow()
	if selectedRow < 0 || selectedRow >= len(bv.brokers) {
		bv.cores.Log("[red]No broker selected")
		return
	}

	broker := bv.brokers[selectedRow]

	// Format the broker information as a string
	infoText := fmt.Sprintf(`[yellow]Broker #%d Information[white]

[aqua]Host:[white] %s
[aqua]Port:[white] %d
[aqua]Controller:[white] %t
[aqua]Version:[white] %s
[aqua]Status:[white] %s
[aqua]Partition Count:[white] %d

This broker is currently hosting %d partitions and its status is %s.
`, broker.ID, broker.Host, broker.Port, broker.Controller, broker.Version, broker.Status, broker.PartitionCount, broker.PartitionCount, broker.Status)

	// Show the information in a modal
	ui.ShowInfoModal(
		bv.pages,
		bv.app,
		fmt.Sprintf("Broker #%d Info", broker.ID),
		infoText,
		func() {
			// Ensure table regains focus after modal is closed
			bv.app.SetFocus(bv.cores.GetTable())
		},
	)
}

// showTopicsForBroker shows topics for the selected broker
func (bv *BrokerView) showTopicsForBroker() {
	selectedRow := bv.cores.GetSelectedRow()
	if selectedRow < 0 || selectedRow >= len(bv.brokers) {
		bv.cores.Log("[red]No broker selected")
		return
	}

	broker := bv.brokers[selectedRow]

	// Initialize topics view if needed
	if bv.topicsView == nil {
		bv.topicsView = NewTopicsView(bv.app, bv.pages, bv.kafkaClient, bv.currentCluster, broker.ID)
		// Copy the current navigation stack to the new view
		bv.topicsView.cores.CopyNavigationStackFrom(bv.cores)
	} else {
		bv.topicsView.currentBroker = broker.ID
		// Update the navigation stack
		bv.topicsView.cores.CopyNavigationStackFrom(bv.cores)
	}

	// Push topics view onto stack
	bv.topicsView.cores.PushView("topics")

	// Show the topics view for this broker
	bv.cores.Log(fmt.Sprintf("[blue]Showing topics for broker: %d", broker.ID))

	// Show the topics page
	bv.pages.SwitchToPage("topics")
	bv.topicsView.refresh()
	bv.app.SetFocus(bv.topicsView.cores.GetTable())
}

// showAllTopics shows all topics across all brokers
func (bv *BrokerView) showAllTopics() {
	// Initialize topics view if needed
	if bv.topicsView == nil {
		bv.topicsView = NewTopicsView(bv.app, bv.pages, bv.kafkaClient, bv.currentCluster, -1)
		// Copy the current navigation stack to the new view
		bv.topicsView.cores.CopyNavigationStackFrom(bv.cores)
	} else {
		bv.topicsView.currentBroker = -1
		// Update the navigation stack
		bv.topicsView.cores.CopyNavigationStackFrom(bv.cores)
	}

	// Push topics view onto stack
	bv.topicsView.cores.PushView("topics")

	// Show all topics
	bv.cores.Log("[blue]Showing all Kafka topics")

	// Show the topics page
	bv.pages.SwitchToPage("topics")
	bv.topicsView.refresh()
	bv.app.SetFocus(bv.topicsView.cores.GetTable())
}

// showAllConsumers shows all consumer groups
func (bv *BrokerView) showAllConsumers() {
	// Initialize consumers view if needed
	if bv.consumersView == nil {
		bv.consumersView = NewConsumersView(bv.app, bv.pages, bv.kafkaClient, bv.currentCluster, "")
		// Copy the current navigation stack to the new view
		bv.consumersView.cores.CopyNavigationStackFrom(bv.cores)
	} else {
		bv.consumersView.currentTopic = ""
		// Update the navigation stack
		bv.consumersView.cores.CopyNavigationStackFrom(bv.cores)
	}

	// Push consumers view onto stack
	bv.consumersView.cores.PushView("consumers")

	// Show all consumers
	bv.cores.Log("[blue]Showing all Kafka consumer groups")

	// Show the consumers page
	bv.pages.SwitchToPage("consumers")
	bv.consumersView.refresh()
	bv.app.SetFocus(bv.consumersView.cores.GetTable())
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1f seconds", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.1f minutes", d.Minutes())
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%.1f hours", d.Hours())
	}
	return fmt.Sprintf("%.1f days", d.Hours()/24)
}

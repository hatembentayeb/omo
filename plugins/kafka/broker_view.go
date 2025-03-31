package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/rivo/tview"

	"omo/ui"
)

// BrokerView manages the UI for viewing Kafka brokers
type BrokerView struct {
	app            *tview.Application
	pages          *tview.Pages
	cores          *ui.Cores
	kafkaClient    *KafkaClient
	currentCluster string
	brokers        []BrokerInfo
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
	bv.cores = ui.NewCores(app, "Kafka Brokers")

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
		}
		return nil
	})

	// Add key bindings
	bv.cores.AddKeyBinding("R", "Refresh", nil)
	bv.cores.AddKeyBinding("C", "Connect", nil)
	bv.cores.AddKeyBinding("?", "Help", nil)
	bv.cores.AddKeyBinding("I", "Info", nil)
	bv.cores.AddKeyBinding("T", "Topics", nil)
	bv.cores.AddKeyBinding("A", "Topics", nil)
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

	// Automatically connect to the local cluster to show some data
	bv.currentCluster = "local"
	bv.cores.Log("[blue]Automatically connecting to local Kafka cluster")

	// Initial refresh to show data
	bv.refresh()

	return bv
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
[green]ESC[white] - Close modal dialogs

[aqua]Usage Tips:[white]
- Select a broker by clicking on it or using arrow keys
- Use the refresh button to update the broker list
- You can sort the list by clicking on column headers
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

	// In a real implementation, we'd get more detailed information from the broker
	infoText := fmt.Sprintf(`[yellow]Broker Details[white]

[aqua]ID:[white] %d
[aqua]Host:[white] %s
[aqua]Port:[white] %d
[aqua]Controller:[white] %t
[aqua]Version:[white] %s
[aqua]Status:[white] %s
[aqua]Partition Count:[white] %d
[aqua]Topic Count:[white] 15
[aqua]JVM Version:[white] OpenJDK 17.0.2
[aqua]Heap Size:[white] 1024 MB
[aqua]Uptime:[white] %s
[aqua]Connections:[white] 24 active
`,
		broker.ID, broker.Host, broker.Port, broker.Controller,
		broker.Version, broker.Status, broker.PartitionCount,
		formatDuration(time.Hour*24*3+time.Hour*7+time.Minute*23)) // Example uptime

	ui.ShowInfoModal(
		bv.pages,
		bv.app,
		fmt.Sprintf("Broker #%d Information", broker.ID),
		infoText,
		func() {
			// Ensure table regains focus after modal is closed
			bv.app.SetFocus(bv.cores.GetTable())
		},
	)
}

// showTopicsForBroker shows topics hosted on the selected broker
func (bv *BrokerView) showTopicsForBroker() {
	selectedRow := bv.cores.GetSelectedRow()
	if selectedRow < 0 || selectedRow >= len(bv.brokers) {
		bv.cores.Log("[red]No broker selected")
		return
	}

	broker := bv.brokers[selectedRow]

	// Create a topics view for this broker
	topicsView := NewTopicsView(bv.app, bv.pages, bv.kafkaClient, bv.currentCluster, broker.ID)

	// Add the topics view as a new page
	bv.pages.AddPage("topics-view", topicsView.GetMainUI(), true, true)

	// Switch to the topics view
	bv.pages.SwitchToPage("topics-view")

	bv.cores.Log(fmt.Sprintf("[blue]Showing topics for broker #%d", broker.ID))
}

// showAllTopics shows all topics
func (bv *BrokerView) showAllTopics() {
	// Create a topics view for all brokers (brokerID = -1 means all brokers)
	topicsView := NewTopicsView(bv.app, bv.pages, bv.kafkaClient, bv.currentCluster, -1)

	// Add the topics view as a new page
	bv.pages.AddPage("topics-view", topicsView.GetMainUI(), true, true)

	// Switch to the topics view
	bv.pages.SwitchToPage("topics-view")

	bv.cores.Log("[blue]Showing topics for all brokers")
}

// showAllConsumers shows all consumers from all brokers
func (bv *BrokerView) showAllConsumers() {
	// Create a consumers view for all consumers (empty string means all topics)
	consumersView := NewConsumersView(bv.app, bv.pages, bv.kafkaClient, bv.currentCluster, "")

	// Add the consumers view as a new page
	bv.pages.AddPage("consumers-view", consumersView.GetMainUI(), true, true)

	// Switch to the consumers view
	bv.pages.SwitchToPage("consumers-view")

	bv.cores.Log("[blue]Showing all consumers")
}

// Helper functions

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%d days, %d hours, %d minutes", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%d hours, %d minutes", hours, minutes)
	}
	return fmt.Sprintf("%d minutes", minutes)
}

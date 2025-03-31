package main

import (
	"fmt"
	"strconv"

	"github.com/rivo/tview"

	"omo/ui"
)

// TopicInfo represents information about a Kafka topic
type TopicInfo struct {
	Name              string
	Partitions        int
	ReplicationFactor int
	LeaderBroker      int
	Status            string
	Size              string
	MessageCount      int64
	ConsumerGroups    int
}

// TopicsView manages the UI for viewing Kafka topics
type TopicsView struct {
	app            *tview.Application
	pages          *tview.Pages
	cores          *ui.Cores
	kafkaClient    *KafkaClient
	currentCluster string
	currentBroker  int // -1 means all brokers
	topics         []TopicInfo
}

// NewTopicsView creates a new topics view
func NewTopicsView(app *tview.Application, pages *tview.Pages, kafkaClient *KafkaClient, cluster string, brokerID int) *TopicsView {
	tv := &TopicsView{
		app:            app,
		pages:          pages,
		kafkaClient:    kafkaClient,
		currentCluster: cluster,
		currentBroker:  brokerID,
		topics:         []TopicInfo{},
	}

	// Create Cores UI component
	var title string
	if brokerID >= 0 {
		title = fmt.Sprintf("Kafka Topics (Broker #%d)", brokerID)
	} else {
		title = "Kafka Topics"
	}
	tv.cores = ui.NewCores(app, title)

	// Set table headers
	tv.cores.SetTableHeaders([]string{"Name", "Partitions", "Replication", "Leader", "Status", "Size", "Messages", "Consumers"})

	// Set up refresh callback to make 'R' key work properly
	tv.cores.SetRefreshCallback(func() ([][]string, error) {
		return tv.refreshTopics()
	})

	// Set action callback to handle keypresses
	tv.cores.SetActionCallback(func(action string, payload map[string]interface{}) error {
		if action == "keypress" {
			if key, ok := payload["key"].(string); ok {
				switch key {
				case "R":
					tv.refresh()
					return nil
				case "?":
					tv.showHelp()
					return nil
				case "I":
					tv.showTopicInfo()
					return nil
				case "C":
					tv.showConsumers()
					return nil
				case "P":
					tv.showPartitionDetails()
					return nil
				case "B":
					tv.returnToBrokers()
					return nil
				}
			}
		}
		return nil
	})

	// Add key bindings
	tv.cores.AddKeyBinding("R", "Refresh", nil)
	tv.cores.AddKeyBinding("?", "Help", nil)
	tv.cores.AddKeyBinding("I", "Info", nil)
	tv.cores.AddKeyBinding("C", "Consumers", nil)
	tv.cores.AddKeyBinding("P", "Partitions", nil)
	tv.cores.AddKeyBinding("B", "Back", nil)

	// Set row selection callback for tracking selection
	tv.cores.SetRowSelectedCallback(func(row int) {
		if row >= 0 && row < len(tv.topics) {
			tv.cores.Log(fmt.Sprintf("[blue]Selected topic: %s (%d partitions)",
				tv.topics[row].Name, tv.topics[row].Partitions))
		}
	})

	// Register the key handlers to actually handle the key events
	tv.cores.RegisterHandlers()

	// Set info text
	if brokerID >= 0 {
		tv.cores.SetInfoText(fmt.Sprintf("[yellow]Kafka Topics[white]\nCluster: %s\nBroker: %d",
			cluster, brokerID))
	} else {
		tv.cores.SetInfoText(fmt.Sprintf("[yellow]Kafka Topics[white]\nCluster: %s\nAll Brokers",
			cluster))
	}

	// Initial refresh to show data
	tv.refresh()

	return tv
}

// GetMainUI returns the main UI component
func (tv *TopicsView) GetMainUI() tview.Primitive {
	// Ensure table gets focus when this view is shown
	tv.app.SetFocus(tv.cores.GetTable())
	return tv.cores.GetLayout()
}

// refreshTopics refreshes the topics list
func (tv *TopicsView) refreshTopics() ([][]string, error) {
	if tv.kafkaClient == nil || tv.currentCluster == "" {
		// No client or cluster, show empty data
		tv.topics = []TopicInfo{}
		tv.cores.SetTableData([][]string{})
		if tv.currentBroker >= 0 {
			tv.cores.SetInfoText(fmt.Sprintf("[yellow]Kafka Topics[white]\nCluster: Not Connected\nBroker: %d", tv.currentBroker))
		} else {
			tv.cores.SetInfoText("[yellow]Kafka Topics[white]\nCluster: Not Connected\nAll Brokers")
		}
		return [][]string{}, nil
	}

	// In a real implementation, we would fetch actual topic data
	// For now, let's simulate some sample data
	tv.topics = []TopicInfo{
		{Name: "orders", Partitions: 8, ReplicationFactor: 3, LeaderBroker: 1, Status: "Online", Size: "2.7 GB", MessageCount: 18500000, ConsumerGroups: 3},
		{Name: "customers", Partitions: 4, ReplicationFactor: 3, LeaderBroker: 2, Status: "Online", Size: "1.2 GB", MessageCount: 3200000, ConsumerGroups: 2},
		{Name: "payments", Partitions: 6, ReplicationFactor: 3, LeaderBroker: 3, Status: "Online", Size: "3.5 GB", MessageCount: 9700000, ConsumerGroups: 4},
		{Name: "inventory", Partitions: 4, ReplicationFactor: 3, LeaderBroker: 1, Status: "Online", Size: "1.8 GB", MessageCount: 4300000, ConsumerGroups: 2},
		{Name: "shipments", Partitions: 2, ReplicationFactor: 3, LeaderBroker: 2, Status: "Online", Size: "950 MB", MessageCount: 1800000, ConsumerGroups: 1},
		{Name: "events", Partitions: 8, ReplicationFactor: 3, LeaderBroker: 1, Status: "Online", Size: "4.2 GB", MessageCount: 25000000, ConsumerGroups: 5},
		{Name: "metrics", Partitions: 2, ReplicationFactor: 3, LeaderBroker: 3, Status: "Online", Size: "1.5 GB", MessageCount: 32000000, ConsumerGroups: 2},
		{Name: "logs", Partitions: 8, ReplicationFactor: 3, LeaderBroker: 1, Status: "Online", Size: "7.3 GB", MessageCount: 48000000, ConsumerGroups: 1},
	}

	// If viewing for a specific broker, filter the topics
	if tv.currentBroker >= 0 {
		filteredTopics := []TopicInfo{}
		for _, topic := range tv.topics {
			if topic.LeaderBroker == tv.currentBroker {
				filteredTopics = append(filteredTopics, topic)
			}
		}
		tv.topics = filteredTopics
	}

	// Convert to table data
	tableData := make([][]string, len(tv.topics))
	for i, topic := range tv.topics {
		tableData[i] = []string{
			topic.Name,
			strconv.Itoa(topic.Partitions),
			strconv.Itoa(topic.ReplicationFactor),
			strconv.Itoa(topic.LeaderBroker),
			topic.Status,
			topic.Size,
			strconv.FormatInt(topic.MessageCount, 10),
			strconv.Itoa(topic.ConsumerGroups),
		}
	}

	// Update table data
	tv.cores.SetTableData(tableData)

	// Update info text
	if tv.currentBroker >= 0 {
		tv.cores.SetInfoText(fmt.Sprintf("[yellow]Kafka Topics[white]\nCluster: %s\nBroker: %d\nTopics: %d",
			tv.currentCluster, tv.currentBroker, len(tv.topics)))
	} else {
		tv.cores.SetInfoText(fmt.Sprintf("[yellow]Kafka Topics[white]\nCluster: %s\nAll Brokers\nTopics: %d",
			tv.currentCluster, len(tv.topics)))
	}

	return tableData, nil
}

// refresh manually refreshes the topics list
func (tv *TopicsView) refresh() {
	tv.cores.RefreshData()
}

// showHelp shows the help modal
func (tv *TopicsView) showHelp() {
	helpText := `[yellow]Kafka Topics View Help[white]

[aqua]Key Bindings:[white]
[green]R[white] - Refresh topics list
[green]I[white] - Show detailed information about the selected topic
[green]C[white] - Show consumer groups for the selected topic
[green]P[white] - Show partition details for the selected topic
[green]B[white] - Return to brokers view
[green]?[white] - Show this help information
[green]ESC[white] - Close modal dialogs

[aqua]Usage Tips:[white]
- Select a topic by clicking on it or using arrow keys
- Use the refresh button to update the topics list
- You can sort the list by clicking on column headers
`

	ui.ShowInfoModal(
		tv.pages,
		tv.app,
		"Help",
		helpText,
		func() {
			// Ensure table regains focus after modal is closed
			tv.app.SetFocus(tv.cores.GetTable())
		},
	)
}

// showTopicInfo shows detailed information about the selected topic
func (tv *TopicsView) showTopicInfo() {
	selectedRow := tv.cores.GetSelectedRow()
	if selectedRow < 0 || selectedRow >= len(tv.topics) {
		tv.cores.Log("[red]No topic selected")
		return
	}

	topic := tv.topics[selectedRow]

	// In a real implementation, we'd get more detailed information about the topic
	infoText := fmt.Sprintf(`[yellow]Topic Details[white]

[aqua]Name:[white] %s
[aqua]Partitions:[white] %d
[aqua]Replication Factor:[white] %d
[aqua]Status:[white] %s
[aqua]Leader Broker:[white] %d
[aqua]Size:[white] %s
[aqua]Messages:[white] %d
[aqua]Consumer Groups:[white] %d
[aqua]Retention Policy:[white] 7 days
[aqua]Cleanup Policy:[white] delete
[aqua]Compression:[white] snappy
[aqua]Creation Time:[white] 2023-06-15 08:45:23
[aqua]Last Modified:[white] 2023-11-02 14:12:57
`,
		topic.Name, topic.Partitions, topic.ReplicationFactor, topic.Status,
		topic.LeaderBroker, topic.Size, topic.MessageCount, topic.ConsumerGroups)

	ui.ShowInfoModal(
		tv.pages,
		tv.app,
		fmt.Sprintf("Topic '%s' Information", topic.Name),
		infoText,
		func() {
			// Ensure table regains focus after modal is closed
			tv.app.SetFocus(tv.cores.GetTable())
		},
	)
}

// showConsumers shows consumer groups for the selected topic
func (tv *TopicsView) showConsumers() {
	selectedRow := tv.cores.GetSelectedRow()
	if selectedRow < 0 || selectedRow >= len(tv.topics) {
		tv.cores.Log("[red]No topic selected")
		return
	}

	topic := tv.topics[selectedRow]

	// Create a consumers view for this topic
	consumersView := NewConsumersView(tv.app, tv.pages, tv.kafkaClient, tv.currentCluster, topic.Name)

	// Add the consumers view as a new page
	tv.pages.AddPage("consumers-view", consumersView.GetMainUI(), true, true)

	// Switch to the consumers view
	tv.pages.SwitchToPage("consumers-view")

	tv.cores.Log(fmt.Sprintf("[blue]Showing consumers for topic '%s'", topic.Name))
}

// showPartitionDetails shows partition details for the selected topic
func (tv *TopicsView) showPartitionDetails() {
	selectedRow := tv.cores.GetSelectedRow()
	if selectedRow < 0 || selectedRow >= len(tv.topics) {
		tv.cores.Log("[red]No topic selected")
		return
	}

	topic := tv.topics[selectedRow]

	// Create a partitions view for this topic
	partitionsView := NewPartitionsView(tv.app, tv.pages, tv.kafkaClient, tv.currentCluster, topic.Name)

	// Add the partitions view as a new page
	tv.pages.AddPage("partitions-view", partitionsView.GetMainUI(), true, true)

	// Switch to the partitions view
	tv.pages.SwitchToPage("partitions-view")

	tv.cores.Log(fmt.Sprintf("[blue]Showing partitions for topic '%s'", topic.Name))
}

// returnToBrokers switches back to the brokers view
func (tv *TopicsView) returnToBrokers() {
	tv.cores.Log("[blue]Returning to brokers view")

	// Return to main page
	tv.pages.SwitchToPage("main")
}

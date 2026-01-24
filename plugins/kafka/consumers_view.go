package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/rivo/tview"

	"omo/pkg/ui"
)

// ConsumerInfo represents information about a Kafka consumer
type ConsumerInfo struct {
	GroupID      string
	Topic        string
	Members      int
	Lag          int64
	Status       string
	Partitions   []int
	LastActivity time.Time
	ClientID     string
}

// ConsumersView manages the UI for viewing Kafka consumer groups
type ConsumersView struct {
	app            *tview.Application
	pages          *tview.Pages
	cores          *ui.Cores
	kafkaClient    *KafkaClient
	currentCluster string
	currentTopic   string // empty string means all topics
	consumers      []ConsumerInfo
}

// NewConsumersView creates a new consumers view
func NewConsumersView(app *tview.Application, pages *tview.Pages, kafkaClient *KafkaClient, cluster string, topic string) *ConsumersView {
	cv := &ConsumersView{
		app:            app,
		pages:          pages,
		kafkaClient:    kafkaClient,
		currentCluster: cluster,
		currentTopic:   topic,
		consumers:      []ConsumerInfo{},
	}

	// Create Cores UI component
	cv.cores = ui.NewCores(app, "")

	// Set table headers
	cv.cores.SetTableHeaders([]string{"Group ID", "Topic", "Partitions", "Lag", "Status", "Members", "Pattern"})

	// Set up refresh callback to make 'R' key work properly
	cv.cores.SetRefreshCallback(func() ([][]string, error) {
		return cv.refreshConsumers()
	})

	// Set action callback to handle keypresses
	cv.cores.SetActionCallback(func(action string, payload map[string]interface{}) error {
		if action == "keypress" {
			if key, ok := payload["key"].(string); ok {
				switch key {
				case "R":
					cv.refresh()
					return nil
				case "?":
					cv.showHelp()
					return nil
				case "I":
					cv.showConsumerInfo()
					return nil
				case "B":
					cv.returnToTopics()
					return nil
				}
			}
		}
		return nil
	})

	// Add key bindings
	cv.cores.AddKeyBinding("R", "Refresh", nil)
	cv.cores.AddKeyBinding("?", "Help", nil)
	cv.cores.AddKeyBinding("I", "Info", nil)
	cv.cores.AddKeyBinding("B", "Back", nil)

	// Set row selection callback for tracking selection
	cv.cores.SetRowSelectedCallback(func(row int) {
		if row >= 0 && row < len(cv.consumers) {
			cv.cores.Log(fmt.Sprintf("[blue]Selected consumer group: %s (%d partitions)",
				cv.consumers[row].GroupID, cv.consumers[row].Partitions))
		}
	})

	// Register the key handlers to actually handle the key events
	cv.cores.RegisterHandlers()

	// Initial refresh to show data
	cv.refresh()

	return cv
}

// GetMainUI returns the main UI component
func (cv *ConsumersView) GetMainUI() tview.Primitive {
	// Ensure table gets focus when this view is shown
	cv.app.SetFocus(cv.cores.GetTable())
	return cv.cores.GetLayout()
}

// refreshConsumers refreshes the consumers list
func (cv *ConsumersView) refreshConsumers() ([][]string, error) {
	if cv.kafkaClient == nil || cv.currentCluster == "" {
		// No client or cluster, show empty data
		cv.consumers = []ConsumerInfo{}
		cv.cores.SetTableData([][]string{})
		if cv.currentTopic != "" {
			cv.cores.SetInfoText(fmt.Sprintf("[yellow]Kafka Consumers[white]\nCluster: Not Connected\nTopic: %s", cv.currentTopic))
		} else {
			cv.cores.SetInfoText("[yellow]Kafka Consumers[white]\nCluster: Not Connected\nAll Topics")
		}
		return [][]string{}, nil
	}

	// In a real implementation, we would fetch actual consumer data
	// For now, let's simulate some sample data
	now := time.Now()
	cv.consumers = []ConsumerInfo{
		{GroupID: "order-processor", Topic: "orders", Members: 4, Lag: 152, Status: "Active", Partitions: []int{0, 1, 2, 3}, LastActivity: now.Add(-time.Minute * 2), ClientID: "order-svc-1"},
		{GroupID: "order-analytics", Topic: "orders", Members: 2, Lag: 4375, Status: "Active", Partitions: []int{0, 1, 2, 3}, LastActivity: now.Add(-time.Minute * 5), ClientID: "analytics-svc"},
		{GroupID: "order-archive", Topic: "orders", Members: 1, Lag: 18924, Status: "Active", Partitions: []int{0, 1, 2, 3}, LastActivity: now.Add(-time.Minute * 10), ClientID: "archive-svc"},
		{GroupID: "payment-processor", Topic: "payments", Members: 3, Lag: 87, Status: "Active", Partitions: []int{0, 1, 2}, LastActivity: now.Add(-time.Second * 45), ClientID: "payment-svc"},
		{GroupID: "customer-processor", Topic: "customers", Members: 2, Lag: 0, Status: "Active", Partitions: []int{0, 1}, LastActivity: now.Add(-time.Second * 30), ClientID: "customer-svc"},
		{GroupID: "metrics-collector", Topic: "metrics", Members: 1, Lag: 12543, Status: "Active", Partitions: []int{0, 1}, LastActivity: now.Add(-time.Minute * 3), ClientID: "metrics-svc"},
		{GroupID: "shipping-processor", Topic: "shipments", Members: 2, Lag: 43, Status: "Active", Partitions: []int{0, 1}, LastActivity: now.Add(-time.Minute * 1), ClientID: "shipping-svc"},
		{GroupID: "inventory-sync", Topic: "inventory", Members: 1, Lag: 275, Status: "Active", Partitions: []int{0, 1, 2, 3}, LastActivity: now.Add(-time.Minute * 7), ClientID: "inventory-svc"},
	}

	// If viewing for a specific topic, filter the consumers
	if cv.currentTopic != "" {
		filteredConsumers := []ConsumerInfo{}
		for _, consumer := range cv.consumers {
			if consumer.Topic == cv.currentTopic {
				filteredConsumers = append(filteredConsumers, consumer)
			}
		}
		cv.consumers = filteredConsumers
	}

	// Convert to table data
	tableData := make([][]string, len(cv.consumers))
	for i, consumer := range cv.consumers {
		tableData[i] = []string{
			consumer.GroupID,
			consumer.Topic,
			strconv.Itoa(len(consumer.Partitions)),
			strconv.FormatInt(consumer.Lag, 10),
			consumer.Status,
			strconv.Itoa(consumer.Members),
			consumer.Topic,
		}
	}

	// Update table data
	cv.cores.SetTableData(tableData)

	// Update info text
	if cv.currentTopic != "" {
		cv.cores.SetInfoText(fmt.Sprintf("[yellow]Kafka Consumers[white]\nCluster: %s\nTopic: %s\nConsumers: %d",
			cv.currentCluster, cv.currentTopic, len(cv.consumers)))
	} else {
		cv.cores.SetInfoText(fmt.Sprintf("[yellow]Kafka Consumers[white]\nCluster: %s\nAll Topics\nConsumers: %d",
			cv.currentCluster, len(cv.consumers)))
	}

	return tableData, nil
}

// refresh manually refreshes the consumers list
func (cv *ConsumersView) refresh() {
	cv.cores.RefreshData()
}

// showHelp shows the help modal
func (cv *ConsumersView) showHelp() {
	helpText := `[yellow]Kafka Consumers View Help[white]

[aqua]Key Bindings:[white]
[green]R[white] - Refresh consumers list
[green]I[white] - Show detailed information about the selected consumer
[green]B[white] - Return to topics view
[green]?[white] - Show this help information
[green]ESC[white] - Navigate back to previous view

[aqua]Navigation:[white]
- Use ESC to go back to previous views
- The breadcrumb at the bottom shows your current location
- Select a consumer by clicking on it or using arrow keys
`

	ui.ShowInfoModal(
		cv.pages,
		cv.app,
		"Help",
		helpText,
		func() {
			// Ensure table regains focus after modal is closed
			cv.app.SetFocus(cv.cores.GetTable())
		},
	)
}

// showConsumerInfo shows detailed information about the selected consumer
func (cv *ConsumersView) showConsumerInfo() {
	selectedRow := cv.cores.GetSelectedRow()
	if selectedRow < 0 || selectedRow >= len(cv.consumers) {
		cv.cores.Log("[red]No consumer selected")
		return
	}

	consumer := cv.consumers[selectedRow]

	// In a real implementation, we'd get more detailed information about the consumer
	infoText := fmt.Sprintf(`[yellow]Consumer Group Details[white]

[aqua]Group ID:[white] %s
[aqua]Topic:[white] %s
[aqua]Status:[white] %s
[aqua]Members:[white] %d
[aqua]Lag:[white] %d messages
[aqua]Last Activity:[white] %s
[aqua]Client ID:[white] %s
[aqua]Group State:[white] Stable
[aqua]Rebalance Timeout:[white] 30 seconds
[aqua]Session Timeout:[white] 45 seconds
[aqua]Assignment Strategy:[white] RangeAssignor
[aqua]Coordinator:[white] Broker #1
`,
		consumer.GroupID, consumer.Topic, consumer.Status,
		consumer.Members, consumer.Lag, formatRelativeTime(consumer.LastActivity),
		consumer.ClientID)

	ui.ShowInfoModal(
		cv.pages,
		cv.app,
		fmt.Sprintf("Consumer Group '%s' Information", consumer.GroupID),
		infoText,
		func() {
			// Ensure table regains focus after modal is closed
			cv.app.SetFocus(cv.cores.GetTable())
		},
	)
}

// returnToTopics switches back to the topics view
func (cv *ConsumersView) returnToTopics() {
	cv.cores.Log("[blue]Returning to topics view")
	cv.pages.SwitchToPage("topics")
}

// formatRelativeTime formats a time relative to now (e.g., "2 minutes ago")
func formatRelativeTime(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	if diff < time.Minute {
		seconds := int(diff.Seconds())
		if seconds == 1 {
			return "1 second ago"
		}
		return fmt.Sprintf("%d seconds ago", seconds)
	} else if diff < time.Hour {
		minutes := int(diff.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	} else if diff < 24*time.Hour {
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else {
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}

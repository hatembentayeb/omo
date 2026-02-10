package main

import (
	"fmt"
	"strconv"
	"strings"

	"omo/pkg/ui"
)

// newTopicsView creates the topics CoreView
func (kv *KafkaView) newTopicsView() *ui.CoreView {
	view := ui.NewCoreView(kv.app, "Kafka Topics")

	// Set table headers
	view.SetTableHeaders([]string{"Name", "Partitions", "Replication", "Internal"})

	// Set up refresh callback
	view.SetRefreshCallback(func() ([][]string, error) {
		return kv.refreshTopics()
	})

	// Add key bindings
	view.AddKeyBinding("R", "Refresh", kv.refresh)
	view.AddKeyBinding("?", "Help", kv.showHelp)
	view.AddKeyBinding("I", "Info", nil)
	view.AddKeyBinding("P", "Partitions", nil)
	view.AddKeyBinding("M", "Messages", nil)
	view.AddKeyBinding("B", "Brokers", nil)
	view.AddKeyBinding("G", "Consumers", nil)

	// Set action callback
	view.SetActionCallback(kv.handleAction)

	// Row selection callback
	view.SetRowSelectedCallback(func(row int) {
		tableData := view.GetTableData()
		if row >= 0 && row < len(tableData) && len(tableData[row]) > 0 {
			view.Log(fmt.Sprintf("[blue]Selected topic: %s (%s partitions)",
				tableData[row][0], tableData[row][1]))
		}
	})

	// Register handlers
	view.RegisterHandlers()

	return view
}

// refreshTopics fetches and returns topic data from the real Kafka cluster
func (kv *KafkaView) refreshTopics() ([][]string, error) {
	if kv.kafkaClient == nil || !kv.kafkaClient.IsConnected() {
		return [][]string{{"Not connected", "Use Ctrl+T to select cluster", "", ""}}, nil
	}

	topics, err := kv.kafkaClient.GetTopics()
	if err != nil {
		return [][]string{{"Error fetching topics", err.Error(), "", ""}}, nil
	}

	if len(topics) == 0 {
		return [][]string{{"No topics found", "", "", ""}}, nil
	}

	tableData := make([][]string, len(topics))
	for i, topic := range topics {
		internal := ""
		if topic.Internal {
			internal = "Yes"
		}
		tableData[i] = []string{
			topic.Name,
			strconv.FormatInt(int64(topic.Partitions), 10),
			strconv.Itoa(int(topic.ReplicationFactor)),
			internal,
		}
	}

	clusterName := kv.kafkaClient.GetCurrentCluster()
	kv.topicsView.SetInfoText(fmt.Sprintf("[green]Kafka Manager[white]\nCluster: %s\nTopics: %d",
		clusterName, len(topics)))

	return tableData, nil
}

// showTopicInfo shows detailed information about the selected topic
func (kv *KafkaView) showTopicInfo() {
	selectedRow := kv.topicsView.GetSelectedRow()
	tableData := kv.topicsView.GetTableData()
	if selectedRow < 0 || selectedRow >= len(tableData) {
		kv.topicsView.Log("[red]No topic selected")
		return
	}

	row := tableData[selectedRow]
	topicName := row[0]

	// Fetch full topic info including config
	topics, err := kv.kafkaClient.GetTopics()
	if err != nil {
		kv.topicsView.Log(fmt.Sprintf("[red]Error getting topic details: %v", err))
		return
	}

	var topicDetail *TopicInfo
	for _, t := range topics {
		if t.Name == topicName {
			topicDetail = &t
			break
		}
	}

	if topicDetail == nil {
		kv.topicsView.Log(fmt.Sprintf("[red]Topic %s not found", topicName))
		return
	}

	infoText := fmt.Sprintf(`[yellow]Topic Details[white]

[aqua]Name:[white] %s
[aqua]Partitions:[white] %d
[aqua]Replication Factor:[white] %d
[aqua]Internal:[white] %v
`,
		topicDetail.Name, topicDetail.Partitions,
		topicDetail.ReplicationFactor, topicDetail.Internal)

	// Add config entries if available
	if len(topicDetail.ConfigEntries) > 0 {
		infoText += "\n[aqua]Configuration:[white]\n"
		for key, val := range topicDetail.ConfigEntries {
			if val != nil {
				infoText += fmt.Sprintf("  %s: %s\n", key, *val)
			}
		}
	}

	ui.ShowInfoModal(
		kv.pages,
		kv.app,
		fmt.Sprintf("Topic '%s' Info", topicName),
		infoText,
		func() {
			kv.app.SetFocus(kv.topicsView.GetTable())
		},
	)
}

// showPartitionsForSelectedTopic navigates to partitions view for the selected topic
func (kv *KafkaView) showPartitionsForSelectedTopic() {
	selectedRow := kv.topicsView.GetSelectedRow()
	tableData := kv.topicsView.GetTableData()
	if selectedRow < 0 || selectedRow >= len(tableData) {
		kv.topicsView.Log("[red]No topic selected")
		return
	}

	topicName := tableData[selectedRow][0]
	if topicName == "" || strings.HasPrefix(topicName, "Not connected") || strings.HasPrefix(topicName, "Error") || strings.HasPrefix(topicName, "No topics") {
		return
	}

	kv.selectedTopic = topicName
	kv.topicsView.Log(fmt.Sprintf("[blue]Showing partitions for topic: %s", topicName))
	kv.showPartitions()
}

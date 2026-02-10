package main

import (
	"fmt"
	"strconv"
	"strings"

	"omo/pkg/ui"
)

// newPartitionsView creates the partitions CoreView
func (kv *KafkaView) newPartitionsView() *ui.CoreView {
	view := ui.NewCoreView(kv.app, "Kafka Partitions")

	// Set table headers
	view.SetTableHeaders([]string{"ID", "Leader", "Replicas", "ISR", "Oldest", "Newest", "Messages"})

	// Set up refresh callback
	view.SetRefreshCallback(func() ([][]string, error) {
		return kv.refreshPartitions()
	})

	// Add key bindings
	view.AddKeyBinding("R", "Refresh", kv.refresh)
	view.AddKeyBinding("?", "Help", kv.showHelp)
	view.AddKeyBinding("I", "Info", nil)
	view.AddKeyBinding("B", "Brokers", nil)
	view.AddKeyBinding("T", "Topics", nil)
	view.AddKeyBinding("G", "Consumers", nil)

	// Set action callback
	view.SetActionCallback(kv.handleAction)

	// Row selection callback
	view.SetRowSelectedCallback(func(row int) {
		tableData := view.GetTableData()
		if row >= 0 && row < len(tableData) && len(tableData[row]) > 0 {
			view.Log(fmt.Sprintf("[blue]Selected partition: %s (leader: %s)",
				tableData[row][0], tableData[row][1]))
		}
	})

	// Register handlers
	view.RegisterHandlers()

	return view
}

// refreshPartitions fetches and returns partition data for the selected topic
func (kv *KafkaView) refreshPartitions() ([][]string, error) {
	if kv.kafkaClient == nil || !kv.kafkaClient.IsConnected() {
		return [][]string{{"Not connected", "Use Ctrl+T to select cluster", "", "", "", "", ""}}, nil
	}

	if kv.selectedTopic == "" {
		return [][]string{{"No topic selected", "Press T to go to Topics, then P", "", "", "", "", ""}}, nil
	}

	partitions, err := kv.kafkaClient.GetTopicPartitions(kv.selectedTopic)
	if err != nil {
		return [][]string{{"Error fetching partitions", err.Error(), "", "", "", "", ""}}, nil
	}

	if len(partitions) == 0 {
		return [][]string{{"No partitions found", "", "", "", "", "", ""}}, nil
	}

	tableData := make([][]string, len(partitions))
	for i, p := range partitions {
		// Format replicas
		replicas := formatInt32Slice(p.Replicas)
		isr := formatInt32Slice(p.ISR)

		messages := p.NewestOffset - p.OldestOffset
		if messages < 0 {
			messages = 0
		}

		tableData[i] = []string{
			strconv.FormatInt(int64(p.ID), 10),
			strconv.FormatInt(int64(p.Leader), 10),
			replicas,
			isr,
			strconv.FormatInt(p.OldestOffset, 10),
			strconv.FormatInt(p.NewestOffset, 10),
			formatLargeNumber(messages),
		}
	}

	clusterName := kv.kafkaClient.GetCurrentCluster()
	kv.partitionsView.SetInfoText(fmt.Sprintf("[green]Kafka Manager[white]\nCluster: %s\nTopic: %s\nPartitions: %d",
		clusterName, kv.selectedTopic, len(partitions)))

	return tableData, nil
}

// showPartitionInfo shows detailed information about the selected partition
func (kv *KafkaView) showPartitionInfo() {
	selectedRow := kv.partitionsView.GetSelectedRow()
	tableData := kv.partitionsView.GetTableData()
	if selectedRow < 0 || selectedRow >= len(tableData) {
		kv.partitionsView.Log("[red]No partition selected")
		return
	}

	row := tableData[selectedRow]
	partID := row[0]
	if partID == "" || strings.HasPrefix(partID, "Not connected") || strings.HasPrefix(partID, "Error") || strings.HasPrefix(partID, "No partition") || strings.HasPrefix(partID, "No topic") {
		return
	}

	oldest := row[4]
	newest := row[5]

	// Calculate under-replicated status
	replicaCount := len(strings.Split(row[2], ","))
	isrCount := len(strings.Split(row[3], ","))
	underReplicated := replicaCount > isrCount

	infoText := fmt.Sprintf(`[yellow]Partition Details[white]

[aqua]Partition ID:[white] %s
[aqua]Topic:[white] %s
[aqua]Leader Broker:[white] %s
[aqua]Replicas:[white] %s
[aqua]In-Sync Replicas:[white] %s
[aqua]Oldest Offset:[white] %s
[aqua]Newest Offset:[white] %s
[aqua]Messages:[white] %s
[aqua]Under-Replicated:[white] %t
[aqua]Cluster:[white] %s
`,
		partID, kv.selectedTopic, row[1], row[2], row[3],
		oldest, newest, row[6], underReplicated, kv.currentCluster)

	ui.ShowInfoModal(
		kv.pages,
		kv.app,
		fmt.Sprintf("Partition %s Info", partID),
		infoText,
		func() {
			kv.app.SetFocus(kv.partitionsView.GetTable())
		},
	)
}

// formatInt32Slice formats a slice of int32 as a comma-separated string
func formatInt32Slice(slice []int32) string {
	if len(slice) == 0 {
		return ""
	}
	parts := make([]string, len(slice))
	for i, v := range slice {
		parts[i] = strconv.FormatInt(int64(v), 10)
	}
	return strings.Join(parts, ", ")
}

// formatLargeNumber formats a large number in a human-readable way
func formatLargeNumber(n int64) string {
	if n >= 1_000_000_000 {
		return fmt.Sprintf("%.2f B", float64(n)/1_000_000_000)
	} else if n >= 1_000_000 {
		return fmt.Sprintf("%.2f M", float64(n)/1_000_000)
	} else if n >= 1_000 {
		return fmt.Sprintf("%.1f K", float64(n)/1_000)
	}
	return strconv.FormatInt(n, 10)
}

package main

import (
	"fmt"
	"strconv"
	"strings"

	"omo/pkg/ui"
)

// newConsumersView creates the consumer groups CoreView
func (kv *KafkaView) newConsumersView() *ui.CoreView {
	view := ui.NewCoreView(kv.app, "Kafka Consumer Groups")

	// Set table headers
	view.SetTableHeaders([]string{"Group ID", "State", "Members", "Protocol", "Protocol Type"})

	// Set up refresh callback
	view.SetRefreshCallback(func() ([][]string, error) {
		return kv.refreshConsumers()
	})

	// Add key bindings
	view.AddKeyBinding("R", "Refresh", kv.refresh)
	view.AddKeyBinding("?", "Help", kv.showHelp)
	view.AddKeyBinding("I", "Info", nil)
	view.AddKeyBinding("O", "Offsets", nil)
	view.AddKeyBinding("B", "Brokers", nil)
	view.AddKeyBinding("T", "Topics", nil)

	// Set action callback
	view.SetActionCallback(kv.handleAction)

	// Row selection callback
	view.SetRowSelectedCallback(func(row int) {
		tableData := view.GetTableData()
		if row >= 0 && row < len(tableData) && len(tableData[row]) > 0 {
			view.Log(fmt.Sprintf("[blue]Selected consumer group: %s (%s)",
				tableData[row][0], tableData[row][1]))
		}
	})

	// Register handlers
	view.RegisterHandlers()

	return view
}

// refreshConsumers fetches and returns consumer group data from the real Kafka cluster
func (kv *KafkaView) refreshConsumers() ([][]string, error) {
	if kv.kafkaClient == nil || !kv.kafkaClient.IsConnected() {
		return [][]string{{"Not connected", "Use Ctrl+T to select cluster", "", "", ""}}, nil
	}

	groups, err := kv.kafkaClient.GetConsumerGroups()
	if err != nil {
		return [][]string{{"Error fetching consumer groups", err.Error(), "", "", ""}}, nil
	}

	if len(groups) == 0 {
		return [][]string{{"No consumer groups found", "", "", "", ""}}, nil
	}

	tableData := make([][]string, len(groups))
	for i, group := range groups {
		tableData[i] = []string{
			group.GroupID,
			group.State,
			strconv.Itoa(group.Members),
			group.Protocol,
			group.ProtocolType,
		}
	}

	clusterName := kv.kafkaClient.GetCurrentCluster()
	kv.consumersView.SetInfoText(fmt.Sprintf("[green]Kafka Manager[white]\nCluster: %s\nConsumer Groups: %d",
		clusterName, len(groups)))

	return tableData, nil
}

// showConsumerGroupInfo shows detailed information about the selected consumer group
func (kv *KafkaView) showConsumerGroupInfo() {
	selectedRow := kv.consumersView.GetSelectedRow()
	tableData := kv.consumersView.GetTableData()
	if selectedRow < 0 || selectedRow >= len(tableData) {
		kv.consumersView.Log("[red]No consumer group selected")
		return
	}

	row := tableData[selectedRow]
	groupID := row[0]

	if groupID == "" || strings.HasPrefix(groupID, "Not connected") || strings.HasPrefix(groupID, "Error") || strings.HasPrefix(groupID, "No consumer") {
		return
	}

	infoText := fmt.Sprintf(`[yellow]Consumer Group Details[white]

[aqua]Group ID:[white] %s
[aqua]State:[white] %s
[aqua]Members:[white] %s
[aqua]Protocol:[white] %s
[aqua]Protocol Type:[white] %s
[aqua]Cluster:[white] %s
`,
		row[0], row[1], row[2], row[3], row[4], kv.currentCluster)

	ui.ShowInfoModal(
		kv.pages,
		kv.app,
		fmt.Sprintf("Consumer Group '%s' Info", groupID),
		infoText,
		func() {
			kv.app.SetFocus(kv.consumersView.GetTable())
		},
	)
}

// showConsumerOffsets shows offset details for the selected consumer group
func (kv *KafkaView) showConsumerOffsets() {
	selectedRow := kv.consumersView.GetSelectedRow()
	tableData := kv.consumersView.GetTableData()
	if selectedRow < 0 || selectedRow >= len(tableData) {
		kv.consumersView.Log("[red]No consumer group selected")
		return
	}

	groupID := tableData[selectedRow][0]
	if groupID == "" || strings.HasPrefix(groupID, "Not connected") || strings.HasPrefix(groupID, "Error") || strings.HasPrefix(groupID, "No consumer") {
		return
	}

	kv.consumersView.Log(fmt.Sprintf("[blue]Fetching offsets for group: %s...", groupID))

	offsets, err := kv.kafkaClient.GetConsumerGroupOffsets(groupID)
	if err != nil {
		kv.consumersView.Log(fmt.Sprintf("[red]Error fetching offsets: %v", err))
		return
	}

	if len(offsets) == 0 {
		ui.ShowInfoModal(
			kv.pages,
			kv.app,
			fmt.Sprintf("Offsets for '%s'", groupID),
			"[yellow]No committed offsets found for this consumer group.[white]",
			func() {
				kv.app.SetFocus(kv.consumersView.GetTable())
			},
		)
		return
	}

	// Build the offset details text
	infoText := fmt.Sprintf("[yellow]Consumer Group Offsets: %s[white]\n\n", groupID)

	var totalLag int64
	currentTopic := ""
	for _, offset := range offsets {
		if offset.Topic != currentTopic {
			if currentTopic != "" {
				infoText += "\n"
			}
			currentTopic = offset.Topic
			infoText += fmt.Sprintf("[aqua]Topic: %s[white]\n", currentTopic)
		}
		infoText += fmt.Sprintf("  Partition %d: offset=%d, lag=%d\n",
			offset.Partition, offset.Offset, offset.Lag)
		totalLag += offset.Lag
	}

	infoText += fmt.Sprintf("\n[yellow]Total Lag:[white] %d", totalLag)

	ui.ShowInfoModal(
		kv.pages,
		kv.app,
		fmt.Sprintf("Offsets for '%s'", groupID),
		infoText,
		func() {
			kv.app.SetFocus(kv.consumersView.GetTable())
		},
	)
}

package main

import (
	"fmt"
	"strconv"
	"strings"

	"omo/pkg/ui"
)

// newMessagesView creates the messages CoreView for browsing topic messages
func (kv *KafkaView) newMessagesView() *ui.CoreView {
	view := ui.NewCoreView(kv.app, "Kafka Messages")

	// Set table headers
	view.SetTableHeaders([]string{"Partition", "Offset", "Key", "Value", "Timestamp"})

	// Set up refresh callback
	view.SetRefreshCallback(func() ([][]string, error) {
		return kv.refreshMessages()
	})

	// Add key bindings
	view.AddKeyBinding("R", "Refresh", kv.refresh)
	view.AddKeyBinding("?", "Help", kv.showHelp)
	view.AddKeyBinding("I", "Info", nil)
	view.AddKeyBinding("T", "Topics", nil)
	view.AddKeyBinding("B", "Brokers", nil)
	view.AddKeyBinding("G", "Consumers", nil)

	// Set action callback
	view.SetActionCallback(kv.handleAction)

	// Row selection callback — show message detail on selection
	view.SetRowSelectedCallback(func(row int) {
		tableData := view.GetTableData()
		if row >= 0 && row < len(tableData) && len(tableData[row]) > 0 {
			view.Log(fmt.Sprintf("[blue]Selected message: partition=%s offset=%s",
				tableData[row][0], tableData[row][1]))
		}
	})

	// Register handlers
	view.RegisterHandlers()

	return view
}

// refreshMessages fetches and returns message data for the selected topic
func (kv *KafkaView) refreshMessages() ([][]string, error) {
	if kv.kafkaClient == nil || !kv.kafkaClient.IsConnected() {
		return [][]string{{"Not connected", "Use Ctrl+T to select cluster", "", "", ""}}, nil
	}

	if kv.selectedTopic == "" {
		return [][]string{{"No topic selected", "Press T to go to Topics, then M", "", "", ""}}, nil
	}

	kv.messagesView.Log(fmt.Sprintf("[blue]Loading messages from topic: %s ...", kv.selectedTopic))

	messages, err := kv.kafkaClient.ConsumeMessages(kv.selectedTopic, 100)
	if err != nil {
		return [][]string{{"Error fetching messages", err.Error(), "", "", ""}}, nil
	}

	if len(messages) == 0 {
		return [][]string{{"No messages found", "", "", "", ""}}, nil
	}

	tableData := make([][]string, len(messages))
	for i, msg := range messages {
		// Truncate value for table display
		value := msg.Value
		if len(value) > 120 {
			value = value[:120] + "..."
		}
		// Replace newlines for table display
		value = strings.ReplaceAll(value, "\n", " ")

		key := msg.Key
		if key == "" {
			key = "(null)"
		}

		ts := ""
		if !msg.Timestamp.IsZero() {
			ts = msg.Timestamp.Format("15:04:05.000")
		}

		tableData[i] = []string{
			strconv.FormatInt(int64(msg.Partition), 10),
			strconv.FormatInt(msg.Offset, 10),
			key,
			value,
			ts,
		}
	}

	clusterName := kv.kafkaClient.GetCurrentCluster()
	kv.messagesView.SetInfoText(fmt.Sprintf("[green]Kafka Manager[white]\nCluster: %s\nTopic: %s\nMessages: %d (latest)",
		clusterName, kv.selectedTopic, len(messages)))

	return tableData, nil
}

// showMessageDetail shows the full message content for the selected row
func (kv *KafkaView) showMessageDetail() {
	selectedRow := kv.messagesView.GetSelectedRow()
	tableData := kv.messagesView.GetTableData()
	if selectedRow < 0 || selectedRow >= len(tableData) {
		kv.messagesView.Log("[red]No message selected")
		return
	}

	row := tableData[selectedRow]
	partStr := row[0]
	if partStr == "" || strings.HasPrefix(partStr, "Not connected") || strings.HasPrefix(partStr, "Error") || strings.HasPrefix(partStr, "No message") || strings.HasPrefix(partStr, "No topic") {
		return
	}

	// Re-fetch the actual full message to show untruncated
	partID, _ := strconv.ParseInt(partStr, 10, 32)
	offset, _ := strconv.ParseInt(row[1], 10, 64)

	// Find the matching message from cached data
	messages, err := kv.kafkaClient.ConsumeMessages(kv.selectedTopic, 200)
	var fullMsg *MessageInfo
	if err == nil {
		for _, m := range messages {
			if m.Partition == int32(partID) && m.Offset == offset {
				fullMsg = &m
				break
			}
		}
	}

	var infoText string
	if fullMsg != nil {
		key := fullMsg.Key
		if key == "" {
			key = "(null)"
		}
		ts := ""
		if !fullMsg.Timestamp.IsZero() {
			ts = fullMsg.Timestamp.Format("2006-01-02 15:04:05.000")
		}

		infoText = fmt.Sprintf(`[yellow]Message Details[white]

[aqua]Topic:[white]     %s
[aqua]Partition:[white] %d
[aqua]Offset:[white]    %d
[aqua]Timestamp:[white] %s
[aqua]Key:[white]       %s
`,
			kv.selectedTopic, fullMsg.Partition, fullMsg.Offset, ts, key)

		if len(fullMsg.Headers) > 0 {
			infoText += "\n[aqua]Headers:[white]\n"
			for k, v := range fullMsg.Headers {
				infoText += fmt.Sprintf("  %s: %s\n", k, v)
			}
		}

		// Show value (format if it looks like JSON)
		value := fullMsg.Value
		if len(value) > 2000 {
			value = value[:2000] + "\n... (truncated)"
		}
		infoText += fmt.Sprintf("\n[aqua]Value:[white]\n%s", value)
	} else {
		// Fall back to table data
		infoText = fmt.Sprintf(`[yellow]Message Details[white]

[aqua]Partition:[white] %s
[aqua]Offset:[white]    %s
[aqua]Key:[white]       %s
[aqua]Timestamp:[white] %s

[aqua]Value:[white]
%s`,
			row[0], row[1], row[2], row[4], row[3])
	}

	ui.ShowInfoModal(
		kv.pages,
		kv.app,
		fmt.Sprintf("Message [P:%s O:%s]", row[0], row[1]),
		infoText,
		func() {
			kv.app.SetFocus(kv.messagesView.GetTable())
		},
	)
}

// showMessagesForSelectedTopic navigates to messages view for the selected topic
func (kv *KafkaView) showMessagesForSelectedTopic() {
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
	kv.topicsView.Log(fmt.Sprintf("[blue]Loading messages for topic: %s", topicName))
	kv.showMessages()
}

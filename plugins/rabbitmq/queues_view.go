package main

import (
	"fmt"
	"strconv"

	"omo/pkg/ui"
)

// newQueuesView creates the queues CoreView
func (rv *RabbitMQView) newQueuesView() *ui.CoreView {
	view := ui.NewCoreView(rv.app, "RabbitMQ Queues")

	view.SetTableHeaders([]string{"Name", "Messages", "Ready", "Unacked", "Consumers", "State", "Type"})

	view.SetRefreshCallback(func() ([][]string, error) {
		return rv.refreshQueues()
	})

	view.AddKeyBinding("R", "Refresh", rv.refresh)
	view.AddKeyBinding("?", "Help", rv.showHelp)
	view.AddKeyBinding("I", "Info", nil)
	view.AddKeyBinding("N", "New Queue", nil)
	view.AddKeyBinding("D", "Delete", nil)
	view.AddKeyBinding("P", "Purge", nil)
	view.AddKeyBinding("M", "Messages", nil)
	view.AddKeyBinding("U", "Publish", nil)
	view.AddKeyBinding("O", "Overview", nil)
	view.AddKeyBinding("E", "Exchanges", nil)
	view.AddKeyBinding("B", "Bindings", nil)

	view.SetActionCallback(rv.handleAction)

	view.SetRowSelectedCallback(func(row int) {
		tableData := view.GetTableData()
		if row >= 0 && row < len(tableData) && len(tableData[row]) > 0 {
			view.Log(fmt.Sprintf("[blue]Selected queue: %s", tableData[row][0]))
		}
	})

	view.RegisterHandlers()

	return view
}

func (rv *RabbitMQView) refreshQueues() ([][]string, error) {
	if rv.rmqClient == nil || !rv.rmqClient.IsConnected() {
		return [][]string{{"Not connected", "", "", "", "", "", ""}}, nil
	}

	queues, err := rv.rmqClient.GetQueues()
	if err != nil {
		return [][]string{{"Error: " + err.Error(), "", "", "", "", "", ""}}, nil
	}

	if len(queues) == 0 {
		return [][]string{{"No queues found", "", "", "", "", "", ""}}, nil
	}

	tableData := make([][]string, len(queues))
	for i, q := range queues {
		qType := q.Type
		if qType == "" {
			qType = "classic"
		}
		tableData[i] = []string{
			q.Name,
			strconv.FormatInt(q.Messages, 10),
			strconv.FormatInt(q.Ready, 10),
			strconv.FormatInt(q.Unacked, 10),
			strconv.Itoa(q.Consumers),
			q.State,
			qType,
		}
	}

	name := rv.rmqClient.GetClusterName()
	rv.queuesView.SetInfoText(fmt.Sprintf("[green]RabbitMQ Manager[white]\nInstance: %s\nQueues: %d", name, len(queues)))

	return tableData, nil
}

func (rv *RabbitMQView) showQueueInfo() {
	selectedRow := rv.queuesView.GetSelectedRow()
	tableData := rv.queuesView.GetTableData()
	if selectedRow < 0 || selectedRow >= len(tableData) {
		rv.queuesView.Log("[red]No queue selected")
		return
	}

	queueName := tableData[selectedRow][0]

	// Fetch full queue list to find details
	queues, err := rv.rmqClient.GetQueues()
	if err != nil {
		rv.queuesView.Log(fmt.Sprintf("[red]Failed to fetch queue info: %v", err))
		return
	}

	var queue *QueueInfo
	for i := range queues {
		if queues[i].Name == queueName {
			queue = &queues[i]
			break
		}
	}

	if queue == nil {
		rv.queuesView.Log(fmt.Sprintf("[red]Queue %s not found", queueName))
		return
	}

	durableStr := "No"
	if queue.Durable {
		durableStr = "Yes"
	}
	autoDeleteStr := "No"
	if queue.AutoDelete {
		autoDeleteStr = "Yes"
	}
	exclusiveStr := "No"
	if queue.Exclusive {
		exclusiveStr = "Yes"
	}
	qType := queue.Type
	if qType == "" {
		qType = "classic"
	}

	infoText := fmt.Sprintf(`[yellow]Queue Details[white]

[aqua]Name:[white]        %s
[aqua]VHost:[white]       %s
[aqua]Type:[white]        %s
[aqua]State:[white]       %s
[aqua]Node:[white]        %s
[aqua]Durable:[white]     %s
[aqua]Auto Delete:[white] %s
[aqua]Exclusive:[white]   %s

[yellow]Messages[white]
[aqua]Total:[white]       %d
[aqua]Ready:[white]       %d
[aqua]Unacked:[white]     %d
[aqua]Consumers:[white]   %d
[aqua]Memory:[white]      %s

[yellow]Message Stats[white]
[aqua]Published:[white]   %d
[aqua]Delivered:[white]   %d
[aqua]Acked:[white]       %d
[aqua]Redelivered:[white] %d`,
		queue.Name, queue.VHost, qType, queue.State, queue.Node,
		durableStr, autoDeleteStr, exclusiveStr,
		queue.Messages, queue.Ready, queue.Unacked, queue.Consumers,
		formatBytes(queue.Memory),
		queue.MessageStats.PublishIn, queue.MessageStats.DeliverGet,
		queue.MessageStats.Ack, queue.MessageStats.Redeliver)

	ui.ShowInfoModal(
		rv.pages,
		rv.app,
		fmt.Sprintf("Queue: %s", queueName),
		infoText,
		func() { rv.app.SetFocus(rv.queuesView.GetTable()) },
	)
}

func (rv *RabbitMQView) showCreateQueueForm() {
	if rv.rmqClient == nil || !rv.rmqClient.IsConnected() {
		rv.queuesView.Log("[yellow]Not connected")
		return
	}

	ui.ShowCompactStyledInputModal(
		rv.pages,
		rv.app,
		"Create Queue",
		"Queue Name",
		"",
		30,
		nil,
		func(name string, cancelled bool) {
			if cancelled || name == "" {
				rv.app.SetFocus(rv.queuesView.GetTable())
				return
			}

			err := rv.rmqClient.CreateQueue(name, true, false)
			if err != nil {
				rv.queuesView.Log(fmt.Sprintf("[red]Failed to create queue: %v", err))
			} else {
				rv.queuesView.Log(fmt.Sprintf("[green]Created queue: %s", name))
				rv.refresh()
			}
			rv.app.SetFocus(rv.queuesView.GetTable())
		},
	)
}

func (rv *RabbitMQView) deleteSelectedQueue() {
	selectedRow := rv.queuesView.GetSelectedRow()
	tableData := rv.queuesView.GetTableData()
	if selectedRow < 0 || selectedRow >= len(tableData) {
		rv.queuesView.Log("[red]No queue selected")
		return
	}

	queueName := tableData[selectedRow][0]

	ui.ShowStandardConfirmationModal(
		rv.pages,
		rv.app,
		"Delete Queue",
		fmt.Sprintf("Are you sure you want to delete queue '[red]%s[white]'?", queueName),
		func(confirmed bool) {
			if confirmed {
				if err := rv.rmqClient.DeleteQueue(queueName); err != nil {
					rv.queuesView.Log(fmt.Sprintf("[red]Failed to delete queue: %v", err))
				} else {
					rv.queuesView.Log(fmt.Sprintf("[yellow]Deleted queue: %s", queueName))
					rv.refresh()
				}
			}
			rv.app.SetFocus(rv.queuesView.GetTable())
		},
	)
}

func (rv *RabbitMQView) purgeSelectedQueue() {
	selectedRow := rv.queuesView.GetSelectedRow()
	tableData := rv.queuesView.GetTableData()
	if selectedRow < 0 || selectedRow >= len(tableData) {
		rv.queuesView.Log("[red]No queue selected")
		return
	}

	queueName := tableData[selectedRow][0]

	ui.ShowStandardConfirmationModal(
		rv.pages,
		rv.app,
		"Purge Queue",
		fmt.Sprintf("Are you sure you want to purge [red]ALL[white] messages from queue '[red]%s[white]'?", queueName),
		func(confirmed bool) {
			if confirmed {
				if err := rv.rmqClient.PurgeQueue(queueName); err != nil {
					rv.queuesView.Log(fmt.Sprintf("[red]Failed to purge queue: %v", err))
				} else {
					rv.queuesView.Log(fmt.Sprintf("[yellow]Purged queue: %s", queueName))
					rv.refresh()
				}
			}
			rv.app.SetFocus(rv.queuesView.GetTable())
		},
	)
}

func (rv *RabbitMQView) browseSelectedQueueMessages() {
	selectedRow := rv.queuesView.GetSelectedRow()
	tableData := rv.queuesView.GetTableData()
	if selectedRow < 0 || selectedRow >= len(tableData) {
		rv.queuesView.Log("[red]No queue selected")
		return
	}

	queueName := tableData[selectedRow][0]
	rv.queuesView.Log(fmt.Sprintf("[blue]Fetching messages from %s...", queueName))

	messages, err := rv.rmqClient.GetQueueMessages(queueName, 10)
	if err != nil {
		rv.queuesView.Log(fmt.Sprintf("[red]Failed to get messages: %v", err))
		return
	}

	if len(messages) == 0 {
		rv.queuesView.Log(fmt.Sprintf("[yellow]No messages in queue %s", queueName))
		return
	}

	content := fmt.Sprintf("[yellow]Messages in %s[white] (%d shown)\n\n", queueName, len(messages))
	for i, msg := range messages {
		payload := ""
		if p, ok := msg["payload"].(string); ok {
			payload = p
		}
		routingKey := ""
		if rk, ok := msg["routing_key"].(string); ok {
			routingKey = rk
		}
		exchange := ""
		if ex, ok := msg["exchange"].(string); ok {
			exchange = ex
		}

		content += fmt.Sprintf("[aqua]#%d[white] routing_key=[green]%s[white] exchange=[green]%s[white]\n%s\n\n",
			i+1, routingKey, exchange, payload)
	}

	ui.ShowInfoModal(
		rv.pages,
		rv.app,
		fmt.Sprintf("Messages: %s", queueName),
		content,
		func() { rv.app.SetFocus(rv.queuesView.GetTable()) },
	)
}

func (rv *RabbitMQView) publishToSelectedQueue() {
	selectedRow := rv.queuesView.GetSelectedRow()
	tableData := rv.queuesView.GetTableData()
	if selectedRow < 0 || selectedRow >= len(tableData) {
		rv.queuesView.Log("[red]No queue selected")
		return
	}

	queueName := tableData[selectedRow][0]

	ui.ShowCompactStyledInputModal(
		rv.pages,
		rv.app,
		"Publish Message",
		"Message body",
		"",
		40,
		nil,
		func(body string, cancelled bool) {
			if cancelled || body == "" {
				rv.app.SetFocus(rv.queuesView.GetTable())
				return
			}

			// Publish to default exchange with queue name as routing key
			err := rv.rmqClient.PublishMessageToExchange("", queueName, body)
			if err != nil {
				rv.queuesView.Log(fmt.Sprintf("[red]Publish failed: %v", err))
			} else {
				rv.queuesView.Log(fmt.Sprintf("[green]Published message to %s", queueName))
				rv.refresh()
			}
			rv.app.SetFocus(rv.queuesView.GetTable())
		},
	)
}

// formatBytes formats bytes into human-readable string
func formatBytes(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	} else if bytes < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	} else if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	}
	return fmt.Sprintf("%.1f GB", float64(bytes)/(1024*1024*1024))
}

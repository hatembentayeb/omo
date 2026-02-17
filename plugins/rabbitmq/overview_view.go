package main

import (
	"fmt"
	"strconv"

	"omo/pkg/ui"
)

// newOverviewView creates the overview CoreView
func (rv *RabbitMQView) newOverviewView() *ui.CoreView {
	view := ui.NewCoreView(rv.app, "RabbitMQ Overview")

	view.SetTableHeaders([]string{"Metric", "Value"})

	view.SetRefreshCallback(func() ([][]string, error) {
		return rv.refreshOverview()
	})

	view.AddKeyBinding("R", "Refresh", rv.refresh)
	view.AddKeyBinding("?", "Help", rv.showHelp)
	view.AddKeyBinding("Q", "Queues", nil)
	view.AddKeyBinding("E", "Exchanges", nil)
	view.AddKeyBinding("B", "Bindings", nil)
	view.AddKeyBinding("C", "Connections", nil)
	view.AddKeyBinding("H", "Channels", nil)
	view.AddKeyBinding("S", "Nodes", nil)

	view.SetActionCallback(rv.handleAction)

	view.RegisterHandlers()

	return view
}

func (rv *RabbitMQView) refreshOverview() ([][]string, error) {
	if rv.rmqClient == nil || !rv.rmqClient.IsConnected() {
		return [][]string{{"Not connected", "Use Ctrl+T to select instance"}}, nil
	}

	overview, err := rv.rmqClient.GetOverview()
	if err != nil {
		return [][]string{{"Error", err.Error()}}, nil
	}

	publishRate := "0.0/s"
	if overview.MessageStats.PublishDetails != nil {
		publishRate = fmt.Sprintf("%.1f/s", overview.MessageStats.PublishDetails.Rate)
	}
	deliverRate := "0.0/s"
	if overview.MessageStats.DeliverGetDetails != nil {
		deliverRate = fmt.Sprintf("%.1f/s", overview.MessageStats.DeliverGetDetails.Rate)
	}
	ackRate := "0.0/s"
	if overview.MessageStats.AckDetails != nil {
		ackRate = fmt.Sprintf("%.1f/s", overview.MessageStats.AckDetails.Rate)
	}

	tableData := [][]string{
		{"RabbitMQ Version", overview.RabbitMQVersion},
		{"Erlang Version", overview.ErlangVersion},
		{"Cluster Name", overview.ClusterName},
		{"Node", overview.Node},
		{"", ""},
		{"--- Queues ---", ""},
		{"Total Messages", strconv.FormatInt(overview.QueueTotals.Messages, 10)},
		{"Messages Ready", strconv.FormatInt(overview.QueueTotals.MessagesReady, 10)},
		{"Messages Unacked", strconv.FormatInt(overview.QueueTotals.MessagesUnack, 10)},
		{"", ""},
		{"--- Objects ---", ""},
		{"Queues", strconv.Itoa(overview.ObjectTotals.Queues)},
		{"Exchanges", strconv.Itoa(overview.ObjectTotals.Exchanges)},
		{"Connections", strconv.Itoa(overview.ObjectTotals.Connections)},
		{"Channels", strconv.Itoa(overview.ObjectTotals.Channels)},
		{"Consumers", strconv.Itoa(overview.ObjectTotals.Consumers)},
		{"", ""},
		{"--- Message Stats ---", ""},
		{"Published", fmt.Sprintf("%d (%s)", overview.MessageStats.Publish, publishRate)},
		{"Delivered/Get", fmt.Sprintf("%d (%s)", overview.MessageStats.DeliverGet, deliverRate)},
		{"Acknowledged", fmt.Sprintf("%d (%s)", overview.MessageStats.Ack, ackRate)},
		{"Confirmed", strconv.FormatInt(overview.MessageStats.Confirm, 10)},
		{"Redelivered", strconv.FormatInt(overview.MessageStats.Redeliver, 10)},
		{"Returned", strconv.FormatInt(overview.MessageStats.ReturnUnroutable, 10)},
		{"Disk Reads", strconv.FormatInt(overview.MessageStats.DiskReads, 10)},
		{"Disk Writes", strconv.FormatInt(overview.MessageStats.DiskWrites, 10)},
	}

	// Add listeners
	if len(overview.Listeners) > 0 {
		tableData = append(tableData, []string{"", ""})
		tableData = append(tableData, []string{"--- Listeners ---", ""})
		for _, l := range overview.Listeners {
			tableData = append(tableData, []string{l.Protocol, strconv.Itoa(l.Port)})
		}
	}

	name := rv.rmqClient.GetClusterName()
	rv.overviewView.SetInfoText(fmt.Sprintf("[green]RabbitMQ Manager[white]\nInstance: %s\nVersion: %s\nQueues: %d\nConnections: %d",
		name, overview.RabbitMQVersion, overview.ObjectTotals.Queues, overview.ObjectTotals.Connections))

	return tableData, nil
}

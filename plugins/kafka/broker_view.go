package main

import (
	"fmt"
	"strconv"

	"omo/pkg/ui"
)

// newBrokersView creates the brokers CoreView
func (kv *KafkaView) newBrokersView() *ui.CoreView {
	view := ui.NewCoreView(kv.app, "Kafka Brokers")

	// Set table headers
	view.SetTableHeaders([]string{"ID", "Address", "Controller"})

	// Set up refresh callback
	view.SetRefreshCallback(func() ([][]string, error) {
		return kv.refreshBrokers()
	})

	// Add key bindings
	view.AddKeyBinding("R", "Refresh", kv.refresh)
	view.AddKeyBinding("?", "Help", kv.showHelp)
	view.AddKeyBinding("I", "Info", nil)
	view.AddKeyBinding("T", "Topics", nil)
	view.AddKeyBinding("G", "Consumers", nil)

	// Set action callback
	view.SetActionCallback(kv.handleAction)

	// Row selection callback
	view.SetRowSelectedCallback(func(row int) {
		tableData := view.GetTableData()
		if row >= 0 && row < len(tableData) && len(tableData[row]) > 0 {
			view.Log(fmt.Sprintf("[blue]Selected broker: %s (%s)", tableData[row][0], tableData[row][1]))
		}
	})

	// Register handlers
	view.RegisterHandlers()

	return view
}

// refreshBrokers fetches and returns broker data from the real Kafka cluster
func (kv *KafkaView) refreshBrokers() ([][]string, error) {
	if kv.kafkaClient == nil || !kv.kafkaClient.IsConnected() {
		return [][]string{{"Not connected", "Use Ctrl+T to select cluster", ""}}, nil
	}

	brokers, err := kv.kafkaClient.GetBrokers()
	if err != nil {
		return [][]string{{"Error fetching brokers", err.Error(), ""}}, nil
	}

	if len(brokers) == 0 {
		return [][]string{{"No brokers found", "", ""}}, nil
	}

	tableData := make([][]string, len(brokers))
	for i, broker := range brokers {
		controller := ""
		if broker.Controller {
			controller = "Yes"
		}
		tableData[i] = []string{
			strconv.FormatInt(int64(broker.ID), 10),
			broker.Address,
			controller,
		}
	}

	clusterName := kv.kafkaClient.GetCurrentCluster()
	kv.brokersView.SetInfoText(fmt.Sprintf("[green]Kafka Manager[white]\nCluster: %s\nBrokers: %d",
		clusterName, len(brokers)))

	return tableData, nil
}

// showBrokerInfo shows detailed information about the selected broker
func (kv *KafkaView) showBrokerInfo() {
	selectedRow := kv.brokersView.GetSelectedRow()
	tableData := kv.brokersView.GetTableData()
	if selectedRow < 0 || selectedRow >= len(tableData) {
		kv.brokersView.Log("[red]No broker selected")
		return
	}

	row := tableData[selectedRow]
	brokerID := row[0]
	address := row[1]
	controller := row[2]
	if controller == "" {
		controller = "No"
	}

	infoText := fmt.Sprintf(`[yellow]Broker Details[white]

[aqua]Broker ID:[white] %s
[aqua]Address:[white] %s
[aqua]Controller:[white] %s
[aqua]Cluster:[white] %s
`,
		brokerID, address, controller, kv.currentCluster)

	ui.ShowInfoModal(
		kv.pages,
		kv.app,
		fmt.Sprintf("Broker #%s Info", brokerID),
		infoText,
		func() {
			kv.app.SetFocus(kv.brokersView.GetTable())
		},
	)
}

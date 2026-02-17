package main

import (
	"fmt"

	"omo/pkg/ui"
)

// newBindingsView creates the bindings CoreView
func (rv *RabbitMQView) newBindingsView() *ui.CoreView {
	view := ui.NewCoreView(rv.app, "RabbitMQ Bindings")

	view.SetTableHeaders([]string{"Source", "Destination", "Dest Type", "Routing Key"})

	view.SetRefreshCallback(func() ([][]string, error) {
		return rv.refreshBindings()
	})

	view.AddKeyBinding("R", "Refresh", rv.refresh)
	view.AddKeyBinding("?", "Help", rv.showHelp)
	view.AddKeyBinding("O", "Overview", nil)
	view.AddKeyBinding("Q", "Queues", nil)
	view.AddKeyBinding("E", "Exchanges", nil)

	view.SetActionCallback(rv.handleAction)

	view.RegisterHandlers()

	return view
}

func (rv *RabbitMQView) refreshBindings() ([][]string, error) {
	if rv.rmqClient == nil || !rv.rmqClient.IsConnected() {
		return [][]string{{"Not connected", "", "", ""}}, nil
	}

	bindings, err := rv.rmqClient.GetBindings()
	if err != nil {
		return [][]string{{"Error: " + err.Error(), "", "", ""}}, nil
	}

	if len(bindings) == 0 {
		return [][]string{{"No bindings found", "", "", ""}}, nil
	}

	tableData := make([][]string, len(bindings))
	for i, b := range bindings {
		source := b.Source
		if source == "" {
			source = "(default)"
		}
		tableData[i] = []string{
			source,
			b.Destination,
			b.DestinationType,
			b.RoutingKey,
		}
	}

	name := rv.rmqClient.GetClusterName()
	rv.bindingsView.SetInfoText(fmt.Sprintf("[green]RabbitMQ Manager[white]\nInstance: %s\nBindings: %d", name, len(bindings)))

	return tableData, nil
}

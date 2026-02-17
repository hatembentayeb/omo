package main

import (
	"fmt"
	"strconv"

	"omo/pkg/ui"
)

// newExchangesView creates the exchanges CoreView
func (rv *RabbitMQView) newExchangesView() *ui.CoreView {
	view := ui.NewCoreView(rv.app, "RabbitMQ Exchanges")

	view.SetTableHeaders([]string{"Name", "Type", "Durable", "Auto Del", "Internal", "Msg In", "Msg Out"})

	view.SetRefreshCallback(func() ([][]string, error) {
		return rv.refreshExchanges()
	})

	view.AddKeyBinding("R", "Refresh", rv.refresh)
	view.AddKeyBinding("?", "Help", rv.showHelp)
	view.AddKeyBinding("I", "Info", nil)
	view.AddKeyBinding("N", "New Exchange", nil)
	view.AddKeyBinding("D", "Delete", nil)
	view.AddKeyBinding("O", "Overview", nil)
	view.AddKeyBinding("Q", "Queues", nil)
	view.AddKeyBinding("B", "Bindings", nil)

	view.SetActionCallback(rv.handleAction)

	view.SetRowSelectedCallback(func(row int) {
		tableData := view.GetTableData()
		if row >= 0 && row < len(tableData) && len(tableData[row]) > 0 {
			view.Log(fmt.Sprintf("[blue]Selected exchange: %s", tableData[row][0]))
		}
	})

	view.RegisterHandlers()

	return view
}

func (rv *RabbitMQView) refreshExchanges() ([][]string, error) {
	if rv.rmqClient == nil || !rv.rmqClient.IsConnected() {
		return [][]string{{"Not connected", "", "", "", "", "", ""}}, nil
	}

	exchanges, err := rv.rmqClient.GetExchanges()
	if err != nil {
		return [][]string{{"Error: " + err.Error(), "", "", "", "", "", ""}}, nil
	}

	if len(exchanges) == 0 {
		return [][]string{{"No exchanges found", "", "", "", "", "", ""}}, nil
	}

	tableData := make([][]string, len(exchanges))
	for i, ex := range exchanges {
		name := ex.Name
		if name == "" {
			name = "(default)"
		}
		durableStr := boolStr(ex.Durable)
		autoDelStr := boolStr(ex.AutoDelete)
		internalStr := boolStr(ex.Internal)

		tableData[i] = []string{
			name,
			ex.Type,
			durableStr,
			autoDelStr,
			internalStr,
			strconv.FormatInt(ex.MessageStats.PublishIn, 10),
			strconv.FormatInt(ex.MessageStats.PublishOut, 10),
		}
	}

	name := rv.rmqClient.GetClusterName()
	rv.exchangesView.SetInfoText(fmt.Sprintf("[green]RabbitMQ Manager[white]\nInstance: %s\nExchanges: %d", name, len(exchanges)))

	return tableData, nil
}

func (rv *RabbitMQView) showExchangeInfo() {
	selectedRow := rv.exchangesView.GetSelectedRow()
	tableData := rv.exchangesView.GetTableData()
	if selectedRow < 0 || selectedRow >= len(tableData) {
		rv.exchangesView.Log("[red]No exchange selected")
		return
	}

	exName := tableData[selectedRow][0]
	if exName == "(default)" {
		exName = ""
	}

	exchanges, err := rv.rmqClient.GetExchanges()
	if err != nil {
		rv.exchangesView.Log(fmt.Sprintf("[red]Failed to fetch exchange info: %v", err))
		return
	}

	var exchange *ExchangeInfo
	for i := range exchanges {
		if exchanges[i].Name == exName {
			exchange = &exchanges[i]
			break
		}
	}

	if exchange == nil {
		rv.exchangesView.Log(fmt.Sprintf("[red]Exchange %s not found", exName))
		return
	}

	displayName := exchange.Name
	if displayName == "" {
		displayName = "(default)"
	}

	infoText := fmt.Sprintf(`[yellow]Exchange Details[white]

[aqua]Name:[white]        %s
[aqua]VHost:[white]       %s
[aqua]Type:[white]        %s
[aqua]Durable:[white]     %s
[aqua]Auto Delete:[white] %s
[aqua]Internal:[white]    %s

[yellow]Message Stats[white]
[aqua]Messages In:[white]  %d
[aqua]Messages Out:[white] %d`,
		displayName, exchange.VHost, exchange.Type,
		boolStr(exchange.Durable), boolStr(exchange.AutoDelete), boolStr(exchange.Internal),
		exchange.MessageStats.PublishIn, exchange.MessageStats.PublishOut)

	ui.ShowInfoModal(
		rv.pages,
		rv.app,
		fmt.Sprintf("Exchange: %s", displayName),
		infoText,
		func() { rv.app.SetFocus(rv.exchangesView.GetTable()) },
	)
}

func (rv *RabbitMQView) showCreateExchangeForm() {
	if rv.rmqClient == nil || !rv.rmqClient.IsConnected() {
		rv.exchangesView.Log("[yellow]Not connected")
		return
	}

	ui.ShowCompactStyledInputModal(
		rv.pages,
		rv.app,
		"Create Exchange",
		"Exchange Name",
		"",
		30,
		nil,
		func(name string, cancelled bool) {
			if cancelled || name == "" {
				rv.app.SetFocus(rv.exchangesView.GetTable())
				return
			}

			// Default to 'direct' type
			err := rv.rmqClient.CreateExchange(name, "direct", true)
			if err != nil {
				rv.exchangesView.Log(fmt.Sprintf("[red]Failed to create exchange: %v", err))
			} else {
				rv.exchangesView.Log(fmt.Sprintf("[green]Created exchange: %s (type: direct)", name))
				rv.refresh()
			}
			rv.app.SetFocus(rv.exchangesView.GetTable())
		},
	)
}

func (rv *RabbitMQView) deleteSelectedExchange() {
	selectedRow := rv.exchangesView.GetSelectedRow()
	tableData := rv.exchangesView.GetTableData()
	if selectedRow < 0 || selectedRow >= len(tableData) {
		rv.exchangesView.Log("[red]No exchange selected")
		return
	}

	exName := tableData[selectedRow][0]
	if exName == "(default)" {
		rv.exchangesView.Log("[red]Cannot delete the default exchange")
		return
	}

	ui.ShowStandardConfirmationModal(
		rv.pages,
		rv.app,
		"Delete Exchange",
		fmt.Sprintf("Are you sure you want to delete exchange '[red]%s[white]'?", exName),
		func(confirmed bool) {
			if confirmed {
				if err := rv.rmqClient.DeleteExchange(exName); err != nil {
					rv.exchangesView.Log(fmt.Sprintf("[red]Failed to delete exchange: %v", err))
				} else {
					rv.exchangesView.Log(fmt.Sprintf("[yellow]Deleted exchange: %s", exName))
					rv.refresh()
				}
			}
			rv.app.SetFocus(rv.exchangesView.GetTable())
		},
	)
}

func boolStr(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

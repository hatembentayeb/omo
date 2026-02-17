package main

import (
	"fmt"
	"strconv"

	"omo/pkg/ui"
)

// newChannelsView creates the channels CoreView
func (rv *RabbitMQView) newChannelsView() *ui.CoreView {
	view := ui.NewCoreView(rv.app, "RabbitMQ Channels")

	view.SetTableHeaders([]string{"Name", "User", "VHost", "State", "Consumers", "Prefetch", "Unacked", "Confirm"})

	view.SetRefreshCallback(func() ([][]string, error) {
		return rv.refreshChannels()
	})

	view.AddKeyBinding("R", "Refresh", rv.refresh)
	view.AddKeyBinding("?", "Help", rv.showHelp)
	view.AddKeyBinding("I", "Info", nil)
	view.AddKeyBinding("O", "Overview", nil)
	view.AddKeyBinding("Q", "Queues", nil)
	view.AddKeyBinding("C", "Connections", nil)

	view.SetActionCallback(rv.handleAction)

	view.SetRowSelectedCallback(func(row int) {
		tableData := view.GetTableData()
		if row >= 0 && row < len(tableData) && len(tableData[row]) > 0 {
			view.Log(fmt.Sprintf("[blue]Selected channel: %s", tableData[row][0]))
		}
	})

	view.RegisterHandlers()

	return view
}

func (rv *RabbitMQView) refreshChannels() ([][]string, error) {
	if rv.rmqClient == nil || !rv.rmqClient.IsConnected() {
		return [][]string{{"Not connected", "", "", "", "", "", "", ""}}, nil
	}

	channels, err := rv.rmqClient.GetChannels()
	if err != nil {
		return [][]string{{"Error: " + err.Error(), "", "", "", "", "", "", ""}}, nil
	}

	if len(channels) == 0 {
		return [][]string{{"No active channels", "", "", "", "", "", "", ""}}, nil
	}

	tableData := make([][]string, len(channels))
	for i, ch := range channels {
		tableData[i] = []string{
			ch.Name,
			ch.User,
			ch.VHost,
			ch.State,
			strconv.Itoa(ch.Consumers),
			strconv.Itoa(ch.PrefetchCount),
			strconv.FormatInt(ch.MessagesUnack, 10),
			boolStr(ch.Confirm),
		}
	}

	name := rv.rmqClient.GetClusterName()
	rv.channelsView.SetInfoText(fmt.Sprintf("[green]RabbitMQ Manager[white]\nInstance: %s\nChannels: %d", name, len(channels)))

	return tableData, nil
}

func (rv *RabbitMQView) showChannelInfo() {
	selectedRow := rv.channelsView.GetSelectedRow()
	tableData := rv.channelsView.GetTableData()
	if selectedRow < 0 || selectedRow >= len(tableData) {
		rv.channelsView.Log("[red]No channel selected")
		return
	}

	chName := tableData[selectedRow][0]

	channels, err := rv.rmqClient.GetChannels()
	if err != nil {
		rv.channelsView.Log(fmt.Sprintf("[red]Failed to fetch channel info: %v", err))
		return
	}

	var channel *ChannelInfo
	for i := range channels {
		if channels[i].Name == chName {
			channel = &channels[i]
			break
		}
	}

	if channel == nil {
		rv.channelsView.Log(fmt.Sprintf("[red]Channel %s not found", chName))
		return
	}

	infoText := fmt.Sprintf(`[yellow]Channel Details[white]

[aqua]Name:[white]          %s
[aqua]Node:[white]          %s
[aqua]User:[white]          %s
[aqua]VHost:[white]         %s
[aqua]Number:[white]        %d
[aqua]State:[white]         %s
[aqua]Consumers:[white]     %d
[aqua]Prefetch:[white]      %d
[aqua]Confirm:[white]       %s
[aqua]Transactional:[white] %s
[aqua]Unacked:[white]       %d
[aqua]Unconfirmed:[white]   %d`,
		channel.Name, channel.Node, channel.User, channel.VHost,
		channel.Number, channel.State,
		channel.Consumers, channel.PrefetchCount,
		boolStr(channel.Confirm), boolStr(channel.Transactional),
		channel.MessagesUnack, channel.MessagesUnconfirmed)

	ui.ShowInfoModal(
		rv.pages,
		rv.app,
		"Channel Info",
		infoText,
		func() { rv.app.SetFocus(rv.channelsView.GetTable()) },
	)
}

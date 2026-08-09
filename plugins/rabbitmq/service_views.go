package rabbitmq

import (
	"fmt"
	"strconv"
	"time"

	"omo/pkg/pluginrpc"
)

func viewNavBindings() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "0", Label: "Overview", Action: "goto_overview"},
		{Key: "1", Label: "Queues", Action: "goto_queues"},
		{Key: "2", Label: "Exchanges", Action: "goto_exchanges"},
		{Key: "3", Label: "Bindings", Action: "goto_bindings"},
		{Key: "4", Label: "Connections", Action: "goto_connections"},
		{Key: "5", Label: "Channels", Action: "goto_channels"},
		{Key: "6", Label: "Nodes", Action: "goto_nodes"},
	}
}

func queuesActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "I", Label: "Info", Action: "info"},
		{Key: "N", Label: "New Queue", Action: "create_queue"},
		{Key: "D", Label: "Delete", Action: "delete_queue"},
		{Key: "P", Label: "Purge", Action: "purge_queue"},
		{Key: "M", Label: "Messages", Action: "browse_messages"},
		{Key: "U", Label: "Publish", Action: "publish"},
	}
}

func exchangesActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "I", Label: "Info", Action: "info"},
		{Key: "N", Label: "New Exchange", Action: "create_exchange"},
		{Key: "D", Label: "Delete", Action: "delete_exchange"},
	}
}

func connectionsActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "I", Label: "Info", Action: "info"},
		{Key: "D", Label: "Close Conn", Action: "close_connection"},
	}
}

func channelsActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "I", Label: "Info", Action: "info"},
	}
}

func nodesActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "I", Label: "Info", Action: "info"},
	}
}

func helpSections() []pluginrpc.HelpSection {
	return pluginrpc.HelpWithGlobal([]pluginrpc.HelpSection{
		{Title: "Views (0-6)", Bindings: viewNavBindings()},
		{Title: "Queues", Bindings: queuesActions()},
		{Title: "Exchanges", Bindings: exchangesActions()},
		{Title: "Connections", Bindings: connectionsActions()},
		{Title: "Channels", Bindings: channelsActions()},
		{Title: "Nodes", Bindings: nodesActions()},
	}...)
}

func decorate(view pluginrpc.ViewData, actions ...pluginrpc.KeyBinding) pluginrpc.ViewData {
	return pluginrpc.Decorate(view, viewNavBindings(), nil, helpSections(), actions...)
}

func (s *Service) baseInfo(extra string) string {
	name := s.name
	if name == "" {
		name = s.host
	}
	msg := fmt.Sprintf("[green]RabbitMQ Manager[white]\nInstance: %s\nHost: %s\nVHost: %s\nStatus: Connected\nView: %s",
		name, s.host, s.vhost, s.currentView)
	if extra != "" {
		msg += "\n" + extra
	}
	return msg
}

func (s *Service) buildViewLocked(viewID string) (pluginrpc.ViewData, error) {
	if viewID == "" {
		viewID = rmqViewOverview
	}
	s.currentView = viewID

	if err := s.ensureConnectedLocked(); err != nil {
		return pluginrpc.ViewData{
			View:    viewID,
			Title:   "RabbitMQ Manager",
			Info:    "[yellow]RabbitMQ Manager[white]\nStatus: Not Connected\n" + err.Error(),
			Status:  "not connected",
			Headers: []string{"Status", "Detail"},
			Rows:    [][]string{{"error", err.Error()}},
			KeyBindings: []pluginrpc.KeyBinding{
				{Key: "R", Label: "Refresh", Action: "refresh"},
			},
		}, nil
	}

	switch viewID {
	case rmqViewQueues:
		return s.viewQueuesLocked()
	case rmqViewExchanges:
		return s.viewExchangesLocked()
	case rmqViewBindings:
		return s.viewBindingsLocked()
	case rmqViewConnections:
		return s.viewConnectionsLocked()
	case rmqViewChannels:
		return s.viewChannelsLocked()
	case rmqViewNodes:
		return s.viewNodesLocked()
	default:
		return s.viewOverviewLocked()
	}
}

func (s *Service) viewOverviewLocked() (pluginrpc.ViewData, error) {
	overview, err := s.client.GetOverview()
	if err != nil {
		return pluginrpc.ViewData{}, err
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

	rows := [][]string{
		{"RabbitMQ Version", overview.RabbitMQVersion},
		{"Erlang Version", overview.ErlangVersion},
		{"Cluster Name", overview.ClusterName},
		{"Node", overview.Node},
		{"Total Messages", strconv.FormatInt(overview.QueueTotals.Messages, 10)},
		{"Messages Ready", strconv.FormatInt(overview.QueueTotals.MessagesReady, 10)},
		{"Messages Unacked", strconv.FormatInt(overview.QueueTotals.MessagesUnack, 10)},
		{"Queues", strconv.Itoa(overview.ObjectTotals.Queues)},
		{"Exchanges", strconv.Itoa(overview.ObjectTotals.Exchanges)},
		{"Connections", strconv.Itoa(overview.ObjectTotals.Connections)},
		{"Channels", strconv.Itoa(overview.ObjectTotals.Channels)},
		{"Consumers", strconv.Itoa(overview.ObjectTotals.Consumers)},
		{"Published", fmt.Sprintf("%d (%s)", overview.MessageStats.Publish, publishRate)},
		{"Delivered/Get", fmt.Sprintf("%d (%s)", overview.MessageStats.DeliverGet, deliverRate)},
		{"Acknowledged", fmt.Sprintf("%d (%s)", overview.MessageStats.Ack, ackRate)},
		{"Confirmed", strconv.FormatInt(overview.MessageStats.Confirm, 10)},
		{"Redelivered", strconv.FormatInt(overview.MessageStats.Redeliver, 10)},
		{"Returned", strconv.FormatInt(overview.MessageStats.ReturnUnroutable, 10)},
		{"Disk Reads", strconv.FormatInt(overview.MessageStats.DiskReads, 10)},
		{"Disk Writes", strconv.FormatInt(overview.MessageStats.DiskWrites, 10)},
	}
	for _, l := range overview.Listeners {
		rows = append(rows, []string{l.Protocol, strconv.Itoa(l.Port)})
	}

	return decorate(pluginrpc.ViewData{
		View:         rmqViewOverview,
		Title:        "RabbitMQ Overview",
		Info:         s.baseInfo(fmt.Sprintf("Version: %s", overview.RabbitMQVersion)),
		Status:       "connected",
		Headers:      []string{"Metric", "Value"},
		Rows:         rows,
		SelectionKey: "Metric",
	}), nil
}

func (s *Service) viewQueuesLocked() (pluginrpc.ViewData, error) {
	queues, err := s.client.GetQueues()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(queues))
	for _, q := range queues {
		qType := q.Type
		if qType == "" {
			qType = "classic"
		}
		rows = append(rows, []string{
			q.Name,
			strconv.FormatInt(q.Messages, 10),
			strconv.FormatInt(q.Ready, 10),
			strconv.FormatInt(q.Unacked, 10),
			strconv.Itoa(q.Consumers),
			q.State,
			qType,
		})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"No queues found", "", "", "", "", "", ""})
	}
	return decorate(pluginrpc.ViewData{
		View:         rmqViewQueues,
		Title:        "RabbitMQ Queues",
		Info:         s.baseInfo(fmt.Sprintf("Queues: %d", len(queues))),
		Status:       "connected",
		Headers:      []string{"Name", "Messages", "Ready", "Unacked", "Consumers", "State", "Type"},
		Rows:         rows,
		SelectionKey: "Name",
	}, queuesActions()...), nil
}

func (s *Service) viewExchangesLocked() (pluginrpc.ViewData, error) {
	exchanges, err := s.client.GetExchanges()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(exchanges))
	for _, ex := range exchanges {
		name := ex.Name
		if name == "" {
			name = "(default)"
		}
		rows = append(rows, []string{
			name, ex.Type, boolStr(ex.Durable), boolStr(ex.AutoDelete), boolStr(ex.Internal),
			strconv.FormatInt(ex.MessageStats.PublishIn, 10),
			strconv.FormatInt(ex.MessageStats.PublishOut, 10),
		})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"No exchanges found", "", "", "", "", "", ""})
	}
	return decorate(pluginrpc.ViewData{
		View:         rmqViewExchanges,
		Title:        "RabbitMQ Exchanges",
		Info:         s.baseInfo(fmt.Sprintf("Exchanges: %d", len(exchanges))),
		Status:       "connected",
		Headers:      []string{"Name", "Type", "Durable", "Auto Del", "Internal", "Msg In", "Msg Out"},
		Rows:         rows,
		SelectionKey: "Name",
	}, exchangesActions()...), nil
}

func (s *Service) viewBindingsLocked() (pluginrpc.ViewData, error) {
	bindings, err := s.client.GetBindings()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(bindings))
	for _, b := range bindings {
		source := b.Source
		if source == "" {
			source = "(default)"
		}
		rows = append(rows, []string{source, b.Destination, b.DestinationType, b.RoutingKey})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"No bindings found", "", "", ""})
	}
	return decorate(pluginrpc.ViewData{
		View:         rmqViewBindings,
		Title:        "RabbitMQ Bindings",
		Info:         s.baseInfo(fmt.Sprintf("Bindings: %d", len(bindings))),
		Status:       "connected",
		Headers:      []string{"Source", "Destination", "Dest Type", "Routing Key"},
		Rows:         rows,
		SelectionKey: "Source",
	}), nil
}

func (s *Service) viewConnectionsLocked() (pluginrpc.ViewData, error) {
	connections, err := s.client.GetConnections()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(connections))
	for _, c := range connections {
		rows = append(rows, []string{
			c.Name, c.User, c.VHost, c.State, strconv.Itoa(c.Channels),
			fmt.Sprintf("%s:%d", c.PeerHost, c.PeerPort), boolStr(c.SSL),
		})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"No active connections", "", "", "", "", "", ""})
	}
	return decorate(pluginrpc.ViewData{
		View:         rmqViewConnections,
		Title:        "RabbitMQ Connections",
		Info:         s.baseInfo(fmt.Sprintf("Connections: %d", len(connections))),
		Status:       "connected",
		Headers:      []string{"Name", "User", "VHost", "State", "Channels", "Peer", "SSL"},
		Rows:         rows,
		SelectionKey: "Name",
	}, connectionsActions()...), nil
}

func (s *Service) viewChannelsLocked() (pluginrpc.ViewData, error) {
	channels, err := s.client.GetChannels()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(channels))
	for _, ch := range channels {
		rows = append(rows, []string{
			ch.Name, ch.User, ch.VHost, ch.State,
			strconv.Itoa(ch.Consumers), strconv.Itoa(ch.PrefetchCount),
			strconv.FormatInt(ch.MessagesUnack, 10), boolStr(ch.Confirm),
		})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"No active channels", "", "", "", "", "", "", ""})
	}
	return decorate(pluginrpc.ViewData{
		View:         rmqViewChannels,
		Title:        "RabbitMQ Channels",
		Info:         s.baseInfo(fmt.Sprintf("Channels: %d", len(channels))),
		Status:       "connected",
		Headers:      []string{"Name", "User", "VHost", "State", "Consumers", "Prefetch", "Unacked", "Confirm"},
		Rows:         rows,
		SelectionKey: "Name",
	}, channelsActions()...), nil
}

func (s *Service) viewNodesLocked() (pluginrpc.ViewData, error) {
	nodes, err := s.client.GetNodes()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(nodes))
	for _, n := range nodes {
		rows = append(rows, []string{
			n.Name, n.Type, boolStr(n.Running),
			formatBytes(n.MemUsed), formatBytes(n.DiskFree),
			fmt.Sprintf("%d/%d", n.FDUsed, n.FDTotal),
			fmt.Sprintf("%d/%d", n.SocketsUsed, n.SocketsTotal),
			formatDuration(time.Duration(n.Uptime) * time.Millisecond),
		})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"No nodes found", "", "", "", "", "", "", ""})
	}
	return decorate(pluginrpc.ViewData{
		View:         rmqViewNodes,
		Title:        "RabbitMQ Nodes",
		Info:         s.baseInfo(fmt.Sprintf("Nodes: %d", len(nodes))),
		Status:       "connected",
		Headers:      []string{"Name", "Type", "Running", "Memory", "Disk Free", "FD Used", "Sockets", "Uptime"},
		Rows:         rows,
		SelectionKey: "Name",
	}, nodesActions()...), nil
}

package main

import (
	"fmt"

	"omo/pkg/ui"
)

func (rv *RedisView) newPubSubView() *ui.Cores {
	cores := ui.NewCores(rv.app, "Redis PubSub")
	cores.SetTableHeaders([]string{"Channel", "Subscribers", "Type"})
	cores.SetRefreshCallback(rv.refreshPubSub)
	cores.AddKeyBinding("K", "Keys", rv.showKeys)
	cores.AddKeyBinding("I", "Server Info", rv.showServerInfo)
	cores.AddKeyBinding("L", "Slowlog", rv.showSlowlog)
	cores.AddKeyBinding("T", "Stats", rv.showStats)
	cores.AddKeyBinding("C", "Clients", rv.showClients)
	cores.AddKeyBinding("G", "Config", rv.showConfig)
	cores.AddKeyBinding("M", "Memory", rv.showMemory)
	cores.AddKeyBinding("P", "Persistence", rv.showPersistence)
	cores.AddKeyBinding("Y", "Replication", rv.showReplication)
	cores.AddKeyBinding("B", "PubSub", rv.showPubSub)
	cores.AddKeyBinding("A", "Key Analysis", rv.showKeyAnalysis)
	cores.AddKeyBinding("W", "Databases", rv.showDatabases)
	cores.AddKeyBinding("X", "Cmd Stats", rv.showCommandStats)
	cores.AddKeyBinding("Z", "Latency", rv.showLatency)
	cores.SetActionCallback(rv.handleAction)
	cores.RegisterHandlers()
	return cores
}

func (rv *RedisView) refreshPubSub() ([][]string, error) {
	if rv.redisClient == nil || !rv.redisClient.IsConnected() {
		return [][]string{{"Status", "Not Connected", "-"}}, nil
	}

	channels, err := rv.redisClient.GetPubSubChannels()
	if err != nil {
		return [][]string{}, err
	}

	data := make([][]string, 0, len(channels))
	for _, ch := range channels {
		channelType := "Channel"
		if ch.Pattern {
			channelType = "Pattern"
		}
		
		data = append(data, []string{
			ch.Channel,
			fmt.Sprintf("%d", ch.Subscribers),
			channelType,
		})
	}

	if len(data) == 0 {
		data = append(data, []string{"-", "0", "No active channels"})
	}

	return data, nil
}

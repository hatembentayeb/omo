package main

import (
	"fmt"

	"omo/pkg/ui"
)

func (rv *RedisView) newLatencyView() *ui.Cores {
	cores := ui.NewCores(rv.app, "Redis Latency")
	cores.SetTableHeaders([]string{"Event", "Timestamp", "Latency (ms)"})
	cores.SetRefreshCallback(rv.refreshLatency)
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

func (rv *RedisView) refreshLatency() ([][]string, error) {
	if rv.redisClient == nil || !rv.redisClient.IsConnected() {
		return [][]string{{"Status", "Not Connected", "-"}}, nil
	}

	events, err := rv.redisClient.GetLatencyHistory()
	if err != nil {
		return [][]string{}, err
	}

	data := make([][]string, 0, len(events))
	for _, event := range events {
		data = append(data, []string{
			event.Event,
			event.Timestamp.Format("2006-01-02 15:04:05"),
			fmt.Sprintf("%d", event.Latency),
		})
	}

	if len(data) == 0 {
		data = append(data, []string{"-", "-", "No latency events recorded"})
	}

	return data, nil
}

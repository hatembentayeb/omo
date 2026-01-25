package main

import (
	"fmt"

	"omo/pkg/ui"
)

func (rv *RedisView) newSlowlogView() *ui.Cores {
	cores := ui.NewCores(rv.app, "Redis Slowlog")
	cores.SetTableHeaders([]string{"ID", "Timestamp", "Duration", "Command", "Client"})
	cores.SetRefreshCallback(rv.refreshSlowlog)
	cores.AddKeyBinding("K", "Keys", rv.showKeys)
	cores.AddKeyBinding("I", "Server Info", rv.showServerInfo)
	cores.AddKeyBinding("T", "Stats", rv.showStats)
	cores.AddKeyBinding("L", "Slowlog", rv.showSlowlog)
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

func (rv *RedisView) refreshSlowlog() ([][]string, error) {
	if rv.redisClient == nil || !rv.redisClient.IsConnected() {
		return [][]string{{"Status", "Not Connected", "", "", ""}}, nil
	}

	entries, err := rv.redisClient.GetSlowLog(20)
	if err != nil {
		return [][]string{}, err
	}

	data := make([][]string, 0, len(entries))
	for _, entry := range entries {
		data = append(data, []string{
			fmt.Sprintf("%d", entry.ID),
			entry.Timestamp.Format("15:04:05"),
			fmt.Sprintf("%s", entry.Duration),
			entry.Command,
			entry.Client,
		})
	}

	if len(data) == 0 {
		data = append(data, []string{"-", "-", "-", "No slowlog entries", "-"})
	}

	return data, nil
}

package main

import (
	"fmt"
	"sort"

	"omo/pkg/ui"
)

func (rv *RedisView) newCommandStatsView() *ui.CoreView {
	cores := ui.NewCoreView(rv.app, "Redis Command Stats")
	cores.SetTableHeaders([]string{"Command", "Calls", "Total Time (μs)", "Avg Time (μs)"})
	cores.SetRefreshCallback(rv.refreshCommandStats)
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

func (rv *RedisView) refreshCommandStats() ([][]string, error) {
	if rv.redisClient == nil || !rv.redisClient.IsConnected() {
		return [][]string{{"Status", "Not Connected", "-", "-"}}, nil
	}

	stats, err := rv.redisClient.GetCommandStats()
	if err != nil {
		return [][]string{}, err
	}

	// Sort by calls descending
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Calls > stats[j].Calls
	})

	data := make([][]string, 0, len(stats))
	for _, stat := range stats {
		data = append(data, []string{
			stat.Command,
			fmt.Sprintf("%d", stat.Calls),
			fmt.Sprintf("%d", stat.Usec),
			fmt.Sprintf("%.2f", stat.UsecPerCall),
		})
	}

	if len(data) == 0 {
		data = append(data, []string{"-", "0", "0", "No command stats"})
	}

	return data, nil
}

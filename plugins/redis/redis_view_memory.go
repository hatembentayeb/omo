package main

import (
	"sort"

	"omo/pkg/ui"
)

func (rv *RedisView) newMemoryView() *ui.CoreView {
	cores := ui.NewCoreView(rv.app, "Redis Memory")
	cores.SetTableHeaders([]string{"Metric", "Value"})
	cores.SetRefreshCallback(rv.refreshMemory)
	cores.AddKeyBinding("D", "Memory Doctor", rv.showMemoryDoctor)
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

func (rv *RedisView) refreshMemory() ([][]string, error) {
	if rv.redisClient == nil || !rv.redisClient.IsConnected() {
		return [][]string{{"Status", "Not Connected"}}, nil
	}

	stats, err := rv.redisClient.GetMemoryStats()
	if err != nil {
		return [][]string{}, err
	}

	keys := make([]string, 0, len(stats))
	for key := range stats {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	data := make([][]string, 0, len(keys))
	for _, key := range keys {
		data = append(data, []string{key, stats[key]})
	}

	return data, nil
}

func (rv *RedisView) showMemoryDoctor() {
	if rv.redisClient == nil || !rv.redisClient.IsConnected() {
		return
	}

	report, err := rv.redisClient.GetMemoryDoctor()
	if err != nil {
		rv.currentCores().Log("[red]" + err.Error())
		return
	}

	ui.ShowInfoModal(
		rv.pages,
		rv.app,
		"Memory Doctor",
		report,
		func() {
			current := rv.currentCores()
			if current != nil {
				rv.app.SetFocus(current.GetTable())
			}
		},
	)
}

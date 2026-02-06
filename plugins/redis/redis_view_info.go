package main

import (
	"omo/pkg/ui"
)

func (rv *RedisView) newInfoView() *ui.CoreView {
	cores := ui.NewCoreView(rv.app, "Redis Server Info")
	cores.SetTableHeaders([]string{"Property", "Value"})
	cores.SetRefreshCallback(rv.refreshServerInfo)
	cores.AddKeyBinding("K", "Keys", rv.showKeys)
	cores.AddKeyBinding("L", "Slowlog", rv.showSlowlog)
	cores.AddKeyBinding("T", "Stats", rv.showStats)
	cores.AddKeyBinding("I", "Info", rv.showServerInfo)
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

func (rv *RedisView) refreshServerInfo() ([][]string, error) {
	if rv.redisClient == nil || !rv.redisClient.IsConnected() {
		return [][]string{{"Status", "Not Connected"}}, nil
	}

	infoMap, err := rv.redisClient.GetInfoMap()
	if err != nil {
		return [][]string{}, err
	}

	fields := []string{
		"redis_version",
		"redis_mode",
		"os",
		"tcp_port",
		"uptime_in_seconds",
		"uptime_in_days",
		"connected_clients",
		"used_memory_human",
		"used_memory_peak_human",
		"role",
	}

	data := make([][]string, 0, len(fields))
	for _, field := range fields {
		if value, ok := infoMap[field]; ok && value != "" {
			data = append(data, []string{field, value})
		}
	}

	return data, nil
}

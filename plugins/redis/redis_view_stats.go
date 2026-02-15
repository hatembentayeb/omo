package main

import (
	"fmt"
	"sort"
	"strings"

	"omo/pkg/ui"
)

func (rv *RedisView) newStatsView() *ui.CoreView {
	cores := ui.NewCoreView(rv.app, "Redis Stats")
	cores.SetTableHeaders([]string{"Metric", "Value"})
	cores.SetRefreshCallback(rv.refreshStats)
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

func (rv *RedisView) refreshStats() ([][]string, error) {
	if rv.redisClient == nil || !rv.redisClient.IsConnected() {
		return [][]string{{"Status", "Not Connected"}}, nil
	}

	infoMap, err := rv.redisClient.GetInfoMap()
	if err != nil {
		return [][]string{}, err
	}

	stats := []string{
		"connected_clients",
		"blocked_clients",
		"instantaneous_ops_per_sec",
		"total_commands_processed",
		"keyspace_hits",
		"keyspace_misses",
		"expired_keys",
		"evicted_keys",
		"used_memory_human",
		"used_memory_peak_human",
	}

	data := make([][]string, 0, len(stats)+1)
	for _, key := range stats {
		if value, ok := infoMap[key]; ok && value != "" {
			data = append(data, []string{key, value})
		}
	}

	keyspace := parseKeyspace(infoMap)
	if keyspace != "" {
		data = append(data, []string{"keyspace", keyspace})
	}

	return data, nil
}

func parseKeyspace(infoMap map[string]string) string {
	dbFields := make([]string, 0)
	for key, value := range infoMap {
		if strings.HasPrefix(key, "db") {
			dbFields = append(dbFields, fmt.Sprintf("%s=%s", key, value))
		}
	}
	sort.Strings(dbFields)
	return strings.Join(dbFields, ", ")
}

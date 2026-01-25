package main

import (
	"sort"

	"omo/pkg/ui"
)

func (rv *RedisView) newReplicationView() *ui.Cores {
	cores := ui.NewCores(rv.app, "Redis Replication")
	cores.SetTableHeaders([]string{"Property", "Value"})
	cores.SetRefreshCallback(rv.refreshReplication)
	cores.AddKeyBinding("K", "Keys", rv.showKeys)
	cores.AddKeyBinding("I", "Server Info", rv.showServerInfo)
	cores.AddKeyBinding("L", "Slowlog", rv.showSlowlog)
	cores.AddKeyBinding("T", "Stats", rv.showStats)
	cores.AddKeyBinding("C", "Clients", rv.showClients)
	cores.AddKeyBinding("G", "Config", rv.showConfig)
	cores.AddKeyBinding("M", "Memory", rv.showMemory)
	cores.AddKeyBinding("P", "Persistence", rv.showPersistence)
	cores.AddKeyBinding("Y", "Replication", rv.showReplication)
	cores.SetActionCallback(rv.handleAction)
	cores.RegisterHandlers()
	return cores
}

func (rv *RedisView) refreshReplication() ([][]string, error) {
	if rv.redisClient == nil || !rv.redisClient.IsConnected() {
		return [][]string{{"Status", "Not Connected"}}, nil
	}

	infoMap, err := rv.redisClient.GetInfoSectionMap("replication")
	if err != nil {
		return [][]string{}, err
	}

	keys := make([]string, 0, len(infoMap))
	for key := range infoMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	data := make([][]string, 0, len(keys))
	for _, key := range keys {
		data = append(data, []string{key, infoMap[key]})
	}

	return data, nil
}

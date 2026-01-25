package main

import (
	"sort"

	"omo/pkg/ui"
)

func (rv *RedisView) newConfigView() *ui.Cores {
	cores := ui.NewCores(rv.app, "Redis Config")
	cores.SetTableHeaders([]string{"Config", "Value"})
	cores.SetRefreshCallback(rv.refreshConfig)
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

func (rv *RedisView) refreshConfig() ([][]string, error) {
	if rv.redisClient == nil || !rv.redisClient.IsConnected() {
		return [][]string{{"Status", "Not Connected"}}, nil
	}

	config, err := rv.redisClient.GetConfig("*")
	if err != nil {
		return [][]string{}, err
	}

	keys := make([]string, 0, len(config))
	for key := range config {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	data := make([][]string, 0, len(keys))
	for _, key := range keys {
		data = append(data, []string{key, config[key]})
	}

	if len(data) == 0 {
		data = append(data, []string{"-", "No config entries"})
	}

	return data, nil
}

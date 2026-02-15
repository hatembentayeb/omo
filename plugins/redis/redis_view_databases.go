package main

import (
	"fmt"
	"sort"

	"omo/pkg/ui"
)

func (rv *RedisView) newDatabasesView() *ui.CoreView {
	cores := ui.NewCoreView(rv.app, "Redis Databases")
	cores.SetTableHeaders([]string{"DB", "Keys", "Expires", "Avg TTL"})
	cores.SetRefreshCallback(rv.refreshDatabases)
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
	cores.AddKeyBinding("S", "Switch DB", rv.showDBSelector)
	cores.SetActionCallback(rv.handleAction)
	cores.RegisterHandlers()
	return cores
}

func (rv *RedisView) refreshDatabases() ([][]string, error) {
	if rv.redisClient == nil || !rv.redisClient.IsConnected() {
		return [][]string{{"Status", "Not Connected", "-", "-"}}, nil
	}

	databases, err := rv.redisClient.GetAllDatabases()
	if err != nil {
		return [][]string{}, err
	}

	// Sort by database ID
	sort.Slice(databases, func(i, j int) bool {
		return databases[i].ID < databases[j].ID
	})

	data := make([][]string, 0, len(databases))
	for _, db := range databases {
		ttlStr := "-"
		if db.AvgTTL > 0 {
			ttlStr = fmt.Sprintf("%ds", db.AvgTTL)
		}

		dbName := fmt.Sprintf("db%d", db.ID)
		if db.ID == rv.currentDatabase {
			dbName = fmt.Sprintf("db%d *", db.ID) // Mark current database
		}

		data = append(data, []string{
			dbName,
			fmt.Sprintf("%d", db.Keys),
			fmt.Sprintf("%d", db.Expires),
			ttlStr,
		})
	}

	if len(data) == 0 {
		data = append(data, []string{"-", "0", "0", "No databases with keys"})
	}

	return data, nil
}

package main

import (
	"omo/pkg/ui"
)

func (rv *RedisView) newClientsView() *ui.Cores {
	cores := ui.NewCores(rv.app, "Redis Clients")
	cores.SetTableHeaders([]string{"ID", "Addr", "Name", "Age", "Idle", "Flags", "DB", "Cmd"})
	cores.SetRefreshCallback(rv.refreshClients)
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

func (rv *RedisView) refreshClients() ([][]string, error) {
	if rv.redisClient == nil || !rv.redisClient.IsConnected() {
		return [][]string{{"Status", "Not Connected", "", "", "", "", "", ""}}, nil
	}

	clients, err := rv.redisClient.GetClients()
	if err != nil {
		return [][]string{}, err
	}

	data := make([][]string, 0, len(clients))
	for _, client := range clients {
		data = append(data, []string{
			client.ID,
			client.Addr,
			client.Name,
			client.Age,
			client.Idle,
			client.Flags,
			client.DB,
			client.Cmd,
		})
	}

	if len(data) == 0 {
		data = append(data, []string{"-", "-", "-", "-", "-", "-", "-", "No clients"})
	}

	return data, nil
}

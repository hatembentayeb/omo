package main

import (
	"fmt"
	"sort"
	"strings"

	"omo/pkg/ui"
)

func (rv *RedisView) newKeyAnalysisView() *ui.CoreView {
	cores := ui.NewCoreView(rv.app, "Redis Key Analysis")
	cores.SetTableHeaders([]string{"Pattern", "Count", "Types", "Avg TTL", "Sample Keys"})
	cores.SetRefreshCallback(rv.refreshKeyAnalysis)
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

func (rv *RedisView) refreshKeyAnalysis() ([][]string, error) {
	if rv.redisClient == nil || !rv.redisClient.IsConnected() {
		return [][]string{{"Status", "Not Connected", "-", "-", "-"}}, nil
	}

	patterns, err := rv.redisClient.AnalyzeKeyPatterns(1000)
	if err != nil {
		return [][]string{}, err
	}

	// Sort by count descending
	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].Count > patterns[j].Count
	})

	data := make([][]string, 0, len(patterns))
	for _, p := range patterns {
		// Format types
		types := make([]string, 0, len(p.Types))
		for t, count := range p.Types {
			types = append(types, fmt.Sprintf("%s:%d", t, count))
		}
		typesStr := strings.Join(types, ", ")

		// Format TTL
		ttlStr := "-"
		if p.AvgTTL > 0 {
			ttlStr = fmt.Sprintf("%ds", p.AvgTTL)
		}

		// Format sample keys
		sampleStr := strings.Join(p.SampleKeys, ", ")
		if len(sampleStr) > 50 {
			sampleStr = sampleStr[:47] + "..."
		}

		data = append(data, []string{
			p.Pattern,
			fmt.Sprintf("%d", p.Count),
			typesStr,
			ttlStr,
			sampleStr,
		})
	}

	if len(data) == 0 {
		data = append(data, []string{"-", "0", "-", "-", "No keys found"})
	}

	return data, nil
}

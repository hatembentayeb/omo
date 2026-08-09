package redis

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"omo/pkg/pluginrpc"
)

// Primary views on digit keys 0-9. Remaining views use letter shortcuts and appear in "?".
func viewNavBindings() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "0", Label: "Keys", Action: "goto_keys"},
		{Key: "1", Label: "Info", Action: "goto_info"},
		{Key: "2", Label: "Slowlog", Action: "goto_slowlog"},
		{Key: "3", Label: "Stats", Action: "goto_stats"},
		{Key: "4", Label: "Clients", Action: "goto_clients"},
		{Key: "5", Label: "Config", Action: "goto_config"},
		{Key: "6", Label: "Memory", Action: "goto_memory"},
		{Key: "7", Label: "Persist", Action: "goto_persistence"},
		{Key: "8", Label: "Repl", Action: "goto_replication"},
		{Key: "9", Label: "PubSub", Action: "goto_pubsub"},
	}
}

func moreViewBindings() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "A", Label: "Key Analysis", Action: "goto_keyanalysis"},
		{Key: "W", Label: "Databases", Action: "goto_databases"},
		{Key: "X", Label: "Cmd Stats", Action: "goto_commandstats"},
		{Key: "Z", Label: "Latency", Action: "goto_latency"},
	}
}

func keysActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "D", Label: "Del Key", Action: "delete"},
		{Key: "F", Label: "Flush DB", Action: "flush"},
		{Key: "N", Label: "New Key", Action: "create_key"},
		{Key: "E", Label: "View Key", Action: "view_key"},
		{Key: "S", Label: "DB Select", Action: "select_db"},
	}
}

func memoryActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "D", Label: "Memory Doctor", Action: "memory_doctor"},
	}
}

func pubsubActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "U", Label: "Publish", Action: "publish"},
		{Key: "E", Label: "Peek Msgs", Action: "subscribe"},
	}
}

func databasesActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "S", Label: "Switch DB", Action: "select_db"},
	}
}

func helpSections() []pluginrpc.HelpSection {
	return pluginrpc.HelpWithGlobal([]pluginrpc.HelpSection{
		{Title: "Views (0-9)", Bindings: viewNavBindings()},
		{Title: "More Views", Bindings: moreViewBindings()},
		{Title: "Keys", Bindings: keysActions()},
		{Title: "Memory", Bindings: memoryActions()},
		{Title: "PubSub", Bindings: pubsubActions()},
		{Title: "Databases", Bindings: databasesActions()},
	}...)
}

func decorate(view pluginrpc.ViewData, actions ...pluginrpc.KeyBinding) pluginrpc.ViewData {
	return pluginrpc.Decorate(view, viewNavBindings(), moreViewBindings(), helpSections(), actions...)
}

func (s *Service) baseInfo(extra string) string {
	msg := fmt.Sprintf("[green]Redis Manager[white]\nServer: %s:%s\nDB: %d\nStatus: Connected\nView: %s",
		s.host, s.port, s.database, s.currentView)
	if extra != "" {
		msg += "\n" + extra
	}
	return msg
}

func (s *Service) buildViewLocked(viewID string) (pluginrpc.ViewData, error) {
	if viewID == "" {
		viewID = viewKeys
	}
	s.currentView = viewID

	if err := s.ensureConnectedLocked(); err != nil {
		return pluginrpc.ViewData{
			View:    viewID,
			Title:   "Redis Manager",
			Info:    "[yellow]Redis Manager[white]\nStatus: Not Connected\n" + err.Error(),
			Status:  "not connected",
			Headers: []string{"Status", "Detail"},
			Rows:    [][]string{{"error", err.Error()}},
			KeyBindings: []pluginrpc.KeyBinding{
				{Key: "R", Label: "Refresh", Action: "refresh"},
			},
		}, nil
	}

	switch viewID {
	case viewInfo:
		return s.viewInfoLocked()
	case viewSlowlog:
		return s.viewSlowlogLocked()
	case viewStats:
		return s.viewStatsLocked()
	case viewClients:
		return s.viewClientsLocked()
	case viewConfig:
		return s.viewConfigLocked()
	case viewMemory:
		return s.viewMemoryLocked()
	case viewPersist:
		return s.viewPersistenceLocked()
	case viewRepl:
		return s.viewReplicationLocked()
	case viewPubSub:
		return s.viewPubSubLocked()
	case viewKeyAnalysis:
		return s.viewKeyAnalysisLocked()
	case viewDatabases:
		return s.viewDatabasesLocked()
	case viewCmdStats:
		return s.viewCommandStatsLocked()
	case viewLatency:
		return s.viewLatencyLocked()
	default:
		return s.viewKeysLocked()
	}
}

func (s *Service) viewKeysLocked() (pluginrpc.ViewData, error) {
	rows, err := s.loadKeysLocked(500)
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	return decorate(pluginrpc.ViewData{
		View:         viewKeys,
		Title:        "Redis Keys",
		Info:         s.baseInfo(fmt.Sprintf("Keys loaded: %d", len(rows))),
		Status:       "connected",
		Headers:      []string{"Key", "Type", "TTL", "Size"},
		Rows:         rows,
		SelectionKey: "Key",
	}, keysActions()...), nil
}

func (s *Service) viewInfoLocked() (pluginrpc.ViewData, error) {
	infoMap, err := s.client.GetInfoMap()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	fields := []string{
		"redis_version", "redis_mode", "os", "tcp_port",
		"uptime_in_seconds", "uptime_in_days", "connected_clients",
		"used_memory_human", "used_memory_peak_human", "role",
	}
	rows := make([][]string, 0, len(fields))
	for _, field := range fields {
		if value, ok := infoMap[field]; ok && value != "" {
			rows = append(rows, []string{field, value})
		}
	}
	return decorate(pluginrpc.ViewData{
		View:         viewInfo,
		Title:        "Redis Server Info",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"Property", "Value"},
		Rows:         rows,
		SelectionKey: "Property",
	}), nil
}

func (s *Service) viewSlowlogLocked() (pluginrpc.ViewData, error) {
	entries, err := s.client.GetSlowLog(20)
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, []string{
			fmt.Sprintf("%d", entry.ID),
			entry.Timestamp.Format("15:04:05"),
			fmt.Sprintf("%s", entry.Duration),
			entry.Command,
			entry.Client,
		})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"-", "-", "-", "No slowlog entries", "-"})
	}
	return decorate(pluginrpc.ViewData{
		View:         viewSlowlog,
		Title:        "Redis Slowlog",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"ID", "Timestamp", "Duration", "Command", "Client"},
		Rows:         rows,
		SelectionKey: "ID",
	}), nil
}

func (s *Service) viewStatsLocked() (pluginrpc.ViewData, error) {
	infoMap, err := s.client.GetInfoMap()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	stats := []string{
		"connected_clients", "blocked_clients", "instantaneous_ops_per_sec",
		"total_commands_processed", "keyspace_hits", "keyspace_misses",
		"expired_keys", "evicted_keys", "used_memory_human", "used_memory_peak_human",
	}
	rows := make([][]string, 0, len(stats)+1)
	for _, key := range stats {
		if value, ok := infoMap[key]; ok && value != "" {
			rows = append(rows, []string{key, value})
		}
	}
	if ks := parseKeyspace(infoMap); ks != "" {
		rows = append(rows, []string{"keyspace", ks})
	}
	return decorate(pluginrpc.ViewData{
		View:         viewStats,
		Title:        "Redis Stats",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"Metric", "Value"},
		Rows:         rows,
		SelectionKey: "Metric",
	}), nil
}

func (s *Service) viewClientsLocked() (pluginrpc.ViewData, error) {
	clients, err := s.client.GetClients()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(clients))
	for _, client := range clients {
		rows = append(rows, []string{
			client.ID, client.Addr, client.Name, client.Age,
			client.Idle, client.Flags, client.DB, client.Cmd,
		})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"-", "-", "-", "-", "-", "-", "-", "No clients"})
	}
	return decorate(pluginrpc.ViewData{
		View:         viewClients,
		Title:        "Redis Clients",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"ID", "Addr", "Name", "Age", "Idle", "Flags", "DB", "Cmd"},
		Rows:         rows,
		SelectionKey: "ID",
	}), nil
}

func (s *Service) viewConfigLocked() (pluginrpc.ViewData, error) {
	config, err := s.client.GetConfig("*")
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	keys := make([]string, 0, len(config))
	for key := range config {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	rows := make([][]string, 0, len(keys))
	for _, key := range keys {
		rows = append(rows, []string{key, config[key]})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"-", "No config entries"})
	}
	return decorate(pluginrpc.ViewData{
		View:         viewConfig,
		Title:        "Redis Config",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"Config", "Value"},
		Rows:         rows,
		SelectionKey: "Config",
	}), nil
}

func (s *Service) viewMemoryLocked() (pluginrpc.ViewData, error) {
	stats, err := s.client.GetMemoryStats()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	keys := make([]string, 0, len(stats))
	for key := range stats {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	rows := make([][]string, 0, len(keys))
	for _, key := range keys {
		rows = append(rows, []string{key, stats[key]})
	}
	return decorate(pluginrpc.ViewData{
		View:         viewMemory,
		Title:        "Redis Memory",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"Metric", "Value"},
		Rows:         rows,
		SelectionKey: "Metric",
	}, memoryActions()...), nil
}

func (s *Service) viewPersistenceLocked() (pluginrpc.ViewData, error) {
	return s.viewSectionLocked(viewPersist, "Redis Persistence", "persistence")
}

func (s *Service) viewReplicationLocked() (pluginrpc.ViewData, error) {
	return s.viewSectionLocked(viewRepl, "Redis Replication", "replication")
}

func (s *Service) viewSectionLocked(viewID, title, section string) (pluginrpc.ViewData, error) {
	infoMap, err := s.client.GetInfoSectionMap(section)
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	keys := make([]string, 0, len(infoMap))
	for key := range infoMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	rows := make([][]string, 0, len(keys))
	for _, key := range keys {
		rows = append(rows, []string{key, infoMap[key]})
	}
	return decorate(pluginrpc.ViewData{
		View:         viewID,
		Title:        title,
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"Property", "Value"},
		Rows:         rows,
		SelectionKey: "Property",
	}), nil
}

func (s *Service) viewPubSubLocked() (pluginrpc.ViewData, error) {
	channels, err := s.client.GetPubSubChannels()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(channels))
	for _, ch := range channels {
		channelType := "Channel"
		if ch.Pattern {
			channelType = "Pattern"
		}
		rows = append(rows, []string{ch.Channel, fmt.Sprintf("%d", ch.Subscribers), channelType})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"-", "0", "No active channels"})
	}
	return decorate(pluginrpc.ViewData{
		View:         viewPubSub,
		Title:        "Redis PubSub",
		Info:         s.baseInfo("Enter=peek messages"),
		Status:       "connected",
		Headers:      []string{"Channel", "Subscribers", "Type"},
		Rows:         rows,
		SelectionKey: "Channel",
	}, pubsubActions()...), nil
}

func (s *Service) viewKeyAnalysisLocked() (pluginrpc.ViewData, error) {
	patterns, err := s.client.AnalyzeKeyPatterns(1000)
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].Count > patterns[j].Count
	})
	rows := make([][]string, 0, len(patterns))
	for _, p := range patterns {
		types := make([]string, 0, len(p.Types))
		for t, count := range p.Types {
			types = append(types, fmt.Sprintf("%s:%d", t, count))
		}
		ttlStr := "-"
		if p.AvgTTL > 0 {
			ttlStr = fmt.Sprintf("%ds", p.AvgTTL)
		}
		sampleStr := strings.Join(p.SampleKeys, ", ")
		if len(sampleStr) > 50 {
			sampleStr = sampleStr[:47] + "..."
		}
		rows = append(rows, []string{
			p.Pattern,
			fmt.Sprintf("%d", p.Count),
			strings.Join(types, ", "),
			ttlStr,
			sampleStr,
		})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"-", "0", "-", "-", "No keys found"})
	}
	return decorate(pluginrpc.ViewData{
		View:         viewKeyAnalysis,
		Title:        "Redis Key Analysis",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"Pattern", "Count", "Types", "Avg TTL", "Sample Keys"},
		Rows:         rows,
		SelectionKey: "Pattern",
	}), nil
}

func (s *Service) viewDatabasesLocked() (pluginrpc.ViewData, error) {
	databases, err := s.client.GetAllDatabases()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	sort.Slice(databases, func(i, j int) bool {
		return databases[i].ID < databases[j].ID
	})
	rows := make([][]string, 0, len(databases))
	for _, db := range databases {
		ttlStr := "-"
		if db.AvgTTL > 0 {
			ttlStr = fmt.Sprintf("%ds", db.AvgTTL)
		}
		dbName := fmt.Sprintf("db%d", db.ID)
		if db.ID == s.database {
			dbName = fmt.Sprintf("db%d *", db.ID)
		}
		rows = append(rows, []string{
			dbName,
			fmt.Sprintf("%d", db.Keys),
			fmt.Sprintf("%d", db.Expires),
			ttlStr,
		})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"-", "0", "0", "No databases with keys"})
	}
	return decorate(pluginrpc.ViewData{
		View:         viewDatabases,
		Title:        "Redis Databases",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"DB", "Keys", "Expires", "Avg TTL"},
		Rows:         rows,
		SelectionKey: "DB",
	}, databasesActions()...), nil
}

func (s *Service) viewCommandStatsLocked() (pluginrpc.ViewData, error) {
	stats, err := s.client.GetCommandStats()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Calls > stats[j].Calls
	})
	rows := make([][]string, 0, len(stats))
	for _, stat := range stats {
		rows = append(rows, []string{
			stat.Command,
			fmt.Sprintf("%d", stat.Calls),
			fmt.Sprintf("%d", stat.Usec),
			fmt.Sprintf("%.2f", stat.UsecPerCall),
		})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"-", "0", "0", "No command stats"})
	}
	return decorate(pluginrpc.ViewData{
		View:         viewCmdStats,
		Title:        "Redis Command Stats",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"Command", "Calls", "Total Time (μs)", "Avg Time (μs)"},
		Rows:         rows,
		SelectionKey: "Command",
	}), nil
}

func (s *Service) viewLatencyLocked() (pluginrpc.ViewData, error) {
	events, err := s.client.GetLatencyHistory()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(events))
	for _, event := range events {
		rows = append(rows, []string{
			event.Event,
			event.Timestamp.Format("2006-01-02 15:04:05"),
			fmt.Sprintf("%d", event.Latency),
		})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"-", "-", "No latency events recorded"})
	}
	return decorate(pluginrpc.ViewData{
		View:         viewLatency,
		Title:        "Redis Latency",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"Event", "Timestamp", "Latency (ms)"},
		Rows:         rows,
		SelectionKey: "Event",
	}), nil
}

func (s *Service) peekPubSubLocked(channel string) (string, error) {
	sub, err := s.client.SubscribeToChannel(channel)
	if err != nil {
		return "", err
	}
	defer sub.Close()

	var msgs []string
	deadline := time.After(2 * time.Second)
	for len(msgs) < 20 {
		select {
		case msg := <-sub.Messages:
			msgs = append(msgs, msg.Payload)
		case <-deadline:
			goto done
		}
	}
done:
	if len(msgs) == 0 {
		return "(no messages in 2s)", nil
	}
	return strings.Join(msgs, "\n"), nil
}

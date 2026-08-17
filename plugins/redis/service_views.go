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
	return pluginrpc.HelpNav(viewNavBindings(), moreViewBindings(),
		pluginrpc.HelpSection{Title: "Keys", Bindings: keysActions()},
		pluginrpc.HelpSection{Title: "Memory", Bindings: memoryActions()},
		pluginrpc.HelpSection{Title: "PubSub", Bindings: pubsubActions()},
		pluginrpc.HelpSection{Title: "Databases", Bindings: databasesActions()},
	)
}

var ui = pluginrpc.ViewUI{
	Views: viewNavBindings,
	More:  moreViewBindings,
	Help:  helpSections,
}

func (s *Service) baseInfo(extra string) string {
	msg := fmt.Sprintf("[green]Redis Manager[white]\nServer: %s:%s\nDB: %d\nStatus: Connected\nView: %s",
		s.host, s.port, s.database, s.currentView)
	return pluginrpc.FormatInfo(msg, extra)
}

func (s *Service) viewDashboardLocked() (pluginrpc.ViewData, error) {
	if err := s.ensureConnectedLocked(); err != nil {
		return pluginrpc.ViewData{}, err
	}
	info, err := s.client.GetServerInfo()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	uptime := dashValue(info["uptime_in_days"])
	if uptime != "-" {
		uptime += "d"
	}
	return pluginrpc.Widget("Redis", "connected", fmt.Sprintf("%s:%s", s.host, s.port), [][2]string{
		{"Version", dashValue(info["redis_version"])},
		{"Uptime", uptime},
		{"Clients", dashValue(info["connected_clients"])},
		{"Memory", dashValue(info["used_memory_human"])},
	}), nil
}

func dashValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func (s *Service) buildViewLocked(viewID string) (pluginrpc.ViewData, error) {
	if viewID == "" {
		viewID = viewKeys
	}
	s.currentView = viewID

	if err := s.ensureConnectedLocked(); err != nil {
		return ui.NotConnectedErr(viewID, "Redis Manager", err)
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
	rows = pluginrpc.EnsureRows(rows, []string{"-", "-", "-", "No keys found"})
	return ui.Connected(viewKeys, "Redis Keys", s.baseInfo(fmt.Sprintf("Keys loaded: %d", len(rows))), []string{"Key", "Type", "TTL", "Size"}, rows, "Key", keysActions()...), nil
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
	return ui.Connected(viewInfo, "Redis Server Info", s.baseInfo(""), []string{"Property", "Value"}, rows, "Property"), nil
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
	rows = pluginrpc.EnsureRows(rows, []string{"-", "-", "-", "No slowlog entries", "-"})
	return ui.Connected(viewSlowlog, "Redis Slowlog", s.baseInfo(""), []string{"ID", "Timestamp", "Duration", "Command", "Client"}, rows, "ID"), nil
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
	return ui.Connected(viewStats, "Redis Stats", s.baseInfo(""), []string{"Metric", "Value"}, rows, "Metric"), nil
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
	rows = pluginrpc.EnsureRows(rows, []string{"-", "-", "-", "-", "-", "-", "-", "No clients"})
	return ui.Connected(viewClients, "Redis Clients", s.baseInfo(""), []string{"ID", "Addr", "Name", "Age", "Idle", "Flags", "DB", "Cmd"}, rows, "ID"), nil
}

func (s *Service) viewConfigLocked() (pluginrpc.ViewData, error) {
	config, err := s.client.GetConfig("*")
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := pluginrpc.EnsureRows(pluginrpc.SortedKVRows(config), []string{"-", "No config entries"})
	return ui.Connected(viewConfig, "Redis Config", s.baseInfo(""), []string{"Config", "Value"}, rows, "Config"), nil
}

func (s *Service) viewMemoryLocked() (pluginrpc.ViewData, error) {
	stats, err := s.client.GetMemoryStats()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := pluginrpc.SortedKVRows(stats)
	return ui.Connected(viewMemory, "Redis Memory", s.baseInfo(""), []string{"Metric", "Value"}, rows, "Metric", memoryActions()...), nil
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
	rows := pluginrpc.SortedKVRows(infoMap)
	return ui.Connected(viewID, title, s.baseInfo(""), []string{"Property", "Value"}, rows, "Property"), nil
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
	rows = pluginrpc.EnsureRows(rows, []string{"-", "0", "No active channels"})
	return ui.Connected(viewPubSub, "Redis PubSub", s.baseInfo("Enter=peek messages"), []string{"Channel", "Subscribers", "Type"}, rows, "Channel", pubsubActions()...), nil
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
		sampleStr := pluginrpc.Truncate(strings.Join(p.SampleKeys, ", "), 50)
		rows = append(rows, []string{
			p.Pattern,
			fmt.Sprintf("%d", p.Count),
			strings.Join(types, ", "),
			ttlStr,
			sampleStr,
		})
	}
	rows = pluginrpc.EnsureRows(rows, []string{"-", "0", "-", "-", "No keys found"})
	return ui.Connected(viewKeyAnalysis, "Redis Key Analysis", s.baseInfo(""), []string{"Pattern", "Count", "Types", "Avg TTL", "Sample Keys"}, rows, "Pattern"), nil
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
	rows = pluginrpc.EnsureRows(rows, []string{"-", "0", "0", "No databases with keys"})
	return ui.Connected(viewDatabases, "Redis Databases", s.baseInfo(""), []string{"DB", "Keys", "Expires", "Avg TTL"}, rows, "DB", databasesActions()...), nil
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
	rows = pluginrpc.EnsureRows(rows, []string{"-", "0", "0", "No command stats"})
	return ui.Connected(viewCmdStats, "Redis Command Stats", s.baseInfo(""), []string{"Command", "Calls", "Total Time (μs)", "Avg Time (μs)"}, rows, "Command"), nil
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
	rows = pluginrpc.EnsureRows(rows, []string{"-", "-", "No latency events recorded"})
	return ui.Connected(viewLatency, "Redis Latency", s.baseInfo(""), []string{"Event", "Timestamp", "Latency (ms)"}, rows, "Event"), nil
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

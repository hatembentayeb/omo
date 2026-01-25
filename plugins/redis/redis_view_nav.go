package main

import (
	"omo/pkg/ui"
)

const (
	viewRoot       = "redis"
	viewKeys       = "keys"
	viewInfo       = "info"
	viewSlowlog    = "slowlog"
	viewStats      = "stats"
	viewClients    = "clients"
	viewConfig     = "config"
	viewMemory     = "memory"
	viewRepl       = "replication"
	viewPersist    = "persistence"
	viewPubSub     = "pubsub"
	viewKeyAnalysis = "keyanalysis"
	viewDatabases  = "databases"
	viewCmdStats   = "commandstats"
	viewLatency    = "latency"
)

func (rv *RedisView) currentCores() *ui.Cores {
	switch rv.currentView {
	case viewInfo:
		return rv.infoView
	case viewSlowlog:
		return rv.slowlogView
	case viewStats:
		return rv.statsView
	case viewClients:
		return rv.clientsView
	case viewConfig:
		return rv.configView
	case viewMemory:
		return rv.memoryView
	case viewRepl:
		return rv.replicationView
	case viewPersist:
		return rv.persistenceView
	case viewPubSub:
		return rv.pubsubView
	case viewKeyAnalysis:
		return rv.keyAnalysisView
	case viewDatabases:
		return rv.databasesView
	case viewCmdStats:
		return rv.commandStatsView
	case viewLatency:
		return rv.latencyView
	default:
		return rv.keysView
	}
}

func (rv *RedisView) setViewStack(cores *ui.Cores, viewName string) {
	if cores == nil {
		return
	}

	stack := []string{viewRoot, viewKeys}
	if viewName != viewKeys {
		stack = append(stack, viewName)
	}
	cores.SetViewStack(stack)
}

func (rv *RedisView) switchView(viewName string) {
	pageName := "redis-" + viewName
	rv.currentView = viewName
	rv.viewPages.SwitchToPage(pageName)

	rv.setViewStack(rv.currentCores(), viewName)
	rv.refresh()
	current := rv.currentCores()
	if current != nil {
		rv.app.SetFocus(current.GetTable())
	}
}

func (rv *RedisView) showServerInfo() {
	rv.switchView(viewInfo)
}

func (rv *RedisView) showSlowlog() {
	rv.switchView(viewSlowlog)
}

func (rv *RedisView) showStats() {
	rv.switchView(viewStats)
}

func (rv *RedisView) showClients() {
	rv.switchView(viewClients)
}

func (rv *RedisView) showConfig() {
	rv.switchView(viewConfig)
}

func (rv *RedisView) showMemory() {
	rv.switchView(viewMemory)
}

func (rv *RedisView) showReplication() {
	rv.switchView(viewRepl)
}

func (rv *RedisView) showPersistence() {
	rv.switchView(viewPersist)
}

func (rv *RedisView) showKeys() {
	rv.switchView(viewKeys)
}

func (rv *RedisView) showPubSub() {
	rv.switchView(viewPubSub)
}

func (rv *RedisView) showKeyAnalysis() {
	rv.switchView(viewKeyAnalysis)
}

func (rv *RedisView) showDatabases() {
	rv.switchView(viewDatabases)
}

func (rv *RedisView) showCommandStats() {
	rv.switchView(viewCmdStats)
}

func (rv *RedisView) showLatency() {
	rv.switchView(viewLatency)
}


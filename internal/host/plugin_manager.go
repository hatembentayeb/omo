package host

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"omo/pkg/pluginapi"
	"omo/pkg/pluginrpc"

	goplugin "github.com/hashicorp/go-plugin"
	"github.com/rivo/tview"
)

// ConnState is the lifecycle of a keep-warm RPC plugin session.
type ConnState int

const (
	ConnRunning ConnState = iota
	ConnPaused
)

// PluginSession is one lazy-connected, keep-warm RPC plugin.
type PluginSession struct {
	Name     string
	BinPath  string
	Client   *goplugin.Client
	Plugin   pluginrpc.Plugin
	State    ConnState
	Cached   *pluginrpc.ViewData
	Renderer *RPCRenderer
	LastUsed time.Time
	loading  bool
}

// PluginManager tracks per-plugin RPC connections (pattern 2: lazy-connect, keep warm).
type PluginManager struct {
	mu       sync.Mutex
	app      *tview.Application
	pages    *tview.Pages
	sessions map[string]*PluginSession
	active   string
	logFn    func(string, ...interface{})
}

func newPluginManager(app *tview.Application, pages *tview.Pages, logFn func(string, ...interface{})) *PluginManager {
	_ = pluginrpc.OpenRPCLog("rpc-host")
	pluginrpc.RPCLog("PluginManager created")
	return &PluginManager{
		app:      app,
		pages:    pages,
		sessions: make(map[string]*PluginSession),
		logFn:    logFn,
	}
}

func (m *PluginManager) setSecrets(pluginapi.SecretsProvider) {
	// Secrets stay in-process on the host; plugins receive Configure settings.
}

// Activate returns a loading UI immediately; RPC work runs asynchronously.
func (m *PluginManager) Activate(name, binPath string) (tview.Primitive, error) {
	pluginrpc.RPCLog("Activate sync begin name=%s bin=%s", name, binPath)
	start := time.Now()

	m.mu.Lock()

	if m.active != "" && m.active != name {
		if prev, ok := m.sessions[m.active]; ok && prev.State == ConnRunning {
			prev.State = ConnPaused
			prev.LastUsed = time.Now()
			pluginrpc.RPCLog("paused %s", m.active)
		}
	}

	sess, ok := m.sessions[name]
	if !ok {
		sess = &PluginSession{Name: name, BinPath: binPath, State: ConnPaused}
		m.sessions[name] = sess
	}

	alreadyLoading := sess.loading
	if !alreadyLoading {
		sess.loading = true
	}
	renderer := sess.Renderer
	needRenderer := renderer == nil
	sess.State = ConnRunning
	sess.LastUsed = time.Now()
	m.active = name
	m.mu.Unlock() // release before tview work; never QueueUpdateDraw from SelectedFunc

	if needRenderer {
		pluginrpc.RPCLog("creating RPCRenderer for %s", name)
		renderer = NewRPCRenderer(m.app, m.pages, name, nil)
		m.mu.Lock()
		if m.sessions[name] == sess {
			sess.Renderer = renderer
		}
		m.mu.Unlock()
	}

	// Do NOT call Apply/Log/SetFocus on this path — Activate runs inside
	// tview's SetSelectedFunc and any QueueUpdateDraw or table.Select can deadlock.
	pluginrpc.RPCLog("Activate: mount empty shell (no sync Apply)")
	renderer.ShowLoading(name)
	prim := renderer.Primitive()
	pluginrpc.RPCLog("Activate sync done in %s (loading UI mounted)", time.Since(start))

	if !alreadyLoading {
		go m.activateAsync(name, binPath)
	} else {
		pluginrpc.RPCLog("Activate: async already in flight for %s", name)
	}
	return prim, nil
}

func (m *PluginManager) activateAsync(name, binPath string) {
	defer func() {
		m.mu.Lock()
		if sess := m.sessions[name]; sess != nil {
			sess.loading = false
		}
		m.mu.Unlock()
		if r := recover(); r != nil {
			pluginrpc.RPCLog("activateAsync PANIC: %v", r)
			m.failSession(name, fmt.Errorf("panic: %v", r))
		}
	}()

	pluginrpc.RPCLog("activateAsync START name=%s", name)
	total := time.Now()

	m.mu.Lock()
	sess := m.sessions[name]
	m.mu.Unlock()
	if sess == nil {
		pluginrpc.RPCLog("activateAsync: session gone")
		return
	}

	if sess.Client == nil {
		pluginrpc.RPCLog("activateAsync: Launch …")
		t0 := time.Now()
		type launchResult struct {
			client *goplugin.Client
			plugin pluginrpc.Plugin
			err    error
		}
		ch := make(chan launchResult, 1)
		go func() {
			c, p, err := pluginrpc.Launch(binPath)
			ch <- launchResult{c, p, err}
		}()

		var lr launchResult
		select {
		case lr = <-ch:
		case <-time.After(20 * time.Second):
			pluginrpc.RPCLog("activateAsync: Launch TIMEOUT after 20s")
			m.failSession(name, fmt.Errorf("launch timed out after 20s — see ~/.omo/logs/rpc-host.log"))
			return
		}
		pluginrpc.RPCLog("activateAsync: Launch finished in %s err=%v", time.Since(t0), lr.err)
		if lr.err != nil {
			m.failSession(name, fmt.Errorf("launch: %w", lr.err))
			return
		}

		m.mu.Lock()
		if m.sessions[name] != sess {
			lr.client.Kill()
			m.mu.Unlock()
			pluginrpc.RPCLog("activateAsync: session replaced during launch")
			return
		}
		sess.Client = lr.client
		sess.Plugin = lr.plugin
		if sess.Renderer != nil {
			sess.Renderer.SetPlugin(lr.plugin)
		}
		m.mu.Unlock()
	}

	pluginrpc.RPCLog("activateAsync: GetMetadata …")
	meta, err := withTimeout(10*time.Second, func() (pluginapi.PluginMetadata, error) {
		return sess.Plugin.GetMetadata()
	})
	if err != nil {
		m.failSession(name, fmt.Errorf("metadata: %w", err))
		return
	}
	pluginrpc.RPCLog("activateAsync: metadata OK name=%s ver=%s", meta.Name, meta.Version)

	pluginrpc.RPCLog("activateAsync: resolvePluginConfig …")
	t0 := time.Now()
	cfg, cfgErr := resolvePluginConfig(name)
	pluginrpc.RPCLog("activateAsync: resolvePluginConfig done in %s err=%v cfg_host=%s", time.Since(t0), cfgErr, cfg["host"])
	if cfgErr != nil {
		pluginrpc.RPCLog("activateAsync: config warn: %v", cfgErr)
	} else {
		pluginrpc.RPCLog("activateAsync: Configure …")
		if err := sess.Plugin.Configure(pluginrpc.ConfigureRequest{Settings: cfg}); err != nil {
			m.failSession(name, fmt.Errorf("configure: %w", err))
			return
		}
	}

	pluginrpc.RPCLog("activateAsync: GetView …")
	t0 = time.Now()
	view, err := withTimeout(30*time.Second, func() (pluginrpc.ViewData, error) {
		return sess.Plugin.GetView(pluginrpc.ViewRequest{})
	})
	pluginrpc.RPCLog("activateAsync: GetView done in %s err=%v status=%q rows=%d", time.Since(t0), err, view.Status, len(view.Rows))
	if err != nil {
		m.failSession(name, fmt.Errorf("get view: %w", err))
		return
	}

	m.mu.Lock()
	if m.sessions[name] != sess || m.active != name {
		m.mu.Unlock()
		pluginrpc.RPCLog("activateAsync: no longer active, skip Apply")
		return
	}
	sess.Cached = &view
	renderer := sess.Renderer
	m.mu.Unlock()

	pluginrpc.RPCLog("activateAsync: QueueUpdateDraw Apply …")
	m.app.QueueUpdateDraw(func() {
		pluginrpc.RPCLog("activateAsync: Apply on UI thread")
		if renderer != nil {
			renderer.Apply(view)
			renderer.FocusTable()
		}
		pluginrpc.RPCLog("activateAsync: Apply done")
	})
	pluginrpc.RPCLog("activateAsync SUCCESS total=%s", time.Since(total))
	m.log("activated RPC plugin %s %s", name, meta.Version)
}

func withTimeout[T any](d time.Duration, fn func() (T, error)) (T, error) {
	type result struct {
		v   T
		err error
	}
	ch := make(chan result, 1)
	go func() {
		v, err := fn()
		ch <- result{v, err}
	}()
	select {
	case r := <-ch:
		return r.v, r.err
	case <-time.After(d):
		var zero T
		return zero, fmt.Errorf("timed out after %s", d)
	}
}

func (m *PluginManager) failSession(name string, err error) {
	pluginrpc.RPCLog("failSession %s: %v", name, err)
	m.log("RPC plugin %s failed: %v", name, err)
	m.mu.Lock()
	sess := m.sessions[name]
	var renderer *RPCRenderer
	if sess != nil {
		renderer = sess.Renderer
		sess.loading = false
	}
	m.mu.Unlock()

	view := pluginrpc.ViewData{
		Title:   name,
		Info:    fmt.Sprintf("[red]%s[white]\nStatus: Error\n%v\n\nSee ~/.omo/logs/rpc-host.log", name, err),
		Status:  "error",
		Headers: []string{"Key", "Type", "TTL", "Size"},
		KeyBindings: []pluginrpc.KeyBinding{
			{Key: "R", Label: "Refresh", Action: "refresh"},
		},
		LogLines: []string{fmt.Sprintf("[red]%v", err)},
	}
	m.app.QueueUpdateDraw(func() {
		if renderer != nil {
			renderer.Apply(view)
		}
	})
}

func (m *PluginManager) PauseActive() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.active == "" {
		return
	}
	if sess, ok := m.sessions[m.active]; ok {
		sess.State = ConnPaused
		sess.LastUsed = time.Now()
		pluginrpc.RPCLog("PauseActive %s", m.active)
	}
	m.active = ""
}

func (m *PluginManager) Kill(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.killLocked(name)
}

func (m *PluginManager) killLocked(name string) {
	sess, ok := m.sessions[name]
	if !ok {
		return
	}
	pluginrpc.RPCLog("kill %s", name)
	if sess.Plugin != nil {
		_ = sess.Plugin.Stop()
	}
	if sess.Client != nil {
		sess.Client.Kill()
	}
	if sess.Renderer != nil {
		sess.Renderer.Destroy()
	}
	delete(m.sessions, name)
	if m.active == name {
		m.active = ""
	}
}

func (m *PluginManager) KillAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for name := range m.sessions {
		m.killLocked(name)
	}
}

func (m *PluginManager) log(format string, args ...interface{}) {
	if m.logFn != nil {
		m.logFn(format, args...)
	}
}

// FocusActive focuses the active RPC plugin table. Returns true if focused.
func (m *PluginManager) FocusActive() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.active == "" {
		return false
	}
	sess := m.sessions[m.active]
	if sess == nil || sess.Renderer == nil || sess.State != ConnRunning {
		return false
	}
	sess.Renderer.FocusTable()
	return true
}

func resolvePluginConfig(pluginName string) (map[string]string, error) {
	pluginrpc.RPCLog("resolvePluginConfig %s", pluginName)
	if !pluginapi.HasSecrets() {
		return nil, fmt.Errorf("secrets unavailable")
	}

	// Prefer the seeded local path; fall back to first non-reference entry.
	preferred := pluginName + "/development/local"
	if entry, err := pluginapi.Secrets().Get(preferred); err == nil && entry != nil && !pluginapi.IsReferenceEntry(entry) {
		pluginrpc.RPCLog("resolvePluginConfig: using preferred %s host=%s user=%s", preferred, entry.URL, entry.UserName)
		return entryToSettings(entry), nil
	}

	paths, err := pluginapi.Secrets().List(pluginName)
	if err != nil {
		return nil, err
	}
	pluginrpc.RPCLog("resolvePluginConfig: listed %d paths", len(paths))

	seen := map[string]bool{}
	for _, p := range paths {
		if seen[p] {
			continue
		}
		seen[p] = true
		if strings.Count(p, "/") < 2 {
			continue
		}
		entry, err := pluginapi.Secrets().Get(p)
		if err != nil || pluginapi.IsReferenceEntry(entry) {
			continue
		}
		pluginrpc.RPCLog("resolvePluginConfig: using %s host=%s user=%s", p, entry.URL, entry.UserName)
		return entryToSettings(entry), nil
	}
	return nil, fmt.Errorf("no KeePass entries under %s/", pluginName)
}

func entryToSettings(entry *pluginapi.SecretEntry) map[string]string {
	settings := map[string]string{
		"name":     entry.Title,
		"host":     entry.URL,
		"url":      entry.URL,
		"username": entry.UserName,
		"password": entry.Password,
		"notes":    entry.Notes,
	}
	if entry.CustomAttributes != nil {
		for k, v := range entry.CustomAttributes {
			settings[k] = v
		}
	}
	// Pass attributes through as-is. Each plugin's Configure applies its own
	// defaults (redis db index, postgres db name, rabbitmq ports, etc.).
	return settings
}

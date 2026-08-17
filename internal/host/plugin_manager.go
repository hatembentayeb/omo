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
	Name       string
	BinPath    string
	Client     *goplugin.Client
	Plugin     pluginrpc.Plugin
	Configured bool
	State      ConnState
	Cached     *pluginrpc.ViewData
	Dashboard  *pluginrpc.ViewData
	Renderer   *RPCRenderer
	LastUsed   time.Time
	LastError  string
	loading    bool
	pulseMu    sync.Mutex
}

// PluginManager tracks per-plugin RPC connections (pattern 2: lazy-connect, keep warm).
type PluginManager struct {
	mu        sync.Mutex
	app       *tview.Application
	pages     *tview.Pages
	sessions  map[string]*PluginSession
	active    string
	logFn     func(string, ...interface{})
	onActions func([]pluginrpc.KeyBinding, func(string))
	onMood    func(phase string, ok bool, action, reaction string)
	onHome    func()
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

// SetActionsHook wires sidebar updates when a plugin view is applied.
func (m *PluginManager) SetActionsHook(fn func([]pluginrpc.KeyBinding, func(string))) {
	m.onActions = fn
}

// SetMoodHook wires the top-left logo reaction flashes for plugin actions.
func (m *PluginManager) SetMoodHook(fn func(phase string, ok bool, action, reaction string)) {
	m.onMood = fn
}

// SetHomeHook wires ESC from a plugin's home view back to the dashboard.
func (m *PluginManager) SetHomeHook(fn func()) {
	m.onHome = fn
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
		renderer.SetActionsHook(m.onActions)
		renderer.SetMoodHook(m.onMood)
		renderer.SetHomeHook(m.onHome)
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
	sess.pulseMu.Lock()
	defer sess.pulseMu.Unlock()

	warm := sess.Client != nil
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
	} else if warm && sess.Configured {
		// Keep-warm sessions (e.g. k8sportforward tunnels) must not be reconfigured
		// on every sidebar click — Configure often resets plugin state.
		// Ctrl+t target switch still calls Configure directly via applyTarget.
		pluginrpc.RPCLog("activateAsync: skip Configure (warm session)")
	} else {
		pluginrpc.RPCLog("activateAsync: Configure …")
		if err := sess.Plugin.Configure(pluginrpc.ConfigureRequest{Settings: cfg}); err != nil {
			m.failSession(name, fmt.Errorf("configure: %w", err))
			return
		}
		sess.Configured = true
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
	sess.LastError = ""
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

// DashboardSnapshot returns a live, compact plugin summary without making the
// plugin active. Calls for one plugin are serialized with normal activation.
func (m *PluginManager) DashboardSnapshot(name, binPath string) pluginrpc.ViewData {
	m.mu.Lock()
	sess := m.sessions[name]
	if sess == nil {
		sess = &PluginSession{Name: name, BinPath: binPath, State: ConnPaused}
		m.sessions[name] = sess
	}
	m.mu.Unlock()

	sess.pulseMu.Lock()
	defer sess.pulseMu.Unlock()

	if sess.Plugin == nil {
		type launchResult struct {
			client *goplugin.Client
			plugin pluginrpc.Plugin
			err    error
		}
		ch := make(chan launchResult, 1)
		go func() {
			client, p, err := pluginrpc.Launch(binPath)
			ch <- launchResult{client: client, plugin: p, err: err}
		}()

		select {
		case result := <-ch:
			if result.err != nil {
				return m.dashboardError(name, "launch", result.err)
			}
			m.mu.Lock()
			if m.sessions[name] != sess {
				m.mu.Unlock()
				result.client.Kill()
				return m.dashboardError(name, "launch", fmt.Errorf("session replaced"))
			}
			sess.Client = result.client
			sess.Plugin = result.plugin
			m.mu.Unlock()
		case <-time.After(8 * time.Second):
			// Launch may finish after this deadline; clean that process up.
			go func() {
				result := <-ch
				if result.client != nil {
					result.client.Kill()
				}
			}()
			return m.dashboardError(name, "launch", fmt.Errorf("timed out after 8s"))
		}
	}

	if !sess.Configured {
		cfg, err := resolvePluginConfigWithReload(name, false)
		if err != nil {
			// Config-free plugins (for example system process inspection) can
			// still provide a live widget. Required-config plugins reject this
			// empty Configure and become a clear not-configured tile.
			cfg = map[string]string{}
		}
		if err := sess.Plugin.Configure(pluginrpc.ConfigureRequest{Settings: cfg}); err != nil {
			return m.dashboardStatus(name, "not configured", err.Error())
		}
		sess.Configured = true
	}

	view, err := withTimeout(8*time.Second, func() (pluginrpc.ViewData, error) {
		return sess.Plugin.GetView(pluginrpc.ViewRequest{View: pluginrpc.DashboardView})
	})
	if err != nil {
		return m.dashboardError(name, "widget", err)
	}
	if view.View != "" && view.View != pluginrpc.DashboardView {
		// Legacy plugins usually route unknown views to their default table.
		// Use that live result as a generic widget, then restore its real view so
		// opening the plugin later does not inherit "dashboard" as currentView.
		restoredView := view.View
		_, _ = withTimeout(3*time.Second, func() (pluginrpc.ViewData, error) {
			return sess.Plugin.GetView(pluginrpc.ViewRequest{View: restoredView})
		})
		status := view.Status
		if status == "" {
			status = "connected"
		}
		view = pluginrpc.Widget(name, status, view.Info, [][2]string{
			{"View", firstDashboardValue(view.Title, restoredView)},
			{"Rows", fmt.Sprintf("%d", len(view.Rows))},
		})
	}
	if view.Title == "" {
		view.Title = name
	}
	if view.Status == "" {
		view.Status = "connected"
	}
	view.View = pluginrpc.DashboardView
	view.ViewBindings = nil
	view.KeyBindings = nil
	view.Actions = nil
	view.HelpSections = nil
	if len(view.Rows) > 4 {
		view.Rows = view.Rows[:4]
	}

	m.mu.Lock()
	sess.Dashboard = &view
	sess.LastError = ""
	sess.LastUsed = time.Now()
	m.mu.Unlock()
	return view
}

func firstDashboardValue(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return "-"
}

func (m *PluginManager) dashboardError(name, phase string, err error) pluginrpc.ViewData {
	detail := phase + ": " + err.Error()
	m.mu.Lock()
	if sess := m.sessions[name]; sess != nil {
		sess.LastError = detail
	}
	m.mu.Unlock()
	return m.dashboardStatus(name, "error", detail)
}

func (m *PluginManager) dashboardStatus(name, status, detail string) pluginrpc.ViewData {
	return pluginrpc.Widget(name, status, "", [][2]string{
		{"Status", status},
		{"Detail", pluginrpc.Truncate(detail, 60)},
	})
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

// ActivePrimitive returns the mounted UI for the active running plugin, if any.
func (m *PluginManager) ActivePrimitive() tview.Primitive {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.active == "" {
		return nil
	}
	sess := m.sessions[m.active]
	if sess == nil || sess.Renderer == nil || sess.State != ConnRunning {
		return nil
	}
	return sess.Renderer.Primitive()
}

func resolvePluginConfig(pluginName string) (map[string]string, error) {
	return resolvePluginConfigWithReload(pluginName, true)
}

// ReloadSecrets refreshes KeePass once before a multi-plugin dashboard pulse.
func (m *PluginManager) ReloadSecrets() {
	if !pluginapi.HasSecrets() {
		return
	}
	if err := pluginapi.Secrets().Reload(); err != nil {
		pluginrpc.RPCLog("ReloadSecrets warn: %v", err)
	}
}

func resolvePluginConfigWithReload(pluginName string, reload bool) (map[string]string, error) {
	pluginrpc.RPCLog("resolvePluginConfig %s reload=%v", pluginName, reload)
	if !pluginapi.HasSecrets() {
		return nil, fmt.Errorf("secrets unavailable")
	}
	if reload {
		if err := pluginapi.Secrets().Reload(); err != nil {
			pluginrpc.RPCLog("resolvePluginConfig: reload warn: %v", err)
		}
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

package host

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"omo/internal/packagemanager"
	"omo/internal/registry"
	"omo/internal/settings"
	"omo/pkg/pluginapi"
	"omo/pkg/pluginrpc"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type Host struct {
	App             *tview.Application
	Pages           *tview.Pages
	MainFrame       *tview.Frame
	MainUI          *tview.Grid
	PluginsList     *tview.List
	ActionsList     *tview.List
	Logo            *LogoMood
	activePluginIdx int
	rpcManager      *PluginManager
	dashboard       *Dashboard
	PluginsDir      string
	logger          *pluginapi.Logger
	version         string
	splashOnce      sync.Once
}

func New(app *tview.Application, pages *tview.Pages, logger *pluginapi.Logger, version string) *Host {
	mainFrame := tview.NewFrame(nil)
	mainFrame.SetBackgroundColor(tcell.ColorDefault)
	// NewFrame defaults to 1-cell margins; keep content flush with the grid border.
	mainFrame.SetBorders(0, 0, 0, 0, 0, 0)
	mainFrame.SetBorderPadding(0, 0, 0, 0)

	mainUI := tview.NewGrid()
	mainUI.SetBackgroundColor(tcell.ColorDefault)

	h := &Host{
		App:             app,
		Pages:           pages,
		MainFrame:       mainFrame,
		MainUI:          mainUI,
		PluginsList:     tview.NewList(),
		Logo:            newLogoMood(app, version),
		activePluginIdx: -1,
		PluginsDir:      pluginapi.PluginsDir(),
		logger:          logger,
		version:         version,
	}
	h.rpcManager = newPluginManager(app, pages, h.log)
	h.rpcManager.SetActionsHook(h.UpdatePluginActions)
	h.rpcManager.SetMoodHook(h.FlashLogo)
	h.rpcManager.SetHomeHook(h.OpenDashboard)
	return h
}

// Shutdown stops all RPC plugin processes (and anything they own, e.g. port-forwards).
// Safe to call multiple times.
func (h *Host) Shutdown() {
	if h == nil || h.rpcManager == nil {
		return
	}
	h.log("shutting down RPC plugins")
	h.rpcManager.KillAll()
}

func (h *Host) log(format string, args ...interface{}) {
	if h.logger != nil {
		h.logger.Info(format, args...)
	}
}

func (h *Host) LoadPlugins() *tview.List {
	list, err := discoverPlugins(h.PluginsDir)
	if err != nil || list == nil {
		list = tview.NewList().ShowSecondaryText(false)
		list.SetMainTextColor(tcell.ColorPurple)
		list.SetBackgroundColor(tcell.ColorDefault)
		stylePluginList(list)
		list.AddItem("No plugins found", "", '0', nil)
		h.log("no plugins found in %s", h.PluginsDir)
	}

	list.SetSelectedFunc(func(i int, s1, s2 string, r rune) {
		if s2 == "" {
			return
		}
		h.activateRPC(list, i, s2)
	})
	stylePluginList(list)

	h.PluginsList = list
	return list
}

func (h *Host) markActive(list *tview.List, i int) string {
	if h.activePluginIdx >= 0 && h.activePluginIdx < list.GetItemCount() {
		prevMain, prevSec := list.GetItemText(h.activePluginIdx)
		prevName := stripPluginPrefix(prevMain)
		list.SetItemText(h.activePluginIdx, "  → "+prevName, prevSec)
	}

	curMain, curSec := list.GetItemText(i)
	curName := stripPluginPrefix(curMain)
	list.SetItemText(i, "  ● "+curName, curSec)
	h.activePluginIdx = i
	return curName
}

func (h *Host) activateRPC(list *tview.List, i int, binPath string) {
	pluginrpc.RPCLog("host.activateRPC begin bin=%s", binPath)
	h.dashboard = nil

	curName := h.markActive(list, i)

	pluginLogger, err := pluginapi.NewLogger(curName)
	if err != nil {
		h.log("failed to create logger for %s: %v", curName, err)
	}
	pluginapi.SetPluginLogger(pluginLogger)

	component, err := h.rpcManager.Activate(curName, binPath)
	if err != nil {
		pluginrpc.RPCLog("host.activateRPC Activate err=%v", err)
		h.log("failed to activate RPC plugin %s: %v", binPath, err)
		h.showPluginLoadError("Failed to activate RPC plugin", err)
		return
	}
	registry.RegisterPlugin(curName, pluginapi.PluginMetadata{Name: curName})
	h.MainFrame.SetPrimitive(component)
	pluginrpc.RPCLog("host.activateRPC mounted loading UI for %s", curName)
}

func (h *Host) activateInstalled(entry installedPlugin) {
	for i := 0; i < h.PluginsList.GetItemCount(); i++ {
		main, secondary := h.PluginsList.GetItemText(i)
		if stripPluginPrefix(main) == entry.Name && secondary == entry.BinPath {
			h.activateRPC(h.PluginsList, i, entry.BinPath)
			return
		}
	}
	// List may be stale after refresh; still open by path.
	h.dashboard = nil
	pluginLogger, err := pluginapi.NewLogger(entry.Name)
	if err != nil {
		h.log("failed to create logger for %s: %v", entry.Name, err)
	}
	pluginapi.SetPluginLogger(pluginLogger)
	component, err := h.rpcManager.Activate(entry.Name, entry.BinPath)
	if err != nil {
		h.showPluginLoadError("Failed to activate RPC plugin", err)
		return
	}
	registry.RegisterPlugin(entry.Name, pluginapi.PluginMetadata{Name: entry.Name})
	h.MainFrame.SetPrimitive(component)
}

// FocusPluginContent moves focus into the active plugin view (RPC table or main frame).
func (h *Host) FocusPluginContent() {
	if h.dashboard != nil {
		h.dashboard.Focus()
		return
	}
	if h.rpcManager != nil && h.rpcManager.FocusActive() {
		return
	}
	if mc := h.MainFrame.GetPrimitive(); mc != nil {
		h.App.SetFocus(mc)
	}
}

// SelectTarget opens the connection/instance picker for the active RPC plugin (Ctrl+t).
func (h *Host) SelectTarget() {
	if h.rpcManager == nil {
		return
	}
	h.rpcManager.ShowTargetSelector()
}

// LogoView returns the top-left OMO mark (also used for action mood flashes).
func (h *Host) LogoView() tview.Primitive {
	if h.Logo == nil {
		h.Logo = newLogoMood(h.App, h.version)
	}
	return h.Logo.View()
}

// FlashLogo drives the sidebar logo reaction for pending/result action beats.
// phase: "pending" | "ok" | "fail" (empty treated as ok/fail via ok bool).
func (h *Host) FlashLogo(phase string, ok bool, action, reaction string) {
	if h == nil || h.Logo == nil {
		return
	}
	switch strings.ToLower(phase) {
	case "pending":
		h.Logo.FlashPending(action)
	default:
		h.Logo.FlashResult(ok, action, reaction)
	}
}

// ActionsView returns the host sidebar list (Refresh Plugins / Package Manager).
func (h *Host) ActionsView() *tview.List {
	list := tview.NewList().ShowSecondaryText(false)
	list.SetMainTextColor(tcell.ColorAqua)
	list.SetBackgroundColor(tcell.ColorDefault)
	stylePluginList(list)
	h.ActionsList = list
	h.resetHostActions()
	return list
}

// UpdatePluginActions is a no-op for the sidebar: host actions stay fixed.
// Per-view plugin actions are shown in the former logs column (Keys panel).
func (h *Host) UpdatePluginActions(_ []pluginrpc.KeyBinding, _ func(string)) {
	if h.ActionsList == nil {
		return
	}
	// Keep sidebar stable — do not swap in plugin actions.
	if h.ActionsList.GetItemCount() == 0 {
		h.resetHostActions()
	}
}

func (h *Host) resetHostActions() {
	if h.ActionsList == nil {
		return
	}
	h.ActionsList.Clear()
	h.ActionsList.AddItem("  ↻ Refresh Plugins", "", 0, func() { h.RefreshPlugins() })
	h.ActionsList.AddItem("  ⬡ Package Manager", "", 0, func() { h.OpenPackageManager() })
	h.ActionsList.AddItem("  ⚙ Settings / Info", "", 0, func() { h.OpenSettings() })
}

// RefreshPlugins reloads the plugins list.
func (h *Host) RefreshPlugins() {
	h.log("refreshing plugins")
	reopenDashboard := h.dashboard != nil
	h.MainUI.RemoveItem(h.PluginsList)
	h.PluginsList = h.LoadPlugins()
	h.MainUI.AddItem(h.PluginsList, 1, 0, 1, 1, 0, 0, true)
	if reopenDashboard {
		h.OpenDashboard()
		return
	}
	h.App.SetFocus(h.PluginsList)
}

const splashPage = "splash"
const splashHold = 1200 * time.Millisecond

// ShowStartupSplash covers the whole terminal with a centered OhMyOps mark,
// then reveals the normal host UI. Any key skips the wait.
func (h *Host) ShowStartupSplash() {
	if h == nil || h.Pages == nil {
		return
	}
	h.Pages.AddAndSwitchToPage(splashPage, Splash(), true)
	go func() {
		time.Sleep(splashHold)
		h.App.QueueUpdateDraw(func() {
			h.DismissSplash()
		})
	}()
}

// SplashVisible reports whether the full-screen startup mark is still showing.
func (h *Host) SplashVisible() bool {
	if h == nil || h.Pages == nil {
		return false
	}
	front, _ := h.Pages.GetFrontPage()
	return front == splashPage
}

// DismissSplash switches to the host UI. Safe to call more than once.
func (h *Host) DismissSplash() {
	if h == nil {
		return
	}
	h.splashOnce.Do(func() {
		if h.Pages != nil {
			h.Pages.SwitchToPage("main")
			h.Pages.RemovePage(splashPage)
		}
		if h.App != nil && h.PluginsList != nil {
			h.App.SetFocus(h.PluginsList)
		}
	})
}

// ShowCover mounts the branded splash and focuses its Enter-to-dashboard CTA.
func (h *Host) ShowCover() {
	h.dashboard = nil
	h.MainFrame.SetPrimitive(Cover(h.App, h.version, h.OpenDashboard))
	if primitive := h.MainFrame.GetPrimitive(); primitive != nil {
		h.App.SetFocus(primitive)
	}
}

// OpenDashboard shows live summaries for all installed RPC plugins.
func (h *Host) OpenDashboard() {
	entries, err := discoverPluginEntries(h.PluginsDir)
	if err != nil {
		h.showPluginLoadError("Failed to load dashboard", err)
		return
	}
	if h.rpcManager != nil {
		h.rpcManager.PauseActive()
	}
	h.dashboard = NewDashboard(
		h.App,
		h.rpcManager,
		entries,
		func(entry installedPlugin) { h.activateInstalled(entry) },
		h.ShowCover,
	)
	h.MainFrame.SetPrimitive(h.dashboard.Primitive())
	h.dashboard.Focus()
	h.dashboard.Refresh()
}

// OpenPackageManager shows the package manager UI.
func (h *Host) OpenPackageManager() {
	h.log("opening package manager")
	const pageID = "packageManager"
	if h.Pages.HasPage(pageID) {
		h.Pages.RemovePage(pageID)
	}

	var pm *packagemanager.Manager
	pm = packagemanager.New(h.App, h.Pages, func() {
		h.RefreshPlugins()
		h.restoreMainContent()
		if h.Pages.HasPage(pageID) {
			h.Pages.RemovePage(pageID)
		}
		if pm != nil {
			pm.Destroy()
		}
		h.resetHostActions()
		h.App.SetFocus(h.PluginsList)
	})

	h.Pages.AddPage(pageID, pm.GetLayout(), true, false)
	h.MainFrame.SetPrimitive(pm.GetLayout())
	h.App.SetFocus(pm.GetTable())
}

// OpenSettings shows host Settings / Info (~/.omo inventory and management).
func (h *Host) OpenSettings() {
	h.log("opening settings")
	const pageID = "settings"
	if h.Pages.HasPage(pageID) {
		h.Pages.RemovePage(pageID)
	}

	var sm *settings.Manager
	sm = settings.New(h.App, h.Pages, h.version, h.RefreshPlugins, func() {
		h.restoreMainContent()
		if h.Pages.HasPage(pageID) {
			h.Pages.RemovePage(pageID)
		}
		if sm != nil {
			sm.Destroy()
		}
		h.resetHostActions()
		h.App.SetFocus(h.PluginsList)
	})

	h.Pages.AddPage(pageID, sm.GetLayout(), true, false)
	h.MainFrame.SetPrimitive(sm.GetLayout())
	h.App.SetFocus(sm.GetTable())
}

func (h *Host) restoreMainContent() {
	if h.dashboard != nil {
		h.MainFrame.SetPrimitive(h.dashboard.Primitive())
		h.dashboard.Focus()
		return
	}
	if h.rpcManager != nil {
		if p := h.rpcManager.ActivePrimitive(); p != nil {
			h.MainFrame.SetPrimitive(p)
			h.rpcManager.FocusActive()
			return
		}
	}
	h.ShowCover()
}

type installedPlugin struct {
	Name    string
	BinPath string
}

func discoverPluginEntries(pluginsDir string) ([]installedPlugin, error) {
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return nil, err
	}
	out := make([]installedPlugin, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		binPath := filepath.Join(pluginsDir, name, name)
		if info, err := os.Stat(binPath); err == nil && isExecutable(info) {
			out = append(out, installedPlugin{Name: name, BinPath: binPath})
		}
	}
	return out, nil
}

func stylePluginList(list *tview.List) {
	if list == nil {
		return
	}
	list.SetHighlightFullLine(true)
	list.SetSelectedTextColor(tcell.ColorBlack)
	list.SetSelectedBackgroundColor(tcell.ColorAqua)
}

func discoverPlugins(pluginsDir string) (*tview.List, error) {
	entries, err := discoverPluginEntries(pluginsDir)
	if err != nil {
		return nil, err
	}

	list := tview.NewList().ShowSecondaryText(false)
	list.SetMainTextColor(tcell.ColorPurple)
	list.SetBackgroundColor(tcell.ColorDefault)
	stylePluginList(list)

	for _, entry := range entries {
		list.AddItem("  → "+entry.Name, entry.BinPath, 0, nil)
	}

	return list, nil
}

func isExecutable(info os.FileInfo) bool {
	return !info.IsDir() && info.Mode()&0111 != 0
}

func stripPluginPrefix(name string) string {
	for strings.Contains(name, "[") && strings.Contains(name, "]") {
		start := strings.Index(name, "[")
		end := strings.Index(name, "]")
		if start < end {
			name = name[:start] + name[end+1:]
		} else {
			break
		}
	}
	name = strings.TrimLeft(name, " →●")
	name = strings.TrimSpace(name)
	return name
}

func (h *Host) showPluginLoadError(title string, err error) {
	infoText := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetText(fmt.Sprintf("%s\n\n%v", title, err)).
		SetTextColor(tcell.ColorRed).
		SetBackgroundColor(tcell.ColorDefault).
		SetBorder(true).
		SetTitle(" Plugin Error ").
		SetTitleAlign(tview.AlignCenter)
	h.MainFrame.SetPrimitive(infoText)
}

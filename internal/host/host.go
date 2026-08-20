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
	"omo/pkg/ui"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type Host struct {
	App             *tview.Application
	Pages           *tview.Pages
	MainFrame       *tview.Frame
	HeaderFrame     *tview.Frame
	Body            *tview.Grid
	MainUI          *tview.Grid
	PluginsList     *tview.Table
	pluginEntries   []installedPlugin
	ActionsList     *tview.List
	Logo            *LogoMood
	Footer          *tview.Flex
	crumbBar        *tview.TextView
	chromeBar       *tview.TextView
	paneBar         *tview.TextView
	versionBar      *tview.TextView
	pluginsOn       bool
	overlayRestyle  func()
	activePluginIdx int
	rpcManager      *PluginManager
	dashboard       *Dashboard
	PluginsDir      string
	logger          *pluginapi.Logger
	version         string
	latestTag       string
	splashOnce      sync.Once
}

func New(app *tview.Application, pages *tview.Pages, logger *pluginapi.Logger, version string) *Host {
	ui.LoadSavedTheme()
	pluginrpc.SetInfoColors(ui.HexInfoKey, ui.HexValue)

	mainFrame := tview.NewFrame(nil)
	mainFrame.SetBackgroundColor(ui.ColorAppBg)
	// NewFrame defaults to 1-cell margins; keep content flush with the grid border.
	mainFrame.SetBorders(0, 0, 0, 0, 0, 0)
	mainFrame.SetBorderPadding(0, 0, 0, 0)

	mainUI := tview.NewGrid()
	mainUI.SetBackgroundColor(ui.ColorAppBg)

	headerFrame := tview.NewFrame(nil)
	headerFrame.SetBackgroundColor(ui.ColorAppBg)
	headerFrame.SetBorders(0, 0, 0, 0, 0, 0)
	headerFrame.SetBorderPadding(0, 0, 0, 0)

	body := tview.NewGrid()
	body.SetBackgroundColor(ui.ColorAppBg)
	body.SetBorders(true)
	body.SetBordersColor(ui.ColorBorder)
	body.SetRows(0).SetColumns(pluginsColWidth, 0)

	h := &Host{
		App:             app,
		Pages:           pages,
		MainFrame:       mainFrame,
		HeaderFrame:     headerFrame,
		Body:            body,
		MainUI:          mainUI,
		PluginsList:     tview.NewTable(),
		Logo:            newLogoMood(app),
		activePluginIdx: -1,
		PluginsDir:      pluginapi.PluginsDir(),
		logger:          logger,
		version:         version,
	}
	h.rpcManager = newPluginManager(app, pages, h.log)
	h.rpcManager.SetActionsHook(h.UpdatePluginActions)
	h.rpcManager.SetMoodHook(h.FlashLogo)
	h.rpcManager.SetHomeHook(h.OpenDashboard)
	h.rpcManager.SetLogo(h.Logo.View())
	h.rpcManager.SetBreadcrumbHook(h.SetCrumbs)
	h.rpcManager.SetHeaderHook(h.SetPluginHeader)
	go h.pollGitHubUpdate()
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

func (h *Host) LoadPlugins() *tview.Table {
	entries, err := discoverPluginEntries(h.PluginsDir)
	if err != nil {
		h.log("no plugins found in %s: %v", h.PluginsDir, err)
		entries = nil
	}
	if len(entries) == 0 {
		h.log("no plugins found in %s", h.PluginsDir)
	}

	var keep string
	if h.activePluginIdx >= 0 && h.activePluginIdx < len(h.pluginEntries) {
		keep = h.pluginEntries[h.activePluginIdx].Name
	}

	table := newPluginTable()
	table.SetSelectedFunc(func(row, column int) {
		if row <= 0 {
			return
		}
		idx := row - 1
		if idx < 0 || idx >= len(h.pluginEntries) {
			return
		}
		if h.pluginEntries[idx].BinPath == "" {
			return
		}
		h.activateRPC(idx, h.pluginEntries[idx].BinPath)
	})

	h.pluginEntries = entries
	fillPluginTable(table, entries)
	h.activePluginIdx = -1
	if keep != "" {
		for i, entry := range entries {
			if entry.Name == keep {
				h.activePluginIdx = i
				table.Select(i+1, 0)
				break
			}
		}
	}
	h.PluginsList = table
	h.wirePluginTable(table)
	return table
}

func (h *Host) markActive(i int) string {
	if i < 0 || i >= len(h.pluginEntries) {
		return ""
	}
	h.activePluginIdx = i
	return h.pluginEntries[i].Name
}

func (h *Host) activateRPC(i int, binPath string) {
	pluginrpc.RPCLog("host.activateRPC begin bin=%s", binPath)
	h.dashboard = nil

	h.overlayRestyle = func() {
		if h.rpcManager != nil {
			h.rpcManager.ApplyTheme()
		}
	}

	curName := h.markActive(i)

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
	for i, existing := range h.pluginEntries {
		if existing.Name == entry.Name && existing.BinPath == entry.BinPath {
			h.activateRPC(i, entry.BinPath)
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

// LogoView returns the OMO mark used in the plugin header (action mood flashes).
func (h *Host) LogoView() tview.Primitive {
	if h.Logo == nil {
		h.Logo = newLogoMood(h.App)
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

// pluginsColWidth is the plugins list column (matches Body grid). The
// pane-tabs slot under it is this plus the two vertical grid borders.
const pluginsColWidth = 20

// VersionColWidth is the status-row slot under the plugins list (column
// plus the two vertical grid borders so the tabs line up with the table).
const VersionColWidth = pluginsColWidth + 2

// StatusRowHeight is one row: Plugins/View tabs | crumbs | version | host actions.
const StatusRowHeight = 1

const versionMidWidth = 22

// FooterView sits under the table: breadcrumbs left (plugin trail), version
// in the middle, host actions pinned to the bottom-right.
func (h *Host) FooterView() tview.Primitive {
	h.versionBar = tview.NewTextView()
	h.versionBar.SetDynamicColors(true)
	h.versionBar.SetBackgroundColor(ui.ColorAppBg)
	h.versionBar.SetTextAlign(tview.AlignCenter)
	h.versionBar.SetWrap(false)

	h.chromeBar = tview.NewTextView()
	h.chromeBar.SetDynamicColors(true)
	h.chromeBar.SetBackgroundColor(ui.ColorAppBg)
	h.chromeBar.SetTextAlign(tview.AlignRight)
	h.chromeBar.SetWrap(false)

	h.crumbBar = tview.NewTextView()
	h.crumbBar.SetDynamicColors(true)
	h.crumbBar.SetTextAlign(tview.AlignLeft)
	h.crumbBar.SetTextStyle(tcell.StyleDefault)
	h.crumbBar.SetBackgroundColor(ui.ColorAppBg)

	h.Footer = tview.NewFlex()
	h.Footer.SetDirection(tview.FlexColumn)
	h.Footer.SetBackgroundColor(ui.ColorAppBg)
	h.Footer.AddItem(h.crumbBar, 0, 1, false).
		AddItem(h.versionBar, versionMidWidth, 0, false).
		AddItem(h.chromeBar, 0, 1, false)
	h.syncPaneChrome(h.pluginsOn)
	return h.Footer
}

// SetCrumbs updates the host footer breadcrumb trail.
func (h *Host) SetCrumbs(text string) {
	if h == nil || h.crumbBar == nil {
		return
	}
	h.crumbBar.SetText(text)
}

// SetPluginHeader mounts a full-width header (Info | Views | Actions | Logo).
// A nil primitive shows the logo alone on the right (cover / dashboard).
func (h *Host) SetPluginHeader(p tview.Primitive) {
	if h == nil || h.HeaderFrame == nil {
		return
	}
	if p == nil {
		h.HeaderFrame.SetPrimitive(h.logoBar())
		return
	}
	h.HeaderFrame.SetPrimitive(p)
}

// ShowHomeHeader puts the OMO mark on the right with an empty left span.
func (h *Host) ShowHomeHeader() {
	if h.rpcManager != nil {
		h.rpcManager.ReleaseLogo()
	}
	h.SetPluginHeader(nil)
}

func (h *Host) logoBar() tview.Primitive {
	bar := tview.NewFlex()
	bar.SetDirection(tview.FlexColumn)
	bar.SetBackgroundColor(ui.ColorAppBg)
	empty := tview.NewBox().SetBackgroundColor(ui.ColorAppBg)
	slot := tview.NewFlex()
	slot.SetDirection(tview.FlexRow)
	slot.SetBackgroundColor(ui.ColorAppBg)
	pad := tview.NewBox().SetBackgroundColor(ui.ColorAppBg)
	slot.AddItem(pad, 0, 1, false).
		AddItem(h.LogoView(), 2, 0, false).
		AddItem(tview.NewBox().SetBackgroundColor(ui.ColorAppBg), 0, 1, false)
	bar.AddItem(empty, 0, 1, false)
	bar.AddItem(slot, ui.HeaderLogoWidth, 0, false)
	return bar
}

// ActionsView returns the host actions list (kept for package manager close reset).
func (h *Host) ActionsView() *tview.List {
	list := tview.NewList().ShowSecondaryText(false)
	list.SetMainTextColor(ui.ColorTableRow)
	list.SetBackgroundColor(ui.ColorAppBg)
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
	h.Body.RemoveItem(h.PluginsList)
	h.PluginsList = h.LoadPlugins()
	h.Body.AddItem(h.PluginsList, 0, 0, 1, 1, 0, 0, true)
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
	h.overlayRestyle = func() { h.ShowCover() }
	h.ShowHomeHeader()
	h.SetCrumbs(ui.FormatBreadcrumbs([]string{"cover"}))
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
	h.ShowHomeHeader()
	h.dashboard = NewDashboard(
		h.App,
		h.rpcManager,
		entries,
		func(entry installedPlugin) { h.activateInstalled(entry) },
		h.ShowCover,
	)
	h.MainFrame.SetPrimitive(h.dashboard.Primitive())
	h.overlayRestyle = func() {
		if h.dashboard != nil {
			h.dashboard.ApplyTheme()
		}
	}
	h.SetCrumbs(ui.FormatBreadcrumbs([]string{"dashboard"}))
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

	if h.rpcManager != nil {
		h.rpcManager.ReleaseLogo()
	}
	pm.SetHeaderLogo(h.LogoView())
	h.SetPluginHeader(pm.DetachHeader())
	h.overlayRestyle = func() { pm.ApplyTheme() }
	h.Pages.AddPage(pageID, pm.GetLayout(), true, false)
	h.MainFrame.SetPrimitive(pm.GetLayout())
	h.SetCrumbs(ui.FormatBreadcrumbs([]string{"omo", "pkgmgr"}))
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

	if h.rpcManager != nil {
		h.rpcManager.ReleaseLogo()
	}
	sm.SetHeaderLogo(h.LogoView())
	h.SetPluginHeader(sm.DetachHeader())
	h.overlayRestyle = func() { sm.ApplyTheme() }
	h.Pages.AddPage(pageID, sm.GetLayout(), true, false)
	h.MainFrame.SetPrimitive(sm.GetLayout())
	h.SetCrumbs(ui.FormatBreadcrumbs([]string{"omo", "settings"}))
	h.App.SetFocus(sm.GetTable())
}

func (h *Host) restoreMainContent() {
	if h.dashboard != nil {
		h.overlayRestyle = func() {
			if h.dashboard != nil {
				h.dashboard.ApplyTheme()
			}
		}
		h.ShowHomeHeader()
		h.MainFrame.SetPrimitive(h.dashboard.Primitive())
		h.dashboard.Focus()
		return
	}
	if h.rpcManager != nil {
		if p := h.rpcManager.ActivePrimitive(); p != nil {
			h.overlayRestyle = func() {
				if h.rpcManager != nil {
					h.rpcManager.ApplyTheme()
				}
			}
			h.rpcManager.ReattachActiveChrome()
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
	list.SetMainTextColor(ui.ColorTableRow)
	list.SetHighlightFullLine(true)
	list.SetSelectedTextColor(ui.ColorHighlightText)
	list.SetSelectedBackgroundColor(ui.ColorHighlight)
	list.SetBorder(false)
	list.SetBorderPadding(0, 0, 1, 1)
}

func newPluginTable() *tview.Table {
	table := tview.NewTable()
	table.SetBorders(false)
	table.SetSelectable(true, false)
	table.SetFixed(1, 0)
	table.SetBackgroundColor(ui.ColorAppBg)
	table.Box.SetBackgroundColor(ui.ColorAppBg)
	table.SetBorder(false)
	table.SetBorderPadding(0, 0, 1, 1)
	table.SetSelectedStyle(tcell.StyleDefault.
		Foreground(ui.ColorHighlightText).
		Background(ui.ColorHighlight))
	header := tview.NewTableCell("Plugins").
		SetTextColor(ui.ColorBorder).
		SetAttributes(tcell.AttrBold).
		SetBackgroundColor(ui.ColorAppBg).
		SetSelectable(false).
		SetExpansion(1)
	table.SetCell(0, 0, header)
	return table
}

func fillPluginTable(table *tview.Table, entries []installedPlugin) {
	// Keep the header cell; drop data rows.
	for row := table.GetRowCount() - 1; row >= 1; row-- {
		table.RemoveRow(row)
	}
	if len(entries) == 0 {
		table.SetCell(1, 0, pluginNameCell("No plugins found", false))
		table.Select(1, 0)
		return
	}
	for i, entry := range entries {
		table.SetCell(i+1, 0, pluginNameCell(entry.Name, true))
	}
	table.Select(1, 0)
}

func pluginNameCell(name string, selectable bool) *tview.TableCell {
	return tview.NewTableCell(name).
		SetTextColor(ui.ColorTableRow).
		SetBackgroundColor(ui.ColorAppBg).
		SetSelectable(selectable).
		SetExpansion(1)
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
		SetBackgroundColor(ui.ColorAppBg).
		SetBorder(true).
		SetTitle(" Plugin Error ").
		SetTitleAlign(tview.AlignCenter)
	h.ShowHomeHeader()
	h.MainFrame.SetPrimitive(infoText)
}

package host

import (
	"fmt"
	"os"
	"path/filepath"
	"plugin"
	"strings"

	"omo/internal/packagemanager"
	"omo/internal/registry"
	"omo/pkg/pluginapi"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type Host struct {
	App             *tview.Application
	Pages           *tview.Pages
	MainFrame       *tview.Frame
	MainUI          *tview.Grid
	PluginsList     *tview.List
	ActivePlugin    pluginapi.Plugin
	activePluginIdx int
	PluginsDir      string
	logger          *pluginapi.Logger
	version         string
}

func New(app *tview.Application, pages *tview.Pages, logger *pluginapi.Logger, version string) *Host {
	mainFrame := tview.NewFrame(nil)
	mainFrame.SetBackgroundColor(tcell.ColorDefault)

	mainUI := tview.NewGrid()
	mainUI.SetBackgroundColor(tcell.ColorDefault)

	return &Host{
		App:             app,
		Pages:           pages,
		MainFrame:       mainFrame,
		MainUI:          mainUI,
		PluginsList:     tview.NewList(),
		activePluginIdx: -1,
		PluginsDir:      pluginapi.PluginsDir(),
		logger:          logger,
		version:         version,
	}
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
		list.AddItem("No plugins found", "", '0', nil)
		h.log("no plugins found in %s", h.PluginsDir)
	}

	list.SetSelectedFunc(func(i int, s1, s2 string, r rune) {
		pluginFile, err := plugin.Open(s2)
		if err != nil {
			h.log("failed to load plugin %s: %v", s2, err)
			h.showPluginLoadError("Failed to load plugin", err)
			return
		}
		startSymbol, err := pluginFile.Lookup("OhmyopsPlugin")
		if err != nil {
			h.log("plugin entrypoint not found in %s: %v", s2, err)
			h.showPluginLoadError("Plugin entrypoint not found", err)
			return
		}

		ohmyopsPlugin, ok := startSymbol.(pluginapi.Plugin)
		if !ok {
			h.log("plugin interface mismatch in %s", s2)
			h.showPluginLoadError("Plugin interface mismatch", fmt.Errorf("invalid plugin type"))
			return
		}

		if h.ActivePlugin != nil {
			if stoppable, ok := h.ActivePlugin.(pluginapi.Stoppable); ok {
				stoppable.Stop()
			}
			pluginapi.ClosePluginLogger()
		}
		h.ActivePlugin = ohmyopsPlugin

		if h.activePluginIdx >= 0 && h.activePluginIdx < list.GetItemCount() {
			prevMain, prevSec := list.GetItemText(h.activePluginIdx)
			prevName := stripPluginPrefix(prevMain)
			list.SetItemText(h.activePluginIdx, "  → "+prevName, prevSec)
		}

		curMain, curSec := list.GetItemText(i)
		curName := stripPluginPrefix(curMain)
		list.SetItemText(i, "[green]  ● [white]"+curName, curSec)
		h.activePluginIdx = i

		metadata := ohmyopsPlugin.GetMetadata()
		registry.RegisterPlugin(curName, metadata)

		// Create per-plugin log file: ~/.omo/logs/<plugin>.log
		pluginLogger, err := pluginapi.NewLogger(curName)
		if err != nil {
			h.log("failed to create logger for %s: %v", curName, err)
		}
		pluginapi.SetPluginLogger(pluginLogger)

		h.log("activated plugin: %s %s", curName, metadata.Version)

		component := ohmyopsPlugin.Start(h.App)
		h.MainFrame.SetPrimitive(component)
	})

	h.PluginsList = list
	return list
}

// LogoView returns a compact OMO logo with version for the top-left corner.
func (h *Host) LogoView() tview.Primitive {
	tv := tview.NewTextView()
	tv.SetDynamicColors(true)
	tv.SetTextAlign(tview.AlignCenter)
	tv.SetBackgroundColor(tcell.ColorDefault)
	tv.SetText(fmt.Sprintf("[#FF6B00::b]█▀█ █▀▄▀█ █▀█\n█▄█ █ ▀ █ █▄█\n[#666666]%s", h.version))
	return tv
}

// ActionsView returns a selectable list for refresh/package-manager actions.
func (h *Host) ActionsView() *tview.List {
	list := tview.NewList().ShowSecondaryText(false)
	list.SetMainTextColor(tcell.ColorAqua)
	list.SetBackgroundColor(tcell.ColorDefault)
	list.AddItem("  ↻ Refresh Plugins", "", 0, func() { h.RefreshPlugins() })
	list.AddItem("  ⬡ Package Manager", "", 0, func() { h.OpenPackageManager() })
	return list
}

// RefreshPlugins reloads the plugins list.
func (h *Host) RefreshPlugins() {
	h.log("refreshing plugins")
	h.MainUI.RemoveItem(h.PluginsList)
	h.PluginsList = h.LoadPlugins()
	h.MainUI.AddItem(h.PluginsList, 1, 0, 1, 1, 0, 0, true)
	h.App.SetFocus(h.PluginsList)
}

// OpenPackageManager shows the package manager UI.
func (h *Host) OpenPackageManager() {
	h.log("opening package manager")
	pm := packagemanager.NewPackageManager(h.App, h.Pages, h.PluginsDir)
	h.Pages.AddPage("packageManager", pm.GetLayout(), true, false)
	h.MainFrame.SetPrimitive(pm.GetLayout())
}

func discoverPlugins(pluginsDir string) (*tview.List, error) {
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return nil, err
	}

	list := tview.NewList().ShowSecondaryText(false)
	list.SetMainTextColor(tcell.ColorPurple)
	list.SetBackgroundColor(tcell.ColorDefault)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		soPath := filepath.Join(pluginsDir, name, name+".so")

		if _, err := os.Stat(soPath); err != nil {
			continue
		}

		list.AddItem("  → "+name, soPath, 0, nil)
	}

	return list, nil
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

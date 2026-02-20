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
	"omo/pkg/ui"

	"github.com/gdamore/tcell/v2"
	"github.com/pgavlin/femto"
	"github.com/rivo/tview"
)

type Host struct {
	App             *tview.Application
	Pages           *tview.Pages
	MainFrame       *tview.Frame
	MainUI          *tview.Grid
	HeaderView      *tview.Grid
	PluginsList     *tview.List
	ActivePlugin    pluginapi.Plugin
	activePluginIdx int
	PluginsDir      string
	ConfigsDir      string
}

func New(app *tview.Application, pages *tview.Pages) *Host {
	mainFrame := tview.NewFrame(nil)
	mainFrame.SetBackgroundColor(tcell.ColorDefault)

	mainUI := tview.NewGrid()
	mainUI.SetBackgroundColor(tcell.ColorDefault)

	headerView := tview.NewGrid()
	headerView.SetBackgroundColor(tcell.ColorDefault)

	return &Host{
		App:             app,
		Pages:           pages,
		MainFrame:       mainFrame,
		MainUI:          mainUI,
		HeaderView:      headerView,
		PluginsList:     tview.NewList(),
		activePluginIdx: -1,
		PluginsDir:      pluginapi.PluginsDir(),
		ConfigsDir:      pluginapi.ConfigsDir(),
	}
}

func (h *Host) LoadPlugins() *tview.List {
	list, err := discoverPlugins(h.PluginsDir)
	if err != nil || list == nil {
		list = tview.NewList().ShowSecondaryText(false)
		list.SetMainTextColor(tcell.ColorPurple)
		list.SetBackgroundColor(tcell.ColorDefault)
		list.AddItem("No plugins found", "", '0', nil)
	}

	list.SetSelectedFunc(func(i int, s1, s2 string, r rune) {
		pluginFile, err := plugin.Open(s2)
		if err != nil {
			h.showPluginLoadError("Failed to load plugin", err)
			return
		}
		startSymbol, err := pluginFile.Lookup("OhmyopsPlugin")
		if err != nil {
			h.showPluginLoadError("Plugin entrypoint not found", err)
			return
		}

		ohmyopsPlugin, ok := startSymbol.(pluginapi.Plugin)
		if !ok {
			h.showPluginLoadError("Plugin interface mismatch", fmt.Errorf("invalid plugin type"))
			return
		}

		if h.ActivePlugin != nil {
			if stoppable, ok := h.ActivePlugin.(pluginapi.Stoppable); ok {
				stoppable.Stop()
			}
		}
		h.ActivePlugin = ohmyopsPlugin

		// Reset previous active item back to arrow prefix
		if h.activePluginIdx >= 0 && h.activePluginIdx < list.GetItemCount() {
			prevMain, prevSec := list.GetItemText(h.activePluginIdx)
			prevName := stripPluginPrefix(prevMain)
			list.SetItemText(h.activePluginIdx, "  → "+prevName, prevSec)
		}

		// Mark current item with green dot
		curMain, curSec := list.GetItemText(i)
		curName := stripPluginPrefix(curMain)
		list.SetItemText(i, "[green]  ● [white]"+curName, curSec)
		h.activePluginIdx = i

		metadata := ohmyopsPlugin.GetMetadata()
		registry.RegisterPlugin(curName, metadata)

		component := ohmyopsPlugin.Start(h.App)
		h.MainFrame.SetPrimitive(component)

		h.UpdateHeader(curName)
	})

	list.SetChangedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		h.UpdateHeader(stripPluginPrefix(mainText))
	})

	h.PluginsList = list
	return list
}

func (h *Host) HelpList() *tview.List {
	list := tview.NewList().
		AddItem("Refresh plugins", "", 'r', nil).
		AddItem("Settings", "", 'a', nil).
		AddItem("Package Manager", "", 'p', nil)
	list.ShowSecondaryText(false)
	list.SetBackgroundColor(tcell.ColorDefault)
	list.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		switch index {
		case 0:
			h.MainUI.RemoveItem(h.PluginsList)
			h.PluginsList = h.LoadPlugins()
			h.MainUI.AddItem(h.PluginsList, 1, 0, 1, 1, 0, 0, true)
			h.UpdateHeader("")
		case 1:
			var editor *femto.View
			settingsGrid := tview.NewGrid()
			settingsGrid.SetRows(0, 0, 0)
			settingsGrid.SetColumns(30, 0, 0)
			settingsGrid.SetBorders(true).SetBordersColor(tcell.ColorGray)
			settingsGrid.SetBackgroundColor(tcell.ColorDefault)

			configList, err := discoverConfigs(h.ConfigsDir)
			if err != nil || configList.GetItemCount() == 0 {
				infoText := tview.NewTextView().
					SetTextAlign(tview.AlignCenter).
					SetText(fmt.Sprintf("No configuration files found.\n\nExpected location: %s/<plugin>/<plugin>.yaml", h.ConfigsDir)).
					SetTextColor(tcell.ColorYellow).
					SetBackgroundColor(tcell.ColorDefault).
					SetBorder(true)
				h.MainFrame.SetPrimitive(infoText)
				return
			}

			configList.SetCurrentItem(0)

			_, initialConfigPath := configList.GetItemText(0)
			editor = newEditor(h.App, h.Pages, initialConfigPath)

			configList.SetSelectedFunc(func(i int, s1, s2 string, r rune) {
				if s2 != "" {
					settingsGrid.RemoveItem(editor)
					editor = newEditor(h.App, h.Pages, s2)
					settingsGrid.AddItem(editor, 0, 1, 3, 2, 0, 0, true)
				}
			})

			settingsGrid.AddItem(configList, 0, 0, 3, 1, 1, 0, true)
			settingsGrid.AddItem(editor, 0, 1, 3, 2, 0, 0, true)
			h.MainFrame.SetPrimitive(settingsGrid)
		case 2:
			packageManager := packagemanager.NewPackageManager(h.App, h.Pages, h.PluginsDir)
			h.Pages.AddPage("packageManager", packageManager.GetLayout(), true, false)
			h.MainFrame.SetPrimitive(packageManager.GetLayout())
			h.UpdateHeader("Package Manager")
		}
	})

	return list
}

// discoverPlugins scans ~/.omo/plugins/ for subdirectories containing .so files.
// Expected layout: ~/.omo/plugins/<name>/<name>.so
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
			continue // no .so in this subdirectory
		}

		list.AddItem("  → "+name, soPath, 0, nil)
	}

	return list, nil
}

// stripPluginPrefix removes the arrow/dot prefix and tview color tags from a plugin display name.
func stripPluginPrefix(name string) string {
	// Strip tview color tags like [green], [white]
	for strings.Contains(name, "[") && strings.Contains(name, "]") {
		start := strings.Index(name, "[")
		end := strings.Index(name, "]")
		if start < end {
			name = name[:start] + name[end+1:]
		} else {
			break
		}
	}
	// Strip prefix symbols
	name = strings.TrimLeft(name, " →●")
	name = strings.TrimSpace(name)
	return name
}

// discoverConfigs scans ~/.omo/configs/ for yaml files in plugin subdirectories.
// Expected layout: ~/.omo/configs/<name>/<name>.yaml
func discoverConfigs(configsDir string) (*tview.List, error) {
	entries, err := os.ReadDir(configsDir)
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

		pluginName := entry.Name()
		pluginConfigDir := filepath.Join(configsDir, pluginName)

		// Find all yaml files in this plugin's config dir
		yamlFiles, err := os.ReadDir(pluginConfigDir)
		if err != nil {
			continue
		}
		for _, yf := range yamlFiles {
			if yf.IsDir() {
				continue
			}
			fname := yf.Name()
			if strings.HasSuffix(fname, ".yaml") || strings.HasSuffix(fname, ".yml") {
				fullPath := filepath.Join(pluginConfigDir, fname)
				displayName := pluginName + "/" + fname

				list.AddItem(displayName, fullPath, 0, nil)
			}
		}
	}

	return list, nil
}

func (h *Host) UpdateHeader(pluginName string) {
	h.HeaderView.Clear()

	pluginHeader := ui.NewPluginHeaderView(func(name string) (interface{}, bool) {
		metadata, exists := registry.GetPluginMetadata(name)
		return metadata, exists
	}).SetPluginInfo(pluginName)

	h.HeaderView.SetRows(0).SetColumns(0)
	h.HeaderView.AddItem(pluginHeader, 0, 0, 1, 1, 0, 0, false)
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
	h.UpdateHeader("")
}

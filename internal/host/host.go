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
	App          *tview.Application
	Pages        *tview.Pages
	MainFrame    *tview.Frame
	MainUI       *tview.Grid
	HeaderView   *tview.Grid
	PluginsList  *tview.List
	ActivePlugin pluginapi.Plugin
	PluginsDir   string
	ConfigsDir   string
}

func New(app *tview.Application, pages *tview.Pages) *Host {
	mainFrame := tview.NewFrame(nil)
	mainFrame.SetBackgroundColor(tcell.ColorDefault)

	mainUI := tview.NewGrid()
	mainUI.SetBackgroundColor(tcell.ColorDefault)

	headerView := tview.NewGrid()
	headerView.SetBackgroundColor(tcell.ColorDefault)

	return &Host{
		App:        app,
		Pages:      pages,
		MainFrame:  mainFrame,
		MainUI:     mainUI,
		HeaderView: headerView,
		PluginsList: tview.NewList(),
		PluginsDir:  pluginapi.PluginsDir(),
		ConfigsDir:  pluginapi.ConfigsDir(),
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

		metadata := ohmyopsPlugin.GetMetadata()
		registry.RegisterPlugin(s1, metadata)

		component := ohmyopsPlugin.Start(h.App)
		h.MainFrame.SetPrimitive(component)

		h.UpdateHeader(s1)
	})

	list.SetChangedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		h.UpdateHeader(mainText)
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
			h.MainUI.AddItem(h.PluginsList, 1, 0, 1, 1, 0, 100, true)
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
					settingsGrid.AddItem(editor, 0, 1, 3, 2, 0, 100, true)
				}
			})

			settingsGrid.AddItem(configList, 0, 0, 3, 1, 1, 100, true)
			settingsGrid.AddItem(editor, 0, 1, 3, 2, 0, 100, true)
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

	shortcutIdx := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		soPath := filepath.Join(pluginsDir, name, name+".so")

		if _, err := os.Stat(soPath); err != nil {
			continue // no .so in this subdirectory
		}

		shortcut := rune(0)
		if shortcutIdx < 9 {
			shortcut = rune('1' + shortcutIdx)
		}
		shortcutIdx++

		list.AddItem(name, soPath, shortcut, nil)
	}

	return list, nil
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

	shortcutIdx := 0
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

				shortcut := rune(0)
				if shortcutIdx < 9 {
					shortcut = rune('1' + shortcutIdx)
				}
				shortcutIdx++

				list.AddItem(displayName, fullPath, shortcut, nil)
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

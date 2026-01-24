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

const (
	DefaultPluginsDir = "./compiled_plugins"
	DefaultConfigDir  = "./config"
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
	ConfigDir    string
}

func New(app *tview.Application, pages *tview.Pages, pluginsDir, configDir string) *Host {
	mainFrame := tview.NewFrame(nil)
	mainFrame.SetBackgroundColor(tcell.ColorDefault)

	mainUI := tview.NewGrid()
	mainUI.SetBackgroundColor(tcell.ColorDefault)

	headerView := tview.NewGrid()
	headerView.SetBackgroundColor(tcell.ColorDefault)

	return &Host{
		App:         app,
		Pages:       pages,
		MainFrame:   mainFrame,
		MainUI:      mainUI,
		HeaderView:  headerView,
		PluginsList: tview.NewList(),
		PluginsDir:  pluginsDir,
		ConfigDir:   configDir,
	}
}

func (h *Host) LoadPlugins() *tview.List {
	list, err := getPluginsNames(h.PluginsDir)
	if err != nil || list == nil {
		// Return empty list instead of nil to prevent crash
		list = tview.NewList().ShowSecondaryText(false)
		list.SetMainTextColor(tcell.ColorPurple)
		list.SetBackgroundColor(tcell.ColorDefault)
		list.AddItem("No plugins found", "Run 'make all' to build plugins", '0', nil)
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

		// Update the header with the selected plugin and its metadata
		h.UpdateHeader(s1)
	})

	// Also set the changed function to update the header when selection changes
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

			configList, err := getPluginsNames(h.ConfigDir)
			if err != nil || configList.GetItemCount() == 0 {
				// Handle case where no config files are found
				infoText := tview.NewTextView().
					SetTextAlign(tview.AlignCenter).
					SetText("No configuration files found in ./config directory.\n\nCreate at least one .yaml file in the config directory.").
					SetTextColor(tcell.ColorYellow).
					SetBackgroundColor(tcell.ColorDefault).
					SetBorder(true)
				h.MainFrame.SetPrimitive(infoText)
				return
			}

			// Default to first item in the list
			configList.SetCurrentItem(0)

			// Create editor for the selected config file
			_, initialConfigPath := configList.GetItemText(0)
			editor = newEditor(h.App, initialConfigPath)

			// Handle selection of config files
			configList.SetSelectedFunc(func(i int, s1, s2 string, r rune) {
				// s2 contains the full path to the config file
				if s2 != "" {
					settingsGrid.RemoveItem(editor)
					editor = newEditor(h.App, s2)
					settingsGrid.AddItem(editor, 0, 1, 3, 2, 0, 100, true)
				}
			})

			settingsGrid.AddItem(configList, 0, 0, 3, 1, 1, 100, true)
			settingsGrid.AddItem(editor, 0, 1, 3, 2, 0, 100, true)
			h.MainFrame.SetPrimitive(settingsGrid)
		case 2:
			// Create and show the package manager
			packageManager := packagemanager.NewPackageManager(h.App, h.Pages, h.PluginsDir)

			// Set up the package manager page
			h.Pages.AddPage("packageManager", packageManager.GetLayout(), true, false)

			// Show the package manager in the main frame
			h.MainFrame.SetPrimitive(packageManager.GetLayout())
			h.UpdateHeader("Package Manager")
		}
	})

	return list
}

func getPluginsNames(dir string) (*tview.List, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	list := tview.NewList().ShowSecondaryText(false)
	list.SetMainTextColor(tcell.ColorPurple)
	list.SetBackgroundColor(tcell.ColorDefault)
	for i, file := range files {
		if !file.IsDir() {
			fileName := file.Name()
			ext := filepath.Ext(fileName)
			fileNameWithoutExt := strings.TrimSuffix(fileName, ext)
			fullPath := filepath.Join(dir, fileName)
			shortcut := rune(0)
			if i < 9 {
				shortcut = rune('1' + i)
			}
			list.AddItem(fileNameWithoutExt, fullPath, shortcut, nil)
		}
	}

	return list, nil
}

func (h *Host) UpdateHeader(pluginName string) {
	// Clear the grid
	h.HeaderView.Clear()

	// Create a plugin header view with metadata provider function
	pluginHeader := ui.NewPluginHeaderView(func(name string) (interface{}, bool) {
		// Get metadata directly from registry
		metadata, exists := registry.GetPluginMetadata(name)
		return metadata, exists
	}).SetPluginInfo(pluginName)

	// Add the custom component to the grid
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

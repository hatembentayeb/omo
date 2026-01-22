package main

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"plugin"
	"strings"
	"time"

	"omo/ui"

	"github.com/gdamore/tcell/v2"
	"github.com/pgavlin/femto"
	"github.com/rivo/tview"
)

const (
	pluginsDir = "./compiled_plugins"
)

var (
	mainFrame   *tview.Frame
	mainUI      *tview.Grid
	app         = tview.NewApplication()
	pluginsList = tview.NewList()
	appPages    = tview.NewPages()
	headerView  *tview.Grid
)

func init() {
	mainFrame = tview.NewFrame(nil)
	mainFrame.SetBackgroundColor(tcell.ColorDefault)
	
	mainUI = tview.NewGrid()
	mainUI.SetBackgroundColor(tcell.ColorDefault)
	
	headerView = tview.NewGrid()
	headerView.SetBackgroundColor(tcell.ColorDefault)
}

// Update the OhmyopsPlugin interface to include UI functions
type OhmyopsPlugin interface {
	Start(*tview.Application) tview.Primitive
	GetMetadata() PluginMetadata
}

func main() {
	// Initialize random seed
	rand.Seed(time.Now().UnixNano())

	// Initialize header
	headerView.SetBackgroundColor(tcell.ColorDefault)

	// Setup default header content
	updateHeader(headerView, "")

	pluginsList = loadPlugins(pluginsDir)
	helpListView := helpList()

	// Adjust grid layout for better space utilization
	// Column alignment: fixed left column width to align with Redis separator
	mainUI.SetRows(12, 0, 3).SetColumns(25, 0)

	// Set less obtrusive borders
	mainUI.SetBorders(true).SetBordersColor(tcell.ColorAqua)
	mainUI.SetBackgroundColor(tcell.ColorDefault)

	// Configure mainFrame to take maximum space with no padding
	mainFrame.SetBorderPadding(0, 0, 0, 0)

	// Set the welcome screen as the initial view
	mainFrame.SetPrimitive(Cover())

	mainUI.AddItem(headerView, 0, 0, 1, 1, 0, 100, false).
		AddItem(pluginsList, 1, 0, 1, 1, 0, 100, true).
		AddItem(mainFrame, 0, 1, 3, 1, 0, 100, false).
		AddItem(helpListView, 2, 0, 1, 1, 0, 100, false)

	// Set up pages with main UI as base page
	appPages.AddPage("main", mainUI, true, true)

	// Setup navigation between panels with SHIFT+TAB
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Handle SHIFT+TAB to cycle between panels
		if event.Key() == tcell.KeyBacktab {
			// Get the current focus
			currentFocus := app.GetFocus()

			// Determine which panel has focus and cycle to the next
			switch currentFocus {
			case pluginsList:
				// Move focus from plugins list to settings list
				app.SetFocus(helpListView)
			case helpListView:
				// Move focus from settings list to main frame content
				mainContent := mainFrame.GetPrimitive()
				// Try to focus the main content if possible
				if mainContent != nil {
					app.SetFocus(mainContent)
				} else {
					// If not possible, cycle back to plugins list
					app.SetFocus(pluginsList)
				}
			default:
				// Move focus back to plugins list from any other panel
				app.SetFocus(pluginsList)
			}
			return nil
		}
		return event
	})

	// Use pages as the root primitive
	if err := app.SetRoot(appPages, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
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
			shortcut := fmt.Sprintf("%d", i+1)
			list.AddItem(fileNameWithoutExt, fullPath, rune(shortcut[0]), nil)
		}
	}

	return list, nil
}

func helpList() *tview.List {
	list := tview.NewList().
		AddItem("Refresh plugins", "", 'r', nil).
		AddItem("Settings", "", 'a', nil).
		AddItem("Package Manager", "", 'p', nil)
	list.ShowSecondaryText(false)
	list.SetBackgroundColor(tcell.ColorDefault)
	list.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {

		switch index {
		case 0:
			mainUI.RemoveItem(pluginsList)
			pluginsList = loadPlugins(pluginsDir)
			mainUI.AddItem(pluginsList, 1, 0, 1, 1, 0, 100, true)
			updateHeader(headerView, "")
		case 1:
			var editor *femto.View
			settingsGrid := tview.NewGrid()
			settingsGrid.SetRows(0, 0, 0)
			settingsGrid.SetColumns(30, 0, 0)
			settingsGrid.SetBorders(true).SetBordersColor(tcell.ColorGray)
			settingsGrid.SetBackgroundColor(tcell.ColorDefault)

			list, err := getPluginsNames("./config")
			if err != nil || list.GetItemCount() == 0 {
				// Handle case where no config files are found
				infoText := tview.NewTextView().
					SetTextAlign(tview.AlignCenter).
					SetText("No configuration files found in ./config directory.\n\nCreate at least one .yml file in the config directory.").
					SetTextColor(tcell.ColorYellow).
					SetBackgroundColor(tcell.ColorDefault).
					SetBorder(true)
				mainFrame.SetPrimitive(infoText)
				return
			}

			// Default to first item in the list
			list.SetCurrentItem(0)

			// Create editor for the selected config file
			_, initialConfigPath := list.GetItemText(0)
			editor = newEditor(initialConfigPath)

			// Handle selection of config files
			list.SetSelectedFunc(func(i int, s1, s2 string, r rune) {
				// s2 contains the full path to the config file
				if s2 != "" {
					settingsGrid.RemoveItem(editor)
					editor = newEditor(s2)
					settingsGrid.AddItem(editor, 0, 1, 3, 2, 0, 100, true)
				}
			})

			settingsGrid.AddItem(list, 0, 0, 3, 1, 1, 100, true)
			settingsGrid.AddItem(editor, 0, 1, 3, 2, 0, 100, true)
			mainFrame.SetPrimitive(settingsGrid)
		case 2:
			// Create and show the package manager
			packageManager := NewPackageManager(app, appPages)

			// Set up the package manager page
			appPages.AddPage("packageManager", packageManager.GetLayout(), true, false)

			// Show the package manager in the main frame
			mainFrame.SetPrimitive(packageManager.GetLayout())
			updateHeader(headerView, "Package Manager")
		}
	})

	return list
}

func loadPlugins(pluginsDir string) *tview.List {
	var err error
	pluginsList, err = getPluginsNames(pluginsDir)
	if err != nil || pluginsList == nil {
		// Return empty list instead of nil to prevent crash
		pluginsList = tview.NewList().ShowSecondaryText(false)
		pluginsList.SetMainTextColor(tcell.ColorPurple)
		pluginsList.SetBackgroundColor(tcell.ColorDefault)
		pluginsList.AddItem("No plugins found", "Run 'make all' to build plugins", '0', nil)
	}

	pluginsList.SetSelectedFunc(func(i int, s1, s2 string, r rune) {
		plugin, err := plugin.Open(s2)
		if err != nil {
			// Silent error handling instead of fmt.Println
			return
		}
		startSymbol, err := plugin.Lookup("OhmyopsPlugin")
		if err != nil {
			// Silent error handling instead of fmt.Println
			return
		}

		// Attempt direct type assertion first
		ohmyopsPlugin, ok := startSymbol.(OhmyopsPlugin)
		if !ok {
			// If type assertion fails, use a more generic approach
			// This avoids the "error asserting type" issue by using a looser interface

			// Try to call the Start method directly through reflection or interface
			if starter, ok := startSymbol.(interface {
				Start(*tview.Application) tview.Primitive
			}); ok {
				// Start the plugin
				component := starter.Start(app)
				mainFrame.SetPrimitive(component)

				// Create default metadata
				metadata := PluginMetadata{
					Name:        s1,
					Version:     "1.0.0",
					Description: "Plugin " + s1,
					Author:      "Unknown",
					License:     "Unknown",
					Tags:        []string{},
					Arch:        []string{"amd64"},
					LastUpdated: time.Now(),
					URL:         "",
				}

				// Try to extract metadata if GetMetadata exists
				if metadataMethod, err := plugin.Lookup("GetMetadata"); err == nil {
					if metadataFunc, ok := metadataMethod.(func() interface{}); ok {
						if rawMetadata := metadataFunc(); rawMetadata != nil {
							// Try to extract fields from the metadata
							if m, ok := rawMetadata.(map[string]interface{}); ok {
								if name, ok := m["Name"].(string); ok {
									metadata.Name = name
								}
								if version, ok := m["Version"].(string); ok {
									metadata.Version = version
								}
								if description, ok := m["Description"].(string); ok {
									metadata.Description = description
								}
								if author, ok := m["Author"].(string); ok {
									metadata.Author = author
								}
								if license, ok := m["License"].(string); ok {
									metadata.License = license
								}
							}
						}
					}
				}

				RegisterPlugin(s1, metadata)
				updateHeader(headerView, s1)
				return
			}

			// Silent error handling instead of fmt.Println
			return
		}

		// If we're here, the direct type assertion worked
		// Get and register the plugin metadata
		metadata := ohmyopsPlugin.GetMetadata()
		RegisterPlugin(s1, metadata)

		// Start the plugin
		component := ohmyopsPlugin.Start(app)
		mainFrame.SetPrimitive(component)

		// Update the header with the selected plugin and its metadata
		updateHeader(headerView, s1)
	})

	// Also set the changed function to update the header when selection changes
	pluginsList.SetChangedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		updateHeader(headerView, mainText)
	})

	return pluginsList
}

func updateHeader(header *tview.Grid, pluginName string) {
	// Clear the grid
	header.Clear()

	// Create a plugin header view with metadata provider function
	pluginHeader := ui.NewPluginHeaderView(func(name string) (interface{}, bool) {
		// Get metadata directly from main package
		metadata, exists := GetPluginMetadata(name)
		return metadata, exists
	}).SetPluginInfo(pluginName)

	// Add the custom component to the grid
	header.SetRows(0).SetColumns(0)
	header.AddItem(pluginHeader, 0, 0, 1, 1, 0, 0, false)
}

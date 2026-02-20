package main

import (
	"fmt"
	"os"

	"omo/internal/host"
	"omo/pkg/pluginapi"
	"omo/pkg/secrets"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func main() {
	// Bootstrap and open the KeePass secrets database.
	secretsProvider, err := secrets.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "omo: failed to initialise secrets: %v\n", err)
		os.Exit(1)
	}
	defer secretsProvider.Close()

	// Register the global provider so plugins can call pluginapi.Secrets().
	pluginapi.SetSecretsProvider(secrets.NewAdapter(secretsProvider))

	app := tview.NewApplication()
	pages := tview.NewPages()
	omoHost := host.New(app, pages)

	// Setup default header content
	omoHost.UpdateHeader("")

	pluginsList := omoHost.LoadPlugins()
	helpListView := omoHost.HelpList()

	// Adjust grid layout for better space utilization
	// Column alignment: fixed left column width to align with Redis separator
	omoHost.MainUI.SetRows(8, 0, 3).SetColumns(20, 0)

	// Set less obtrusive borders
	omoHost.MainUI.SetBorders(true).SetBordersColor(tcell.ColorAqua)
	omoHost.MainUI.SetBackgroundColor(tcell.ColorDefault)

	// Configure mainFrame to take maximum space with no padding
	omoHost.MainFrame.SetBorderPadding(0, 0, 0, 0)

	// Set the welcome screen as the initial view
	omoHost.MainFrame.SetPrimitive(host.Cover(app))

	// minWidth=0 so the grid renders on small terminals (e.g. Termux)
	omoHost.MainUI.AddItem(omoHost.HeaderView, 0, 0, 1, 1, 0, 0, false).
		AddItem(pluginsList, 1, 0, 1, 1, 0, 0, true).
		AddItem(omoHost.MainFrame, 0, 1, 3, 1, 0, 0, false).
		AddItem(helpListView, 2, 0, 1, 1, 0, 0, false)

	// Set up pages with main UI as base page
	pages.AddPage("main", omoHost.MainUI, true, true)

	// Panel cycling: Shift+Tab or Ctrl+Down to cycle between panels.
	// Ctrl+Down is needed because Shift+Tab doesn't work on many mobile terminals (Termux).
	cyclePanels := func() {
		currentFocus := app.GetFocus()
		switch currentFocus {
		case omoHost.PluginsList:
			app.SetFocus(helpListView)
		case helpListView:
			mainContent := omoHost.MainFrame.GetPrimitive()
			if mainContent != nil {
				app.SetFocus(mainContent)
			} else {
				app.SetFocus(omoHost.PluginsList)
			}
		default:
			app.SetFocus(omoHost.PluginsList)
		}
	}
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyBacktab ||
			(event.Key() == tcell.KeyDown && event.Modifiers()&tcell.ModCtrl != 0) {
			cyclePanels()
			return nil
		}
		return event
	})

	// Use pages as the root primitive
	if err := app.SetRoot(pages, true).Run(); err != nil {
		panic(err)
	}
}

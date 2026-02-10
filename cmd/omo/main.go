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
	omoHost.MainUI.SetRows(12, 0, 3).SetColumns(25, 0)

	// Set less obtrusive borders
	omoHost.MainUI.SetBorders(true).SetBordersColor(tcell.ColorAqua)
	omoHost.MainUI.SetBackgroundColor(tcell.ColorDefault)

	// Configure mainFrame to take maximum space with no padding
	omoHost.MainFrame.SetBorderPadding(0, 0, 0, 0)

	// Set the welcome screen as the initial view
	omoHost.MainFrame.SetPrimitive(host.Cover(app))

	omoHost.MainUI.AddItem(omoHost.HeaderView, 0, 0, 1, 1, 0, 100, false).
		AddItem(pluginsList, 1, 0, 1, 1, 0, 100, true).
		AddItem(omoHost.MainFrame, 0, 1, 3, 1, 0, 100, false).
		AddItem(helpListView, 2, 0, 1, 1, 0, 100, false)

	// Set up pages with main UI as base page
	pages.AddPage("main", omoHost.MainUI, true, true)

	// Setup navigation between panels with SHIFT+TAB
	// Store this as a named function so plugins can chain to it
	shiftTabHandler := func(event *tcell.EventKey) *tcell.EventKey {
		// Handle SHIFT+TAB to cycle between panels
		if event.Key() == tcell.KeyBacktab {
			// Get the current focus
			currentFocus := app.GetFocus()

			// Determine which panel has focus and cycle to the next
			switch currentFocus {
			case omoHost.PluginsList:
				// Move focus from plugins list to settings list
				app.SetFocus(helpListView)
			case helpListView:
				// Move focus from settings list to main frame content
				mainContent := omoHost.MainFrame.GetPrimitive()
				// Try to focus the main content if possible
				if mainContent != nil {
					app.SetFocus(mainContent)
				} else {
					// If not possible, cycle back to plugins list
					app.SetFocus(omoHost.PluginsList)
				}
			default:
				// Move focus back to plugins list from any other panel
				app.SetFocus(omoHost.PluginsList)
			}
			return nil
		}
		return event
	}
	app.SetInputCapture(shiftTabHandler)

	// Use pages as the root primitive
	if err := app.SetRoot(pages, true).Run(); err != nil {
		panic(err)
	}
}

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
	// App logger: ~/.omo/logs/omo.log
	logger, err := pluginapi.NewLogger("omo")
	if err != nil {
		fmt.Fprintf(os.Stderr, "omo: failed to initialise logger: %v\n", err)
	}
	if logger != nil {
		defer logger.Close()
	}

	secretsProvider, err := secrets.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "omo: failed to initialise secrets: %v\n", err)
		if logger != nil {
			logger.Error("failed to initialise secrets: %v", err)
		}
		os.Exit(1)
	}
	defer secretsProvider.Close()
	if logger != nil {
		logger.Info("secrets provider initialised")
	}

	pluginapi.SetSecretsProvider(secrets.NewAdapter(secretsProvider))

	app := tview.NewApplication()
	pages := tview.NewPages()
	omoHost := host.New(app, pages, logger)

	pluginsList := omoHost.LoadPlugins()
	logoView := omoHost.LogoView()
	actionsView := omoHost.ActionsView()

	// Three rows: logo (4) + plugins list (flex) + actions (4)
	// Two columns: sidebar (20 wide) + main content (flex)
	omoHost.MainUI.SetRows(4, 0, 4).SetColumns(20, 0)

	omoHost.MainUI.SetBorders(true).SetBordersColor(tcell.ColorAqua)
	omoHost.MainUI.SetBackgroundColor(tcell.ColorDefault)

	omoHost.MainFrame.SetBorderPadding(0, 0, 0, 0)
	omoHost.MainFrame.SetPrimitive(host.Cover(app))

	omoHost.MainUI.AddItem(logoView, 0, 0, 1, 1, 0, 0, false).
		AddItem(omoHost.MainFrame, 0, 1, 3, 1, 0, 0, false).
		AddItem(pluginsList, 1, 0, 1, 1, 0, 0, true).
		AddItem(actionsView, 2, 0, 1, 1, 0, 0, false)

	pages.AddPage("main", omoHost.MainUI, true, true)

	// Global key bindings
	// Tab cycles: plugins list → main content → actions → plugins list
	// Shift+Tab cycles in reverse
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTab {
			focus := app.GetFocus()
			switch {
			case focus == pluginsList:
				if mc := omoHost.MainFrame.GetPrimitive(); mc != nil {
					app.SetFocus(mc)
				}
			case focus == actionsView:
				app.SetFocus(pluginsList)
			default:
				app.SetFocus(actionsView)
			}
			return nil
		}

		if event.Key() == tcell.KeyBacktab {
			focus := app.GetFocus()
			switch {
			case focus == pluginsList:
				app.SetFocus(actionsView)
			case focus == actionsView:
				if mc := omoHost.MainFrame.GetPrimitive(); mc != nil {
					app.SetFocus(mc)
				}
			default:
				app.SetFocus(pluginsList)
			}
			return nil
		}

		// 'r' when sidebar is focused: refresh plugins
		if event.Rune() == 'r' && app.GetFocus() == pluginsList {
			omoHost.RefreshPlugins()
			return nil
		}

		// 'p' when sidebar is focused: open package manager
		if event.Rune() == 'p' && app.GetFocus() == pluginsList {
			omoHost.OpenPackageManager()
			return nil
		}

		return event
	})

	if logger != nil {
		logger.Info("omo started")
	}

	if err := app.SetRoot(pages, true).Run(); err != nil {
		if logger != nil {
			logger.Error("app crashed: %v", err)
		}
		panic(err)
	}
}

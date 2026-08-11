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

// Version is injected at build time via -ldflags.
var Version = "dev"

func main() {
	// Dispatch CLI subcommands before starting the TUI.
	if len(os.Args) > 1 && os.Args[1] == "secrets" {
		runSecretsCLI(os.Args[2:])
		return
	}

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
	omoHost := host.New(app, pages, logger, Version)

	pluginsList := omoHost.LoadPlugins()
	logoView := omoHost.LogoView()
	actionsView := omoHost.ActionsView()
	defer omoHost.Shutdown()

	// Three rows: logo+version (5) + plugins list (flex) + actions (4)
	// Two columns: sidebar (20 wide) + main content (flex)
	omoHost.MainUI.SetRows(5, 0, 4).SetColumns(20, 0)

	omoHost.MainUI.SetBorders(true).SetBordersColor(tcell.ColorAqua)
	omoHost.MainUI.SetBackgroundColor(tcell.ColorDefault)

	omoHost.MainFrame.SetBorders(0, 0, 0, 0, 0, 0)
	omoHost.MainFrame.SetBorderPadding(0, 0, 0, 0)
	omoHost.MainFrame.SetPrimitive(host.Cover(app, Version))

	omoHost.MainUI.AddItem(logoView, 0, 0, 1, 1, 0, 0, false).
		AddItem(omoHost.MainFrame, 0, 1, 3, 1, 0, 0, false).
		AddItem(pluginsList, 1, 0, 1, 1, 0, 0, true).
		AddItem(actionsView, 2, 0, 1, 1, 0, 0, false)

	pages.AddPage("main", omoHost.MainUI, true, true)

	// Always compare against omoHost.PluginsList — RefreshPlugins replaces the list
	// primitive, so a captured local pointer goes stale and breaks Tab / p / r.
	pluginsFocused := func() bool {
		return app.GetFocus() == omoHost.PluginsList
	}

	// Global key bindings
	// Tab cycles: plugins list → main content → actions → plugins list
	// Shift+Tab cycles in reverse
	// Ctrl+t opens target/instance selector for the active RPC plugin
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlT {
			// Don't intercept while a modal page is open.
			if front, _ := pages.GetFrontPage(); front != "" && front != "main" {
				return event
			}
			omoHost.SelectTarget()
			return nil
		}

		if event.Key() == tcell.KeyTab {
			focus := app.GetFocus()
			switch {
			case pluginsFocused():
				omoHost.FocusPluginContent()
			case focus == actionsView:
				app.SetFocus(omoHost.PluginsList)
			default:
				app.SetFocus(actionsView)
			}
			return nil
		}

		if event.Key() == tcell.KeyBacktab {
			focus := app.GetFocus()
			switch {
			case pluginsFocused():
				app.SetFocus(actionsView)
			case focus == actionsView:
				omoHost.FocusPluginContent()
			default:
				app.SetFocus(omoHost.PluginsList)
			}
			return nil
		}

		// Host sidebar shortcuts (plugins list must be focused).
		if pluginsFocused() {
			switch event.Rune() {
			case 'r', 'R':
				omoHost.RefreshPlugins()
				return nil
			case 'p', 'P':
				omoHost.OpenPackageManager()
				return nil
			}
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

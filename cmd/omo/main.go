package main

import (
	"fmt"
	"os"

	"omo/internal/host"
	"omo/pkg/pluginapi"
	"omo/pkg/secrets"
	"omo/pkg/ui"

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
	app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		screen.Fill(' ', tcell.StyleDefault.Background(ui.ColorAppBg))
		return false
	})
	pages := tview.NewPages()
	omoHost := host.New(app, pages, logger, Version)

	pluginsList := omoHost.LoadPlugins()
	defer omoHost.Shutdown()

	// Header is full width. Plugins + table share one warm gold frame.
	// Below the frame: Plugins/View tabs | breadcrumbs | version | host actions.
	omoHost.MainUI.SetRows(5, 0, host.StatusRowHeight).SetColumns(0)

	omoHost.MainUI.SetBorders(false)
	omoHost.MainUI.SetBackgroundColor(ui.ColorAppBg)

	omoHost.MainFrame.SetBorders(0, 0, 0, 0, 0, 0)
	omoHost.MainFrame.SetBorderPadding(0, 0, 0, 0)

	omoHost.ShowCover()

	status := tview.NewFlex()
	status.SetDirection(tview.FlexColumn)
	status.SetBackgroundColor(ui.ColorAppBg)
	status.AddItem(omoHost.PaneView(), host.VersionColWidth, 0, false).
		AddItem(omoHost.FooterView(), 0, 1, false)

	omoHost.Body.AddItem(pluginsList, 0, 0, 1, 1, 0, 0, true).
		AddItem(omoHost.MainFrame, 0, 1, 1, 1, 0, 0, false)

	omoHost.MainUI.AddItem(omoHost.HeaderFrame, 0, 0, 1, 1, 0, 0, false).
		AddItem(omoHost.Body, 1, 0, 1, 1, 0, 0, false).
		AddItem(status, 2, 0, 1, 1, 0, 0, false)

	pages.AddPage("main", omoHost.MainUI, true, true)
	omoHost.ShowStartupSplash()

	// Always compare against omoHost.PluginsList — RefreshPlugins replaces the table
	// primitive, so a captured local pointer goes stale and breaks Tab / p / r.
	pluginsFocused := func() bool {
		return app.GetFocus() == omoHost.PluginsList
	}
	// Any page other than "main" is a modal (confirm, input, info, …).
	modalOpen := func() bool {
		front, _ := pages.GetFrontPage()
		return front != "" && front != "main"
	}

	// Global key bindings
	// Tab cycles: plugins list → main content → plugins list
	// Shift+Tab cycles in reverse
	// While a modal is open, Tab/Shift+Tab stay inside that modal (fields/buttons).
	// Ctrl+t opens target/instance selector for the active RPC plugin
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if omoHost.SplashVisible() {
			omoHost.DismissSplash()
			return nil
		}

		if event.Key() == tcell.KeyCtrlT {
			if modalOpen() {
				return event
			}
			omoHost.SelectTarget()
			return nil
		}

		if event.Key() == tcell.KeyTab {
			if modalOpen() {
				return event // modal form/list owns Tab
			}
			if pluginsFocused() {
				omoHost.FocusPluginContent()
			} else {
				app.SetFocus(omoHost.PluginsList)
			}
			return nil
		}

		if event.Key() == tcell.KeyBacktab {
			if modalOpen() {
				return event // modal form/list owns Shift+Tab
			}
			if pluginsFocused() {
				omoHost.FocusPluginContent()
			} else {
				app.SetFocus(omoHost.PluginsList)
			}
			return nil
		}

		// Host sidebar shortcuts (plugins list must be focused).
		if pluginsFocused() && !modalOpen() {
			switch event.Rune() {
			case 'r', 'R':
				omoHost.RefreshPlugins()
				return nil
			case 'p', 'P':
				omoHost.OpenPackageManager()
				return nil
			case 'i', 'I', 's', 'S':
				omoHost.OpenSettings()
				return nil
			case 'D':
				omoHost.OpenDashboard()
				return nil
			case 't', 'T':
				omoHost.OpenThemes()
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

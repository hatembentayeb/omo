package host

import (
	"fmt"

	"omo/pkg/ui"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func coverLogo() string {
	return fmt.Sprintf("[%s]\n"+
		" ██████  ██   ██ ███    ███ ██    ██  ██████  ██████  ███████ \n"+
		"██    ██ ██   ██ ████  ████  ██  ██  ██    ██ ██   ██ ██      \n"+
		"██    ██ ███████ ██ ████ ██   ████   ██    ██ ██████  ███████ \n"+
		"██    ██ ██   ██ ██  ██  ██    ██    ██    ██ ██           ██ \n"+
		" ██████  ██   ██ ██      ██    ██     ██████  ██      ███████ [%s]\n",
		ui.HexActionKey, ui.HexValue)
}

func emptyBox() *tview.Box {
	return tview.NewBox().SetBackgroundColor(ui.ColorAppBg)
}

// Splash is the full-screen startup screen: centered OhMyOps mark, nothing else.
func Splash() tview.Primitive {
	mark := tview.NewTextView()
	mark.SetDynamicColors(true)
	mark.SetTextAlign(tview.AlignCenter)
	mark.SetBackgroundColor(ui.ColorAppBg)
	mark.SetText(coverLogo() + fmt.Sprintf("\n[%s::b]OhMyOps[%s::-]", ui.HexInfoKey, ui.HexValue))

	inner := tview.NewFlex().SetDirection(tview.FlexRow)
	inner.SetBackgroundColor(ui.ColorAppBg)
	inner.AddItem(emptyBox(), 0, 1, false).
		AddItem(mark, 9, 0, true).
		AddItem(emptyBox(), 0, 1, false)

	root := tview.NewFlex()
	root.SetBackgroundColor(ui.ColorAppBg)
	root.AddItem(emptyBox(), 0, 1, false).
		AddItem(inner, 64, 0, true).
		AddItem(emptyBox(), 0, 1, false)
	return root
}

// Cover returns the home splash shown before the live plugin dashboard.
func Cover(app *tview.Application, version string, onDashboard func()) tview.Primitive {
	if version == "" {
		version = "dev"
	}

	logoBox := tview.NewTextView()
	logoBox.SetDynamicColors(true)
	logoBox.SetTextAlign(tview.AlignCenter)
	logoBox.SetBackgroundColor(ui.ColorAppBg)
	logoBox.SetText(coverLogo() + fmt.Sprintf("\n[%s]Terminal operations, all plugins at a glance[%s]", ui.HexInfoKey, ui.HexValue))

	about := fmt.Sprintf(
		"[%s]OhMyOps[%s]\n\n"+
			"[%s]Version[%s]  %s\n\n"+
			"[%s]A TUI host for ops plugins — Docker, Redis,\n"+
			"Kubernetes, Kafka, Git, S3, Postgres, and more.\n\n"+
			"Open the live dashboard or pick a sidebar plugin.\n"+
			"Every plugin shares: Info · Views · Actions.[%s]",
		ui.HexInfoKey, ui.HexValue,
		ui.HexActionKey, ui.HexValue, version,
		ui.HexLabel, ui.HexValue,
	)

	aboutBox := tview.NewTextView()
	aboutBox.SetDynamicColors(true)
	aboutBox.SetTextAlign(tview.AlignLeft)
	aboutBox.SetBorder(true)
	aboutBox.SetBorderColor(ui.ColorBorder)
	aboutBox.SetTitle(" About ")
	aboutBox.SetTitleColor(ui.ColorHighlight)
	aboutBox.SetBorderPadding(1, 1, 2, 2)
	aboutBox.SetBackgroundColor(ui.ColorAppBg)
	aboutBox.SetText(about)

	start := fmt.Sprintf("[%s]Start here[%s]\n\n", ui.HexInfoKey, ui.HexValue) +
		fmt.Sprintf("[%s]<enter>[%s] Open live plugin dashboard\n", ui.HexActionKey, ui.HexLabel) +
		fmt.Sprintf("[%s]<D>[%s]     Sidebar: load dashboard tiles\n", ui.HexActionKey, ui.HexLabel) +
		fmt.Sprintf("[%s]<enter>[%s] Sidebar: open selected plugin\n", ui.HexActionKey, ui.HexLabel) +
		fmt.Sprintf("[%s]<tab>[%s]   Cycle sidebar · content\n", ui.HexActionKey, ui.HexLabel) +
		fmt.Sprintf("[%s]<p>[%s]     Package Manager\n", ui.HexActionKey, ui.HexLabel) +
		fmt.Sprintf("[%s]<i>[%s]     Settings / Info\n", ui.HexActionKey, ui.HexLabel) +
		fmt.Sprintf("[%s]<t>[%s]     Themes\n", ui.HexActionKey, ui.HexLabel) +
		fmt.Sprintf("[%s]<r>[%s]     Refresh plugin list\n\n", ui.HexActionKey, ui.HexLabel) +
		fmt.Sprintf("[%s]Inside a plugin[%s]\n", ui.HexActionKey, ui.HexValue) +
		fmt.Sprintf("[%s]<0-9>[%s] Views · [%s]</>[%s] Filter · [%s]<R>[%s] Refresh\n",
			ui.HexViewKey, ui.HexLabel, ui.HexActionKey, ui.HexLabel, ui.HexActionKey, ui.HexLabel) +
		fmt.Sprintf("[%s]<esc>[%s] Back to dashboard", ui.HexActionKey, ui.HexLabel)

	startBox := tview.NewTextView()
	startBox.SetDynamicColors(true)
	startBox.SetTextAlign(tview.AlignLeft)
	startBox.SetBorder(true)
	startBox.SetBorderColor(ui.ColorBorder)
	startBox.SetTitle(" Keys ")
	startBox.SetTitleColor(ui.ColorHighlight)
	startBox.SetBorderPadding(1, 1, 2, 2)
	startBox.SetBackgroundColor(ui.ColorAppBg)
	startBox.SetText(start)

	grid := tview.NewGrid()
	grid.SetColumns(0, 42, 42, 0)
	grid.SetRows(0, 11, 1, 17, 0)
	grid.SetBorders(false)
	grid.SetBackgroundColor(ui.ColorAppBg)
	grid.AddItem(logoBox, 1, 1, 1, 2, 0, 0, true)
	grid.AddItem(aboutBox, 3, 1, 1, 1, 0, 0, false)
	grid.AddItem(startBox, 3, 2, 1, 1, 0, 0, false)

	frame := tview.NewFrame(grid)
	frame.SetBorders(0, 0, 0, 0, 0, 0)
	frame.SetBackgroundColor(ui.ColorAppBg)
	frame.AddText("Enter Dashboard · D Tiles · t Themes · p Packages · i Info", true, tview.AlignCenter, tcell.ColorDimGray)
	frame.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEnter && onDashboard != nil {
			onDashboard()
			return nil
		}
		return event
	})

	app.SetFocus(logoBox)
	return frame
}

// Center returns a centered primitive within a container.
func Center(width, height int, p tview.Primitive) tview.Primitive {
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(p, height, 1, true).
			AddItem(nil, 0, 1, false), width, 1, true).
		AddItem(nil, 0, 1, false)
}

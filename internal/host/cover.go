package host

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const logo = `[#FF6B00]
 ██████  ██   ██ ███    ███ ██    ██  ██████  ██████  ███████ 
██    ██ ██   ██ ████  ████  ██  ██  ██    ██ ██   ██ ██      
██    ██ ███████ ██ ████ ██   ████   ██    ██ ██████  ███████ 
██    ██ ██   ██ ██  ██  ██    ██    ██    ██ ██           ██ 
 ██████  ██   ██ ██      ██    ██     ██████  ██      ███████ [white]
`

func emptyBox() *tview.Box {
	return tview.NewBox().SetBackgroundColor(tcell.ColorDefault)
}

// Splash is the full-screen startup screen: centered OhMyOps mark, nothing else.
func Splash() tview.Primitive {
	mark := tview.NewTextView()
	mark.SetDynamicColors(true)
	mark.SetTextAlign(tview.AlignCenter)
	mark.SetBackgroundColor(tcell.ColorDefault)
	mark.SetText(logo + "\n[#FFD700::b]OhMyOps[white::-]")

	inner := tview.NewFlex().SetDirection(tview.FlexRow)
	inner.SetBackgroundColor(tcell.ColorDefault)
	inner.AddItem(emptyBox(), 0, 1, false).
		AddItem(mark, 9, 0, true).
		AddItem(emptyBox(), 0, 1, false)

	root := tview.NewFlex()
	root.SetBackgroundColor(tcell.ColorDefault)
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
	logoBox.SetBackgroundColor(tcell.ColorDefault)
	logoBox.SetText(logo + "\n[#FF8C40]Terminal operations, all plugins at a glance[white]")

	about := fmt.Sprintf(
		"[#FFD700]OhMyOps[white]\n\n"+
			"[#4CAF50]Version[white]  %s\n\n"+
			"[#00BCD4]A TUI host for ops plugins — Docker, Redis,\n"+
			"Kubernetes, Kafka, Git, S3, Postgres, and more.\n\n"+
			"Open the live dashboard or pick a sidebar plugin.\n"+
			"Every plugin shares: Info · Views · Actions.[white]",
		version,
	)

	aboutBox := tview.NewTextView()
	aboutBox.SetDynamicColors(true)
	aboutBox.SetTextAlign(tview.AlignLeft)
	aboutBox.SetBorder(true)
	aboutBox.SetBorderColor(tcell.ColorDarkCyan)
	aboutBox.SetTitle(" About ")
	aboutBox.SetTitleColor(tcell.ColorOrange)
	aboutBox.SetBorderPadding(1, 1, 2, 2)
	aboutBox.SetBackgroundColor(tcell.ColorDefault)
	aboutBox.SetText(about)

	start := "[#FFD700]Start here[white]\n\n" +
		"[#5FD7FF]<enter>[#BCBCBC] Open live plugin dashboard\n" +
		"[#5FD7FF]<D>[#BCBCBC]     Sidebar: load dashboard tiles\n" +
		"[#5FD7FF]<enter>[#BCBCBC] Sidebar: open selected plugin\n" +
		"[#5FD7FF]<tab>[#BCBCBC]   Cycle sidebar · content · actions\n" +
		"[#5FD7FF]<p>[#BCBCBC]     Package Manager\n" +
		"[#5FD7FF]<i>[#BCBCBC]     Settings / Info\n" +
		"[#5FD7FF]<r>[#BCBCBC]     Refresh plugin list\n\n" +
		"[#4CAF50]Inside a plugin[white]\n" +
		"[#FF87FF]<0-9>[#BCBCBC] Views · [#5FD7FF]</>[#BCBCBC] Filter · [#5FD7FF]<R>[#BCBCBC] Refresh\n" +
		"[#5FD7FF]<esc>[#BCBCBC] Back to dashboard"

	startBox := tview.NewTextView()
	startBox.SetDynamicColors(true)
	startBox.SetTextAlign(tview.AlignLeft)
	startBox.SetBorder(true)
	startBox.SetBorderColor(tcell.ColorDarkCyan)
	startBox.SetTitle(" Keys ")
	startBox.SetTitleColor(tcell.ColorOrange)
	startBox.SetBorderPadding(1, 1, 2, 2)
	startBox.SetBackgroundColor(tcell.ColorDefault)
	startBox.SetText(start)

	grid := tview.NewGrid()
	grid.SetColumns(0, 42, 42, 0)
	grid.SetRows(0, 11, 1, 17, 0)
	grid.SetBorders(false)
	grid.SetBackgroundColor(tcell.ColorDefault)
	grid.AddItem(logoBox, 1, 1, 1, 2, 0, 0, true)
	grid.AddItem(aboutBox, 3, 1, 1, 1, 0, 0, false)
	grid.AddItem(startBox, 3, 2, 1, 1, 0, 0, false)

	frame := tview.NewFrame(grid)
	frame.SetBorders(0, 0, 0, 0, 0, 0)
	frame.SetBackgroundColor(tcell.ColorDefault)
	frame.AddText("Enter Dashboard · Shift+D Tiles · sidebar Enter Plugin · p Package Manager · i Settings", true, tview.AlignCenter, tcell.ColorDimGray)
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

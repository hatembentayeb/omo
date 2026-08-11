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

// Cover returns the home splash shown before a plugin is selected.
func Cover(app *tview.Application, version string) tview.Primitive {
	if version == "" {
		version = "dev"
	}

	logoBox := tview.NewTextView()
	logoBox.SetDynamicColors(true)
	logoBox.SetTextAlign(tview.AlignCenter)
	logoBox.SetBackgroundColor(tcell.ColorDefault)
	logoBox.SetText(logo + "\n[#FF8C40]Terminal operations, one plugin at a time[white]")

	about := fmt.Sprintf(
		"[#FFD700]OhMyOps[white]\n\n"+
			"[#4CAF50]Version[white]  %s\n\n"+
			"[#00BCD4]A TUI host for ops plugins — Docker, Redis,\n"+
			"Kubernetes, Kafka, Git, S3, Postgres, and more.\n\n"+
			"Pick a plugin from the sidebar. Each screen\n"+
			"shares the same chrome: Info · Views · Actions.[white]",
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
		"[#FF9900]Enter[white]   Open selected plugin\n" +
		"[#FF9900]Tab[white]     Cycle sidebar · content · actions\n" +
		"[#FF9900]p[white]       Package Manager (install / update)\n" +
		"[#FF9900]r[white]       Refresh plugin list\n" +
		"[#FF9900]Ctrl+t[white]  Switch target / connection\n" +
		"[#FF9900]?[white]       Help for the active view\n\n" +
		"[#4CAF50]Inside a plugin[white]\n" +
		"[#FF9900]0-9[white]     Switch views\n" +
		"[#FF9900]/[white]       Filter the table\n" +
		"[#FF9900]R[white]       Refresh data\n" +
		"[#FF9900]ESC[white]     Back"

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
	grid.SetRows(0, 11, 1, 16, 0)
	grid.SetBorders(false)
	grid.SetBackgroundColor(tcell.ColorDefault)
	grid.AddItem(logoBox, 1, 1, 1, 2, 0, 0, true)
	grid.AddItem(aboutBox, 3, 1, 1, 1, 0, 0, false)
	grid.AddItem(startBox, 3, 2, 1, 1, 0, 0, false)

	frame := tview.NewFrame(grid)
	frame.SetBorders(0, 0, 0, 0, 0, 0)
	frame.SetBackgroundColor(tcell.ColorDefault)
	frame.AddText("Select a plugin · p opens Package Manager", true, tview.AlignCenter, tcell.ColorDimGray)

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

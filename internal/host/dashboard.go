package host

import (
	"fmt"
	"strings"
	"sync"

	"omo/pkg/pluginrpc"
	"omo/pkg/ui"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const dashboardColumns = 3

// Dashboard is the host-owned live overview of installed RPC plugins.
type Dashboard struct {
	app        *tview.Application
	manager    *PluginManager
	entries    []installedPlugin
	onOpen     func(installedPlugin)
	onClose    func()
	root       *tview.Flex
	title      *tview.TextView
	help       *tview.TextView
	grid       *tview.Grid
	cards      []*tview.TextView
	selected   int
	mu         sync.Mutex
	generation int
}

func NewDashboard(
	app *tview.Application,
	manager *PluginManager,
	entries []installedPlugin,
	onOpen func(installedPlugin),
	onClose func(),
) *Dashboard {
	d := &Dashboard{
		app:     app,
		manager: manager,
		entries: entries,
		onOpen:  onOpen,
		onClose: onClose,
	}
	d.build()
	return d
}

func (d *Dashboard) Primitive() tview.Primitive { return d.root }

func (d *Dashboard) Focus() {
	if d.app != nil && d.root != nil {
		d.app.SetFocus(d.root)
	}
}

func (d *Dashboard) build() {
	d.title = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText(dashboardTitle("live summaries"))
	d.title.SetBackgroundColor(ui.ColorAppBg)

	d.help = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText(dashboardHelp())
	d.help.SetBackgroundColor(ui.ColorAppBg)

	if len(d.entries) == 0 {
		d.grid = newDashboardGrid(1)
		empty := tview.NewTextView().
			SetDynamicColors(true).
			SetTextAlign(tview.AlignCenter).
			SetText("\n[yellow]No installed plugins found[white]\n\nInstall one from Package Manager.")
		empty.SetBackgroundColor(ui.ColorAppBg)
		empty.SetBorder(true)
		d.grid.AddItem(empty, 0, 0, 1, dashboardColumns, 0, 0, false)
	} else {
		rows := (len(d.entries) + dashboardColumns - 1) / dashboardColumns
		d.grid = newDashboardGrid(rows)
		d.cards = make([]*tview.TextView, len(d.entries))
		for i, entry := range d.entries {
			card := tview.NewTextView()
			card.SetDynamicColors(true)
			card.SetWrap(false)
			card.SetBackgroundColor(ui.ColorAppBg)
			card.SetBorder(true)
			card.SetBorderPadding(0, 0, 1, 1)
			card.SetTitle(" " + entry.Name + " ")
			card.SetText("[yellow]loading…[white]\n[gray]Preparing live summary")
			d.cards[i] = card
			d.grid.AddItem(card, i/dashboardColumns, i%dashboardColumns, 1, 1, 0, 0, false)
		}
	}

	d.root = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(d.title, 1, 0, false).
		AddItem(d.grid, 0, 1, false).
		AddItem(d.help, 1, 0, false)
	d.root.SetBackgroundColor(ui.ColorAppBg)
	d.root.SetInputCapture(d.handleKey)
	d.paintSelection()
	d.updateTitle()
}

// newDashboardGrid uses proportional rows so tiles always fill the content
// area instead of leaving dead space below a fixed-height grid.
func newDashboardGrid(rows int) *tview.Grid {
	grid := tview.NewGrid()
	grid.SetBackgroundColor(ui.ColorAppBg)
	grid.SetBorders(false)
	grid.SetColumns(0, 0, 0)
	grid.SetRows(make([]int, rows)...)
	return grid
}

func (d *Dashboard) handleKey(event *tcell.EventKey) *tcell.EventKey {
	if len(d.entries) == 0 {
		if event.Key() == tcell.KeyEscape && d.onClose != nil {
			d.onClose()
			return nil
		}
		if event.Key() == tcell.KeyRune && (event.Rune() == 'r' || event.Rune() == 'R') {
			d.Refresh()
			return nil
		}
		return event
	}

	switch event.Key() {
	case tcell.KeyEscape:
		if d.onClose != nil {
			d.onClose()
		}
		return nil
	case tcell.KeyEnter:
		if d.onOpen != nil {
			d.onOpen(d.entries[d.selected])
		}
		return nil
	case tcell.KeyLeft:
		d.move(-1)
		return nil
	case tcell.KeyRight:
		d.move(1)
		return nil
	case tcell.KeyUp:
		d.move(-dashboardColumns)
		return nil
	case tcell.KeyDown:
		d.move(dashboardColumns)
		return nil
	case tcell.KeyRune:
		switch event.Rune() {
		case 'h':
			d.move(-1)
			return nil
		case 'l':
			d.move(1)
			return nil
		case 'k':
			d.move(-dashboardColumns)
			return nil
		case 'j':
			d.move(dashboardColumns)
			return nil
		case 'r', 'R':
			d.Refresh()
			return nil
		case 'o':
			if d.onOpen != nil {
				d.onOpen(d.entries[d.selected])
			}
			return nil
		}
	}
	return event
}

func (d *Dashboard) move(delta int) {
	next := d.selected + delta
	if next < 0 {
		next = 0
	}
	if next >= len(d.entries) {
		next = len(d.entries) - 1
	}
	d.selected = next
	d.paintSelection()
	d.updateTitle()
}

func (d *Dashboard) paintSelection() {
	for i, card := range d.cards {
		if i == d.selected {
			card.SetBorderColor(ui.ColorHighlight)
			card.SetTitleColor(ui.ColorHighlight)
		} else {
			card.SetBorderColor(ui.ColorBorder)
			card.SetTitleColor(ui.ColorBorder)
		}
	}
}

func (d *Dashboard) updateTitle() {
	if len(d.entries) == 0 {
		return
	}
	d.title.SetText(fmt.Sprintf(
		"%s  [%s]%s · %d/%d[-]",
		dashboardHeading(),
		ui.HexLabel,
		d.entries[d.selected].Name, d.selected+1, len(d.entries),
	))
}

func dashboardHeading() string {
	return fmt.Sprintf("[%s::b]Plugin Dashboard[%s::-]", ui.HexActionKey, ui.HexValue)
}

func dashboardTitle(suffix string) string {
	return fmt.Sprintf("%s  [%s]%s[-]", dashboardHeading(), ui.HexLabel, suffix)
}

func dashboardHelp() string {
	return fmt.Sprintf("[%s]<↑↓←→>[%s] select  [%s]<enter>[%s] open  [%s]<R>[%s] refresh  [%s]<esc>[%s] cover",
		ui.HexActionKey, ui.HexLabel, ui.HexActionKey, ui.HexLabel, ui.HexActionKey, ui.HexLabel, ui.HexActionKey, ui.HexLabel)
}

// Refresh starts a bounded live pulse. Tiles remain interactive while results
// arrive and stale results from a previous refresh are ignored.
func (d *Dashboard) Refresh() {
	d.mu.Lock()
	d.generation++
	generation := d.generation
	d.mu.Unlock()

	for i, card := range d.cards {
		card.SetTitle(" " + d.entries[i].Name + " ")
		card.SetText("[yellow]loading…[white]\n[gray]Fetching plugin data")
	}

	if d.manager != nil {
		d.manager.ReloadSecrets()
	}

	jobs := make(chan int)
	workers := dashboardColumns
	if len(d.entries) < workers {
		workers = len(d.entries)
	}
	for n := 0; n < workers; n++ {
		go func() {
			for i := range jobs {
				entry := d.entries[i]
				view := d.manager.DashboardSnapshot(entry.Name, entry.BinPath)
				d.app.QueueUpdateDraw(func() {
					d.mu.Lock()
					current := d.generation
					d.mu.Unlock()
					if current != generation || i >= len(d.cards) {
						return
					}
					d.renderCard(i, view)
				})
			}
		}()
	}
	go func() {
		defer close(jobs)
		for i := range d.entries {
			jobs <- i
		}
	}()
}

func (d *Dashboard) renderCard(index int, view pluginrpc.ViewData) {
	card := d.cards[index]
	title := strings.TrimSpace(view.Title)
	if title == "" {
		title = d.entries[index].Name
	}
	card.SetTitle(" " + title + " ")

	status := strings.ToLower(strings.TrimSpace(view.Status))
	statusColor := "green"
	switch {
	case strings.Contains(status, "error"), strings.Contains(status, "fail"):
		statusColor = "red"
	case strings.Contains(status, "not"), strings.Contains(status, "idle"), strings.Contains(status, "warn"):
		statusColor = "yellow"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "[%s::b]%s[white::-]\n", statusColor, dashStatus(status))
	shown := 0
	for _, row := range view.Rows {
		if shown >= 4 || len(row) == 0 {
			break
		}
		key := pluginrpc.Truncate(stripPluginPrefix(row[0]), 16)
		// The header line already carries the status; don't repeat it as a row.
		if strings.EqualFold(key, "Status") {
			continue
		}
		value := "-"
		if len(row) > 1 {
			value = pluginrpc.Truncate(row[1], 28)
		}
		fmt.Fprintf(&b, "[%s]%s:[%s] %s\n", ui.HexInfoKey, key, ui.HexValue, value)
		shown++
	}
	if shown == 0 {
		b.WriteString("[gray]No summary rows")
	}
	card.SetText(strings.TrimRight(b.String(), "\n"))
}

func dashStatus(status string) string {
	if status == "" {
		return "connected"
	}
	return status
}

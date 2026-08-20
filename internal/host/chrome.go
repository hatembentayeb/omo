package host

import (
	"fmt"

	"omo/pkg/ui"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (h *Host) wirePluginTable(table *tview.Table) {
	if table == nil {
		return
	}
	table.SetFocusFunc(func() { h.syncPaneChrome(true) })
	table.SetBlurFunc(func() { h.syncPaneChrome(false) })
}

func (h *Host) syncPaneChrome(pluginsOn bool) {
	if h == nil {
		return
	}
	h.pluginsOn = pluginsOn
	h.setVersionText()
	if h.paneBar != nil {
		h.paneBar.SetText(" " + formatPaneTabs(pluginsOn))
	}
	if h.chromeBar != nil {
		h.chromeBar.SetText(formatHostChrome(pluginsOn) + " ")
	}
	h.paintPluginHeader()
}

func (h *Host) paintPluginHeader() {
	if h.PluginsList == nil {
		return
	}
	cell := h.PluginsList.GetCell(0, 0)
	if cell == nil {
		return
	}
	cell.SetTextColor(ui.ColorBorder)
	cell.SetBackgroundColor(ui.ColorAppBg)
	cell.SetAttributes(tcell.AttrBold)
}

func formatPaneTabs(pluginsOn bool) string {
	return tabPill("Plugins", pluginsOn) + tabPill("View", !pluginsOn)
}

func tabPill(label string, on bool) string {
	if on {
		return fmt.Sprintf("[%s:%s:b] %s [-:-]", ui.HexHighlightText, ui.HexHighlight, label)
	}
	return fmt.Sprintf("[%s] %s [-]", ui.HexLabel, label)
}

func formatHostChrome(pluginsOn bool) string {
	if pluginsOn {
		return hostActionPill("D", "Dashboard") + " " +
			hostActionPill("p", "Packages") + " " +
			hostActionPill("i", "Info") + " " +
			hostActionPill("t", "Themes")
	}
	return fmt.Sprintf("[%s]<D> Dashboard  <p> Packages  <i> Info  <t> Themes[-]", ui.HexLabel)
}

func hostActionPill(key, label string) string {
	return fmt.Sprintf("[%s:%s:b] %s [-:-][%s]%s[-]",
		ui.HexHighlightText, ui.HexHighlight, key, ui.HexValue, label)
}

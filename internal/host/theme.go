package host

import (
	"omo/pkg/pluginrpc"
	"omo/pkg/ui"

	"github.com/gdamore/tcell/v2"
)

// OpenThemes lists Omarchy + built-in palettes and applies the selection.
func (h *Host) OpenThemes() {
	if h == nil || h.Pages == nil {
		return
	}
	themes := ui.ListThemes()
	items := make([][]string, 0, len(themes))
	current := ui.ActiveThemeID()
	for _, th := range themes {
		desc := th.Source
		if th.ID == current {
			desc = "current · " + desc
		}
		items = append(items, []string{th.Name, desc})
	}
	ui.ShowStandardListSelectorModal(h.Pages, h.App, "Themes", items, func(index int, _ string, cancelled bool) {
		if cancelled || index < 0 || index >= len(themes) {
			if h.PluginsList != nil {
				h.App.SetFocus(h.PluginsList)
			}
			return
		}
		ui.ApplyAndSaveTheme(themes[index].ID)
		pluginrpc.SetInfoColors(ui.HexInfoKey, ui.HexValue)
		h.restyle()
		if h.PluginsList != nil {
			h.App.SetFocus(h.PluginsList)
		}
	})
}

func (h *Host) restyle() {
	if h == nil {
		return
	}
	h.restyleShell()
	h.syncPaneChrome(h.pluginsOn)
	if h.Logo != nil {
		h.Logo.Restyle()
	}
	if h.proverb != nil {
		h.proverb.Restyle()
	}
	if h.overlayRestyle != nil {
		h.overlayRestyle()
		return
	}
	if h.dashboard != nil {
		h.dashboard.ApplyTheme()
		return
	}
	if h.rpcManager != nil {
		h.rpcManager.ApplyTheme()
	}
}

func (h *Host) restyleShell() {
	bg := ui.ColorAppBg
	if h.MainFrame != nil {
		h.MainFrame.SetBackgroundColor(bg)
	}
	if h.MainUI != nil {
		h.MainUI.SetBackgroundColor(bg)
	}
	if h.HeaderFrame != nil {
		h.HeaderFrame.SetBackgroundColor(bg)
	}
	if h.Body != nil {
		h.Body.SetBackgroundColor(bg)
		h.Body.SetBordersColor(ui.ColorBorder)
	}
	if h.Footer != nil {
		h.Footer.SetBackgroundColor(bg)
	}
	if h.crumbBar != nil {
		h.crumbBar.SetBackgroundColor(bg)
	}
	if h.chromeBar != nil {
		h.chromeBar.SetBackgroundColor(bg)
	}
	if h.paneBar != nil {
		h.paneBar.SetBackgroundColor(bg)
	}
	if h.versionBar != nil {
		h.versionBar.SetBackgroundColor(bg)
	}
	h.restylePluginTable()
}

func (h *Host) restylePluginTable() {
	table := h.PluginsList
	if table == nil {
		return
	}
	table.SetBackgroundColor(ui.ColorAppBg)
	table.Box.SetBackgroundColor(ui.ColorAppBg)
	table.SetSelectedStyle(tcell.StyleDefault.
		Foreground(ui.ColorHighlightText).
		Background(ui.ColorHighlight))
	for row := 0; row < table.GetRowCount(); row++ {
		cell := table.GetCell(row, 0)
		if cell == nil {
			continue
		}
		if row == 0 {
			continue
		}
		cell.SetTextColor(ui.ColorTableRow)
		cell.SetBackgroundColor(ui.ColorAppBg)
	}
	h.paintPluginHeader()
}

func (d *Dashboard) ApplyTheme() {
	if d == nil {
		return
	}
	if d.root != nil {
		d.root.SetBackgroundColor(ui.ColorAppBg)
	}
	if d.grid != nil {
		d.grid.SetBackgroundColor(ui.ColorAppBg)
	}
	for _, card := range d.cards {
		card.SetBackgroundColor(ui.ColorAppBg)
	}
	d.paintSelection()
}

func (m *PluginManager) ApplyTheme() {
	if m == nil {
		return
	}
	type job struct {
		r *RPCRenderer
		v *pluginrpc.ViewData
	}
	m.mu.Lock()
	jobs := make([]job, 0, len(m.sessions))
	for _, sess := range m.sessions {
		if sess == nil || sess.Renderer == nil {
			continue
		}
		jobs = append(jobs, job{r: sess.Renderer, v: sess.Cached})
	}
	m.mu.Unlock()
	for _, j := range jobs {
		j.r.ApplyTheme()
		if j.v != nil {
			j.r.Apply(*j.v)
		}
	}
}

func (r *RPCRenderer) ApplyTheme() {
	if r == nil || r.core == nil {
		return
	}
	r.core.ApplyTheme()
}

package packagemanager

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"omo/pkg/pluginapi"
	"omo/pkg/ui"

	"github.com/rivo/tview"
)

const (
	viewAll       = "all"
	viewInstalled = "installed"
	viewUpdates   = "updates"
	viewAvailable = "available"
)

// Manager is the Package Manager UI controller.
type Manager struct {
	core       *ui.CoreView
	app        *tview.Application
	pages      *tview.Pages
	index      *pluginapi.PluginIndex
	filterView string
	onClose    func()
	statusMsg  string
	closed     bool
}

// New builds the Package Manager. onClose runs when the user exits (Q/ESC);
// the host should RefreshPlugins, restore MainFrame, and RemovePage.
func New(app *tview.Application, pages *tview.Pages, onClose func()) *Manager {
	m := &Manager{
		app:        app,
		pages:      pages,
		filterView: viewAll,
		onClose:    onClose,
	}
	m.core = ui.NewCoreView(app, "Package Manager")
	m.core.SetModalPages(pages)
	m.core.SetTableHeaders([]string{"", "Plugin", "Installed", "Latest", "Status", "Tags"})
	m.core.SetSelectionKey("Plugin")
	m.core.SetViewStack([]string{"omo", "pkgmgr"})

	m.core.SetRefreshCallback(m.refreshRows)
	m.core.SetRowSelectedCallback(func(row int) {
		data := m.core.GetTableData()
		if row >= 0 && row < len(data) && len(data[row]) > 1 {
			m.showDetail(data[row][1])
			m.rebindChrome()
		}
	})
	m.core.SetActionCallback(m.handleCoreAction)

	m.installHelp()
	m.rebindChrome()
	m.core.RegisterHandlers()
	m.core.RefreshData()
	m.maybeAutoSync()
	return m
}

// GetLayout returns the CoreView layout for embedding.
func (m *Manager) GetLayout() tview.Primitive { return m.core.GetLayout() }

// GetTable returns the table for focus.
func (m *Manager) GetTable() *ui.Table { return m.core.GetTable() }

// Destroy cleans up the CoreView.
func (m *Manager) Destroy() {
	if m.core != nil {
		m.core.Destroy()
	}
}

func (m *Manager) handleCoreAction(action string, payload map[string]interface{}) error {
	switch action {
	case "rowSelected":
		if rowData, ok := payload["rowData"].([]string); ok && len(rowData) > 1 {
			m.showDetail(rowData[1])
			m.rebindChrome()
		}
		return nil
	case "navigate_back", "back":
		m.close()
		return nil
	default:
		return fmt.Errorf("unhandled")
	}
}

func (m *Manager) close() {
	if m.closed {
		return
	}
	m.closed = true
	m.core.UnregisterHandlers()
	m.core.StopAutoRefresh()
	if m.onClose != nil {
		m.onClose()
	}
}

func (m *Manager) setStatus(msg string) {
	m.statusMsg = msg
	row := m.core.GetSelectedRowData()
	if len(row) > 1 {
		m.showDetail(row[1])
	} else {
		m.showOverview()
	}
}

func (m *Manager) installHelp() {
	views := []ui.KeyBindingHelp{
		{Key: "0", Label: "All"},
		{Key: "1", Label: "Installed"},
		{Key: "2", Label: "Updates"},
		{Key: "3", Label: "Available"},
	}
	actions := []ui.KeyBindingHelp{
		{Key: "I", Label: "Install"},
		{Key: "U", Label: "Update"},
		{Key: "D", Label: "Remove"},
		{Key: "S", Label: "Sync index"},
		{Key: "A", Label: "Install all"},
		{Key: "Z", Label: "Update all"},
		{Key: "Q", Label: "Back"},
	}
	global := []ui.KeyBindingHelp{
		{Key: "R", Label: "Refresh list"},
		{Key: "/", Label: "Filter / search"},
		{Key: "?", Label: "Help"},
		{Key: "ESC", Label: "Back"},
	}
	m.core.SetHelpSections([]ui.HelpSection{
		{Title: "Views (0-3)", Bindings: views},
		{Title: "Actions", Bindings: actions},
		{Title: "Global", Bindings: global},
	})
}

func (m *Manager) rebindChrome() {
	m.core.ClearKeyBindings()

	m.core.AddViewBinding("0", "All", viewAll, func() { m.setFilterView(viewAll) })
	m.core.AddViewBinding("1", "Installed", viewInstalled, func() { m.setFilterView(viewInstalled) })
	m.core.AddViewBinding("2", "Updates", viewUpdates, func() { m.setFilterView(viewUpdates) })
	m.core.AddViewBinding("3", "Available", viewAvailable, func() { m.setFilterView(viewAvailable) })
	m.core.SetActiveView(m.filterView)

	m.core.AddKeyBinding("R", "Refresh", m.refreshLocal)
	m.core.AddKeyBinding("?", "Help", func() { m.core.ShowHelpModal() })
	m.core.AddKeyBinding("/", "Filter", func() { m.core.ShowFilterModal() })
	m.core.AddKeyBinding("S", "Sync Index", m.syncIndex)
	m.core.AddKeyBinding("A", "Install All", m.installAll)
	m.core.AddKeyBinding("Z", "Update All", m.updateAll)
	m.core.AddKeyBinding("Q", "Back", m.close)

	row := m.core.GetSelectedRowData()
	name := ""
	if len(row) > 1 {
		name = row[1]
	}
	installed := name != "" && pluginapi.IsInstalled(name)
	latest := ""
	if name != "" && m.index != nil {
		if e := findEntry(m.index, name); e != nil {
			latest = e.Version
		}
	}
	current := ""
	if installed {
		current = pluginapi.InstalledVersion(name)
	}

	switch {
	case name != "" && !installed:
		m.core.AddKeyBinding("I", "Install", m.installSelected)
	case installed && current != latest && latest != "":
		m.core.AddKeyBinding("U", "Update", m.updateSelected)
		m.core.AddKeyBinding("D", "Remove", m.removeSelected)
	case installed:
		m.core.AddKeyBinding("D", "Remove", m.removeSelected)
	}
}

func (m *Manager) setFilterView(id string) {
	m.filterView = id
	m.core.SetActiveView(id)
	m.core.RefreshData()
	m.rebindChrome()
	m.app.SetFocus(m.core.GetTable())
}

func (m *Manager) refreshLocal() {
	m.setStatus("[yellow]Refreshing…")
	m.core.RefreshData()
	m.setStatus(fmt.Sprintf("[green]Refreshed · %s/%s", runtime.GOOS, runtime.GOARCH))
	m.rebindChrome()
}

func (m *Manager) refreshRows() ([][]string, error) {
	if m.index == nil {
		cached, err := pluginapi.LoadLocalIndex()
		if err == nil && cached != nil {
			m.index = cached
		}
	}
	if m.index == nil || len(m.index.Plugins) == 0 {
		m.setStatus("[yellow]No index — press S to sync")
		return [][]string{}, nil
	}

	rows := make([][]string, 0, len(m.index.Plugins))
	for _, entry := range m.index.Plugins {
		installed := pluginapi.IsInstalled(entry.Name)
		installedVer := pluginapi.InstalledVersion(entry.Name)
		updateAvail := installed && installedVer != entry.Version
		available := !installed

		switch m.filterView {
		case viewInstalled:
			if !installed {
				continue
			}
		case viewUpdates:
			if !updateAvail {
				continue
			}
		case viewAvailable:
			if !available {
				continue
			}
		}

		icon := " "
		status := "[red]Not installed"
		verDisplay := "[gray]-"
		if installed {
			icon = "[green]●"
			if installedVer != "" {
				verDisplay = "[white]" + installedVer
			} else {
				verDisplay = "[yellow]?"
			}
			if updateAvail {
				icon = "[yellow]●"
				status = "[yellow::b]Update available"
			} else {
				status = "[green]Up to date"
			}
		}

		if !entry.SupportsArch() {
			status = "[red]Unsupported arch"
		} else if entry.Checksum() == "" && !installed {
			status = status + " [gray](no checksum)"
		}

		tags := "[gray]" + strings.Join(entry.Tags, ", ")
		rows = append(rows, []string{icon, entry.Name, verDisplay, entry.Version, status, tags})
	}

	if len(m.core.GetSelectedRowData()) <= 1 {
		m.showOverviewFrom(rows)
	}
	return rows, nil
}

func (m *Manager) showOverview() {
	total := 0
	installed := 0
	updates := 0
	totalSize := int64(0)
	if m.index != nil {
		total = len(m.index.Plugins)
		for _, entry := range m.index.Plugins {
			if !pluginapi.IsInstalled(entry.Name) {
				continue
			}
			installed++
			if pluginapi.InstalledVersion(entry.Name) != entry.Version {
				updates++
			}
			if path, ok := pluginapi.InstalledPluginPath(entry.Name); ok {
				if info, err := os.Stat(path); err == nil {
					totalSize += info.Size()
				}
			}
		}
	}
	m.showOverviewStats(total, installed, updates, totalSize)
}

func (m *Manager) showOverviewFrom(data [][]string) {
	total := len(data)
	if m.index != nil {
		total = len(m.index.Plugins)
	}
	installed := 0
	updates := 0
	totalSize := int64(0)
	if m.index != nil {
		for _, entry := range m.index.Plugins {
			if !pluginapi.IsInstalled(entry.Name) {
				continue
			}
			installed++
			if pluginapi.InstalledVersion(entry.Name) != entry.Version {
				updates++
			}
			if path, ok := pluginapi.InstalledPluginPath(entry.Name); ok {
				if info, err := os.Stat(path); err == nil {
					totalSize += info.Size()
				}
			}
		}
	}
	_ = data
	m.showOverviewStats(total, installed, updates, totalSize)
}

func (m *Manager) showOverviewStats(total, installed, updates int, totalSize int64) {
	available := total - installed
	statusLine := ""
	if m.statusMsg != "" {
		statusLine = "\n\n" + m.statusMsg
	}
	text := "[aqua::b]── Overview ──[white::-]\n\n" +
		"[aqua]Total:      [white::b]" + strconv.Itoa(total) + "[white::-]\n" +
		"[aqua]Installed:  [green::b]" + strconv.Itoa(installed) + "[white::-]\n" +
		"[aqua]Available:  [red]" + strconv.Itoa(available) + "[white::-]\n" +
		"[aqua]Updates:    [yellow::b]" + strconv.Itoa(updates) + "[white::-]\n" +
		"[aqua]Disk Usage: [white]" + formatBytes(totalSize) + "\n\n" +
		"[aqua::b]── Platform ──[white::-]\n\n" +
		"[aqua]OS:         [white]" + runtime.GOOS + "\n" +
		"[aqua]Arch:       [white]" + runtime.GOARCH + "\n" +
		"[aqua]View:       [white]" + m.filterView +
		statusLine
	m.core.SetInfoText(text)
}

func (m *Manager) showDetail(name string) {
	if m.index == nil {
		return
	}
	entry := findEntry(m.index, name)
	if entry == nil {
		return
	}
	installed := pluginapi.IsInstalled(entry.Name)
	installedVer := pluginapi.InstalledVersion(entry.Name)

	statusLine := "[red]Not installed"
	if installed {
		if installedVer == entry.Version {
			statusLine = "[green]● Installed (up to date)"
		} else {
			statusLine = fmt.Sprintf("[yellow]● Installed (%s → %s)", installedVer, entry.Version)
		}
	}
	if !entry.SupportsArch() {
		statusLine += "\n[red]This platform arch is not listed for this plugin"
	}

	pluginPath, ok := pluginapi.InstalledPluginPath(entry.Name)
	sizeStr := "-"
	if ok {
		if info, err := os.Stat(pluginPath); err == nil {
			sizeStr = formatBytes(info.Size())
		}
	}

	checksumLine := "[yellow]not provided"
	if cs := entry.Checksum(); cs != "" {
		checksumLine = "[green]sha256:" + cs[:12] + "..."
	}

	urlLine := entry.URL
	if urlLine == "" {
		urlLine = "-"
	}

	statusLineExtra := ""
	if m.statusMsg != "" {
		statusLineExtra = "\n\n" + m.statusMsg
	}

	text := fmt.Sprintf(
		"[aqua::b]%s[white::-]\n\n"+
			"[aqua]Status:     [white]%s\n"+
			"[aqua]Version:    [white]%s\n"+
			"[aqua]Author:     [white]%s\n"+
			"[aqua]License:    [white]%s\n"+
			"[aqua]Arch:       [white]%s\n"+
			"[aqua]Size:       [white]%s\n"+
			"[aqua]Integrity:  [white]%s\n"+
			"[aqua]URL:        [white]%s\n"+
			"[aqua]Tags:       [white]%s\n\n"+
			"[gray]%s%s",
		entry.Name,
		statusLine,
		entry.Version,
		entry.Author,
		entry.License,
		strings.Join(entry.Arch, ", "),
		sizeStr,
		checksumLine,
		urlLine,
		strings.Join(entry.Tags, ", "),
		entry.Description,
		statusLineExtra,
	)
	m.core.SetInfoText(text)
}

func (m *Manager) maybeAutoSync() {
	go func() {
		cached, err := pluginapi.LoadLocalIndex()
		if err == nil && cached != nil && len(cached.Plugins) > 0 {
			return
		}
		m.app.QueueUpdateDraw(func() {
			m.setStatus("[yellow]Auto-syncing plugin index…")
		})
		fetched, err := pluginapi.FetchIndex("")
		m.app.QueueUpdateDraw(func() {
			if err != nil {
				m.setStatus(fmt.Sprintf("[red]Auto-sync failed: %v", err))
				return
			}
			m.index = fetched
			_ = pluginapi.SaveLocalIndex(m.index)
			m.setStatus(fmt.Sprintf("[green]Index synced — %d plugins", len(m.index.Plugins)))
			m.core.RefreshData()
			m.rebindChrome()
		})
	}()
}

func (m *Manager) syncIndex() {
	m.setStatus("[yellow]Syncing plugin index…")
	go func() {
		fetched, err := pluginapi.FetchIndex("")
		m.app.QueueUpdateDraw(func() {
			if err != nil {
				data, readErr := os.ReadFile("index.yaml")
				if readErr != nil {
					m.setStatus(fmt.Sprintf("[red]Sync failed: %v", err))
					return
				}
				parsed, parseErr := pluginapi.ParseIndex(data)
				if parseErr != nil {
					m.setStatus(fmt.Sprintf("[red]Local index parse failed: %v", parseErr))
					return
				}
				m.index = parsed
				_ = pluginapi.SaveLocalIndex(m.index)
				m.setStatus(fmt.Sprintf("[green]Loaded %d plugins from local index.yaml", len(m.index.Plugins)))
				m.core.RefreshData()
				m.rebindChrome()
				return
			}
			m.index = fetched
			_ = pluginapi.SaveLocalIndex(m.index)
			m.setStatus(fmt.Sprintf("[green]Index synced — %d plugins", len(m.index.Plugins)))
			m.core.RefreshData()
			m.rebindChrome()
		})
	}()
}

func (m *Manager) installSelected() {
	row := m.core.GetSelectedRowData()
	if len(row) < 2 {
		m.setStatus("[red]No plugin selected")
		return
	}
	name := row[1]
	if pluginapi.IsInstalled(name) {
		m.setStatus(fmt.Sprintf("[yellow]%s is already installed", name))
		return
	}
	if m.index == nil {
		m.setStatus("[red]No index — press S to sync")
		return
	}
	entry := findEntry(m.index, name)
	if entry == nil {
		return
	}
	if !entry.SupportsArch() {
		m.setStatus(fmt.Sprintf("[red]%s does not support %s", name, runtime.GOARCH))
		return
	}
	if entry.Checksum() == "" {
		m.setStatus(fmt.Sprintf("[yellow]Warning: %s has no checksum for this platform", name))
	}

	pm := ui.NewProgressModal(m.pages, m.app, fmt.Sprintf("Installing %s", name), 100)
	pm.SetCancellable(false)
	pm.SetAutoClose(false)
	pm.Show()
	m.setStatus(fmt.Sprintf("[yellow]Installing %s v%s…", name, entry.Version))

	go func() {
		onProgress := func(downloaded, total int64) {
			pct := 0
			status := fmt.Sprintf("Downloading %s... %s", name, formatBytes(downloaded))
			if total > 0 {
				pct = int(float64(downloaded) / float64(total) * 90)
				status = fmt.Sprintf("Downloading %s... %s / %s", name, formatBytes(downloaded), formatBytes(total))
			}
			pm.UpdateProgress(pct, status)
		}
		err := downloadPlugin(entry, m.index.DownloadURLTemplate, onProgress)
		if err != nil {
			pm.UpdateProgress(100, fmt.Sprintf("[red]Failed: %v", err))
			m.app.QueueUpdateDraw(func() {
				m.setStatus(fmt.Sprintf("[red]Install failed for %s: %v", name, err))
			})
			time.AfterFunc(2*time.Second, func() {
				m.app.QueueUpdateDraw(func() { pm.Close() })
			})
			return
		}
		pm.UpdateProgress(95, fmt.Sprintf("Verifying %s...", name))
		_ = pluginapi.RecordInstalledVersion(name, entry.Version)
		pm.UpdateProgress(100, fmt.Sprintf("[green]%s installed!", name))
		m.app.QueueUpdateDraw(func() {
			m.setStatus(fmt.Sprintf("[green]%s v%s installed", name, entry.Version))
		})
		time.AfterFunc(time.Second, func() {
			m.app.QueueUpdateDraw(func() {
				pm.Close()
				m.core.RefreshData()
				m.rebindChrome()
			})
		})
	}()
}

func (m *Manager) updateSelected() {
	row := m.core.GetSelectedRowData()
	if len(row) < 2 {
		m.setStatus("[red]No plugin selected")
		return
	}
	name := row[1]
	if !pluginapi.IsInstalled(name) {
		m.setStatus(fmt.Sprintf("[yellow]%s is not installed — press I", name))
		return
	}
	if m.index == nil {
		m.setStatus("[red]No index loaded")
		return
	}
	entry := findEntry(m.index, name)
	if entry == nil {
		return
	}
	current := pluginapi.InstalledVersion(name)
	if current == entry.Version {
		m.setStatus(fmt.Sprintf("[green]%s already at latest (%s)", name, current))
		return
	}
	if !entry.SupportsArch() {
		m.setStatus(fmt.Sprintf("[red]%s does not support %s", name, runtime.GOARCH))
		return
	}

	pm := ui.NewProgressModal(m.pages, m.app, fmt.Sprintf("Updating %s", name), 100)
	pm.SetCancellable(false)
	pm.SetAutoClose(false)
	pm.Show()
	m.setStatus(fmt.Sprintf("[yellow]Updating %s: %s → %s…", name, current, entry.Version))

	go func() {
		onProgress := func(downloaded, total int64) {
			pct := 0
			status := fmt.Sprintf("Downloading %s... %s", name, formatBytes(downloaded))
			if total > 0 {
				pct = int(float64(downloaded) / float64(total) * 90)
				status = fmt.Sprintf("Downloading %s... %s / %s", name, formatBytes(downloaded), formatBytes(total))
			}
			pm.UpdateProgress(pct, status)
		}
		err := downloadPlugin(entry, m.index.DownloadURLTemplate, onProgress)
		if err != nil {
			pm.UpdateProgress(100, fmt.Sprintf("[red]Failed: %v", err))
			m.app.QueueUpdateDraw(func() {
				m.setStatus(fmt.Sprintf("[red]Update failed for %s: %v", name, err))
			})
			time.AfterFunc(2*time.Second, func() {
				m.app.QueueUpdateDraw(func() { pm.Close() })
			})
			return
		}
		pm.UpdateProgress(95, fmt.Sprintf("Verifying %s...", name))
		_ = pluginapi.RecordInstalledVersion(name, entry.Version)
		pm.UpdateProgress(100, fmt.Sprintf("[green]%s updated!", name))
		m.app.QueueUpdateDraw(func() {
			m.setStatus(fmt.Sprintf("[green]%s updated to v%s", name, entry.Version))
		})
		time.AfterFunc(time.Second, func() {
			m.app.QueueUpdateDraw(func() {
				pm.Close()
				m.core.RefreshData()
				m.rebindChrome()
			})
		})
	}()
}

func (m *Manager) removeSelected() {
	row := m.core.GetSelectedRowData()
	if len(row) < 2 {
		m.setStatus("[red]No plugin selected")
		return
	}
	name := row[1]
	if !pluginapi.IsInstalled(name) {
		m.setStatus(fmt.Sprintf("[yellow]%s is not installed", name))
		return
	}
	pluginPath, ok := pluginapi.InstalledPluginPath(name)
	sizeStr := ""
	if ok {
		if info, err := os.Stat(pluginPath); err == nil {
			sizeStr = fmt.Sprintf(" (%s)", formatBytes(info.Size()))
		}
	}
	ui.ShowStandardConfirmationModal(
		m.pages, m.app,
		"Remove Plugin",
		fmt.Sprintf("Remove [red::b]%s[white::-]%s?\n\nThis will delete the plugin binary.", name, sizeStr),
		func(confirmed bool) {
			if !confirmed {
				m.app.SetFocus(m.core.GetTable())
				return
			}
			pluginDir := filepath.Join(pluginapi.PluginsDir(), name)
			if err := os.RemoveAll(pluginDir); err != nil {
				m.setStatus(fmt.Sprintf("[red]Remove failed: %v", err))
			} else {
				_ = pluginapi.RemoveInstalledRecord(name)
				m.setStatus(fmt.Sprintf("[green]%s removed", name))
				m.core.RefreshData()
				m.rebindChrome()
			}
			m.app.SetFocus(m.core.GetTable())
		},
	)
}

func (m *Manager) installAll() {
	if m.index == nil {
		m.setStatus("[red]No index — press S to sync")
		return
	}
	var toInstall []pluginapi.IndexEntry
	for _, entry := range m.index.Plugins {
		if pluginapi.IsInstalled(entry.Name) {
			continue
		}
		if !entry.SupportsArch() {
			continue
		}
		toInstall = append(toInstall, entry)
	}
	if len(toInstall) == 0 {
		m.setStatus("[green]Nothing to install for this platform")
		return
	}
	ui.ShowStandardConfirmationModal(
		m.pages, m.app,
		"Install All",
		fmt.Sprintf("Install [aqua::b]%d[white::-] plugins?\n\n%s", len(toInstall), pluginNameList(toInstall)),
		func(confirmed bool) {
			if !confirmed {
				m.app.SetFocus(m.core.GetTable())
				return
			}
			m.app.SetFocus(m.core.GetTable())
			m.runBulkInstall(toInstall)
		},
	)
}

func (m *Manager) runBulkInstall(toInstall []pluginapi.IndexEntry) {
	total := len(toInstall)
	pm := ui.NewProgressModal(m.pages, m.app, "Installing All Plugins", total)
	pm.SetCancellable(false)
	pm.SetAutoClose(false)
	pm.Show()
	m.setStatus(fmt.Sprintf("[yellow]Installing %d plugins…", total))

	go func() {
		var (
			mu        sync.Mutex
			installed int
			failed    int
			done      int
			wg        sync.WaitGroup
		)
		sem := make(chan struct{}, 3)
		for _, entry := range toInstall {
			wg.Add(1)
			go func(e pluginapi.IndexEntry) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				err := downloadPlugin(&e, m.index.DownloadURLTemplate, nil)
				mu.Lock()
				done++
				cur := done
				if err != nil {
					failed++
					mu.Unlock()
					pm.UpdateProgress(cur, fmt.Sprintf("Failed %s (%d/%d)", e.Name, cur, total))
					return
				}
				_ = pluginapi.RecordInstalledVersion(e.Name, e.Version)
				installed++
				mu.Unlock()
				pm.UpdateProgress(cur, fmt.Sprintf("Installed %s (%d/%d)", e.Name, cur, total))
			}(entry)
		}
		wg.Wait()
		msg := fmt.Sprintf("[green]Done: %d installed, %d failed", installed, failed)
		if failed == 0 {
			msg = fmt.Sprintf("[green]All %d plugins installed!", installed)
		}
		pm.UpdateProgress(total, msg)
		m.app.QueueUpdateDraw(func() {
			m.setStatus(msg)
		})
		time.AfterFunc(2*time.Second, func() {
			m.app.QueueUpdateDraw(func() {
				pm.Close()
				m.core.RefreshData()
				m.rebindChrome()
			})
		})
	}()
}

func (m *Manager) updateAll() {
	if m.index == nil {
		m.setStatus("[red]No index loaded")
		return
	}
	var toUpdate []pluginapi.IndexEntry
	for _, entry := range m.index.Plugins {
		if !pluginapi.IsInstalled(entry.Name) {
			continue
		}
		if pluginapi.InstalledVersion(entry.Name) == entry.Version {
			continue
		}
		if !entry.SupportsArch() {
			continue
		}
		toUpdate = append(toUpdate, entry)
	}
	if len(toUpdate) == 0 {
		m.setStatus("[green]All installed plugins are up to date")
		return
	}
	ui.ShowStandardConfirmationModal(
		m.pages, m.app,
		"Update All",
		fmt.Sprintf("Update [aqua::b]%d[white::-] plugins?\n\n%s", len(toUpdate), pluginNameList(toUpdate)),
		func(confirmed bool) {
			if !confirmed {
				m.app.SetFocus(m.core.GetTable())
				return
			}
			m.app.SetFocus(m.core.GetTable())
			m.runBulkUpdate(toUpdate)
		},
	)
}

func (m *Manager) runBulkUpdate(toUpdate []pluginapi.IndexEntry) {
	total := len(toUpdate)
	pm := ui.NewProgressModal(m.pages, m.app, "Updating All Plugins", total)
	pm.SetCancellable(false)
	pm.SetAutoClose(false)
	pm.Show()
	m.setStatus(fmt.Sprintf("[yellow]Updating %d plugins…", total))

	go func() {
		updated := 0
		failed := 0
		for i, entry := range toUpdate {
			pm.UpdateProgress(i, fmt.Sprintf("Updating %s (%d/%d)…", entry.Name, i+1, total))
			if err := downloadPlugin(&entry, m.index.DownloadURLTemplate, nil); err != nil {
				failed++
				continue
			}
			_ = pluginapi.RecordInstalledVersion(entry.Name, entry.Version)
			updated++
		}
		msg := fmt.Sprintf("[yellow]Done: %d updated, %d failed", updated, failed)
		if failed == 0 {
			msg = fmt.Sprintf("[green]All %d plugins updated!", updated)
		}
		pm.UpdateProgress(total, msg)
		m.app.QueueUpdateDraw(func() { m.setStatus(msg) })
		time.AfterFunc(2*time.Second, func() {
			m.app.QueueUpdateDraw(func() {
				pm.Close()
				m.core.RefreshData()
				m.rebindChrome()
			})
		})
	}()
}

// --- download / extract (unchanged pipeline) ---

type progressFunc func(downloaded, total int64)

func downloadPlugin(entry *pluginapi.IndexEntry, urlTemplate string, onProgress progressFunc) error {
	url := entry.DownloadURL(urlTemplate)
	if err := pluginapi.EnsurePluginDirs(entry.Name); err != nil {
		return fmt.Errorf("create dirs: %w", err)
	}
	destPath := pluginapi.PluginBinPath(entry.Name)
	tmpPath := destPath + ".tmp"
	archivePath := destPath + ".download"

	client := pluginapi.NewHTTPClient(300 * time.Second)
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	totalBytes := resp.ContentLength
	archiveFile, err := os.Create(archivePath)
	if err != nil {
		return fmt.Errorf("create archive: %w", err)
	}
	hasher := sha256.New()
	dest := io.MultiWriter(archiveFile, hasher)
	var src io.Reader = resp.Body
	if onProgress != nil {
		src = &progressReader{reader: resp.Body, total: totalBytes, onProgress: onProgress}
	}
	if _, err := io.Copy(dest, src); err != nil {
		archiveFile.Close()
		os.Remove(archivePath)
		return fmt.Errorf("download write: %w", err)
	}
	archiveFile.Close()

	expectedHash := entry.Checksum()
	if expectedHash != "" {
		actualHash := hex.EncodeToString(hasher.Sum(nil))
		if actualHash != expectedHash {
			os.Remove(archivePath)
			return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash[:16]+"...", actualHash[:16]+"...")
		}
	}

	archiveReader, err := os.Open(archivePath)
	if err != nil {
		os.Remove(archivePath)
		return fmt.Errorf("reopen archive: %w", err)
	}
	defer func() {
		archiveReader.Close()
		os.Remove(archivePath)
	}()

	if strings.HasSuffix(url, ".tar.gz") || strings.HasSuffix(url, ".tgz") {
		if err := extractPluginFromTarGz(archiveReader, entry.Name, tmpPath); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("extract: %w", err)
		}
	} else {
		archiveReader.Close()
		if err := os.Rename(archivePath, tmpPath); err != nil {
			return fmt.Errorf("rename: %w", err)
		}
	}
	if err := os.Chmod(tmpPath, 0755); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, destPath)
}

type progressReader struct {
	reader     io.Reader
	total      int64
	downloaded int64
	onProgress progressFunc
	lastReport time.Time
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.downloaded += int64(n)
	if time.Since(pr.lastReport) > 100*time.Millisecond || err == io.EOF {
		pr.lastReport = time.Now()
		pr.onProgress(pr.downloaded, pr.total)
	}
	return n, err
}

func extractPluginFromTarGz(r io.Reader, pluginName, destPath string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var anyFile []byte
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		name := filepath.Base(hdr.Name)
		data, err := io.ReadAll(tr)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		if name == pluginName {
			return os.WriteFile(destPath, data, 0755)
		}
		if anyFile == nil && name != "" && !strings.HasPrefix(name, ".") {
			anyFile = data
		}
	}
	if anyFile != nil {
		return os.WriteFile(destPath, anyFile, 0755)
	}
	return fmt.Errorf("no plugin binary %q found in archive", pluginName)
}

func findEntry(index *pluginapi.PluginIndex, name string) *pluginapi.IndexEntry {
	for i := range index.Plugins {
		if index.Plugins[i].Name == name {
			return &index.Plugins[i]
		}
	}
	return nil
}

func pluginNameList(entries []pluginapi.IndexEntry) string {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name
	}
	return strings.Join(names, ", ")
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

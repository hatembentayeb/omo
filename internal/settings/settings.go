package settings

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"omo/pkg/pluginapi"
	"omo/pkg/secrets"
	"omo/pkg/ui"

	"github.com/rivo/tview"
)

const (
	viewOverview = "overview"
	viewPaths    = "paths"
	viewPlugins  = "plugins"
	viewSecrets  = "secrets"
	viewLogs     = "logs"
	viewEnv      = "env"
)

// Manager is the host Settings / Info UI (sibling of Package Manager).
type Manager struct {
	core      *ui.CoreView
	app       *tview.Application
	pages     *tview.Pages
	version   string
	viewID    string
	onClose   func()
	onRefresh func() // host RefreshPlugins
	statusMsg string
	closed    bool
}

// New builds Settings. onClose should restore MainFrame / focus plugins list.
// onRefreshPlugins is optional (sidebar plugin list reload).
func New(app *tview.Application, pages *tview.Pages, version string, onRefreshPlugins, onClose func()) *Manager {
	if version == "" {
		version = "dev"
	}
	m := &Manager{
		app:       app,
		pages:     pages,
		version:   version,
		viewID:    viewOverview,
		onClose:   onClose,
		onRefresh: onRefreshPlugins,
	}
	m.core = ui.NewCoreView(app, "Settings")
	m.core.SetModalPages(pages)
	m.core.SetTableHeaders([]string{"Item", "Value", "Detail"})
	m.core.SetSelectionKey("Item")
	m.core.SetViewStack([]string{"omo", "settings"})

	m.core.SetRefreshCallback(m.refreshRows)
	m.core.SetRowSelectedCallback(func(row int) {
		m.showRowDetail(row)
		m.rebindChrome()
	})
	m.core.SetActionCallback(m.handleCoreAction)

	m.installHelp()
	m.rebindChrome()
	m.core.RegisterHandlers()
	m.core.RefreshData()
	return m
}

func (m *Manager) GetLayout() tview.Primitive { return m.core.GetLayout() }
func (m *Manager) GetTable() *ui.Table        { return m.core.GetTable() }

func (m *Manager) ApplyTheme() {
	if m == nil || m.core == nil {
		return
	}
	m.core.ApplyTheme()
	m.refreshRows()
}

// DetachHeader lifts Info | Views | Actions | Logo for the host header row.
func (m *Manager) DetachHeader() tview.Primitive { return m.core.DetachHeader() }

// SetHeaderLogo places the OMO mark on the right of the settings header.
func (m *Manager) SetHeaderLogo(p tview.Primitive) { m.core.SetHeaderLogo(p) }

func (m *Manager) Destroy() {
	if m.core != nil {
		m.core.Destroy()
	}
}

func (m *Manager) handleCoreAction(action string, payload map[string]interface{}) error {
	switch action {
	case "rowSelected":
		if row, ok := payload["row"].(int); ok {
			m.showRowDetail(row)
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
	m.updateInfo()
}

func (m *Manager) installHelp() {
	m.core.SetHelpSections([]ui.HelpSection{
		{Title: "Views (0-5)", Bindings: []ui.KeyBindingHelp{
			{Key: "0", Label: "Overview"},
			{Key: "1", Label: "Paths"},
			{Key: "2", Label: "Plugins"},
			{Key: "3", Label: "Secrets"},
			{Key: "4", Label: "Logs"},
			{Key: "5", Label: "Env"},
		}},
		{Title: "Actions", Bindings: []ui.KeyBindingHelp{
			{Key: "S", Label: "Sync plugin index"},
			{Key: "P", Label: "Show paths"},
			{Key: "K", Label: "Reload secrets"},
			{Key: "L", Label: "Clear logs"},
			{Key: "X", Label: "Reset secrets help"},
			{Key: "E", Label: "Row detail"},
			{Key: "Q", Label: "Back"},
		}},
		{Title: "Global", Bindings: []ui.KeyBindingHelp{
			{Key: "R", Label: "Refresh inventory"},
			{Key: "?", Label: "Help"},
			{Key: "ESC", Label: "Back"},
		}},
	})
}

func (m *Manager) rebindChrome() {
	m.core.ClearKeyBindings()
	m.core.AddViewBinding("0", "Overview", viewOverview, func() { m.setView(viewOverview) })
	m.core.AddViewBinding("1", "Paths", viewPaths, func() { m.setView(viewPaths) })
	m.core.AddViewBinding("2", "Plugins", viewPlugins, func() { m.setView(viewPlugins) })
	m.core.AddViewBinding("3", "Secrets", viewSecrets, func() { m.setView(viewSecrets) })
	m.core.AddViewBinding("4", "Logs", viewLogs, func() { m.setView(viewLogs) })
	m.core.AddViewBinding("5", "Env", viewEnv, func() { m.setView(viewEnv) })
	m.core.SetActiveView(m.viewID)

	m.core.AddKeyBinding("R", "Refresh", m.refreshLocal)
	m.core.AddKeyBinding("?", "Help", func() { m.core.ShowHelpModal() })
	m.core.AddKeyBinding("S", "Sync Index", m.syncIndex)
	m.core.AddKeyBinding("P", "Paths", m.showPathsModal)
	m.core.AddKeyBinding("K", "Reload Sec", m.reloadSecrets)
	m.core.AddKeyBinding("L", "Clear Logs", m.clearLogs)
	m.core.AddKeyBinding("X", "Reset Help", m.resetSecretsHelp)
	m.core.AddKeyBinding("E", "Detail", func() {
		m.showRowDetail(m.core.GetSelectedRow())
	})
	m.core.AddKeyBinding("Q", "Back", m.close)
	m.updateInfo()
}

func (m *Manager) setView(id string) {
	m.viewID = id
	m.core.SetActiveView(id)
	m.core.SetViewStack([]string{"omo", "settings", id})
	m.core.RefreshData()
	m.rebindChrome()
	m.app.SetFocus(m.core.GetTable())
}

func (m *Manager) refreshLocal() {
	m.setStatus("[yellow]Refreshing…")
	m.core.RefreshData()
	if m.onRefresh != nil {
		m.onRefresh()
	}
	m.setStatus(fmt.Sprintf("[green]Refreshed · %s/%s", runtime.GOOS, runtime.GOARCH))
	m.rebindChrome()
}

func (m *Manager) refreshRows() ([][]string, error) {
	switch m.viewID {
	case viewPaths:
		return rowsPaths(), nil
	case viewPlugins:
		return rowsPlugins(), nil
	case viewSecrets:
		return rowsSecrets(), nil
	case viewLogs:
		return rowsLogs(), nil
	case viewEnv:
		return rowsEnv(), nil
	default:
		return rowsOverview(m.version), nil
	}
}

func (m *Manager) updateInfo() {
	m.core.SetInfoText(buildInfo(m.version, m.statusMsg))
}

func buildInfo(version, status string) string {
	omo := pluginapi.OmoDir()
	plugins := countInstalledPlugins()
	entries := countSecretEntries()
	logs := countLogFiles()
	indexN := 0
	if idx, err := pluginapi.LoadLocalIndex(); err == nil && idx != nil {
		indexN = len(idx.Plugins)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "[#FF6B00::b]omo[white]  Settings / Info\n")
	fmt.Fprintf(&b, "Version:  [#50FA7B]%s[white]\n", version)
	fmt.Fprintf(&b, "Home:     %s\n", omo)
	fmt.Fprintf(&b, "Plugins:  %d installed · index %d\n", plugins, indexN)
	fmt.Fprintf(&b, "Secrets:  %s\n", secretsStatusLine(entries))
	fmt.Fprintf(&b, "Logs:     %d files under logs/\n", logs)
	fmt.Fprintf(&b, "Runtime:  %s/%s  go %s\n", runtime.GOOS, runtime.GOARCH, runtime.Version())
	if status != "" {
		fmt.Fprintf(&b, "\n%s", status)
	} else {
		fmt.Fprintf(&b, "\n[#666666]Enter/E row detail · P all paths · Q back[white]")
	}
	return b.String()
}

func secretsStatusLine(entries int) string {
	db := secrets.DefaultDBPath()
	key := secrets.DefaultKeyPath()
	dbOK := fileExists(db)
	keyOK := fileExists(key)
	prov := "off"
	if pluginapi.HasSecrets() {
		prov = "loaded"
	}
	switch {
	case dbOK && keyOK:
		return fmt.Sprintf("%s · %d entries · db+key ok", prov, entries)
	case !dbOK:
		return fmt.Sprintf("%s · db missing", prov)
	case !keyOK:
		return fmt.Sprintf("%s · key missing", prov)
	default:
		return prov
	}
}

func rowsOverview(version string) [][]string {
	return [][]string{
		{"version", version, "host binary (ldflags)"},
		{"omo_home", pluginapi.OmoDir(), "all local state"},
		{"plugins", fmt.Sprintf("%d", countInstalledPlugins()), pluginapi.PluginsDir()},
		{"secrets_db", boolMark(fileExists(secrets.DefaultDBPath())), secrets.DefaultDBPath()},
		{"secrets_key", boolMark(fileExists(secrets.DefaultKeyPath())), secrets.DefaultKeyPath()},
		{"index", fileSummary(pluginapi.IndexPath()), pluginapi.IndexPath()},
		{"installed", fileSummary(pluginapi.InstalledManifestPath()), pluginapi.InstalledManifestPath()},
		{"logs", fmt.Sprintf("%d files", countLogFiles()), pluginapi.LogsDir()},
		{"website", "https://oh-myops.com", "product site"},
		{"github", "https://github.com/hatembentayeb/omo", "source / issues"},
		{"cli", "omo secrets …", "vault without GUI"},
	}
}

func rowsPaths() [][]string {
	items := []struct{ name, path, note string }{
		{"omo", pluginapi.OmoDir(), "root"},
		{"plugins", pluginapi.PluginsDir(), "RPC binaries"},
		{"secrets", pluginapi.SecretsDir(), "KeePass dir"},
		{"secrets/omo.kdbx", secrets.DefaultDBPath(), "vault"},
		{"keys", pluginapi.KeysDir(), "key file dir"},
		{"keys/omo.key", secrets.DefaultKeyPath(), "master key — back up"},
		{"index.yaml", pluginapi.IndexPath(), "plugin catalog cache"},
		{"installed.yaml", pluginapi.InstalledManifestPath(), "installed versions"},
		{"logs", pluginapi.LogsDir(), "host + plugin logs"},
	}
	rows := make([][]string, 0, len(items))
	for _, it := range items {
		rows = append(rows, []string{it.name, it.path, pathDetail(it.path, it.note)})
	}
	return rows
}

func rowsPlugins() [][]string {
	dir := pluginapi.PluginsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return [][]string{{"(error)", err.Error(), dir}}
	}
	var rows [][]string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		bin := pluginapi.PluginBinPath(name)
		ver := pluginapi.InstalledVersion(name)
		if ver == "" {
			ver = "—"
		}
		detail := "missing binary"
		if info, err := os.Stat(bin); err == nil && !info.IsDir() {
			detail = fmt.Sprintf("%s · %s", humanSize(info.Size()), info.ModTime().Format("2006-01-02"))
		}
		rows = append(rows, []string{name, ver, detail})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })
	if len(rows) == 0 {
		return [][]string{{"(none)", "—", "open Package Manager (p) to install"}}
	}
	return rows
}

func rowsSecrets() [][]string {
	rows := [][]string{
		{"provider", boolMark(pluginapi.HasSecrets()), "in-process KeePass"},
		{"database", boolMark(fileExists(secrets.DefaultDBPath())), secrets.DefaultDBPath()},
		{"key_file", boolMark(fileExists(secrets.DefaultKeyPath())), secrets.DefaultKeyPath()},
	}
	if !pluginapi.HasSecrets() {
		rows = append(rows, []string{"entries", "—", "provider not loaded"})
		return rows
	}
	list, err := pluginapi.Secrets().List("")
	if err != nil {
		rows = append(rows, []string{"entries", "error", err.Error()})
		return rows
	}
	sort.Strings(list)
	rows = append(rows, []string{"entries", fmt.Sprintf("%d", len(list)), "plugin/env/instance paths"})
	for _, path := range list {
		note := ""
		if e, err := pluginapi.Secrets().Get(path); err == nil && e != nil {
			parts := []string{}
			if e.URL != "" {
				parts = append(parts, "url")
			}
			if e.UserName != "" {
				parts = append(parts, "user")
			}
			if e.Password != "" {
				parts = append(parts, "pass")
			}
			if len(e.CustomAttributes) > 0 {
				parts = append(parts, fmt.Sprintf("%d attrs", len(e.CustomAttributes)))
			}
			note = strings.Join(parts, ", ")
		}
		rows = append(rows, []string{path, "entry", note})
	}
	return rows
}

func rowsLogs() [][]string {
	dir := pluginapi.LogsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return [][]string{{"(error)", err.Error(), dir}}
	}
	var rows [][]string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		path := filepath.Join(dir, name)
		info, err := e.Info()
		if err != nil {
			rows = append(rows, []string{name, "?", path})
			continue
		}
		rows = append(rows, []string{
			name,
			humanSize(info.Size()),
			info.ModTime().Format(time.RFC3339),
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })
	if len(rows) == 0 {
		return [][]string{{"(none)", "—", dir}}
	}
	return rows
}

func rowsEnv() [][]string {
	keys := []string{
		"OMO_SECRETS_RESET",
		"OMO_RPC_LOG",
		"OMO_INSTALL_DIR",
		"HOME",
		"USER",
		"XDG_CONFIG_HOME",
		"KUBECONFIG",
		"DOCKER_HOST",
		"EDITOR",
		"SHELL",
	}
	rows := make([][]string, 0, len(keys)+2)
	rows = append(rows, []string{"GOOS", runtime.GOOS, "compile target"})
	rows = append(rows, []string{"GOARCH", runtime.GOARCH, "compile target"})
	for _, k := range keys {
		v := os.Getenv(k)
		note := "unset"
		if v != "" {
			note = "set"
			if k == "OMO_SECRETS_RESET" && v == "1" {
				note = "wipes db on next New()"
			}
		}
		display := v
		if display == "" {
			display = "—"
		}
		if len(display) > 48 {
			display = display[:45] + "…"
		}
		rows = append(rows, []string{k, display, note})
	}
	return rows
}

func (m *Manager) showRowDetail(row int) {
	data := m.core.GetTableData()
	if row < 0 || row >= len(data) || len(data[row]) < 2 {
		return
	}
	item, value := data[row][0], data[row][1]
	detail := ""
	if len(data[row]) > 2 {
		detail = data[row][2]
	}
	body := fmt.Sprintf("Item:   %s\nValue:  %s\nDetail: %s\n", item, value, detail)
	// Prefer absolute path if value looks like one.
	if strings.HasPrefix(value, "/") || strings.HasPrefix(value, string(os.PathSeparator)) {
		body += "\n" + pathDetail(value, "")
	}
	ui.ShowInfoModal(m.pages, m.app, item, body, func() {
		m.app.SetFocus(m.core.GetTable())
	})
}

func (m *Manager) showPathsModal() {
	var b strings.Builder
	b.WriteString("~/.omo layout (absolute paths)\n\n")
	for _, row := range rowsPaths() {
		fmt.Fprintf(&b, "%-18s  %s\n", row[0], row[1])
	}
	b.WriteString("\nCLI:\n  omo secrets list\n  omo secrets get <plugin/env/name>\n  omo secrets reset --yes   # then restart omo\n")
	ui.ShowInfoModal(m.pages, m.app, "omo paths", b.String(), func() {
		m.app.SetFocus(m.core.GetTable())
	})
}

func (m *Manager) syncIndex() {
	m.setStatus("[yellow]Syncing plugin index…")
	go func() {
		fetched, err := pluginapi.FetchIndex("")
		m.app.QueueUpdateDraw(func() {
			if err != nil {
				m.setStatus(fmt.Sprintf("[red]Sync failed: %v", err))
				return
			}
			if err := pluginapi.SaveLocalIndex(fetched); err != nil {
				m.setStatus(fmt.Sprintf("[red]Save failed: %v", err))
				return
			}
			m.setStatus(fmt.Sprintf("[green]Index synced — %d plugins → %s", len(fetched.Plugins), pluginapi.IndexPath()))
			m.core.RefreshData()
			m.rebindChrome()
		})
	}()
}

func (m *Manager) reloadSecrets() {
	if !pluginapi.HasSecrets() {
		m.setStatus("[red]Secrets provider not loaded")
		return
	}
	if err := pluginapi.Secrets().Reload(); err != nil {
		m.setStatus(fmt.Sprintf("[red]Reload failed: %v", err))
		return
	}
	m.setStatus("[green]Secrets reloaded from KeePass")
	m.core.RefreshData()
	m.rebindChrome()
}

func (m *Manager) clearLogs() {
	ui.ShowStandardConfirmationModal(
		m.pages,
		m.app,
		"Clear logs",
		"Clear log files under ~/.omo/logs?\nOpen log handles may keep writing until restart.",
		func(confirmed bool) {
			m.app.SetFocus(m.core.GetTable())
			if !confirmed {
				m.setStatus("[yellow]Clear logs cancelled")
				return
			}
			n, err := truncateLogs(pluginapi.LogsDir())
			if err != nil {
				m.setStatus(fmt.Sprintf("[red]Clear logs: %v", err))
				return
			}
			m.setStatus(fmt.Sprintf("[green]Cleared %d log file(s)", n))
			m.core.RefreshData()
			m.rebindChrome()
		},
	)
}

func (m *Manager) resetSecretsHelp() {
	body := "Resetting the vault deletes ~/.omo/secrets/omo.kdbx.\n" +
		"The key file (~/.omo/keys/omo.key) is kept.\n\n" +
		"While omo is running the DB is open — use the CLI, then restart:\n\n" +
		"  omo secrets reset --yes\n" +
		"  # or: OMO_SECRETS_RESET=1 omo\n\n" +
		"Back up omo.key first if you still need old vaults."
	ui.ShowInfoModal(m.pages, m.app, "Reset secrets", body, func() {
		m.app.SetFocus(m.core.GetTable())
	})
}

func truncateLogs(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		if err := os.WriteFile(path, nil, 0644); err != nil {
			// try remove if truncate fails
			if rmErr := os.Remove(path); rmErr != nil {
				return n, fmt.Errorf("%s: %v", e.Name(), err)
			}
		}
		n++
	}
	return n, nil
}

func countInstalledPlugins() int {
	entries, err := os.ReadDir(pluginapi.PluginsDir())
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if info, err := os.Stat(pluginapi.PluginBinPath(e.Name())); err == nil && !info.IsDir() {
			n++
		}
	}
	return n
}

func countSecretEntries() int {
	if !pluginapi.HasSecrets() {
		return 0
	}
	list, err := pluginapi.Secrets().List("")
	if err != nil {
		return 0
	}
	return len(list)
}

func countLogFiles() int {
	entries, err := os.ReadDir(pluginapi.LogsDir())
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".log") {
			n++
		}
	}
	return n
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func boolMark(ok bool) string {
	if ok {
		return "yes"
	}
	return "no"
}

func fileSummary(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return "missing"
	}
	return fmt.Sprintf("%s · %s", humanSize(info.Size()), info.ModTime().Format("2006-01-02"))
}

func pathDetail(path, note string) string {
	info, err := os.Stat(path)
	if err != nil {
		if note != "" {
			return note + " · missing"
		}
		return "missing"
	}
	kind := "file"
	if info.IsDir() {
		kind = "dir"
	}
	s := fmt.Sprintf("%s · %s · %s", kind, humanSize(info.Size()), info.ModTime().Format("2006-01-02 15:04"))
	if note != "" {
		return note + " · " + s
	}
	return s
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

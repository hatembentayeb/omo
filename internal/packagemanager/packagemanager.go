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

func NewPackageManager(app *tview.Application, pages *tview.Pages, pluginsDir string) *ui.CoreView {
	core := ui.NewCoreView(app, "Package Manager")
	core.SetModalPages(pages)

	core.SetTableHeaders([]string{"", "Plugin", "Installed", "Latest", "Status", "Tags"})
	core.SetSelectionKey("Plugin")

	core.AddKeyBinding("I", "Install", nil)
	core.AddKeyBinding("U", "Update", nil)
	core.AddKeyBinding("D", "Remove", nil)
	core.AddKeyBinding("S", "Sync Index", nil)
	core.AddKeyBinding("A", "Install All", nil)
	core.AddKeyBinding("Z", "Update All", nil)
	core.AddKeyBinding("Q", "Back", nil)

	var index *pluginapi.PluginIndex

	core.SetRefreshCallback(func() ([][]string, error) {
		return refreshPluginList(core, &index)
	})

	core.SetRowSelectedCallback(func(row int) {
		data := core.GetTableData()
		if row >= 0 && row < len(data) {
			updateDetailPanel(core, &index, data[row][1])
		}
	})

	core.SetActionCallback(func(action string, payload map[string]interface{}) error {
		if action == "rowSelected" {
			if rowData, ok := payload["rowData"].([]string); ok && len(rowData) > 1 {
				updateDetailPanel(core, &index, rowData[1])
			}
			return nil
		}

		if action != "keypress" {
			return nil
		}
		key, ok := payload["key"].(string)
		if !ok {
			return nil
		}

		switch key {
		case "S":
			handleSyncIndex(core, app, pages, &index)
		case "I":
			handleInstallPlugin(core, app, pages, index)
		case "U":
			handleUpdatePlugin(core, app, pages, index)
		case "D":
			handleRemovePlugin(core, app, pages)
		case "A":
			handleInstallAll(core, app, pages, index)
		case "Z":
			handleUpdateAll(core, app, pages, index)
		case "Q":
			core.UnregisterHandlers()
			core.StopAutoRefresh()
			pages.SwitchToPage("main")
		case "?":
			ui.ShowInfoModal(pages, app, "Package Manager Help", helpText(), func() {
				app.SetFocus(core.GetTable())
			})
		}
		return nil
	})

	core.RefreshData()
	core.RegisterHandlers()
	core.StartAutoRefresh(120 * time.Second)

	if index == nil {
		go func() {
			cached, err := pluginapi.LoadLocalIndex()
			if err == nil && cached != nil && len(cached.Plugins) > 0 {
				return
			}
			app.QueueUpdateDraw(func() {
				core.Log("[yellow]⟳ Auto-syncing plugin index...")
			})
			fetched, err := pluginapi.FetchIndex("")
			app.QueueUpdateDraw(func() {
				if err != nil {
					core.Log(fmt.Sprintf("[red]✗ Auto-sync failed: %v", err))
					return
				}
				index = fetched
				pluginapi.SaveLocalIndex(index)
				core.Log(fmt.Sprintf("[green]✓ Index synced — %d plugins available", len(index.Plugins)))
				core.RefreshData()
			})
		}()
	}

	return core
}

func refreshPluginList(core *ui.CoreView, index **pluginapi.PluginIndex) ([][]string, error) {
	if *index == nil {
		cached, err := pluginapi.LoadLocalIndex()
		if err == nil && cached != nil {
			*index = cached
		}
	}

	if *index == nil || len((*index).Plugins) == 0 {
		core.Log("[yellow]No plugin index loaded — press [white::b]S[yellow::-] to sync from remote")
		return [][]string{}, nil
	}

	rows := make([][]string, 0, len((*index).Plugins))
	for _, entry := range (*index).Plugins {
		installedVer := pluginapi.InstalledVersion(entry.Name)
		installed := pluginapi.IsInstalled(entry.Name)

		icon := "[red]○"
		status := "[red::b]Not installed"
		verDisplay := "[gray]-"
		if installed {
			icon = "[green]●"
			if installedVer != "" {
				verDisplay = "[white]" + installedVer
			} else {
				verDisplay = "[yellow]?"
			}
			if installedVer == entry.Version {
				status = "[green]Up to date"
			} else {
				status = "[yellow::b]Update available"
			}
		}

		tags := "[gray]" + strings.Join(entry.Tags, ", ")

		rows = append(rows, []string{
			icon,
			entry.Name,
			verDisplay,
			entry.Version,
			status,
			tags,
		})
	}

	updateInfoPanel(core, rows)
	return rows, nil
}

func updateDetailPanel(core *ui.CoreView, index **pluginapi.PluginIndex, name string) {
	if *index == nil {
		return
	}
	entry := findEntry(*index, name)
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

	soPath := pluginapi.PluginSOPath(entry.Name)
	sizeStr := "-"
	if info, err := os.Stat(soPath); err == nil {
		sizeStr = formatBytes(info.Size())
	}

	archStr := strings.Join(entry.Arch, ", ")

	checksumLine := "[gray]not provided"
	if cs := entry.Checksum(); cs != "" {
		checksumLine = "[green]sha256:" + cs[:12] + "..."
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
			"[aqua]Tags:       [white]%s\n\n"+
			"[gray]%s",
		entry.Name,
		statusLine,
		entry.Version,
		entry.Author,
		entry.License,
		archStr,
		sizeStr,
		checksumLine,
		strings.Join(entry.Tags, ", "),
		entry.Description,
	)

	core.SetInfoText(text)
}

func handleSyncIndex(core *ui.CoreView, app *tview.Application, pages *tview.Pages, index **pluginapi.PluginIndex) {
	core.Log("[yellow]⟳ Syncing plugin index...")
	go func() {
		fetched, err := pluginapi.FetchIndex("")
		app.QueueUpdateDraw(func() {
			if err != nil {
				core.Log(fmt.Sprintf("[red]✗ Sync failed: %v", err))
				localPath := "index.yaml"
				data, readErr := os.ReadFile(localPath)
				if readErr != nil {
					core.Log("[red]✗ No local fallback index available")
					return
				}
				parsed, parseErr := pluginapi.ParseIndex(data)
				if parseErr != nil {
					core.Log(fmt.Sprintf("[red]✗ Failed to parse local index: %v", parseErr))
					return
				}
				*index = parsed
				pluginapi.SaveLocalIndex(*index)
				core.Log(fmt.Sprintf("[green]✓ Loaded %d plugins from local index", len((*index).Plugins)))
				core.RefreshData()
				return
			}
			*index = fetched
			pluginapi.SaveLocalIndex(*index)
			core.Log(fmt.Sprintf("[green]✓ Index synced — %d plugins available", len((*index).Plugins)))
			core.RefreshData()
		})
	}()
}

func handleInstallPlugin(core *ui.CoreView, app *tview.Application, pages *tview.Pages, index *pluginapi.PluginIndex) {
	row := core.GetSelectedRowData()
	if len(row) < 2 {
		core.Log("[red]✗ No plugin selected")
		return
	}
	name := row[1]
	if pluginapi.IsInstalled(name) {
		core.Log(fmt.Sprintf("[yellow]⚡ %s is already installed", name))
		return
	}
	if index == nil {
		core.Log("[red]✗ No index loaded — press S to sync first")
		return
	}
	entry := findEntry(index, name)
	if entry == nil {
		return
	}

	pm := ui.NewProgressModal(pages, app, fmt.Sprintf("Installing %s", name), 100)
	pm.SetCancellable(false)
	pm.SetAutoClose(false)
	pm.Show()

	core.Log(fmt.Sprintf("[yellow]⟳ Installing %s v%s...", name, entry.Version))

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

		err := downloadPlugin(entry, index.DownloadURLTemplate, onProgress)

		if err != nil {
			pm.UpdateProgress(100, fmt.Sprintf("[red]✗ Failed: %v", err))
			app.QueueUpdateDraw(func() {
				core.Log(fmt.Sprintf("[red]✗ Install failed for %s: %v", name, err))
			})
			time.AfterFunc(2*time.Second, func() {
				app.QueueUpdateDraw(func() { pm.Close() })
			})
			return
		}

		pm.UpdateProgress(95, fmt.Sprintf("Verifying %s...", name))
		pluginapi.RecordInstalledVersion(name, entry.Version)
		pm.UpdateProgress(100, fmt.Sprintf("[green]✓ %s installed!", name))
		app.QueueUpdateDraw(func() {
			core.Log(fmt.Sprintf("[green]✓ %s v%s installed successfully", name, entry.Version))
		})
		time.AfterFunc(time.Second, func() {
			app.QueueUpdateDraw(func() {
				pm.Close()
				core.RefreshData()
			})
		})
	}()
}

func handleUpdatePlugin(core *ui.CoreView, app *tview.Application, pages *tview.Pages, index *pluginapi.PluginIndex) {
	row := core.GetSelectedRowData()
	if len(row) < 2 {
		core.Log("[red]✗ No plugin selected")
		return
	}
	name := row[1]
	if !pluginapi.IsInstalled(name) {
		core.Log(fmt.Sprintf("[yellow]%s is not installed — press I to install", name))
		return
	}
	if index == nil {
		core.Log("[red]✗ No index loaded")
		return
	}
	entry := findEntry(index, name)
	if entry == nil {
		return
	}
	current := pluginapi.InstalledVersion(name)
	if current == entry.Version {
		core.Log(fmt.Sprintf("[green]✓ %s is already at latest (%s)", name, current))
		return
	}

	pm := ui.NewProgressModal(pages, app, fmt.Sprintf("Updating %s", name), 100)
	pm.SetCancellable(false)
	pm.SetAutoClose(false)
	pm.Show()

	core.Log(fmt.Sprintf("[yellow]⟳ Updating %s: %s → %s...", name, current, entry.Version))

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

		err := downloadPlugin(entry, index.DownloadURLTemplate, onProgress)

		if err != nil {
			pm.UpdateProgress(100, fmt.Sprintf("[red]✗ Failed: %v", err))
			app.QueueUpdateDraw(func() {
				core.Log(fmt.Sprintf("[red]✗ Update failed for %s: %v", name, err))
			})
			time.AfterFunc(2*time.Second, func() {
				app.QueueUpdateDraw(func() { pm.Close() })
			})
			return
		}

		pm.UpdateProgress(95, fmt.Sprintf("Verifying %s...", name))
		pluginapi.RecordInstalledVersion(name, entry.Version)
		pm.UpdateProgress(100, fmt.Sprintf("[green]✓ %s updated!", name))
		app.QueueUpdateDraw(func() {
			core.Log(fmt.Sprintf("[green]✓ %s updated to v%s", name, entry.Version))
		})
		time.AfterFunc(time.Second, func() {
			app.QueueUpdateDraw(func() {
				pm.Close()
				core.RefreshData()
			})
		})
	}()
}

func handleRemovePlugin(core *ui.CoreView, app *tview.Application, pages *tview.Pages) {
	row := core.GetSelectedRowData()
	if len(row) < 2 {
		core.Log("[red]✗ No plugin selected")
		return
	}
	name := row[1]
	if !pluginapi.IsInstalled(name) {
		core.Log(fmt.Sprintf("[yellow]%s is not installed", name))
		return
	}

	soPath := pluginapi.PluginSOPath(name)
	sizeStr := ""
	if info, err := os.Stat(soPath); err == nil {
		sizeStr = fmt.Sprintf(" (%s)", formatBytes(info.Size()))
	}

	ui.ShowStandardConfirmationModal(
		pages, app,
		"Remove Plugin",
		fmt.Sprintf("Remove [red::b]%s[white::-]%s?\n\nThis will delete the plugin binary.", name, sizeStr),
		func(confirmed bool) {
			if !confirmed {
				app.SetFocus(core.GetTable())
				return
			}
			pluginDir := filepath.Join(pluginapi.PluginsDir(), name)
			if err := os.RemoveAll(pluginDir); err != nil {
				core.Log(fmt.Sprintf("[red]✗ Remove failed: %v", err))
			} else {
				pluginapi.RemoveInstalledRecord(name)
				core.Log(fmt.Sprintf("[green]✓ %s removed", name))
				core.RefreshData()
			}
			app.SetFocus(core.GetTable())
		},
	)
}

func handleInstallAll(core *ui.CoreView, app *tview.Application, pages *tview.Pages, index *pluginapi.PluginIndex) {
	if index == nil {
		core.Log("[red]✗ No index loaded — press S to sync first")
		return
	}

	var toInstall []pluginapi.IndexEntry
	for _, entry := range index.Plugins {
		if !pluginapi.IsInstalled(entry.Name) {
			toInstall = append(toInstall, entry)
		}
	}

	if len(toInstall) == 0 {
		core.Log("[green]✓ All plugins are already installed")
		return
	}

	ui.ShowStandardConfirmationModal(
		pages, app,
		"Install All",
		fmt.Sprintf("Install [aqua::b]%d[white::-] plugins?\n\n%s", len(toInstall), pluginNameList(toInstall)),
		func(confirmed bool) {
			if !confirmed {
				app.SetFocus(core.GetTable())
				return
			}
			app.SetFocus(core.GetTable())

			total := len(toInstall)
			pm := ui.NewProgressModal(pages, app, "Installing All Plugins", total)
			pm.SetCancellable(false)
			pm.SetAutoClose(false)
			pm.Show()

			core.Log(fmt.Sprintf("[yellow]⟳ Installing %d plugins...", total))

			go func() {
				installed := 0
				failed := 0
				var mu sync.Mutex
				var wg sync.WaitGroup
				sem := make(chan struct{}, 3)

				for i, entry := range toInstall {
					wg.Add(1)
					go func(idx int, e pluginapi.IndexEntry) {
						defer wg.Done()
						sem <- struct{}{}
						defer func() { <-sem }()

					pm.UpdateProgress(idx, fmt.Sprintf("Downloading %s (%d/%d)...", e.Name, idx+1, total))

						if err := downloadPlugin(&e, index.DownloadURLTemplate, nil); err != nil {
							mu.Lock()
							failed++
							mu.Unlock()
							app.QueueUpdateDraw(func() {
								core.Log(fmt.Sprintf("[red]✗ %s: %v", e.Name, err))
							})
							return
						}
						pluginapi.RecordInstalledVersion(e.Name, e.Version)
						mu.Lock()
						installed++
						mu.Unlock()
						app.QueueUpdateDraw(func() {
							core.Log(fmt.Sprintf("[green]✓ %s v%s installed", e.Name, e.Version))
						})
					}(i, entry)
				}

				wg.Wait()

				if failed == 0 {
					pm.UpdateProgress(total, fmt.Sprintf("[green]✓ All %d plugins installed!", installed))
				} else {
					pm.UpdateProgress(total, fmt.Sprintf("[yellow]Done: %d installed, %d failed", installed, failed))
				}
				app.QueueUpdateDraw(func() {
					core.Log(fmt.Sprintf("[green]✓ Bulk install complete: %d installed, %d failed", installed, failed))
				})
				time.AfterFunc(2*time.Second, func() {
					app.QueueUpdateDraw(func() {
						pm.Close()
						core.RefreshData()
					})
				})
			}()
		},
	)
}

func handleUpdateAll(core *ui.CoreView, app *tview.Application, pages *tview.Pages, index *pluginapi.PluginIndex) {
	if index == nil {
		core.Log("[red]✗ No index loaded")
		return
	}

	var toUpdate []pluginapi.IndexEntry
	for _, entry := range index.Plugins {
		if pluginapi.IsInstalled(entry.Name) {
			current := pluginapi.InstalledVersion(entry.Name)
			if current != entry.Version {
				toUpdate = append(toUpdate, entry)
			}
		}
	}

	if len(toUpdate) == 0 {
		core.Log("[green]✓ All installed plugins are up to date")
		return
	}

	total := len(toUpdate)
	pm := ui.NewProgressModal(pages, app, "Updating All Plugins", total)
	pm.SetCancellable(false)
	pm.SetAutoClose(false)
	pm.Show()

	core.Log(fmt.Sprintf("[yellow]⟳ Updating %d plugins...", total))

	go func() {
		updated := 0
		failed := 0
		for i, entry := range toUpdate {
			pm.UpdateProgress(i, fmt.Sprintf("Updating %s (%d/%d)...", entry.Name, i+1, total))
			if err := downloadPlugin(&entry, index.DownloadURLTemplate, nil); err != nil {
				failed++
				app.QueueUpdateDraw(func() {
					core.Log(fmt.Sprintf("[red]✗ Failed to update %s: %v", entry.Name, err))
				})
				continue
			}
			pluginapi.RecordInstalledVersion(entry.Name, entry.Version)
			updated++
			app.QueueUpdateDraw(func() {
				core.Log(fmt.Sprintf("[green]✓ %s updated to v%s", entry.Name, entry.Version))
			})
		}

		if failed == 0 {
			pm.UpdateProgress(total, fmt.Sprintf("[green]✓ All %d plugins updated!", updated))
		} else {
			pm.UpdateProgress(total, fmt.Sprintf("[yellow]Done: %d updated, %d failed", updated, failed))
		}
		app.QueueUpdateDraw(func() {
			core.Log(fmt.Sprintf("[green]✓ Bulk update complete: %d updated, %d failed", updated, failed))
		})
		time.AfterFunc(2*time.Second, func() {
			app.QueueUpdateDraw(func() {
				pm.Close()
				core.RefreshData()
			})
		})
	}()
}

// progressFunc is called during download with (bytesDownloaded, totalBytes).
// totalBytes is -1 if Content-Length is unknown.
type progressFunc func(downloaded, total int64)

func downloadPlugin(entry *pluginapi.IndexEntry, urlTemplate string, onProgress progressFunc) error {
	url := entry.DownloadURL(urlTemplate)

	if err := pluginapi.EnsurePluginDirs(entry.Name); err != nil {
		return fmt.Errorf("create dirs: %w", err)
	}

	destPath := pluginapi.PluginSOPath(entry.Name)
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
		if err := extractSOFromTarGz(archiveReader, entry.Name, tmpPath); err != nil {
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

func extractSOFromTarGz(r io.Reader, pluginName, destPath string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}

		name := filepath.Base(hdr.Name)
		if strings.HasSuffix(name, ".so") && strings.HasPrefix(name, pluginName) {
			out, err := os.Create(destPath)
			if err != nil {
				return fmt.Errorf("create: %w", err)
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return fmt.Errorf("write: %w", err)
			}
			out.Close()
			return nil
		}
	}

	return fmt.Errorf("no .so file for %q found in archive", pluginName)
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

func helpText() string {
	return fmt.Sprintf(`[yellow::b]Package Manager[white::-]
[gray]Platform: %s/%s

[aqua::b]Actions[white::-]
[green]S[white]  Sync index from remote
[green]I[white]  Install selected plugin
[green]U[white]  Update selected plugin
[green]D[white]  Remove selected plugin
[green]A[white]  Install all available plugins
[green]Z[white]  Update all installed plugins
[green]Q[white]  Back to main menu
[green]?[white]  Show this help

[aqua::b]Navigation[white::-]
[green]↑/↓[white]  Navigate plugin list
[green]R[white]    Refresh list
[green]/[white]    Filter/search
[green]Esc[white]  Close modal

[aqua::b]Status Icons[white::-]
[green]●[white]  Installed & up to date
[yellow]●[white]  Update available
[red]○[white]  Not installed

[aqua::b]Paths[white::-]
Plugins: ~/.omo/plugins/<name>/
Configs: ~/.omo/configs/<name>/`, runtime.GOOS, runtime.GOARCH)
}

func updateInfoPanel(core *ui.CoreView, data [][]string) {
	total := len(data)
	installed := 0
	updates := 0
	totalSize := int64(0)

	for _, row := range data {
		if len(row) < 5 {
			continue
		}
		if strings.Contains(row[4], "Up to date") || strings.Contains(row[4], "Update") {
			installed++
			name := row[1]
			soPath := pluginapi.PluginSOPath(name)
			if info, err := os.Stat(soPath); err == nil {
				totalSize += info.Size()
			}
		}
		if strings.Contains(row[4], "Update available") {
			updates++
		}
	}

	available := total - installed

	text := "[aqua::b]── Overview ──[white::-]\n\n" +
		"[aqua]Total:      [white::b]" + strconv.Itoa(total) + "[white::-]\n" +
		"[aqua]Installed:  [green::b]" + strconv.Itoa(installed) + "[white::-]\n" +
		"[aqua]Available:  [red]" + strconv.Itoa(available) + "[white::-]\n" +
		"[aqua]Updates:    [yellow::b]" + strconv.Itoa(updates) + "[white::-]\n" +
		"[aqua]Disk Usage: [white]" + formatBytes(totalSize) + "\n\n" +
		"[aqua::b]── Platform ──[white::-]\n\n" +
		"[aqua]OS:         [white]" + runtime.GOOS + "\n" +
		"[aqua]Arch:       [white]" + runtime.GOARCH + "\n\n" +
		"[gray]Last refresh: " + time.Now().Format("15:04:05")

	core.SetInfoText(text)
}

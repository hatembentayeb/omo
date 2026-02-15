package packagemanager

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"omo/pkg/pluginapi"
	"omo/pkg/ui"

	"github.com/rivo/tview"
)

// NewPackageManager creates the package manager UI view.
func NewPackageManager(app *tview.Application, pages *tview.Pages, pluginsDir string) *ui.CoreView {
	core := ui.NewCoreView(app, "Package Manager")
	core.SetModalPages(pages)

	core.SetTableHeaders([]string{"Name", "Installed", "Latest", "Status", "Description"})
	core.SetSelectionKey("Name")

	core.AddKeyBinding("I", "Install", nil)
	core.AddKeyBinding("U", "Update", nil)
	core.AddKeyBinding("D", "Remove", nil)
	core.AddKeyBinding("S", "Sync Index", nil)
	core.AddKeyBinding("Z", "Update All", nil)
	core.AddKeyBinding("Q", "Back", nil)

	// Shared state: the index, loaded once and refreshed on Sync.
	var index *pluginapi.PluginIndex

	core.SetRefreshCallback(func() ([][]string, error) {
		// Load cached index if we don't have one yet
		if index == nil {
			cached, err := pluginapi.LoadLocalIndex()
			if err == nil && cached != nil {
				index = cached
			}
		}

		// If still no index, show a hint
		if index == nil || len(index.Plugins) == 0 {
			core.Log("[yellow]No plugin index loaded. Press S to sync from remote.")
			return [][]string{}, nil
		}

		rows := make([][]string, 0, len(index.Plugins))
		for _, entry := range index.Plugins {
			installedVer := pluginapi.InstalledVersion(entry.Name)
			installed := pluginapi.IsInstalled(entry.Name)

			status := "[red]Not Installed"
			verDisplay := "-"
			if installed {
				if installedVer != "" {
					verDisplay = installedVer
				} else {
					verDisplay = "?"
				}
				if installedVer == entry.Version {
					status = "[green]Up to date"
				} else {
					status = "[yellow]Update available"
				}
			}

			rows = append(rows, []string{
				entry.Name,
				verDisplay,
				entry.Version,
				status,
				entry.Description,
			})
		}

		updateInfoPanel(core, rows)
		return rows, nil
	})

	core.SetActionCallback(func(action string, payload map[string]interface{}) error {
		if action != "keypress" {
			return nil
		}
		key, ok := payload["key"].(string)
		if !ok {
			return nil
		}

		switch key {
		case "S":
			// Sync index from remote
			core.Log("[yellow]Syncing plugin index...")
			go func() {
				fetched, err := pluginapi.FetchIndex("")
				app.QueueUpdateDraw(func() {
					if err != nil {
						core.Log(fmt.Sprintf("[red]Sync failed: %v", err))
						// Fall back to loading from the repo's local index.yaml
						localPath := "index.yaml"
						data, readErr := os.ReadFile(localPath)
						if readErr != nil {
							core.Log("[red]No local index.yaml fallback either")
							return
						}
						parsed, parseErr := pluginapi.ParseIndex(data)
						if parseErr != nil {
							core.Log(fmt.Sprintf("[red]Failed to parse local index: %v", parseErr))
							return
						}
						index = parsed
						pluginapi.SaveLocalIndex(index)
						core.Log(fmt.Sprintf("[green]Loaded %d plugins from local index", len(index.Plugins)))
						core.RefreshData()
						return
					}
					index = fetched
					pluginapi.SaveLocalIndex(index)
					core.Log(fmt.Sprintf("[green]Index synced: %d plugins available", len(index.Plugins)))
					core.RefreshData()
				})
			}()

		case "I":
			row := core.GetSelectedRowData()
			if row == nil {
				core.Log("[red]No plugin selected")
				return nil
			}
			name := row[0]
			if pluginapi.IsInstalled(name) {
				core.Log(fmt.Sprintf("[yellow]%s is already installed", name))
				return nil
			}
			if index == nil {
				core.Log("[red]No index loaded. Press S to sync first.")
				return nil
			}
			entry := findEntry(index, name)
			if entry == nil {
				return nil
			}
			core.Log(fmt.Sprintf("[yellow]Installing %s v%s...", name, entry.Version))
			go func() {
				err := downloadPlugin(entry, index.DownloadURLTemplate)
				app.QueueUpdateDraw(func() {
					if err != nil {
						core.Log(fmt.Sprintf("[red]Install failed: %v", err))
						return
					}
					pluginapi.RecordInstalledVersion(name, entry.Version)
					core.Log(fmt.Sprintf("[green]%s v%s installed", name, entry.Version))
					core.RefreshData()
				})
			}()

		case "U":
			row := core.GetSelectedRowData()
			if row == nil {
				core.Log("[red]No plugin selected")
				return nil
			}
			name := row[0]
			if !pluginapi.IsInstalled(name) {
				core.Log(fmt.Sprintf("[yellow]%s is not installed", name))
				return nil
			}
			if index == nil {
				core.Log("[red]No index loaded")
				return nil
			}
			entry := findEntry(index, name)
			if entry == nil {
				return nil
			}
			current := pluginapi.InstalledVersion(name)
			if current == entry.Version {
				core.Log(fmt.Sprintf("[yellow]%s is already at v%s", name, current))
				return nil
			}
			core.Log(fmt.Sprintf("[yellow]Updating %s %s → %s...", name, current, entry.Version))
			go func() {
				err := downloadPlugin(entry, index.DownloadURLTemplate)
				app.QueueUpdateDraw(func() {
					if err != nil {
						core.Log(fmt.Sprintf("[red]Update failed: %v", err))
						return
					}
					pluginapi.RecordInstalledVersion(name, entry.Version)
					core.Log(fmt.Sprintf("[green]%s updated to v%s", name, entry.Version))
					core.RefreshData()
				})
			}()

		case "D":
			row := core.GetSelectedRowData()
			if row == nil {
				core.Log("[red]No plugin selected")
				return nil
			}
			name := row[0]
			if !pluginapi.IsInstalled(name) {
				core.Log(fmt.Sprintf("[yellow]%s is not installed", name))
				return nil
			}

			ui.ShowStandardConfirmationModal(
				pages, app,
				"Remove Plugin",
				fmt.Sprintf("Remove plugin [red]%s[white]?\nThis will delete the .so file.", name),
				func(confirmed bool) {
					if !confirmed {
						app.SetFocus(core.GetTable())
						return
					}
					pluginDir := filepath.Join(pluginapi.PluginsDir(), name)
					if err := os.RemoveAll(pluginDir); err != nil {
						core.Log(fmt.Sprintf("[red]Remove failed: %v", err))
					} else {
						pluginapi.RemoveInstalledRecord(name)
						core.Log(fmt.Sprintf("[green]%s removed", name))
						core.RefreshData()
					}
					app.SetFocus(core.GetTable())
				},
			)

		case "Z":
			if index == nil {
				core.Log("[red]No index loaded")
				return nil
			}
			core.Log("[yellow]Updating all plugins...")
			go func() {
				updated := 0
				for _, entry := range index.Plugins {
					if !pluginapi.IsInstalled(entry.Name) {
						continue
					}
					current := pluginapi.InstalledVersion(entry.Name)
					if current == entry.Version {
						continue
					}
					if err := downloadPlugin(&entry, index.DownloadURLTemplate); err != nil {
						app.QueueUpdateDraw(func() {
							core.Log(fmt.Sprintf("[red]Failed to update %s: %v", entry.Name, err))
						})
						continue
					}
					pluginapi.RecordInstalledVersion(entry.Name, entry.Version)
					updated++
				}
				app.QueueUpdateDraw(func() {
					if updated == 0 {
						core.Log("[green]All plugins are up to date")
					} else {
						core.Log(fmt.Sprintf("[green]Updated %d plugin(s)", updated))
					}
					core.RefreshData()
				})
			}()

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
	core.StartAutoRefresh(60 * time.Second)

	return core
}

// downloadPlugin fetches the .so from the release URL and saves it.
func downloadPlugin(entry *pluginapi.IndexEntry, urlTemplate string) error {
	url := entry.DownloadURL(urlTemplate)

	if err := pluginapi.EnsurePluginDirs(entry.Name); err != nil {
		return fmt.Errorf("create dirs: %w", err)
	}

	destPath := pluginapi.PluginSOPath(entry.Name)
	tmpPath := destPath + ".tmp"

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned HTTP %d from %s", resp.StatusCode, url)
	}

	out, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}

	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write: %w", err)
	}
	out.Close()

	// Make executable
	if err := os.Chmod(tmpPath, 0755); err != nil {
		os.Remove(tmpPath)
		return err
	}

	// Atomic rename
	return os.Rename(tmpPath, destPath)
}

func findEntry(index *pluginapi.PluginIndex, name string) *pluginapi.IndexEntry {
	for i := range index.Plugins {
		if index.Plugins[i].Name == name {
			return &index.Plugins[i]
		}
	}
	return nil
}

func helpText() string {
	return `[yellow]Package Manager Help[white]

[green]Actions:[white]
S       - Sync plugin index from remote
I       - Install selected plugin
U       - Update selected plugin
D       - Remove selected plugin
Z       - Update all installed plugins
Q       - Back to main UI
?       - Show this help

[green]Navigation:[white]
↑/↓     - Navigate list
Enter   - Select plugin
Esc     - Close modal

[green]How it works:[white]
  1. Press S to sync the official plugin index
  2. Browse available plugins in the list
  3. Press I to install, U to update, D to remove
  4. Plugins are downloaded to ~/.omo/plugins/<name>/
  5. Configs live at ~/.omo/configs/<name>/`
}

func updateInfoPanel(core *ui.CoreView, data [][]string) {
	total := len(data)
	installed := 0
	updates := 0

	for _, row := range data {
		if row[1] != "-" { // has an installed version
			installed++
			if row[1] != row[2] {
				updates++
			}
		}
	}

	available := total - installed

	core.SetInfoText(
		"[aqua::b]Total:[white::b] " + strconv.Itoa(total) + "\n" +
			"[aqua::b]Installed:[white::b] " + strconv.Itoa(installed) + "\n" +
			"[aqua::b]Available:[white::b] " + strconv.Itoa(available) + "\n" +
			"[aqua::b]Updates:[yellow::b] " + strconv.Itoa(updates) + "\n" +
			"[aqua::b]Last Check:[white::b] " + time.Now().Format("15:04:05"))
}

package host

import (
	"fmt"
	"strings"

	"omo/pkg/pluginapi"
	"omo/pkg/pluginrpc"
	"omo/pkg/ui"
)

type secretTarget struct {
	Path     string
	Settings map[string]string
	Label    string
	Detail   string
}

// ShowTargetSelector lists KeePass targets for the active RPC plugin (Ctrl+t).
func (m *PluginManager) ShowTargetSelector() {
	m.mu.Lock()
	name := m.active
	sess := m.sessions[name]
	m.mu.Unlock()

	if name == "" || sess == nil || sess.Plugin == nil || sess.State != ConnRunning {
		return
	}

	targets, err := listSecretTargets(name)
	if err != nil {
		if sess.Renderer != nil && sess.Renderer.core != nil {
			sess.Renderer.core.Log("[red]" + err.Error())
		}
		return
	}
	if len(targets) == 0 {
		if sess.Renderer != nil && sess.Renderer.core != nil {
			sess.Renderer.core.Log("[yellow]no KeePass targets under " + name + "/")
		}
		return
	}

	items := make([][]string, len(targets))
	for i, t := range targets {
		items[i] = []string{t.Label, t.Detail}
	}

	ui.ShowStandardListSelectorModal(m.pages, m.app, "Select "+name+" target", items,
		func(index int, _ string, cancelled bool) {
			if sess.Renderer != nil {
				sess.Renderer.FocusTable()
			}
			if cancelled || index < 0 || index >= len(targets) {
				return
			}
			m.applyTarget(name, targets[index])
		})
}

func (m *PluginManager) applyTarget(name string, target secretTarget) {
	m.mu.Lock()
	sess := m.sessions[name]
	m.mu.Unlock()
	if sess == nil || sess.Plugin == nil {
		return
	}

	go func() {
		pluginrpc.RPCLog("SelectTarget: Configure %s path=%s", name, target.Path)
		err := sess.Plugin.Configure(pluginrpc.ConfigureRequest{Settings: target.Settings})
		if err != nil {
			m.app.QueueUpdateDraw(func() {
				if sess.Renderer != nil && sess.Renderer.core != nil {
					sess.Renderer.core.Log("[red]configure failed: " + err.Error())
					sess.Renderer.FocusTable()
				}
			})
			return
		}
		viewID := ""
		if sess.Renderer != nil {
			viewID = sess.Renderer.currentView
		}
		view, err := sess.Plugin.GetView(pluginrpc.ViewRequest{View: viewID})
		m.app.QueueUpdateDraw(func() {
			if sess.Renderer == nil {
				return
			}
			if err != nil {
				sess.Renderer.core.Log("[red]refresh failed: " + err.Error())
				sess.Renderer.FocusTable()
				return
			}
			sess.Renderer.core.Log("[green]switched to " + target.Label)
			sess.Renderer.Apply(view)
			sess.Renderer.FocusTable()
		})
	}()
}

func listSecretTargets(pluginName string) ([]secretTarget, error) {
	if !pluginapi.HasSecrets() {
		return nil, fmt.Errorf("secrets unavailable")
	}
	if err := pluginapi.Secrets().Reload(); err != nil {
		return nil, fmt.Errorf("reload secrets: %w", err)
	}
	paths, err := pluginapi.ListNonReferenceSecrets(pluginName)
	if err != nil {
		return nil, err
	}

	var out []secretTarget
	seen := map[string]bool{}
	for _, p := range paths {
		if seen[p] || strings.Count(p, "/") < 2 {
			continue
		}
		seen[p] = true
		entry, err := pluginapi.Secrets().Get(p)
		if err != nil || entry == nil || pluginapi.IsReferenceEntry(entry) {
			continue
		}
		settings := entryToSettings(entry)
		label := entry.Title
		if label == "" {
			parts := strings.Split(p, "/")
			label = parts[len(parts)-1]
		}
		env := ""
		if parts := strings.Split(p, "/"); len(parts) >= 2 {
			env = parts[1]
		}
		host := entry.URL
		if host == "" {
			host = settings["host"]
		}
		detail := host
		if env != "" {
			detail = fmt.Sprintf("%s · %s", env, host)
		}
		out = append(out, secretTarget{
			Path:     p,
			Settings: settings,
			Label:    label,
			Detail:   detail,
		})
	}
	return out, nil
}

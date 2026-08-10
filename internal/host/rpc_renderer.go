package host

import (
	"fmt"
	"strconv"
	"strings"

	"omo/pkg/pluginrpc"
	"omo/pkg/ui"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// RPCRenderer is a host-owned CoreView driven by pluginrpc.ViewData.
type RPCRenderer struct {
	app         *tview.Application
	pages       *tview.Pages
	name        string
	plugin      pluginrpc.Plugin
	core        *ui.CoreView
	root        *tview.Pages
	currentView string
	homeView    string // first/default view id for breadcrumbs + ESC
	onActions   func([]pluginrpc.KeyBinding, func(string))
}

// NewRPCRenderer builds a CoreView shell for an RPC plugin.
func NewRPCRenderer(app *tview.Application, pages *tview.Pages, name string, p pluginrpc.Plugin) *RPCRenderer {
	pluginrpc.RPCLog("NewRPCRenderer: NewCoreView …")
	r := &RPCRenderer{
		app:         app,
		pages:       pages,
		name:        name,
		plugin:      p,
		root:        tview.NewPages(),
		core:        ui.NewCoreView(app, name),
		currentView: "keys",
		homeView:    "keys",
	}
	pluginrpc.RPCLog("NewRPCRenderer: wiring callbacks …")
	r.core.SetModalPages(pages)
	r.core.SetRefreshCallback(r.fetchRows)
	r.core.SetActionCallback(r.handleCoreAction)
	r.core.AddKeyBinding("R", "Refresh", r.refresh)
	r.core.SetViewStack([]string{name, "keys"})
	r.core.RegisterHandlers()
	r.root.AddPage("main", r.core.GetLayout(), true, true)
	pluginrpc.RPCLog("NewRPCRenderer: done")
	return r
}

// SetPlugin updates the RPC client used for refresh/actions (after async launch).
func (r *RPCRenderer) SetPlugin(p pluginrpc.Plugin) {
	r.plugin = p
}

// SetActionsHook wires the host sidebar to per-view plugin actions.
func (r *RPCRenderer) SetActionsHook(fn func([]pluginrpc.KeyBinding, func(string))) {
	r.onActions = fn
}

// ShowLoading sets a loading message without QueueUpdateDraw / table.Select.
// Safe to call from SetSelectedFunc.
func (r *RPCRenderer) ShowLoading(name string) {
	if r.core == nil {
		return
	}
	r.core.SetInfoText(fmt.Sprintf(
		"[yellow]%s[white]\nStatus: Loading…\nSee ~/.omo/logs/rpc-host.log",
		name,
	))
}

// Apply paints ViewData into the CoreView and rebinds action keys.
// Call only from QueueUpdateDraw (or after the event handler has returned).
func (r *RPCRenderer) Apply(view pluginrpc.ViewData) tview.Primitive {
	pluginrpc.RPCLog("RPCRenderer.Apply begin view=%q status=%q rows=%d", view.View, view.Status, len(view.Rows))
	if view.View != "" {
		r.currentView = view.View
	}
	if r.homeView == "" {
		// First paint establishes the plugin's home view id (redis: keys).
		if r.currentView != "" {
			r.homeView = r.currentView
		} else {
			r.homeView = "keys"
		}
	}
	root := r.name
	if root == "" {
		root = "plugin"
	}
	stack := []string{root, r.homeView}
	if r.currentView != "" && r.currentView != r.homeView {
		stack = append(stack, r.currentView)
	}
	r.core.SetViewStack(stack)

	title := view.Title
	if title == "" {
		title = r.name
	}
	r.core.ClearKeyBindings()
	r.core.ClearHelpSections()

	// Globals always live in the Keys column (former logs).
	r.core.AddKeyBinding("R", "Refresh", r.refresh)
	r.core.AddKeyBinding("?", "Help", func() { r.core.ShowHelpModal() })
	r.core.AddKeyBinding("/", "Filter", nil)
	r.core.AddKeyBinding("^t", "Target", nil) // handled globally in main (Ctrl+t)

	// Middle column: explicit view switches (0-9).
	for _, kb := range view.ViewBindings {
		r.bindViewSwitch(kb)
	}

	// Legacy / extra KeyBindings: classify goto_/digits → Views, else → Keys.
	for _, kb := range view.KeyBindings {
		if isViewSwitchBinding(kb) {
			// Non-digit overflow views (A/W/X/Z): bind silently, listed in "?".
			if len(kb.Key) == 1 && kb.Key[0] >= '0' && kb.Key[0] <= '9' {
				r.bindViewSwitch(kb)
			} else if kb.Key != "" && kb.Action != "" {
				action := kb.Action
				r.core.BindKey(kb.Key, func() { r.dispatchAction(action) })
			}
			continue
		}
		r.bindKeyShortcut(kb)
	}

	// Current-view actions → Keys column (expanded) + sidebar only.
	for _, kb := range view.Actions {
		r.bindKeyShortcut(kb)
	}
	r.core.SetActiveView(r.currentView)

	if len(view.HelpSections) > 0 {
		sections := make([]ui.HelpSection, 0, len(view.HelpSections))
		for _, s := range view.HelpSections {
			bindings := make([]ui.KeyBindingHelp, 0, len(s.Bindings))
			for _, b := range s.Bindings {
				bindings = append(bindings, ui.KeyBindingHelp{Key: b.Key, Label: b.Label})
			}
			sections = append(sections, ui.HelpSection{Title: s.Title, Bindings: bindings})
		}
		r.core.SetHelpSections(sections)
	}

	if r.onActions != nil {
		r.onActions(view.Actions, r.dispatchAction)
	}

	if view.SelectionKey != "" {
		r.core.SetSelectionKey(view.SelectionKey)
	}
	if len(view.Headers) > 0 {
		r.core.SetTableHeaders(view.Headers)
	}
	r.core.SetTableData(view.Rows)
	if view.Info != "" {
		r.core.SetInfoText(view.Info)
	} else if view.Status != "" {
		r.core.SetInfoText(fmt.Sprintf("[green]%s[white]\n%s", title, view.Status))
	}

	// Enter: view key / peek pubsub channel
	if table := r.core.GetTable(); table != nil {
		table.SetSelectedFunc(func(row, _ int) {
			if row <= 0 {
				return
			}
			switch r.currentView {
			case "keys":
				r.dispatchAction("view_key")
			case "pubsub":
				r.dispatchAction("subscribe")
			case "servers":
				r.dispatchAction("shell")
			case "buckets":
				r.dispatchAction("open_objects")
			case "objects":
				r.dispatchAction("navigate")
			case "workloads", "services", "pods":
				r.dispatchAction("start_forward")
			case "forwards", "ports":
				r.dispatchAction("connection_info")
			case "namespaces":
				r.dispatchAction("filter_namespace")
			}
		})
	}

	r.core.RegisterHandlers()
	r.root.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		return r.core.StandardKeyHandler(event, nil)
	})
	pluginrpc.RPCLog("RPCRenderer.Apply done bindings=%d view=%s", len(view.KeyBindings), r.currentView)
	return r.root
}

// FocusTable moves keyboard focus to the table. Call only from the tview
// thread (e.g. inside QueueUpdateDraw), never from SetSelectedFunc.
func (r *RPCRenderer) FocusTable() {
	if r.core == nil {
		return
	}
	table := r.core.GetTable()
	if table == nil {
		r.app.SetFocus(r.root)
		return
	}
	r.app.SetFocus(table)
}

// Primitive returns the root pages primitive.
func (r *RPCRenderer) Primitive() tview.Primitive {
	return r.root
}

func isViewSwitchBinding(kb pluginrpc.KeyBinding) bool {
	if strings.HasPrefix(kb.Action, "goto_") {
		return true
	}
	return len(kb.Key) == 1 && kb.Key[0] >= '0' && kb.Key[0] <= '9'
}

func (r *RPCRenderer) bindViewSwitch(kb pluginrpc.KeyBinding) {
	if kb.Key == "" || kb.Action == "" {
		return
	}
	action := kb.Action
	viewID := strings.TrimPrefix(action, "goto_")
	r.core.AddViewBinding(kb.Key, kb.Label, viewID, func() {
		r.dispatchAction(action)
	})
}

func (r *RPCRenderer) bindKeyShortcut(kb pluginrpc.KeyBinding) {
	key := kb.Key
	if key == "" || key == "R" || key == "?" || key == "/" || key == "^t" {
		return
	}
	label := kb.Label
	action := kb.Action
	if action == "" {
		r.core.AddKeyBinding(key, label, nil)
		return
	}
	r.core.AddKeyBinding(key, label, func() {
		r.dispatchAction(action)
	})
}

// Destroy cleans up the CoreView.
func (r *RPCRenderer) Destroy() {
	if r.core != nil {
		r.core.Destroy()
	}
}

func (r *RPCRenderer) handleCoreAction(action string, payload map[string]interface{}) error {
	home := r.homeView
	if home == "" {
		home = "keys"
	}
	switch action {
	case "navigate_back":
		if r.currentView != "" && r.currentView != home {
			r.dispatchAction("goto_" + home)
		} else {
			root := r.name
			if root == "" {
				root = "plugin"
			}
			r.core.SetViewStack([]string{root, home})
		}
		return nil
	case "back":
		return nil
	default:
		return fmt.Errorf("unhandled")
	}
}

func (r *RPCRenderer) fetchRows() ([][]string, error) {
	if r.plugin == nil {
		return nil, fmt.Errorf("plugin not ready")
	}
	view, err := r.plugin.GetView(pluginrpc.ViewRequest{View: r.currentView})
	if err != nil {
		return nil, err
	}
	if view.Info != "" {
		r.core.SetInfoText(view.Info)
	}
	return view.Rows, nil
}

func (r *RPCRenderer) refresh() {
	if r.plugin == nil {
		r.core.Log("[yellow]plugin still loading…")
		return
	}
	viewID := r.currentView
	go func() {
		view, err := r.plugin.GetView(pluginrpc.ViewRequest{View: viewID})
		r.app.QueueUpdateDraw(func() {
			if err != nil {
				r.core.Log(fmt.Sprintf("[red]refresh failed: %v", err))
				return
			}
			r.Apply(view)
			r.FocusTable()
		})
	}()
}

func (r *RPCRenderer) dispatchAction(action string) {
	switch action {
	case "delete":
		key := r.selectedKey()
		if key == "" {
			r.core.Log("[yellow]no key selected")
			return
		}
		payload := r.selectionPayload()
		ui.ShowStandardConfirmationModal(r.pages, r.app, "Confirm Delete",
			fmt.Sprintf("Delete %q?", key),
			func(ok bool) {
				r.FocusTable()
				if ok {
					r.runAction("delete", payload)
				}
			})
	case "flush":
		ui.ShowStandardConfirmationModal(r.pages, r.app, "Flush DB",
			"Flush the current database? This cannot be undone.",
			func(ok bool) {
				r.FocusTable()
				if ok {
					r.runAction("flush", nil)
				}
			})
	case "create_key":
		r.promptCreateKey()
	case "create_bucket":
		r.promptCreateBucket()
	case "create_folder":
		r.promptCreateFolder()
	case "select_db":
		r.promptSelectDB()
	case "publish":
		r.promptPublish()
	case "start_forward":
		r.promptStartForward()
	case "stop_forward":
		r.promptStopForward()
	case "stop_all_forwards":
		ui.ShowStandardConfirmationModal(r.pages, r.app, "Stop All Forwards",
			"Stop every active port-forward?",
			func(ok bool) {
				r.FocusTable()
				if ok {
					r.runAction("stop_all_forwards", nil)
				}
			})
	case "filter_namespace":
		r.promptFilterNamespace()
	default:
		r.runAction(action, r.selectionPayload())
	}
}

func (r *RPCRenderer) selectedKey() string {
	row := r.core.GetSelectedRowData()
	if len(row) == 0 {
		return ""
	}
	return row[0]
}

func (r *RPCRenderer) selectionPayload() map[string]string {
	payload := map[string]string{}
	row := r.core.GetSelectedRowData()
	if len(row) == 0 {
		return payload
	}
	payload["key"] = row[0]
	payload["channel"] = row[0]
	for i, cell := range row {
		payload[fmt.Sprintf("col%d", i)] = cell
	}
	return payload
}

func (r *RPCRenderer) promptCreateKey() {
	ui.ShowCompactStyledInputModal(r.pages, r.app, "New Key", "Key:", "", 40, nil,
		func(key string, cancelled bool) {
			if cancelled || strings.TrimSpace(key) == "" {
				r.FocusTable()
				return
			}
			ui.ShowCompactStyledInputModal(r.pages, r.app, "New Key", "Value:", "", 40, nil,
				func(value string, cancelled bool) {
					r.FocusTable()
					if cancelled {
						return
					}
					r.runAction("create_key", map[string]string{
						"key":   strings.TrimSpace(key),
						"value": value,
						"ttl":   "-1",
					})
				})
		})
}

func (r *RPCRenderer) promptCreateBucket() {
	ui.ShowCompactStyledInputModal(r.pages, r.app, "New Bucket", "Name:", "", 48, nil,
		func(name string, cancelled bool) {
			r.FocusTable()
			if cancelled || strings.TrimSpace(name) == "" {
				return
			}
			r.runAction("create_bucket", map[string]string{"name": strings.TrimSpace(name)})
		})
}

func (r *RPCRenderer) promptCreateFolder() {
	ui.ShowCompactStyledInputModal(r.pages, r.app, "New Folder", "Name:", "", 40, nil,
		func(name string, cancelled bool) {
			r.FocusTable()
			if cancelled || strings.TrimSpace(name) == "" {
				return
			}
			r.runAction("create_folder", map[string]string{"name": strings.TrimSpace(name)})
		})
}

func (r *RPCRenderer) promptSelectDB() {
	placeholder := "0"
	if r.currentView == "databases" {
		if sel := r.selectedKey(); sel != "" {
			sel = strings.TrimPrefix(sel, "db")
			sel = strings.TrimSuffix(sel, " *")
			if _, err := strconv.Atoi(sel); err == nil {
				placeholder = sel
			}
		}
	}
	ui.ShowCompactStyledInputModal(r.pages, r.app, "Select Database", "DB (0-15):", placeholder, 8,
		func(text string, _ rune) bool {
			if text == "" {
				return true
			}
			n, err := strconv.Atoi(text)
			return err == nil && n >= 0 && n <= 15
		},
		func(text string, cancelled bool) {
			r.FocusTable()
			if cancelled || strings.TrimSpace(text) == "" {
				return
			}
			r.runAction("select_db", map[string]string{"db": strings.TrimSpace(text)})
		})
}

func (r *RPCRenderer) promptPublish() {
	channel := r.selectedKey()
	if channel == "-" || channel == "*" {
		channel = ""
	}
	ui.ShowCompactStyledInputModal(r.pages, r.app, "Publish", "Channel:", channel, 40, nil,
		func(ch string, cancelled bool) {
			if cancelled || strings.TrimSpace(ch) == "" {
				r.FocusTable()
				return
			}
			ui.ShowCompactStyledInputModal(r.pages, r.app, "Publish", "Message:", "", 40, nil,
				func(msg string, cancelled bool) {
					r.FocusTable()
					if cancelled || msg == "" {
						return
					}
					r.runAction("publish", map[string]string{
						"channel": strings.TrimSpace(ch),
						"message": msg,
					})
				})
		})
}

func (r *RPCRenderer) promptStartForward() {
	payload := r.selectionPayload()
	if len(payload) == 0 || (payload["col3"] == "" && payload["col2"] == "" && r.currentView != "forwards") {
		// Allow forwards view reconnect-style only from resource rows.
		if r.currentView != "workloads" && r.currentView != "services" && r.currentView != "pods" {
			r.core.Log("[yellow]select a workload, service, or pod")
			return
		}
	}

	remoteDefault := firstPortFromCell(payload["col6"])
	if remoteDefault == "" {
		remoteDefault = firstPortFromCell(payload["ports"])
	}
	localDefault := strings.TrimSpace(payload["col7"])
	if localDefault == "" || localDefault == "-" {
		if remoteDefault != "" {
			localDefault = remoteDefault
		} else {
			localDefault = "auto"
		}
	}

	target := strings.TrimSpace(payload["col1"] + " " + payload["col2"] + "/" + payload["col3"])
	ui.ShowCompactStyledInputModal(r.pages, r.app, "Port Forward", "Remote port:", remoteDefault, 12,
		func(text string, _ rune) bool {
			text = strings.TrimSpace(text)
			if text == "" {
				return true
			}
			n, err := strconv.Atoi(strings.Split(text, "/")[0])
			return err == nil && n > 0 && n <= 65535
		},
		func(remote string, cancelled bool) {
			if cancelled {
				r.FocusTable()
				return
			}
			remote = strings.TrimSpace(remote)
			if remote == "" {
				remote = remoteDefault
			}
			ui.ShowCompactStyledInputModal(r.pages, r.app, "Port Forward",
				"Local port (or auto):", localDefault, 12,
				func(text string, _ rune) bool {
					text = strings.TrimSpace(strings.ToLower(text))
					if text == "" || text == "auto" || text == "0" {
						return true
					}
					n, err := strconv.Atoi(text)
					return err == nil && n > 0 && n <= 65535
				},
				func(local string, cancelled bool) {
					if cancelled {
						r.FocusTable()
						return
					}
					local = strings.TrimSpace(local)
					if local == "" {
						local = localDefault
					}
					confirm := fmt.Sprintf("Forward %s\n  remote :%s  →  127.0.0.1:%s ?",
						strings.TrimSpace(target), remote, local)
					ui.ShowStandardConfirmationModal(r.pages, r.app, "Confirm Forward", confirm,
						func(ok bool) {
							r.FocusTable()
							if !ok {
								return
							}
							payload["remote_port"] = remote
							payload["local_port"] = local
							payload["kind"] = payload["col1"]
							payload["namespace"] = payload["col2"]
							payload["name"] = payload["col3"]
							payload["ports"] = payload["col6"]
							r.runAction("start_forward", payload)
						})
				})
		})
}

func (r *RPCRenderer) promptStopForward() {
	payload := r.selectionPayload()
	label := payload["key"]
	if payload["col3"] != "" && payload["col2"] != "" {
		label = payload["col2"] + "/" + payload["col3"]
	}
	if label == "" {
		r.core.Log("[yellow]nothing selected")
		return
	}
	ui.ShowStandardConfirmationModal(r.pages, r.app, "Stop Forward",
		fmt.Sprintf("Stop port-forward for %q?", label),
		func(ok bool) {
			r.FocusTable()
			if ok {
				r.runAction("stop_forward", payload)
			}
		})
}

func (r *RPCRenderer) promptFilterNamespace() {
	payload := r.selectionPayload()
	placeholder := payload["key"]
	if r.currentView == "namespaces" && placeholder != "" {
		r.runAction("filter_namespace", map[string]string{
			"namespace": placeholder,
			"key":       placeholder,
		})
		return
	}
	ui.ShowCompactStyledInputModal(r.pages, r.app, "Filter Namespace", "Namespace (or all):", placeholder, 40, nil,
		func(ns string, cancelled bool) {
			r.FocusTable()
			if cancelled {
				return
			}
			r.runAction("filter_namespace", map[string]string{
				"namespace": strings.TrimSpace(ns),
			})
		})
}

func firstPortFromCell(cell string) string {
	cell = strings.TrimSpace(cell)
	if cell == "" || cell == "-" {
		return ""
	}
	part := strings.Split(cell, ",")[0]
	part = strings.TrimSpace(strings.Split(part, "/")[0])
	if _, err := strconv.Atoi(part); err != nil {
		return ""
	}
	return part
}

func (r *RPCRenderer) runAction(action string, payload map[string]string) {
	if r.plugin == nil {
		r.core.Log("[yellow]plugin still loading…")
		return
	}
	if payload == nil {
		payload = r.selectionPayload()
	}
	viewID := r.currentView
	go func() {
		result, err := r.plugin.DoAction(pluginrpc.ActionRequest{
			Action:  action,
			View:    viewID,
			Payload: payload,
		})
		if err == nil && result.ExternalSession != nil {
			sess := *result.ExternalSession
			r.app.QueueUpdate(func() {
				if result.Message != "" {
					r.core.Log("[green]" + result.Message)
				}
				r.launchExternalSession(sess)
			})
			return
		}
		r.app.QueueUpdateDraw(func() {
			if err != nil {
				r.core.Log(fmt.Sprintf("[red]action %s failed: %v", action, err))
				return
			}
			if result.Message != "" {
				if result.OK {
					r.core.Log("[green]" + result.Message)
				} else {
					r.core.Log("[red]" + result.Message)
				}
			}
			if result.ModalTitle != "" || result.ModalBody != "" {
				title := result.ModalTitle
				if title == "" {
					title = "Detail"
				}
				ui.ShowInfoModal(r.pages, r.app, title, result.ModalBody, func() {
					r.FocusTable()
				})
			}
			if result.Next != nil {
				r.Apply(*result.Next)
				r.FocusTable()
				return
			}
			if result.OK && result.ModalBody == "" {
				r.refresh()
			}
		})
	}()
}

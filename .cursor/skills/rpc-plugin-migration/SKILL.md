---
name: rpc-plugin-migration
description: >-
  Migrates OMO native .so plugins to HashiCorp go-plugin RPC using the redis
  pilot as the reference. Use when converting a plugin to RPC, adding
  pluginrpc ViewData/DoAction support, extending RPC_PLUGINS, debugging
  rpc-host.log freezes/keybindings/breadcrumbs, or porting redis-parity
  features to docker/postgres/kafka/etc.
---

# OMO RPC Plugin Migration

Reference implementation: any plugin under `plugins/*/service.go` (redis was the pilot).
All shipped plugins are in `RPC_PLUGINS` in the Makefile.
Host UI is shared — do **not** reimplement CoreView inside the plugin process.

## Architecture (required)

1. **Host owns TUI** (`internal/host/rpc_renderer.go` + `pkg/ui.CoreView`)
2. **Plugin owns data** via `pkg/pluginrpc`: `GetMetadata`, `Configure`, `GetView`, `DoAction`, `Stop`
3. **Host resolves secrets in-process** and pushes settings with `Configure` — never call secrets broker from the plugin during `GetView` (nested RPC deadlocks)
4. Dual-load: executable → RPC; `.so` → native `plugin.Open`

## Host fixes already shared (all RPC plugins get these)

Do not re-copy into each plugin. They live in host/`pkg/ui`:

| Fix | Where |
|-----|--------|
| `Log` never blocks UI thread | `pkg/ui/core_info.go` — `go app.QueueUpdate(...)` |
| `?` opens info modal | `CoreView.ShowHelpModal` |
| Breadcrumbs on view switch | `RPCRenderer.Apply` → `SetViewStack` |
| ESC → previous view | `RPCRenderer.handleCoreAction` |
| Tab focuses plugin table | `Host.FocusPluginContent` + `main.go` Tab |
| Activate without tview deadlock | sync mount shell only; async Launch→Configure→GetView→`QueueUpdateDraw(Apply)` |
| Confirm/input/detail modals | `RPCRenderer.dispatchAction` + `ActionResult.ModalTitle/Body` |
| Track current view on refresh | `ViewRequest.View` / `ViewData.View` |

When migrating plugin N, only add plugin-specific actions to `dispatchAction` if needed (or keep generic `runAction`).

Breadcrumbs use `r.name` + `homeView` (first `ViewData.View`); ESC returns to `goto_<homeView>`.

## Per-plugin checklist

1. Extract `Service` (no tview) implementing `pluginrpc.Plugin`
2. `cmd/<name>/main.go` → `pluginrpc.Serve(NewService())`
3. Port every native view to `GetView`/`buildView` with stable view IDs
4. Port every keybinding to `KeyBindings` + `DoAction` (`goto_*`, mutations, details)
5. Reuse existing client/API code from the plugin package
6. Add name to `RPC_PLUGINS` in `Makefile`; add `plugin-<name>` target if useful
7. `Configure` accepts host-resolved settings map (same shape as KeePass entry fields)
8. Build: `make plugin-<name>` installs to `~/.omo/plugins/<name>/<name>`
9. Verify with `~/.omo/logs/rpc-host.log` and `*-rpc.log`

## ViewData / Action conventions

```text
ViewData.View     = "keys" | "info" | …   # round-trip id
KeyBinding.Action = "goto_info" | "delete" | "refresh" | …
ActionResult.Next = full replacement snapshot (preferred after mutations/nav)
ActionResult.ModalTitle/Body = host ShowInfoModal (key content, doctor, help-like)
```

Nav actions: `goto_<viewID>`. Refresh: rebuild `currentView`. Host always rebinds `R` `?` `/`.

## Hard rules (from redis freezes)

- Never `QueueUpdate`/`QueueUpdateDraw` **from inside** a `QueueUpdateDraw` callback without `go` (deadlock)
- Never `SetFocus` / `QueueUpdateDraw` / blocking `Log` from list `SetSelectedFunc` (sync activate path)
- Never MuxBroker secrets during plugin `GetView`
- OpenRPCLog must not re-enter its own mutex (`pkg/pluginrpc/log.go`)

## Reference files

- Protocol: `pkg/pluginrpc/{plugin,view,rpc,serve}.go`
- Host: `internal/host/{plugin_manager,rpc_renderer,host}.go`
- Redis service: `plugins/redis/service.go`, `service_views.go`, `cmd/redis/main.go`
- Deeper pitfalls: [reference.md](reference.md)

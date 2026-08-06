# RPC plugin reference (redis pilot lessons)

## Activate lifecycle

```
SelectedFunc (UI thread)
  → sync: NewRPCRenderer + ShowLoading + mount shell  (NO Apply, NO SetFocus)
  → go activateAsync:
       Launch → GetMetadata → resolvePluginConfig → Configure → GetView
       → QueueUpdateDraw(Apply + FocusTable)
```

## Configure settings map

Typical keys from KeePass entry: `name`, `host`/`URL`, `port`, `username`, `password`, `database`.
Host: `resolvePluginConfig` in `plugin_manager.go` prefers `<plugin>/development/local`.

## Breadcrumbs

Host sets `[]string{pluginName, homeView, …secondary}` via `SetViewStack` in `RPCRenderer.Apply`.
ESC returns to `goto_<homeView>`.

## Modal actions host should special-case (optional)

| Action | Host UX |
|--------|---------|
| `delete` / destructive | `ShowStandardConfirmationModal` then `DoAction` |
| `create_*` / forms | `ShowCompactStyledInputModal` then `DoAction` with payload |
| `view_*` / doctor | `DoAction` → `ModalTitle`/`ModalBody` → `ShowInfoModal` |
| interactive shell (ssh) | `ActionResult.ExternalSession` → host Suspend + exec |
| live streams (pubsub) | peek window or poll; avoid long blocking without UI feedback |

## Makefile / release

```make
RPC_PLUGINS := redis docker …   # all plugins
```

Install path: `~/.omo/plugins/<name>/<name>` (executable).

CI (`.github/workflows/release.yml`) builds `./plugins/<name>/cmd/<name>` into
`{name}-{VERSION}-linux-{amd64,arm64}.tar.gz` (binary named `<name>` inside).
Package Manager extracts to `PluginBinPath`.

## Logs

- Host: `~/.omo/logs/rpc-host.log`
- Plugin: `~/.omo/logs/<name>-rpc.log` (via pluginrpc OpenRPCLog)

## Feature checklist

1. Inventory `KeyBindings` + view IDs in `service.go` / `service_views.go`
2. Map each to view id + action string
3. Implement `buildViewLocked` / `DoAction` cases
4. Build with `make plugin-<name>` and verify via rpc logs

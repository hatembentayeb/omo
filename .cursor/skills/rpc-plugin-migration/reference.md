# RPC migration reference (redis pilot lessons)

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

Native stack pattern: `[]string{"<plugin>", "defaultView"}` then append secondary view id.
RPC host already does `redis > keys [> view]` — for other plugins, generalize `SetViewStack` in `Apply` using `view.View` and plugin name (update `RPCRenderer` to use `r.name` instead of hardcoding `"redis"` when migrating the second plugin).

## Modal actions host should special-case (optional)

| Action | Host UX |
|--------|---------|
| `delete` / destructive | `ShowStandardConfirmationModal` then `DoAction` |
| `create_*` / forms | `ShowCompactStyledInputModal` then `DoAction` with payload |
| `view_*` / doctor | `DoAction` → `ModalTitle`/`ModalBody` → `ShowInfoModal` |
| live streams (pubsub) | peek window or poll; avoid long blocking without UI feedback |

## Makefile

```make
RPC_PLUGINS := redis docker   # space-separated
```

Install path: `~/.omo/plugins/<name>/<name>` (executable, not `.so`).

## Logs

- Host: `~/.omo/logs/rpc-host.log`
- Plugin: `~/.omo/logs/<name>-rpc.log` (via pluginrpc OpenRPCLog)

## Feature parity method

1. Inventory native `AddKeyBinding` + view list from `*_view*.go`
2. Map each to view id + action string
3. Implement `buildViewLocked` / `DoAction` cases
4. Keep native `.so` path working until RPC is default for that plugin

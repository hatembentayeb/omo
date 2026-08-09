---
name: plugin-ui-views
description: >-
  OMO RPC plugin header UI: Views 0-9, Actions in former logs column, ? help
  grouped by view. Use when adding/migrating plugin service_views bindings,
  ViewBindings/Actions/HelpSections, digit view nav, or decorating ViewData.
---

# Plugin UI: Views / Actions / Help

Host renders three header columns: **Info** | **Views** | **Actions** (former logs).
Sidebar stays host-only (Refresh Plugins / Package Manager).

Reference: `plugins/redis/service_views.go`, `plugins/docker/service_views.go`.
Host wiring: `internal/host/rpc_renderer.go` (`Apply`).

## ViewData fields

| Field | Column / use |
|-------|----------------|
| `ViewBindings` | Middle **Views** — digits `0`–`9` → `goto_<id>` |
| `Actions` | Right **Actions** (former logs) — **this view only** |
| `KeyBindings` | Overflow views only (letter keys); host binds silently, listed in `?` |
| `HelpSections` | `?` modal, grouped by view / topic |

Do **not** stuff nav + actions into `KeyBindings` (legacy `withNav`).

## Required helpers in `service_views.go`

```go
func viewNavBindings() []pluginrpc.KeyBinding { /* 0..N goto_* */ }

func moreViewBindings() []pluginrpc.KeyBinding { /* optional overflow letters */ }

func fooActions() []pluginrpc.KeyBinding { /* per-view mutations */ }

func helpSections() []pluginrpc.HelpSection {
	return []pluginrpc.HelpSection{
		{Title: "Views (0-N)", Bindings: viewNavBindings()},
		{Title: "More Views", Bindings: moreViewBindings()}, // omit if empty
		{Title: "Foo", Bindings: fooActions()},
		{Title: "Global", Bindings: []pluginrpc.KeyBinding{
			{Key: "R", Label: "Refresh"},
			{Key: "?", Label: "Help (this screen)"},
			{Key: "/", Label: "Filter"},
			{Key: "^t", Label: "Switch target"},
			{Key: "ESC", Label: "Back / home"},
		}},
	}
}

func decorate(view pluginrpc.ViewData, actions ...pluginrpc.KeyBinding) pluginrpc.ViewData {
	view.ViewBindings = viewNavBindings()
	view.KeyBindings = moreViewBindings() // or nil if ≤10 views
	view.Actions = actions
	view.HelpSections = helpSections()
	return view
}
```

Each view builder:

```go
return decorate(pluginrpc.ViewData{
	View: viewFoo, Title: "...", Info: s.baseInfo(""),
	Status: "connected", Headers: ..., Rows: ..., SelectionKey: ...,
}, fooActions()...), nil
```

Views with no actions: `decorate(viewData)` (no extras).

## Rules

1. Primary views on digits only (`0`–`9`). If >10 views, overflow letters in `KeyBindings` + **More Views** help section.
2. Action letter keys must not collide with overflow view letters on the same screen (digits freed letters for actions).
3. Every `DoAction` mutation users need must appear in that view’s `Actions` (or Enter via host/`SelectedFunc`).
4. Host adds globals `R` `?` `/` `^t` to the Actions column — do not duplicate in `Actions`.
5. Never put plugin actions in the left sidebar.

## Checklist (migrate / new plugin)

- [ ] Replace `withNav` / letter nav with `viewNavBindings` + `decorate`
- [ ] Split per-view `*Actions()` helpers
- [ ] `HelpSections` covers all views + Global
- [ ] Bind any previously unbound `DoAction` cases worth exposing
- [ ] `make plugin-<name>` (or `make`) and smoke-test digits / actions / `?`

## Host behavior (do not reimplement)

- Digit / `goto_*` → Views column (`AddViewBinding`)
- Non-digit `goto_*` in `KeyBindings` → silent `BindKey` + `?`
- `Actions` → Actions column (`AddKeyBinding`)
- `?` → `HelpSections` modal

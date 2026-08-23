# manifest.json

```json
{
	"schemaVersion": 1,
	"id": "usse.music",
	"name": "Music",
	"version": "1.0.0",
	"author": "you",
	"description": "A music plugin",
	"kinds": ["bar-widget", "panel"],
	"entryPoints": {
		"barWidget": "ui/BarWidget.qml",
		"panel": "ui/Panel.qml"
	}
}
```

## Fields

- `schemaVersion` — must be `1`.
- `id` — dotted `<author>.<name>`; `oma create` builds it from `-a` (or the OS
  user when omitted) plus the project dir name. The `omarchy.*` namespace is
  reserved for built-ins and rejected by `omarchy plugin validate`.

Optional metadata: scaffolded manifests carry `"framework": "oma"` so
tooling can identify the generator; omarchy ignores it and it must never be
required.
- `kinds` — any combination of the plugin kinds (below).
- `entryPoints` — **camelCase** keys, values are paths relative to the plugin root.

## Plugin kinds

| Kind | What it is |
|---|---|
| `bar` | a full status bar (replaces the built-in `omarchy.bar`) |
| `bar-widget` | a widget in the bar (clock, workspaces, tray) |
| `menu` | a summoned launcher/menu |
| `panel` | a summoned settings/status panel (e.g. OSD) |
| `overlay` | a fullscreen popup layer |
| `service` | a headless singleton, no UI |

`app` is **not** a plugin kind — a standalone app is a normal Linux app.

## entryPoint keys

`bar`, `barWidget`, `menu`, `panel`, `overlay`, `service`. The surface QML files are
`ui/Bar.qml`, `ui/BarWidget.qml`, `ui/Menu.qml`, `ui/Panel.qml`, `ui/Overlay.qml`,
`ui/Service.qml` respectively.

## Extra kind-specific fields

Optional top-level fields exist per kind, e.g. `barWidget` (`displayName`,
`category`, `defaultSection`, `defaults`, `schema`) and `keepLoaded`/`activation`.
`omarchy plugin validate <dir>` enforces the full schema.

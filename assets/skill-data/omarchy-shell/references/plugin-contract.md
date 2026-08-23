# Plugin contract (omarchy-shell)

Extracted from the installed shell source (`/usr/share/omarchy/shell/`
— `services/PluginRegistry.qml`, `shell.qml`, `Ui/BarWidget.qml`).

## Discovery and enablement

- Third-party plugins live in `~/.config/omarchy/plugins/<id>/` with a
  `manifest.json`. The registry rescans on shell start and watches the plugins
  dir via inotify (`close_write/create/delete/move`) — a changed plugin file
  triggers a per-plugin reload, not a full shell restart.
- Manifest validation requires `schemaVersion: 1`, non-empty `id`, `name`,
  `version`, `kinds`, `entryPoints`. Entry points must be relative paths inside
  the plugin dir (`/`-prefixed or containing `..` rejects the whole manifest).
- **The `omarchy.*` id namespace is reserved** for first-party plugins;
  third-party ids with that prefix are rejected.
- Optional manifest fields the shell honors:
  - `barWidget.defaultSection` — `left | center | right` (default `center`)
    where a newly enabled bar widget lands.
  - `keepLoaded: true` — panel surfaces stay mounted after first summon.
  - `omarchy.clonedFrom` — clone bookkeeping (managed by omarchy itself).

## Enabled means "referenced in shell.json"

- kind includes `bar`: enabled when `bar.id` equals the plugin id.
- kind is `bar-widget`: enabled when an entry `{ id: "<id>", ...overrides }`
  exists in `bar.layout.left|center|right`.
- everything else: enabled when `{ id }` appears in the top-level `plugins[]`
  array. First-party non-bar plugins are implicitly enabled unless listed in
  `disabledPlugins[]`.

One placement suffices for the whole plugin — the shell treats any entry in
`bar.layout` **or** `plugins[]` as "enabled", so a `bar-widget` plugin with
additional kinds (service/overlay) still loads them from its `bar.layout`
entry alone. The tricky transition: a plugin whose kinds gained `bar-widget`
**after** an earlier enable stays stuck in `plugins[]` — `setEnabled` skips
the widget branch for ids it already knows and `putBarWidget` silently reports
`"ok"` without placing anything, so neither can migrate it. `oma install`
detects and migrates this state automatically; `oma status` flags it.

## Surface injection (what the shell sets on your root object)

Panels/overlays/menus/services are loaded through a `Loader`; on load it sets,
if the property exists on your root:

| Property | Value |
|---|---|
| `omarchyPath` | Omarchy install path |
| `shell` | the shell object (has `hide(pluginId)`, IPC helpers) |
| `manifest` | your parsed manifest.json |
| `barWidgetRegistry` / `pluginRegistry` | live registries |
| `service` | matching service singleton, if one was loaded |

## Surface lifecycle: churn

A panel/overlay/menu Loader stays active only while the surface is open
(`keepLoaded: true` keeps it mounted after first summon). Hiding the surface
**destroys** the QML instance — the bridge QtObject and every QML-local
variable with it — while the plugin's JS module survives in the engine
(module instances are cached per URL).

Consequences for plugin authors:

- The JS module (state, config, services) is the only durable home for shared
  state. Never cache QObject references across instances.
- The generated bridge handles this: it re-targets its persistence channel on
  every mount, so `config()` data survives churn. QML-local variables reset
  per summon — hydrate them from bridged state in `Component.onCompleted`.
- `oma build` prints a note when `config()` is used without `keepLoaded`;
  that is informational, not an error (destroy-on-hide is the right default
  for most panels).

Bar widgets extend `qs.Ui.BarWidget`, which declares the injected trio:
`bar` (host bar), `moduleName` (must equal the manifest id — it keys settings
lookup and inline IPC routing), and `settings` (the layout entry's extra keys).
Helpers: `setting(key, fallback)` reads one override; `broadcast(method)` runs
a method on every live instance across monitors.

## Shell IPC

CLI: `omarchy-shell <target> <method> [args...]` forwards to `qs ipc` against
the running shell. Target `"shell"` exposes:

```
ping() · rescanPlugins() · reloadConfig() · toggleBarTransparency()
setPluginEnabled(id, enabled) · enablePlugin(id, placementJson)
putBarWidget(id, placementJson) · moveBarWidget(id, placementJson)
setBarWidget(id, key, valueJson, selectorJson)
listPlugins() · listShellConfig() · debugBarGeometry()
summon(id, payloadJson) · hide(id) · toggle(id, payloadJson)
togglePanelAt(section, index) · call(id, method, arg)
```

Semantics worth knowing:

- `summon` requires the plugin to be **enabled**; disabled plugins answer
  plainly ("not enabled") instead of silently no-op'ing.
- Bar-widget panels don't take payloads — payloadJson is dropped for them.
- Panel roots should implement `open(payloadJson)` / `close()` and expose
  `opened`; `hide()` calls `close()` then clears open-state. Dismissing from
  inside must call `shell.hide(pluginId)` so the shell's state stays in sync.
- `call(id, method, arg)` invokes an arbitrary function on a loaded surface —
  this is how launcher entries with custom actions work.
- `putBarWidget` **silently no-ops** when the widget id is already known in
  `plugins[]`: it returns `"ok"` while placing nothing (the underlying
  `moveBarEntry` error is discarded). Re-run `oma install` to migrate such a
  stuck placement instead of calling it manually.

`call` sharp edges (all verified against the shipped shell):

- **Arity is strict.** `call(id, method, arg)` takes exactly three strings;
  `omarchy-shell shell call <id> <method>` fails with "Too few arguments
  provided". The wrapper script only auto-fills `"{}"` for `summon`/`toggle` —
  for `call` you must pass the filler yourself: `shell call <id> clearDone '{}'`.
  (`oma ipc` does this automatically.)
- **`"unknown"` conflates two failures.** The shell returns `"unknown"` both
  when the surface is not loaded (hidden panels are destroyed — see above)
  and when the method does not exist. Distinguish by loading the surface
  (`summon`), or diff against a known method; `oma ipc` checks the installed
  entry points locally.
- **Args are raw strings.** `"$@"` passes through verbatim — there is no JSON
  decoding. A quoted test string arrives containing the literal quote
  characters; pass already-encoded JSON and decode in the action.
- **Desktop Exec cannot carry empty args.** A launcher `exec` like
  `omarchy-shell <id> startFocus ''` loses the empty argument when the
  `.desktop` entry is parsed, and a typed `IpcHandler` param is mandatory —
  the launcher then fails with "Too few arguments provided". Give
  launcher-targeted actions a **zero-arg handler** that applies defaults:
  `function startFocus(): string { logic.startFocus(""); return "ok" }`.

## Settings persistence convention

Per-plugin settings live in `~/.config/omarchy/<id>.json`. oma-generated
bridges do this automatically for `config()` stores. Widget-level inline
overrides additionally ride along the `bar.layout` entries via `settings`.

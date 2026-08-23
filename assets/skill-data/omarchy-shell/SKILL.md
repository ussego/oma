---
name: omarchy-shell
description: Use when developing Omarchy shell plugins — the PluginRegistry manifest contract (kinds, entryPoints, defaultSection, keepLoaded), what properties the shell injects into surfaces, shell IPC (summon/toggle/hide/call, enable/put/move bar widgets), and the per-plugin hot-reload + <id>.json settings conventions. Distinct from end-user Omarchy configuration (the distro omarchy skill).
---

# omarchy-shell

Omarchy's shell is a single long-running Quickshell 0.3 process
(`omarchy-shell`) that hosts every plugin. Surfaces are Quickshell windows,
not separate processes. (End-user desktop customization belongs to the distro's
`omarchy` skill; this one is about the plugin runtime.)

## The contract

The full extracted contract lives in
[references/plugin-contract.md](references/plugin-contract.md) — discovery,
enablement rules (shell.json `bar.id` / `bar.layout.*` / `plugins[]`),
manifest extras (`barWidget.defaultSection`, `keepLoaded`), surface injection
(`shell`, `manifest`, `settings`, `service`, …), and the complete IPC table.

Fast facts:

- Plugins: `~/.config/omarchy/plugins/<id>/`; `omarchy.*` ids are reserved.
- Hot reload is per-plugin via inotify on that directory.
- Panel roots get `open(payloadJson)` / `close()` called by summon/hide; expose
  `opened`. Bar widgets extend `qs.Ui.BarWidget` (`moduleName` must equal the
  manifest id).
- Summon from CLI/scripts: `omarchy-shell shell summon <id> '{"json":"payload"}'`
  (also `toggle`, `hide`, `call`).

## Window types

- `FloatingWindow` (`import Quickshell`) — normal floating top-level window;
  panels wrap one, `visible: false` initially.
- `PanelWindow` — layer-shell panel surface (overlays use it with an
  exclusive-keyboard scrim).

## Interpreter constraints (verified)

- **`../` relative script imports do not resolve** — keep the JS bundle in the
  same directory as the QML that imports it.
- **qmldir singletons never run `Component.onCompleted`** — don't rely on
  singleton completion for wiring.
- Window types create their own QML context.

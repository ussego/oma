---
name: oma
description: Use when building, scaffolding, or debugging oma plugins for the Omarchy shell — writing state()/derived()/config()/ipc logic in plain JavaScript, generating the JS→QML bridge, authoring manifest.json, running the oma CLI (create/build/package/install), or debugging QML↔JS state sync.
---

# oma

oma is an SDK and CLI for building Omarchy shell plugins. Plain JavaScript is the
logic layer (TypeScript annotations are accepted too — esbuild strips them at
build; no external toolchain, `oma` bundles everything itself); QML is the UI;
one shared core serves every surface (`bar`, `bar-widget`, `menu`, `panel`,
`overlay`, `service`).

## Project layout

```text
project/
├── manifest.json     # plugin manifest (schemaVersion 1)
├── oma.json          # optional project config ($schema points at the bundled schema)
├── src/index.js      # shared logic: state, actions, services (.ts works too)
└── ui/               # QML surfaces + generated bridge (index.mjs, <Name>.qml)
```

## Runtime (import from "@oma/runtime")

The specifier `@oma/runtime` resolves to the runtime embedded in the `oma`
binary during build (also published on JSR as `@oma/runtime` for editor
types). Exports:

- `state(value)` / `state({...})` — reactive value/object with `.subscribe`.
  Object fields are read/written directly (`music.playing = true`).
- `derived(() => expr)` — recomputes when dependencies change; read via `.value`.
- `config(defaults)` — per-instance store `{ get, set, subscribe }`. Persists to
  `~/.config/omarchy/<id>.json` automatically via the generated bridge.
- `emit(event, payload)` / `on(event, fn)` — in-process event bus (surfaces
  share one shell process).

Install for editor types / unit tests (optional — builds embed the runtime):
`deno add jsr:@oma/runtime`, `pnpm add jsr:@oma/runtime` (10.9+), or
`npx jsr add @oma/runtime` for npm/bun (commit the generated `.npmrc`).

## CLI

- `oma create <name> [-s kinds] [-a author] [-d desc] [-v ver]` — scaffold
  multiple surfaces (TUI multi-select for anything not passed as a flag;
  `-v` is flag-only and defaults to `0.1.0`; `--panel-mode attached|window|both`
  picks the panel presentation, persisted in `oma.json`).
- `oma surface add [kinds...]` — add surfaces to an existing project (run in
  the project dir; idempotent, interactive multi-select when no kinds given;
  `--panel-mode` switches an existing panel's presentation).
- `oma status` — one-shot read-only state: build freshness, installed copy,
  launcher entries, tools.
- `oma build` — bundle `src/index.js` → `ui/index.mjs` (esbuild embedded in the
  Go binary, target ES2016 for QJSEngine), generate `ui/<Name>.qml` (the
  bridge) by statically scanning exports.
- `oma package` — assemble `pkg/` (manifest.json + ui/) and `omarchy plugin validate`.
- `oma install` / `oma uninstall` / `oma restart` (alias: `r`). Install also
  enables the plugin (discovered plugins start disabled — summon ignores them)
  and creates launcher entries declared in `oma.json`.
- `oma launcher add | remove` — upsert/drop entries in oma.json (`add` is a
  wizard without arguments) and keep the `.desktop` files in sync.
- `oma skills list` / `oma skills get <name> [--full]` / `oma skills get --all`
  — pull this skill's deep dives.

## The bridge

`oma build` bundles `src/index.js` to `ui/index.mjs`, then scans the source
statically: every `export const x = state({...})` becomes reactive properties,
every exported lowercase function becomes a bridge method on `ui/<Name>.qml`
(a `QtObject` with auto-NOTIFY properties and a subscribe loop that pushes
state into QML). State initializers must be object literals with fixed keys —
spreads, computed keys and arrays fail the build with a clear error.
See `references/bridge.md`.

## Libraries

Standard ESM: relative imports plus whatever your package manager puts in
`node_modules` (esbuild resolves it natively during `oma build`). Keep runtime
APIs within what QJSEngine knows — no `Object.fromEntries`, no BigInt (the
bundler defines it away as `Number`).

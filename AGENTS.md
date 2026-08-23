# AGENTS.md

## Project

This repository contains **oma**, an SDK and development framework for building native software for Omarchy.

Oma is designed around one core principle:

> **One project, shared logic, multiple surfaces.**

A single project may expose any combination of surfaces:

- `app` (a standalone native application)
- `bar` (the status bar)
- `bar-widget` (a widget in the bar)
- `menu` (a launcher)
- `panel` (a settings/status panel)
- `overlay` (a popup layer)
- `service` (a background service)

All surfaces must be able to reuse the same JavaScript logic, state, services, actions, configuration, and IPC.

---

## Architecture

```text
             oma
              │
      Shared JavaScript
              │
 ┌────────────┼────────────┐
 │            │            │
App       Bar Widget   Panel / Overlay
 │            │            │
 └────────────┼────────────┘
              │
             QML
              │
          Quickshell
              │
           Omarchy
```

Keep responsibilities separate:

- **JavaScript** (plain ESM) — application logic, state, services, actions,
  integrations, and data. No TypeScript toolchain; JSDoc where types help editors.
- **QML** — native UI, layouts, bindings, animations, and Quickshell-facing UI.
- **oma runtime** (`oma.js`, embedded in the CLI binary) — reactive state, config,
  ipc; bridges JS and QML.
- **oma CLI** — creation, development, validation, building, and packaging.
  Bundles with esbuild embedded as a Go library; one static binary, no external
  toolchain required.

Do not duplicate business logic between surfaces.

---

## Core Principles

### Prefer existing technology

Before implementing infrastructure, verify whether the capability already exists in:

1. Omarchy
2. Quickshell
3. Qt/QML
4. Deno and TypeScript/JavaScript
5. Existing project dependencies

Only build new infrastructure when there is a concrete requirement.

Do not recreate existing platform functionality without a strong reason.

### Keep oma small

Avoid speculative abstractions.

Do not add infrastructure merely because it might be useful later.

Prefer simple APIs over large frameworks, registries, factories, or abstraction layers.

### Prefer composition

Prefer small composable APIs.

Good:

```ts
registerSurface(...)
state(...)
derived(...)
action(...)
```

Avoid unnecessary hierarchies and indirection.

### Reuse code across surfaces

App, bar-widget, panel, and overlay are **surfaces**, not separate applications.

The same project should be able to expose multiple surfaces while keeping the underlying logic shared.

---

# JavaScript Runtime

Plain modern JavaScript (ESM) is the language for oma plugin logic.

Plugins are single `src/index.js` modules (`src/index.ts` also works: esbuild
strips TypeScript annotations during bundling, but keep logic JS-first). There
is no TypeScript toolchain and no external runtime: the `oma` CLI embeds
esbuild as a Go library and bundles everything itself.

Use JavaScript for:

- state
- actions
- services
- API clients
- configuration
- IPC
- integrations
- application logic

Add JSDoc where types help editors; do not reintroduce a TS build step.

Libraries come from `node_modules` (any package manager; esbuild resolves it
natively during `oma build`) or plain relative files. The specifier
`@oma/runtime` resolves to the runtime embedded in the binary (also published
on JSR for editor types).

---

# Reactive State

Oma uses a simple reactive state API inspired by Svelte.

The preferred API is:

```js
const count = state(0);
const name = state("Oma");
const enabled = state(true);
```

Object state:

```js
const music = state({
	playing: false,
	song: "",
	volume: 100,
});
```

Fields are read and written directly (`music.playing = true`); subscribers fire
on any change, including deep mutations.

### Derived state

Support derived/reactive values when needed:

```js
const status = derived(() => (music.playing ? "Playing" : "Paused"));
```

Derived values are not bridged into QML — only `state({...})` exports are.

---

# JavaScript ↔ QML State

Reactive state is not limited to one side.

The same state must be observable from QML.

Conceptually:

```text
         JavaScript
             │
         state(...)
            │
            ▼
       oma runtime
        /       \
       /         \
JavaScript       QML
```

Example:

```ts
export const music = state({
	playing: false,
	song: "",
	volume: 100,
});

export function toggle() {
	music.playing = !music.playing;
}
```

QML should be able to consume the same API:

```qml
Text {
    text: Music.song
}

OmaButton {
    text: Music.playing ? "Pause" : "Play"

    onClicked: Music.toggle()
}
```

Do not create separate state stores for JS and QML.

There must be one source of truth.

### Bridge mechanism (verified)

The bridge is code generation, not a runtime object. `oma build` bundles `src/index.js` to `ui/index.mjs` (ESM — QJSEngine
runs `import "index.mjs" as Logic`), then statically scans the source (Go, `bridge.go`) and emits a QML `QtObject` that:

- declares one `property` per `state({...})` field (auto-NOTIFY, so QML bindings react),
- declares one `function` per action that delegates to `Logic.<fn>(...)`,
- on `Component.onCompleted`, subscribes to each state and pushes its fields into the properties.

The scan never executes plugin code: every `export const x = state({...})` must be an object literal with fixed keys
(spreads, computed keys and arrays fail the build); exported lowercase functions become actions. Bundling uses esbuild
embedded in the CLI (`bundle.go`) with `target: "es2016"`, which transpiles async arrows, object spread, class fields,
optional chaining, and nullish down to what QJSEngine parses. A module-scoped `globalThis` stand-in is injected; `BigInt`
is defined away as `Number`. If real-world state patterns ever outgrow the static scanner, the escalation path is
running the bundle under goja at build time — not planned, documented only.

Hard QJSEngine facts, verified against Quickshell 0.3 / Qt 6.11:

- **Plain JS values are not reactive in QML** — mutating a `Proxy`/`var`/`let` leaves bindings stale. Only a QObject
  property change re-evaluates bindings, so the generated `QtObject` is the single reactive surface.
- esbuild lowers syntax but does not polyfill runtime APIs, so keep `Object.fromEntries` out of runtime code.
  `Proxy`, `WeakMap`, `Set`, and array spread are supported.

---

# Editor Support and Autocomplete

A major goal of oma is strong editor support in both JavaScript and QML.

For example, after:

```js
export const music = state({
	playing: false,
	song: "",
	volume: 100,
});
```

QML can discover the public shape of the bridge:

```text
Music
├── playing: bool
├── song: string
├── volume: double
└── toggle(): void
```

The bridge generator derives this shape statically from `state({...})` literals,
and JSDoc comments give editors type hints on the JS side.

If necessary, oma may generate additional declarations or metadata. The exact
implementation should be kept as simple as possible.

Do not sacrifice developer experience by exposing an untyped dynamic object when
static metadata can be derived reliably.

---

# QML

QML is the native UI layer.

Use QML for:

- windows
- panels
- overlays
- bar widgets
- layouts
- animations
- bindings
- visual components
- Quickshell integration

Oma should make QML easier to use, not replace QML entirely.

Advanced developers must still be able to use normal QML where necessary.

---

# UI Components

Do not build a parallel component library. Omarchy already ships the components and theme that plugins import directly:

- `import qs.Ui` — `Button`, `TextField`, `Toggle`, `ToggleSwitch`, `ConfirmDialog`, `PopupCard`, `Dropdown`,
  `MultiSelect`, `SearchableDropdown`, `NumberField`, `Panel` + `Panel*` widgets, `BarWidget`, `WidgetButton`,
  `OpticalGlyph`.
- `import qs.Commons` — `Style`, `Color`, `Border`, `Util` singletons (theme and design system).

Prefer these over new components. Add a new component only when no existing `qs.Ui` type covers a concrete use case
(e.g. a `Toast`), and build it on `qs.Commons` values.

Avoid hardcoding:

- colors
- typography
- border radii
- spacing
- shadows

when the corresponding Omarchy or Quickshell values are available.

---

# Plugin Kinds

Omarchy shell plugins declare one or more kinds in their `manifest.json`. The kinds oma targets are:

```text
bar
bar-widget
menu
panel
overlay
service
```

A project can enable any combination.

Example:

```toml
[kinds]
bar-widget = true
overlay = true
```

- `bar` — the status bar itself.
- `bar-widget` — a widget in the bar (clock, workspaces, tray).
- `menu` — a launcher/menu.
- `panel` — a settings or status panel (e.g. OSD).
- `overlay` — a popup layer (clipboard, emoji picker).
- `service` — a background service (notifications, idle/lock).

`app` is not an Omarchy shell plugin kind. A standalone native application is a normal Linux app, not a shell plugin.

A `bar-widget` may open or interact with another kind, but that is optional.

Do not assume every bar widget requires a panel.

---

# Shared Core

A project must be able to share its logic across all surfaces.

Example:

```text
      Shared Core
   /      |       \
  /       |        \
App    Bar Widget  Overlay
 │         │          │
 └─────────┴──────────┘
```

The shared core may contain:

- state
- derived state
- actions
- services
- API clients
- configuration
- IPC
- integrations

Do not duplicate business logic inside individual QML surfaces.

---

# Project Structure

A typical oma project may look like:

```text
project/
├── manifest.json
├── oma.json          (optional)
├── src/
│   ├── index.js
│   ├── services/
│   └── actions/
└── ui/
    ├── App.qml
    ├── BarWidget.qml
    ├── Panel.qml
    └── Overlay.qml
```

The exact structure may evolve.

Do not lock the architecture to a directory layout before real use cases justify it.

---

# Omarchy Compatibility

Oma is built on top of Omarchy and Quickshell.

It is not a replacement plugin system.

Generated projects should remain compatible with the existing Omarchy plugin model.

Support existing Omarchy concepts such as:

- `bar`
- `bar-widget`
- `menu`
- `panel`
- `overlay`
- `service`

All of these run inside a single Quickshell process (`omarchy-shell`). oma must not assume a separate process per
surface.

Do not create an incompatible parallel marketplace or installation system.

---

# Marketplace

Oma projects should be publishable as normal Omarchy plugins.

The marketplace should not need to understand oma's internal runtime.

Oma should generate the required metadata and entry points for the existing Omarchy plugin system.

A generated manifest must follow Omarchy's schema (`schemaVersion: 1`, dotted `id`, `kinds`, `entryPoints`):

```json
{
	"schemaVersion": 1,
	"id": "usse.music",
	"name": "Music",
	"version": "1.0.0",
	"author": "...",
	"description": "...",
	"kinds": ["bar-widget", "overlay"],
	"entryPoints": {
		"barWidget": "ui/BarWidget.qml",
		"overlay": "ui/Overlay.qml"
	},
	"framework": "oma"
}
```

Entry point keys are camelCase (`bar`, `barWidget`, `menu`, `panel`, `overlay`, `service`). Plugin IDs are dotted
(`<author>.<name>`, author from `-a` or the OS user); the `omarchy.*` namespace is reserved for built-ins and is rejected by `omarchy plugin validate`.
User plugins live in `~/.config/omarchy/plugins/<id>/`.

Scaffolded manifests carry `"framework": "oma"` so marketplaces and tooling can
identify the generator. The field is optional metadata: omarchy's registry
ignores it, and it must never be required for installation.

---

# App + Plugin Model

A project may be both a native application and an Omarchy plugin.

Example:

```text
omusic
├── Native App
├── Bar Widget
└── Overlay
```

The project is installed once.

Users may enable whichever surfaces they want.

Avoid forcing developers to maintain:

```text
omusic-app
omusic-bar
omusic-overlay
```

as separate implementations.

---

# CLI

The CLI is written in Go, using [Bubble Tea](https://github.com/charmbracelet/bubbletea) and
[Bubbles](https://github.com/charmbracelet/bubbles) for the interactive TUI. Bundling uses esbuild embedded as a Go
library — one static binary with no external toolchain requirement.

The primary CLI is:

```bash
oma
```

Expected commands:

```bash
oma create <name>   # flags: -s kinds, -a author, -d desc, -v version, --panel-mode
oma surface add     # add surfaces to an existing project
oma status          # one-shot project state: build freshness, install, launcher
oma build
oma package
oma install         # also enables the plugin + materializes oma.json launcher entries
oma launcher        # add (wizard/upsert into oma.json) or remove entries
```

`oma create` should be able to scaffold multiple surfaces in a single project.
All inputs can be passed directly (bun-style flags, `--flag val` or `--flag=val`);
anything missing is prompted for in the wizard. Version is flag-only — it is
never prompted and defaults to `0.1.0`.

Panel presentation is chosen with `--panel-mode attached|window|both`
(default `attached`) on both `oma create` and `oma surface add`:

- `attached` — bar-anchored popup + widget (ships as a `bar-widget` kind).
- `window` — standalone draggable `FloatingWindow` (`ui/Panel.qml`, `panel`
  kind, no widget).
- `both` — the attached pair plus `ui/PanelWindow.qml` (kinds `bar-widget` +
  `panel`; `PanelWindow.qml` is the `panel` entry point).

The mode is persisted in `oma.json` (`"panel": {"mode": ...}`) and
`oma surface add` honors it as the default when the flag is omitted, switching
an existing panel between modes when the flag differs.

Example:

```bash
oma create omusic -s panel,overlay -a ussego -d "Music plugin"
```

Possible selection:

```text
bar-widget
panel
overlay
```

`oma surface add [kinds...]` extends an already-scaffolded project: appends to
`manifest.json` `kinds`/`entryPoints` and generates missing `ui/<Kind>.qml`
skeletons without touching existing files. Idempotent; run inside the project
dir. (`oma add` stays reserved for a future top-level flow.)

Multiple selections must be supported.

## Project configuration (`oma.json`)

Optional opt-in file at the project root. **Configuration is data, not code** —
JSON only, so the Go CLI reads it today and omarchy itself (plugin add / shell)
can consume it natively later without running anything. `oma create` scaffolds
a stub carrying `"$schema"` pointing at the schema's versioned raw URL
(`https://raw.githubusercontent.com/ussego/oma/v<tag>/assets/schemas/oma.json` —
tags are immutable; dev builds fall back to `main`), so editors autocomplete
and validate every available key without a local copy.

```json
{
  "icon": "utilities-system-monitor",
  "launchers": [
    {
      "name": "Play Next Music",
      "action": "summon",
      "icon": "",
      "comment": ""
    }
  ]
}
```

- `icon` — launcher icon: freedesktop name, file path (absolute or
  project-relative), or http(s) URL; default `application-x-executable`.
  URLs and files are installed into `~/.local/share/icons/hicolor/` by
  `oma install` / `oma launcher add` (entry name becomes the icon name;
  svg goes to `scalable/apps`, other formats to `256x256/apps`).
- `barIcon` — bar-widget glyph: a Nerd Fonts codepoint (e.g. `\uf013`) or
  any text rendered by `OpticalGlyph` in the bar font; scaffolded bar
  widgets fall back to a cog glyph.
- `panel` — panel presentation mode: `attached` (bar-anchored popup + widget,
  default), `window` (standalone FloatingWindow, no widget) or `both`. Set by
  `oma create` / `oma surface add --panel-mode`.
- `launchers[]` — opt-in launcher entries materialized to
  `~/.local/share/applications/<id>.desktop` by `oma install` and
  `oma launcher add`. `action` is `summon | toggle | hide` (default `summon`);
  `exec` may fully override the command line.
  `oma launcher add [name] [--action …] [--exec …] [--icon …]` upserts the
  entry into oma.json (wizard when no name is given) and writes the file in one
  step; same-name entries update in place. `oma launcher remove [names|--all]`
  drops them from oma.json again (multi-select wizard when unnamed) and deletes
  only files carrying the `X-Oma-Managed=true` marker. Absent `launchers[]` ⇒
  nothing is ever created.

Do not implement unused commands merely for completeness.

---

# Distribution

The CLI ships as one static binary (pure Go, CGO off, assets embedded) for
`linux/amd64` and `linux/arm64`.

- **Version**: `var cliVersion` in `main.go`; release builds inject it with
  `-ldflags "-X main.cliVersion=<tag>"`. `oma --version` prints it. When the
  flag is not injected (`go install pkg@version`, local `go build`), the CLI
  falls back to the module version embedded by the toolchain
  (`runtime/debug.ReadBuildInfo`), or `dev` outside a tagged checkout.
- **Releases**: `mise run release <X.Y.Z>` tags `vX.Y.Z` and pushes it (it
  validates the version, a clean tree, and main being current). The
  tag-push workflow builds both arches, tars them as
  `oma-<tag>-linux-<arch>.tar.gz`, publishes checksums to the GitHub
  Release, publishes `@oma/runtime` to JSR, and auto-bumps
  `packaging/aur/PKGBUILD` (pkgver + checksums, committed only when
  changed) via the `bump-pkgbuild` job.
- **Channels**: AUR `oma-bin` (`packaging/aur/PKGBUILD` — kept current by
  the release workflow; submitting to aur.archlinux.org is the only manual
  step, it needs the maintainer's AUR SSH key) and the curl installer
  (`install.sh`, installs to
  `$PREFIX` or `~/.local/bin`). Module path is
  `github.com/ussego/oma` (go.mod at the repo root), so
  `go install github.com/ussego/oma@latest` installs the `oma` binary.
- Keep release artifacts reproducible: same inputs → same flags; `-trimpath`
  always.

---

# Development Mode

`oma dev` is deliberately **not implemented**. Omarchy already hot-reloads a single plugin when files under
`~/.config/omarchy/plugins/<id>/` change (the shell runs `inotifywait` and reloads only that plugin, not the whole
shell). When fast iteration is needed, `oma install` plus the shell's per-plugin hot-reload covers it. Reconsider a
watch+rebuild loop only if that workflow proves insufficient.

---

# Build System

Implement:

```bash
oma build
```

A build should:

1. Bundle `src/index.js` to `ui/index.mjs` (esbuild embedded in the CLI).
2. Statically scan exports and generate the bridge QML.
3. Validate entry points exist.
4. Generate/update plugin metadata where necessary.

Generated artifacts must be reproducible.

Current layout emitted by `oma build`:

```text
project/
└── ui/
    ├── index.mjs     ESM bundle of src/*.js (esbuild, target ES2016)
    └── <Name>.qml    generated bridge QtObject (imports "index.mjs")
```

`<Name>` is the capitalized manifest name. Bundle, bridge, and surfaces share `ui/` because Quickshell's script loader
cannot resolve `../` imports and its qmldir singletons never run `Component.onCompleted`. `oma package` assembles `pkg/`
with `manifest.json` + `ui/` (+ `README`/`LICENSE`/`preview.*` if present) and validates it with `omarchy plugin validate`.
Built `ui/index.mjs` + `ui/<Name>.qml` (+ `ui/LauncherWriter.qml` when `oma.json:launchers[]` is set) must be committed —
`omarchy plugin add <git-url>` clones `HEAD` and validates without running `oma build`. `pkg/` is local validation only and
is gitignored by scaffold.

---

# Build Toolchain

The CLI is a single static Go binary. Bundling runs in-process via esbuild
embedded as a Go library (`github.com/evanw/esbuild`); the bridge generator is
plain Go (`bridge.go`). No Deno, no Node, no external toolchain — users need
only the `oma` binary to build a plugin.

Libraries are imported from `node_modules` (esbuild resolves them natively) or
as plain relative files.

---

# IPC

Provide a high-level IPC API when needed.

It should support communication between:

- surfaces
- plugin processes
- Quickshell
- external services
- Omarchy components

Example:

```js
emit("player:play");
on("player:play", (payload) => {
	// ...
});
```

Hide transport details behind the public API.

Do not invent a new IPC protocol unless existing mechanisms are insufficient.

---

# Configuration

Provide a unified configuration API.

Example:

```js
settings.get("volume");
settings.set("volume", 80);
```

Configuration must be accessible consistently across surfaces.

Persistence is automatic: the generated bridge loads
`~/.config/omarchy/<id>.json` on startup, seeds every `config()` store with the
saved values over their defaults, and writes changes back through a debounced
FileView — the same per-plugin settings file convention omarchy plugins use.
Authors never wire persistence manually.

---

# Theming

Oma UI should follow the active Omarchy theme.

Do not create a parallel theme system unless technically necessary.

Components should adapt when the Omarchy theme changes.

---

# Dependencies

Keep runtime dependencies minimal.

Dependencies come from `node_modules` via any package manager; esbuild bundles
them during `oma build`. Remember everything ships inside the plugin bundle and
must run under QJSEngine after ES2016 transpilation.

Before adding a dependency, check:

- whether Omarchy already provides the functionality
- whether Quickshell or Qt already provides it
- whether plain JavaScript already provides it
- whether an existing dependency already solves it
- whether the dependency materially reduces complexity

Avoid dependencies for trivial functionality.

---

# Testing

Prioritize testing public behavior.

Important areas include:

- JavaScript ↔ QML communication
- reactive state propagation
- derived state
- actions
- configuration
- IPC
- manifest generation
- packaging
- build output
- bridge metadata generation

Do not write tests that merely assert implementation details.

## Live verification

Hermetic tests cannot catch QJSEngine-only breakage. `live_test.go`
(build tag `live`) runs the real engine in two tiers:

```sh
go test -tags live -run Offscreen                # tier 1: safe, offscreen
OMA_LIVE_TEST=1 go test -tags live -run LiveShell # tier 2: restarts the shell
```

Tier 1 boots Quickshell offscreen on a generated fixture and asserts the
persistence round-trip (seeded read + debounced write) with zero session
impact — run it after any change to the runtime, bridge template, or bundler.
Tier 2 installs into the real plugins dir, restarts omarchy-shell, drives
summon/call/hide over IPC and asserts values plus clean session logs; it is
the release gate. Both skip cleanly when their prerequisites are missing.
Run live verification before tagging a release: offline checks have missed
TDZ ordering, QtObject child limits, tree-shaking holes and recursion loops
that the real engine caught immediately.

---

# Code Style

Prefer:

- small functions
- explicit APIs
- composition
- readable names
- minimal indirection
- straightforward control flow

Avoid:

- premature abstractions
- deep inheritance hierarchies
- unnecessary design patterns
- speculative extensibility
- duplicated logic
- framework code without a concrete use case

Comments should explain **why**, not restate **what**.

---

# Commits

Conventional Commits, no emojis, no em dashes:

```text
feat: add oma log / oma tail
fix(bridge): emit export-* scan notes
docs(skills): document --panel-mode
test(log): cover live count grouping
chore: bump deps
```

- Types: `feat` (new user-facing capability), `fix` (bug fix), `docs`
  (docs/skills), `test` (test-only), `refactor` (behavior-preserving
  restructure), `chore` (maintenance, no behavior change). Optional scope in
  parens (`bridge`, `log`, `skills`, `cli`, `examples`, ...).
- Summary: imperative, lowercase, under ~72 chars. Body bullets explain why.
- **Message-only by default**: small fixes and one-liners get no body at all
  (`fix(create): use npx jsr add @oma/runtime`). Add a body only when it
  carries information the summary cannot - context, trade-offs, or a
  non-obvious why.
- **No emojis and no em dashes - hyphens only, not even when it really,
  really seems necessary.**

# AI Agent Guidelines

When working on oma:

1. Inspect the existing code before changing architecture.
2. Verify how Omarchy, Quickshell, and Qt already solve the problem.
3. Reuse existing functionality when practical.
4. Implement the smallest useful solution.
5. Avoid speculative infrastructure.
6. Do not refactor unrelated code.
7. Do not invent APIs or behavior without verifying them.
8. Preserve compatibility with Omarchy's plugin model.
9. Keep shared logic independent from UI surfaces.
10. Prefer JSDoc over reintroducing a TypeScript build step.

When implementing state, prefer:

```js
const stateValue = state({
	enabled: false,
	value: 0,
});
```

over introducing unnecessary state classes or wrapper objects.

When adding an abstraction, there should be a concrete use case for it.

---

# Definition of Done

A feature is complete when:

1. It solves the requested problem.
2. It fits the existing oma architecture.
3. Shared logic remains reusable across surfaces.
4. Shared logic stays plain, portable JavaScript (ESM).
5. QML remains native and flexible.
6. JavaScript ↔ QML state synchronization works correctly.
7. Relevant checks/tests/builds pass.
8. Omarchy compatibility is preserved.
9. Public behavior is documented when necessary.

---

# Guiding Rules

> **One project, shared logic, multiple surfaces.**

> **Use plain ESM JavaScript for logic.**

> **Use QML for native UI and reactive bindings.**

> **Keep one source of truth for state.**

> **Prefer `state(value)` and `state({...})`.**

> **Reuse the platform before building new infrastructure.**

> **Build the smallest useful thing.**

```
```

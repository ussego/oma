# oma CLI

The CLI is Go (Bubble Tea + Bubbles + Lip Gloss for the TUI) with esbuild embedded
as a library — one static binary, no external toolchain required. The primary binary is `oma`.

## Commands

| Command | What it does |
|---|---|
| `oma create <name> [flags]` | scaffold a project; flags `-s kinds`, `-a author`, `-d desc`, `-v ver`, `--panel-mode attached|window|both` (both `--flag val` and `--flag=val`; version is flag-only) |
| `oma surface add [kinds...]` | add surfaces to an existing project (run in the project dir; idempotent, multi-select TUI with no args; `--panel-mode` switches an existing panel's presentation) |
| `oma status [dir]` | read-only project state: entry points, build/install freshness, launcher entries, tools |
| `oma build [dir]` | bundle `src/index.js` or `.ts` → `ui/index.mjs` + generate `ui/<Name>.qml` |
| `oma package [dir]` | build, then assemble `pkg/` (manifest.json + ui/) and `omarchy plugin validate` |
| `oma validate [dir]` | `omarchy plugin validate` |
| `oma install [dir]` | copy into `~/.config/omarchy/plugins/<id>/`, enable it (rescan + poll + `omarchy plugin enable`), create oma.json launcher entries |
| `oma uninstall [dir]` | remove from the plugins dir, delete oma-managed entries and the persisted `<id>.json` settings file |
| `oma launcher add [name] [--action a] [--exec cmd] [--icon i] [--comment c] [--terminal]` | upsert an entry into oma.json launchers[] (wizard when no name) and write the .desktop file |
| `oma launcher remove [names...] \| --all` | drop entries from oma.json launchers[] (multi-select wizard when unnamed) and delete their oma-managed .desktop files |
| `oma restart [dir]` (`r`) | install + `omarchy restart shell` |
| `oma log [flags]` | show shell-log lines filtered to this plugin (run in the project dir; `-f` follows, `--json` for agents, `--all` for everything, `--level warn` for errors only). Follow mode collapses identical lines arriving within a second into a live `(×N)` count; one-shot output is lossless |
| `oma tail [flags]` | follow the shell log (alias for `oma log -f`) |
| `oma skills list` | list skills: `name<TAB>description` |
| `oma skills get <name> [--full]` | print a skill's SKILL.md (+ references) |
| `oma skills get --all [--full]` | print every skill (SKILL.md + references) |

Every command has a bun-style help page: `oma <command> --help`.

## Validation

- project name: lowercase letters/digits/hyphens, 2-40 chars, no `omarchy`
  prefix (reserved namespace)
- version (`-v`): semver `MAJOR.MINOR.PATCH` (optional pre-release/build suffix)
- author (`-a`): lowercase letters/digits/hyphens, max 30 chars

The wizard shows inline errors (theme red) and re-prompts; flags skip the
wizard for the fields they cover.

## Build layout

```text
project/
└── ui/
    ├── index.mjs         ESM bundle (esbuild embedded in oma, target ES2016)
    └── <Name>.qml        generated bridge QtObject (imports "index.mjs")
                          (<Name>Bridge.qml when <Name>.qml would collide
                          with a surface file)
```

`<Name>` is the manifest name in QML-safe casing (`my-plugin` → `MyPlugin`);
the bridge never overwrites a
surface `.qml` (a project named "panel" gets `PanelBridge.qml`). `oma package`
assembles `pkg/` with `manifest.json` + `ui/` and validates it with
`omarchy plugin validate`.

## Scaffold

`oma create` writes:

- `manifest.json` — `schemaVersion: 1`, dotted `id` (`<author>.<dirname>`; author
  comes from `-a`, else the OS user), `kinds`, camelCase `entryPoints`
  (`ui/<Surface>.qml`). Description defaults to
  "A custom plugin for Omarchy - built with Oma" when `-d` is omitted.
- `src/index.js` — a starter `state({...})` + config + actions, importing from
  `"@oma/runtime"` (resolved to the runtime embedded in the binary at build
  time — no import map). `.ts` entries work equally; esbuild strips annotations.
- `ui/<Surface>.qml` — per-kind skeletons:
  - `panel` — presentation chosen with `--panel-mode attached|window|both`
    (default `attached`):
    - `attached` — bar-attached pair: `ui/BarWidget.qml` (button that toggles
      the panel) + `ui/Panel.qml` (`qs.Ui.Panel` + `KeyboardPanel` anchored to
      the button). Manifest kind becomes `bar-widget`; `Panel.qml` is loaded by
      the widget's Loader, not a separate entry point. The host widget must
      expose `open()` / `close()` / `opened` so shell summon/toggle/hide can
      route to it.
    - `window` — standalone draggable `FloatingWindow` (`ui/Panel.qml`), no bar
      widget; the manifest kind stays `panel`.
    - `both` — the attached pair plus `ui/PanelWindow.qml` (the `panel` entry
      point), kinds `bar-widget` + `panel`.
    The mode is persisted in `oma.json` (`"panel": {"mode": "..."}`) and
    honored by `oma surface add`, which can switch an existing panel between
    modes (`--panel-mode`).
  - `overlay`/`menu` — fullscreen-scrim layer-shell card (clipboard-style).
  - `bar-widget` — standalone glyph button wired to the shared core.
  - Bar-widget buttons use `onPressed` (`WidgetButton` has no clicked signal).

After writing, each file is logged with a checkmark and a total count.

## Bar-widget moduleName

The bar-widget skeleton sets `moduleName: <manifest id>`. That field belongs to
Omarchy's `qs.Ui.BarWidget` base: the shell host uses it to find every live
instance of a widget across monitors (`broadcast()` IPC relay), to look up
per-widget settings from `shell.json`, and to route inline IPC. It must stay
equal to the manifest id so registry, settings keys, and IPC targeting share one
identity; leaving it empty silently breaks those lookups.

## Project configuration (`oma.json`)

Optional opt-in file at the project root. Data only (JSON) — the Go CLI reads
it today and omarchy itself can consume it natively later. `oma create`
scaffolds a stub pointing `$schema` at the schema's versioned raw URL
(`https://raw.githubusercontent.com/ussego/oma/v<tag>/assets/schemas/oma.json` —
immutable tags; dev builds fall back to `main`), so editors autocomplete and
validate every key (`action` is enum-checked, unknown keys rejected by
editors).

```json
{
  "icon": "utilities-system-monitor",
  "launchers": [
    { "name": "Play Next Music", "action": "toggle" },
    { "name": "Music Settings", "comment": "Open settings" }
  ]
}
```

- `icon` — global icon string (freedesktop name or path), default
  `application-x-executable`. Entries may override per-entry.
- `panel` — panel presentation mode: `attached` (bar-anchored popup + widget,
  default), `window` (standalone FloatingWindow, no widget) or `both`. Set by
  `oma create` / `oma surface add --panel-mode`.
- `launchers[]` — opt-in launcher entries. `name` required; `action` is
  `summon | toggle | hide` (default `summon`); `exec` fully overrides the
  command line; remaining fields map 1:1 to Desktop Entry keys with sensible
  defaults (`GenericName` = kind label, `Comment` = manifest description,
  keywords derived from name + id).

Behavior:

- `oma launcher add` upserts the entry into oma.json first (creating the file
  with `$schema` when missing; same-name entries replace in place), then writes
  every declared `.desktop`. Config problems fail loudly before anything is
  written. Without a name it prompts one thing at a time: name (no default),
  then a command — real presets like `omarchy-shell shell toggle <id>` or your
  own via `custom…` — then an optional icon (blank = inherit). Flags mirror
  the JSON keys and skip the prompts entirely.
- `oma launcher remove` takes names, `--all`, or a multi-select wizard; unknown
  names fail atomically. Surviving `.desktop` files are regenerated so removal
  never leaves stale index-shifted files; deletion still only touches files
  carrying `X-Oma-Managed=true`.
- `oma install` writes entries after copying the plugin; config problems
  warn-and-skip (install never fails on a broken optional config).
- Absent file or empty `launchers[]` ⇒ nothing is ever created.
- End users installing via `omarchy plugin add <url> --enable` don't run oma;
  for them the generated `ui/Service.qml` + `ui/LauncherWriter.qml` pair writes
  the entries on first shell load (oma build generates both automatically when
  `launchers[]` is non-empty).

## Hot reload caveat

The shell's per-plugin hot reload re-bundles bar widgets and services, but an
**open panel keeps its mounted QML instance** — after editing surface QML,
close/hide the panel (or restart the shell) to see changes.

## Skills data

Skills are embedded in the `oma` binary and extracted to the user cache dir on
first use; nothing outside the binary is needed. The repo also publishes a
discovery stub under `skills/` for the open agent skills ecosystem
(`npx skills add ussego/oma`) — agents use it to route to the commands above,
and `oma create` prints the install command at the end of scaffolding.

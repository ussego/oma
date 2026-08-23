# AGENTS.md

__NAME__ — __DESC__

An Omarchy shell plugin built with oma (id `__ID__`, runtime __VERSION__). One
project, shared logic, multiple surfaces — see manifest.json for the current
kinds and entry points.

## Ground rules

- All logic lives in src/index.js (plain ESM; JSDoc for types). QML is only
  the native UI layer — never duplicate business logic in a surface.
- One source of truth per state: `state({...})` object literals with fixed
  keys (the build scans them statically — spreads, computed keys and arrays
  fail the bridge scan). `derived(fn, { bridge: "name" })` for bridged
  read-only values.
- Never edit generated artifacts: ui/index.mjs, ui/__BRIDGE__.qml and
  ui/LauncherWriter.qml are build output — change src/ or the surface QML
  and run `oma build` (or `oma dev` to watch). They must also be committed:
  `omarchy plugin add` clones HEAD and validates without building.
- The generated bridge (__BRIDGE__.qml) is the only reactive JS→QML surface:
  surfaces bind to its properties and call its methods; plain JS mutations
  never update QML bindings.
- config() persists automatically to ~/.config/omarchy/__ID__.json through
  the bridge — never hand-roll persistence.

## Runtime contract (QJSEngine, ES2016 target)

- No Object.fromEntries or other modern runtime APIs; dependencies ship
  inside the bundle — keep them minimal.
- State fields are reactive proxies: call snap() before JSON.stringify or
  any IPC return (stringifying a proxy yields {"0": ...}).
- Bridge state fields are undefined until the bridge's first push on load —
  guard list reads in new QML surfaces ((logic.items || [])).
- Launcher-targeted actions must be zero-arg handlers; desktop Exec cannot
  carry empty string arguments.

## Commands

oma build | install | restart | dev | status | ipc | skills — run
`oma skills get oma --full` for the full framework reference.

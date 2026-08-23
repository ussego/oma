---
name: quickshell
description: Use when working with Quickshell 0.3 types in Omarchy plugins — FloatingWindow/PanelWindow surfaces, Quickshell.Io (FileView, Process, IpcHandler), and the QsInterpreter import-resolution constraints (no ../ script imports, singleton quirks).
---

# Quickshell

Omarchy's shell is a single long-running Quickshell 0.3 process (`omarchy-shell`)
that hosts every plugin. Surfaces are Quickshell windows, not separate processes.
Full type docs: https://quickshell.outfoxxed.me/docs/v0.3.0/types

## Window types

- `FloatingWindow` (`import Quickshell`) — a normal floating top-level window. A
  panel/overlay entrypoint wraps one: `visible: false` initially, `open()` sets
  `visible = true`, `close()` sets it back.
- `PanelWindow` — a layer-shell panel surface (pair with `WlrLayershell` /
  `WlrKeyboardFocus` for overlays).

## Interpreter constraints (verified)

- **`../` relative script imports do not resolve** from a `.qml`/`.mjs` import — keep
  the JS bundle in the same directory as the file that imports it.
- **qmldir singletons load but never run `Component.onCompleted`** — don't rely on a
  singleton's completion for wiring; use a plain `QtObject` component instead.
- Window types create their own QML context.

## Quickshell.Io patterns

**FileView** (`import Quickshell.Io`) — read/write small text files:
- Properties: `path`, `preload` (default true), `watchChanges` (default false),
  `atomicWrites` (default true), `printErrors`, `blockLoading`.
- Read via `text()` after the async load completes — handle `onLoaded` /
  `onLoadFailed`; do NOT read text() in `Component.onCompleted`.
- Write via `setText(str)` (emits `saved()` / `saveFailed()`).
- This is exactly what oma-generated bridges use for config persistence.

**Process** — spawn subprocesses: `command: ["bash", "-c", "..."]`, `running: true`,
collect stdout with `StdioCollector { waitForEnd: true }` or parse streaming lines
with `SplitParser { onRead: function(line) {...} }`.

**IpcHandler** — expose plugin methods over IPC:

```qml
IpcHandler {
    target: "my-plugin"
    function open(payload: string): string { ...; return "ok" }
}
```

Call from anywhere: `omarchy-shell <target> <method> [args]` (or `qs ipc call`).
Typed string params only; return values must be strings.

## Panel entrypoint shape

A summoned surface's root is an `Item` with `open(payloadJson)` / `close()` and an
`opened` state; the shell injects `shell`, `manifest`, `service`, and more — see the
omarchy-shell skill for the full injection table.

# stopwatch

Every oma runtime feature in one small project: a bar widget with a live
stopwatch readout plus an attached panel with controls. The UI drives the
tick loop via a QML `Timer` (setInterval is unavailable in QJSEngine module
scope) and the shared JS core holds all logic and state.

Runtime features covered:

- **`state({...})`** — object state, bridged into QML: `timer.running`,
  `timer.seconds`, `timer.tickMs`, `timer.lastEvent` are live properties on
  the generated bridge, so QML bindings react (bar readout, panel texts, the
  Timer's `interval`/`running`).
- **`state(value)`** — primitive state, JS-side only: `tickCount` counts every
  tick and is read through the `getTickCount()` action (primitive state is not
  bridged — see `derived` for the mirror pattern).
- **`derived(...)`** — `formatted` recomputes only when `seconds` changes; its
  value is mirrored into the bridged `timer.display` field so QML can show it.
- **`config({...})`** — `settings` persists to
  `~/.config/omarchy/examples.stopwatch.json` automatically; `start()` re-applies
  the saved `tickMs` and `setTickMs()` writes changes straight back.
- **`emit()` / `on()`** — in-process IPC: "Reset via IPC" emits
  `stopwatch:reset`, the module-level handler resets the timer and records the
  event in `lastEvent`, which the panel displays.
- **actions** — every exported lowercase function becomes a bridge method
  (`toggle`, `reset`, `setTickMs`, `getTitle`, …).

## Run it

```sh
oma build     # bundles src/ into ui/ + generates ui/Stopwatch.qml (the bridge)
oma install   # copies into ~/.config/omarchy/plugins/ and enables
oma restart   # install + restart the shell
```

Click the readout in the bar to start/stop. The panel opens from a launcher
binding or `omarchy-shell shell summon examples.stopwatch`; it holds the
controls, the last event (proving IPC), and the live tick interval.

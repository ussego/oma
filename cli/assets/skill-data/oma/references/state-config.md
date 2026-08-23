# State, derived, and config

## state

```js
import { state } from "@oma/runtime";

export const music = state({
	playing: false,
	song: "",
	volume: 100,
});

export function toggle() {
	music.playing = !music.playing;
}
```

- Object state exposes fields directly (`music.playing`).
- `state(value)` for primitives gives `.value` + `.subscribe`.
- `music.subscribe(fn)` returns an unsubscribe function; it fires on any field
  change (including deep mutations).
- Exports must be object literals with fixed keys — the bridge generator scans
  them statically. Spreads, computed keys, arrays and class instances fail the
  build (arrays/classes can't become QML property sets).

## derived

```js
export const status = derived(() => (music.playing ? "Playing" : "Paused"));
```

- Recomputes only when its dependencies change; read via `status.value`.
- Subscribe to it like `state`.
- Derived values are not bridged into QML — only `state({...})` exports are.

## config

```js
import { config } from "@oma/runtime";

export const settings = config({ volume: 80 });

export function louder() {
	settings.set("volume", settings.get("volume") + 10);
}
```

- `config(defaults)` returns a `{ get, set, subscribe }` store seeded from defaults.
- Validate inside your setters if you need to guard types (plain JS, no schema
  layer); persisted values are trusted on load.

## Persistence (automatic)

The generated bridge (`ui/<Name>.qml`) wires persistence for you:

- On load it reads `~/.config/omarchy/<id>.json` (the same per-plugin settings
  file convention omarchy plugins use) and seeds every config with saved values
  over defaults.
- Every `set()` writes the merged snapshot back through a debounced FileView
  (200ms, atomic writes).
- No boilerplate: just call `config({...})` and `set()`.
- Keys are flat and shared across all configs of a plugin — give them unique names.
- `oma uninstall` removes `<id>.json` along with the plugin.

In QML surfaces, config values reach you through actions/functions or by
mirroring them into a `state({...})` export (only state fields become reactive
QML properties).

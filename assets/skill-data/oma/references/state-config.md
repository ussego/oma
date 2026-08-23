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
  build (arrays/classes can't become QML property sets). Comments inside the
  literal are fine.
- Wholesale replacement (`todo.items = [...]`) is the recommended mutation
  pattern: obvious, cheap, and it is what the bridge snapshot expects.
  Deep mutations (`todo.items.push(...)`) notify too, but leave QML seeing a
  fresh snapshot either way.

## derived

```js
export const status = derived(() => (music.playing ? "Playing" : "Paused"));
```

- Recomputes only when its dependencies change; read via `status.value`.
- Subscribe to it like `state`.
- `derived(fn)` stays JS-only. To surface a derived value in QML as a
  read-only property, opt in explicitly:

```js
export const open = derived(() => music.playing ? 1 : 0, { bridge: "openCount" });
```

QML then binds `logic.openCount` directly (see bridge.md).

## config

```js
import { config } from "@oma/runtime";

export const settings = config({ volume: 80 });

export function louder() {
	settings.set("volume", settings.get("volume") + 10);
}
```

- `config(defaults)` returns a `{ get, set, subscribe, onReady, ready }` store
  seeded from defaults.
- `config("namespace", defaults)` prefixes every key with `namespace.` in the
  shared settings file — use it when a plugin has several configs so keys can
  never collide.
- `config(defaults, { validate(key, value) })` sanitizes persisted input at
  seed time: return the value to store, or undefined/null to keep the default.
  Persisted blobs are otherwise trusted on load — validate anything that comes
  from an older version of the plugin. **`validate` runs on seed only**;
  `set()` writes pass through untouched, so mirror the limits with `coerce`
  (below) or in your own setters.
- `config(defaults, { coerce(key, value) })` sanitizes writes at `set()` time:
  return the value to store, or undefined/null to reject the write (old value
  kept). Pair it with `validate` so the persisted store can never drift
  outside the limits the seed enforces:
  ```js
  export const settings = config({ volume: 80 }, {
    validate(key, value) { return key === "volume" ? Math.min(100, Math.max(0, value)) : value; },
    coerce(key, value)    { return key === "volume" ? Math.min(100, Math.max(0, value)) : value; },
  });
  ```
- `config(defaults, { debounceMs })` overrides the 200ms write debounce (the
  slowest config in a plugin wins).
- `onReady(cb)` fires exactly once, when the settings file has been read and
  stores reflect disk. `ready` is true afterwards. Reads before that see
  defaults.

### Persistence (automatic)

The generated bridge (`ui/<Name>.qml`) wires persistence for you:

- On load it reads `~/.config/omarchy/<id>.json` (the same per-plugin settings
  file convention omarchy plugins use) and seeds every config with saved
  values over defaults.
- Every `set()` writes the merged snapshot back through a debounced FileView
  (200ms by default, atomic writes).
- `oma uninstall` removes `<id>.json` along with the plugin.

The contract is safe by construction:

- **Writes are never silently dropped.** Before the first bind they are
  buffered and replayed after seeding (and win over the disk seed, since they
  are deliberate mutations); while no bridge is alive (panel hidden, registry
  rescan) they buffer until the next bind.
- **Reads before the first bind see defaults.** Gate UI that must reflect
  disk on `config.ready` / `onReady` if the defaults would flash, or mirror
  config into state on `onReady` (the `-t todo` template does this).
- **Surface churn is harmless.** The JS module survives bridge replacement;
  `Component.onDestruction` releases the write channel and the runtime
  re-targets it on the next bind. Only QML-local variables reset per summon.

If a plugin outgrows config() (custom file formats, multiple files, schema
migrations), the fallback is the recipe the templates used before config()
hardened: a surface-owned `FileView` + `loadJson`/`saveJson` actions + a
`fsReady` gate. It survives churn by construction, but it is manual — prefer
config() until there is a concrete reason not to.

## IPC events

`emit(name, payload)` / `on(name, handler)` form an in-process bus (all
surfaces share one shell process). Payload conventions:

- Always pass a plain object: `emit("player:play", { track: 7 })`.
- Event names are `domain:verb` (`player:play`, `todos:changed`); no wildcards
  or surface scoping — a handler receives every emit of its event.
- Handlers run synchronously on emit; keep them cheap or defer work.

In QML surfaces, the equivalent wiring goes through bridge actions: expose
`emitX(...)` functions in `src/index.js` and call them from QML; subscribe in
JS via `on(...)`.

# The JS → QML bridge

The bridge is **code generation**, not a runtime object. `oma build` does two things:

1. Bundles `src/index.js` to `ui/index.mjs` (ESM) with esbuild embedded in the
   oma binary at `target: "es2016"` (relative imports + node_modules; the
   specifier `@oma/runtime` aliases the embedded runtime).
2. Statically scans `src/index.js` and emits `ui/<Name>.qml`.

`<Name>` is the manifest name in QML-safe casing (`my-plugin` → `MyPlugin`).
Bundle, bridge, and surfaces share `ui/`
because Quickshell's script loader cannot resolve `../` imports, and its qmldir
singletons never run `Component.onCompleted`.

## What `<Name>.qml` contains

```qml
import QtQuick
import "index.mjs" as Logic

QtObject {
  id: root
  property var unsubscribers: []

  property bool playing   // one per state({...}) field; auto-NOTIFY
  property string song
  property double volume

  function toggle() { return Logic.toggle.apply(null, arguments) }  // one per exported action

  Component.onCompleted: {
    var apply0 = function() {
      root.playing = Logic.__omaSnap(Logic.music.playing)
      root.song = Logic.__omaSnap(Logic.music.song)
      root.volume = Logic.__omaSnap(Logic.music.volume)
    }
    apply0()
    unsubscribers.push(Logic.music.subscribe(apply0))
  }

  Component.onDestruction: {
    for (var i = 0; i < unsubscribers.length; i++) unsubscribers[i]()
    Logic.__omaUnbindRef(root.omaSink)
  }
}
```

Why each piece exists:

- **`property` per field, not plain JS** — QML bindings only re-evaluate on a QObject
  property change. Mutating a `Proxy`/`var`/`let` leaves bindings stale, so the
  generated `QtObject` is the single reactive surface.
- **`Logic.__omaSnap(...)` around every assignment** — state values are reactive
  proxies; assigning one into a QML property leaks the Proxy across the boundary,
  where QJSEngine can mangle array shapes (`{"0":…}`) and break ListView models.
  `snap` deep-clones to plain arrays/objects first (primitives pass through).
- **`root.<field> = ...`** — explicit `id`-scoped assignment; robust inside the
  callback function.
- **function delegation via `.apply(null, arguments)`** — QML objects have a fixed
  property set, so you can't assign JS functions onto them; forward calls instead.
- **`unsubscribers` + `Component.onDestruction`** — a panel closes by destroying the
  bridge object. Without unsubscribing, the dead `apply` stays in the module's
  listener set and the next `notify()` throws when it calls it (the QObject is
  gone), so a re-summoned instance never receives updates. Always unsubscribe on
  destruction.

## Array fields in QML

Array-valued `state({...})` fields arrive as **plain arrays of plain objects**
(snapshots), so `ListView { model: logic.items }` and binding expressions like
`logic.items.filter(...)` work directly. Every state change replaces the whole
snapshot — bindings re-evaluate, and it is cheap at plugin scale.

If you must cross the boundary yourself (e.g. passing state into a custom
component), use `Logic.snap(...)` / `snap()` from the runtime to get the same
plain projection.

## Bridged derived values

`derived(fn, { bridge: "propName" })` exports land as read-only auto-NOTIFY
properties driven by the bridge (plain `derived(fn)` stays JS-only):

```js
export const open = derived(() => todo.items.filter(t => !t.done).length, { bridge: "openCount" });
```

```qml
Text { text: logic.openCount }  // updates when todo.items changes
```

The property name must not collide with a state field (the build rejects it).

## Persistence (config)

The generated bridge owns the settings file `~/.config/omarchy/<id>.json`
(`FileView`, atomic writes, debounced 200ms — `config({...}, { debounceMs })`
overrides the interval). The lifecycle, in order:

1. The bridge mounts, `FileView` loads the file asynchronously.
2. `__omaLoad` → `Logic.__omaBindRef(saved, root.__omaPersist)` — the runtime
   seeds every `config()` store with saved values over defaults, fires
   `onReady` callbacks, and stores the returned handle in `omaSink`.
3. `omaReady` flips `true`. Before that: reads see defaults, writes are
   buffered (and win over the disk seed — they are deliberate mutations).
   Gate UI on `omaReady` only when reads-before-ready matter.
4. `config().set()` → debounced write through the **newest live bridge**.

Surface churn (panel hide/show, registry rescans) destroys the bridge but the
JS module survives: `Component.onDestruction` unsubscribes, flushes a pending
write, and calls `Logic.__omaUnbindRef(root.omaSink)`; the runtime buffers any
writes until the next bridge binds and re-targets the sink. **Persistence
cannot be lost to churn**; only the QML-local variables of the destroyed
surface reset (see `keepLoaded` in the plugin contract).

## Static scan rules

- `export const name = state({...})` → properties (types inferred from the literal:
  bool / double / string, everything else `var`). Field names must start lowercase.
- Exported lowercase functions (declarations or arrows) → bridge methods.
  Uppercase-initial names are skipped (QML methods must start lowercase).
- `derived(...)` without `{ bridge: "propName" }` is skipped with a note;
  `config(...)` instances are skipped (methods-only).
- Spreads/computed keys inside `state({...})`, array/class states, and duplicate
  fields fail the build with the export name, line number and the offending
  source line. Comments inside `state({...})` literals are fine.

## Action-to-property flow

`Button.onClicked: music.toggle()` → `function toggle()` → `Logic.toggle`
→ JS `music.playing = !music.playing` → proxy `set` trap → `notify()` → `apply`
re-reads the state, snaps it, and assigns the QML properties → the binding
reading `playing` re-evaluates.

## QJSEngine constraints (verified against Quickshell 0.3 / Qt 6.11)

- Plain JS values are not reactive in QML (above).
- The bundle targets ES2016: async arrows, class fields, spread and optional
  chaining are transpiled by esbuild. Runtime APIs are NOT polyfilled — no
  `Object.fromEntries`, no BigInt (defined away as `Number` at bundle time).
- QJSEngine exposes no global object; a module-scoped `globalThis` stand-in is
  injected into every bundle.

## Generated files are disposable

`ui/index.mjs` and `ui/<Name>.qml` are regenerated by `oma build`; never hand-edit
them. Keep the shared logic in `src/index.js` and the UI in the hand-written surface
QML files.

# @oma/runtime

The shared core every [oma](https://github.com/ussego/oma) plugin imports:
reactive state, derived values, persistent config and an in-process IPC event
bus. State declared in JS becomes live QML properties through oma's generated
bridge, so one codebase drives every surface (bar widget, panel, overlay...).

## Install

```sh
deno add jsr:@oma/runtime
pnpm add jsr:@oma/runtime # pnpm 10.9+
npx jsr add @oma/runtime  # npm, bun, older yarn/pnpm
```

> Builds don't need the install: the `oma` CLI aliases this package to the
> runtime embedded in its binary. Installing it gives editors types and lets
> you unit-test plugin logic under node/deno/vitest.

## Usage

```js
import { state, derived, config } from "@oma/runtime";

// Reactive state - object literals are deeply reactive.
export const music = state({ playing: false, song: "", volume: 100 });

// Derived values recompute when their dependencies change.
export const status = derived(() => (music.playing ? "Playing" : "Paused"));

// Config persists across restarts (~/.config/omarchy/<id>.json) when the
// plugin runs inside omarchy-shell; inert in plain node.
export const settings = config({ startOnOpen: false });
```

QML surfaces bind to the same state through the bridge `oma build` generates:

```qml
Text { text: Music.song }
OmaButton { onClicked: Music.toggle() }
```

## API

| Export | Purpose |
|---|---|
| `state(initial)` | reactive value or deep-reactive object with `.subscribe()` |
| `derived(fn)` | computed value, tracked lazily |
| `config(defaults)` | persisted key/value store `{ get, set, subscribe }` |
| `on(event, handler)` | subscribe to an IPC event; returns unsubscribe |
| `emit(event, payload)` | fire an IPC event across surfaces |

Full docs: `oma skills get oma`.

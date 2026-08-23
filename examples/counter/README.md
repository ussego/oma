# counter

The smallest complete oma project: a bar widget plus an attached panel,
sharing one JS core.

- `src/index.js` — reactive `counter` state + actions; `setStep` writes
  through `config()`, which persists to
  `~/.config/omarchy/examples.counter.json` with zero wiring
- `ui/BarWidget.qml` — shows the live count, opens the panel on click, and
  re-applies the saved step once the bridge has seeded config (`omaBound`)
- `ui/Panel.qml` — anchored popup bound to the same state

## Run it

```sh
oma build     # bundles src/ into ui/ and generates ui/Counter.qml (the bridge)
oma install   # copies into ~/.config/omarchy/plugins/ and enables
oma restart   # install + restart the shell
```

Click the count in the bar; +/- and Reset act on the shared state.

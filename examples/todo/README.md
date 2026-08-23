# todo

An overlay-surface example: press your launcher binding (or
`omarchy-shell shell summon examples.todo`) and a centered popup appears.

- `src/index.js` — todos stored one-per-line inside object-literal state
  (the bridge scan requires fixed keys, so lists are newline strings);
  `add` / `complete` / `reopen` / `clearDone` actions splice the lines
- `ui/Overlay.qml` — layer-shell popup with scrim, escape-to-close and the
  shell summon/hide contract

## Run it

```sh
oma build
oma install
oma restart
```

Then summon it: add the plugin's id to a keybind or run
`omarchy-shell shell toggle examples.todo`.

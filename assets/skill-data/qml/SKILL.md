---
name: qml
description: Use when writing or debugging QML for Omarchy shell plugins and Quickshell surfaces — properties, bindings, signals, QtObject, components, singletons, why plain JS mutations do not update bindings, and the qs.Commons theme tokens (Style/Color/Border).
---

# QML

QML is the declarative UI layer. In oma, JavaScript is the logic and QML is the
surface; do not duplicate business logic in QML. (End-user desktop
customization — Hyprland config, keybinds, themes — belongs to the distro's
`omarchy` skill, not this one.)

## Core rules

- `property <type> <name>` auto-generates a change signal (NOTIFY), so bindings that
  read it re-evaluate when it changes.
- **Only QObject property changes are reactive.** Mutating a plain JS object / `var` /
  `let` leaves bindings stale. This is why oma generates a bridge QtObject with one
  property per state field — bind to those, never to raw JS values.
- Bind with `id`-qualified references; avoid unqualified lookups across files.
- `Component.onCompleted` runs for instantiated components but NOT for qmldir
  singletons — see the quickshell skill for the interpreter constraint set.

## Theme tokens

All colors, fonts, spacing and radii come from `qs.Commons`
(`Style`, `Color`, `Border`, `Util`). The complete generated token list —
including nested paths like `Style.font.title`, `Style.bar.iconCanvas`,
`Color.menu.background`, `Border.controlSpec(...)` — lives in
[references/theme-tokens.md](references/theme-tokens.md).

Common ones:

- `Style.space(n)` — scaled spacing unit; multiply, don't invent pixel values.
- `Style.font.family` / `.body` / `.title` / `.heading` / `.display` / `.caption`.
- `Color.foreground` / `Color.background` / `Color.accent` / `Color.urgent`,
  plus per-surface groups (`Color.menu.*`, `Color.notifications.*`, …).

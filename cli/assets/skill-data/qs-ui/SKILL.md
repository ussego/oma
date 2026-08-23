---
name: qs-ui
description: Use when building Omarchy plugin UI — the full qs.Ui component inventory (Button, Panel, BarWidget, WidgetButton, Dropdown, ConfirmDialog, PanelSlider, …) with real property/signal APIs, and why to theme off qs.Commons instead of hardcoding.
---

# qs.Ui + qs.Commons

Omarchy ships a component library and theme inside the shell
(`/usr/share/omarchy/shell/`). Import them directly; never build a parallel
component library.

- `import qs.Ui` — 30+ components: `Button`, `TextField`, `Toggle`, `ToggleSwitch`,
  `ConfirmDialog`, `PopupCard`, `Dropdown`, `SearchableDropdown`, `MultiSelect`,
  `NumberField`, `Panel` + `Panel*` widgets (Slider, SectionHeader, ToolTip, Hero,
  ActionButton, KeyCatcher…), `BarWidget`, `BarIndicator`, `BarIconButton`,
  `WidgetButton`, `OpticalGlyph`, `KeyboardPanel`, `PanelController`.
- `import qs.Commons` — `Style`, `Color`, `Border`, `Util` singletons.

## Full API reference

Every component's root-level properties, signals and functions (extracted from
the installed shell source) are in [references/components.md](references/components.md).
Read it before guessing an API — components take far more props than the common
ones (`Button` alone has `selected`, `active`, `focusable`, `bordered`,
per-state foreground/background/accent…).

## Rules

- Theme everything off `qs.Commons` (`Style.space(n)`, `Style.font.title`,
  `Color.accent`, `Border.*`) — never hardcode colors, typography, radii,
  spacing or shadows. Components adapt to the active Omarchy theme.
- `BarWidget` base injects `bar`, `moduleName`, `settings`; use
  `setting(key, fallback)` for per-widget overrides and `broadcast(method)` to
  hit every instance across monitors. See the omarchy-shell skill for the
  injection contract.
- `WidgetButton` has no `clicked` signal — use `onPressed(function(button))`.

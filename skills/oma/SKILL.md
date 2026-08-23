---
name: oma
description: Discovery skill for oma, the SDK and CLI for building Omarchy shell plugins — plain JavaScript logic (reactive state, derived values, persisted config, in-process IPC) with native QML surfaces, a generated JS→QML bridge, and one CLI for create/build/package/install. Use when scaffolding or working on an oma plugin project, authoring plugin state or actions, writing QML surfaces or bar widgets, understanding the plugin contract (manifest, kinds, entry points, shell IPC), or debugging JS↔QML state sync. Full docs are pulled on demand via the oma CLI.
---

# oma

oma is an SDK and CLI for building Omarchy shell plugins. Plugin logic is plain
JavaScript — reactive state (`state`), derived values, persisted config, and an
in-process IPC bus — and the UI is native QML. One shared core serves every
surface (bar-widget, panel, overlay, menu, service). `oma build` bundles the JS
and generates a bridge that exposes state as live QML properties.

## Start here

This file is a discovery stub for agents that installed oma once with a skills
installer such as `npx skills add ussego/oma` (the `oma create` wizard prints
this command). Before implementing or explaining oma plugin work, load the
current skill content with the installed CLI — it ships inside the binary, so
it is always current and works offline:

```bash
oma skills list                 # every skill, name<TAB>description
oma skills get oma --full       # the SDK itself: runtime, bridge, CLI, manifest
```

Routing — load the skill that matches the task:

- `oma skills get omarchy-shell` — the plugin contract: manifest schema, kinds,
  entry points, what the shell injects, and shell IPC (summon/hide/toggle/call).
- `oma skills get qml` — writing QML surfaces and bindings.
- `oma skills get qs-ui` — the qs.Ui component library and qs.Commons theme.
- `oma skills get quickshell` — window types and Quickshell.Io (FileView,
  Process, IpcHandler).
- `oma skills get qt` — QJSEngine limits: supported ES features, the ES2016
  bundle target, what to keep out of runtime code.
- `oma skills get <name> --full` — body plus the references/*.md deep dives.

If the `oma` CLI is not installed, get it with:

```bash
curl -fsSL https://raw.githubusercontent.com/ussego/oma/main/install.sh | bash
```

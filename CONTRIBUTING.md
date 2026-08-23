# Contributing

Thanks for wanting to help with **oma** — the SDK and CLI for building Omarchy
shell plugins. One project, shared logic, multiple surfaces.

## Orientation

- [README.md](README.md) — what oma is and how to use it
- [AGENTS.md](AGENTS.md) — the design spec. **Read it before changing
  behavior**: architecture, bridge mechanics, CLI expectations, style rules
- [examples/](examples) — complete runnable projects (counter, todo, stopwatch)
- [skills/oma/SKILL.md](skills/oma/SKILL.md) — the agent-facing discovery stub
  (full docs ship inside the binary: `oma skills get <name> [--full]`)

## Development setup

Building oma requires [Go 1.27+](https://go.dev) only — everything else is
vendored into the module.

```sh
# with mise (recommended — see mise.toml)
mise run build        # compile dist/oma, callable as `oma` inside the repo
mise run check        # gofmt + vet + hermetic tests + skills_test.sh
mise run live         # tier-1 offscreen verification against real Quickshell (safe)
mise run live-shell   # tier-2 full round trip — RESTARTS YOUR SHELL
mise run install-dev  # symlink dist/oma to ~/.local/bin/oma-dev

# or plain Go from the repo root
go build -trimpath -ldflags "-s -w -X main.cliVersion=dev" -o dist/oma ./cli
go test ./...
```

## Testing

| Check | Command | When |
|---|---|---|
| Hermetic Go tests | `go test -count=1 ./...` (repo root) | always |
| Skills CLI smoke | `bash skills_test.sh` | always |
| Runtime behavior | `node cli/assets/oma.test.js` | after runtime changes |
| Tier 1 (offscreen) | `mise run live` | **after any runtime/bridge/bundler change** |
| Tier 2 (real shell) | `mise run live-shell` | release gate — restarts your shell, run before tagging |

Hermetic tests cannot catch QJSEngine-only breakage (TDZ ordering, QtObject
child limits, tree-shaking holes) — that is what the live tiers are for.

## Style rules

- **Plain ESM JavaScript** for logic; no TypeScript toolchain. JSDoc where
  types help editors. `src/index.ts` works (annotations stripped by esbuild)
  but keep logic JS-first.
- **QML stays native** — layouts, bindings, animations belong in QML, never
  in JS.
- Prefer `state({...})` and `derived(...)` over wrapper objects or state
  classes. One source of truth per value.
- Small composable APIs (`state`, `derived`, `action`, `config`) over
  frameworks, registries and abstraction layers.
- **Reuse the platform** — `qs.Ui` components and `qs.Commons` theme tokens
  before building anything new; Omarchy/Quickshell/Qt before new
  infrastructure.
- Comments explain **why**, not what.

## Changing the runtime or the bridge

- Runtime: `cli/assets/oma.js` (single source of truth; `jsr/` is publish
  staging that CI copies from it — don't edit `jsr/` directly).
- Bridge template: `cli/bridge.go`; scanner/parser tests in
  `cli/bridge_test.go`.
- After any change: `node cli/assets/oma.test.js`, the hermetic suite, and
  **tier 1** (`mise run live`). If the user-facing behavior or generated
  bridge shape changed, update the skill docs in `cli/assets/skill-data/`.

## Adding a skill

- Full content lives in `cli/assets/skill-data/<name>/` (`SKILL.md` +
  `references/`), embedded in the binary and served by `oma skills get`.
- The npx-facing discovery stub is `skills/oma/SKILL.md` — it routes agents
  to the CLI (`oma skills get <name>`). The parity test
  (`TestSkillStubRoutesToRealSkills`) enforces that every skill-data skill is
  reachable from the stub.
- `skills_test.sh` covers the CLI surface (list/get/--full/--all).

## Adding an example

- Create `examples/<name>/` with `manifest.json`, `src/index.js` and the
  `ui/` surfaces.
- Run `oma build` inside the example and **commit the built artifacts**
  (`ui/index.mjs` + `ui/<Name>.qml`) — `TestExamplesBuild` fails if they go
  stale or if a surface references the wrong bridge name.

## Reporting bugs

Open an issue with:

- `oma --version` and your distro / session (Hyprland, …)
- The plugin id if the bug is plugin-related
- Steps to reproduce and expected vs actual behavior
- Diagnostics: `oma status`, and `oma log --all` / `oma tail --all` output
  (paste text, not screenshots)
- For crashes: `coredumpctl info <pid>` and whether the shell or a plugin
  process died

## Submitting changes

Open a PR against `main`. The checklist:

- [ ] `gofmt -l .` clean
- [ ] `go vet ./...` and `go vet -tags live ./...` pass
- [ ] `go test -count=1 ./...` passes
- [ ] `bash skills_test.sh` passes
- [ ] `node cli/assets/oma.test.js` passes
- [ ] Tier 1 (`mise run live`) run when the runtime, bridge template or
      bundler changed
- [ ] Docs/skills updated when behavior changed
- [ ] Describe what changed and why (AGENTS.md is the spec — call out
      deviations)

## Releasing

1. Commit everything, then `git tag v0.1.0 && git push origin main --tags` —
   the workflow builds both arches, publishes tarballs + checksums and
   publishes `@oma/runtime` to JSR.
2. Before the JSR job: link `@oma/runtime` to the repo in the jsr.io package
   settings (one-time).
3. After the workflow finishes: fill the
   `REPLACE_WITH_sha256sums.txt_VALUE` placeholders in
   `packaging/aur/PKGBUILD` with the release's `sha256sums.txt` values. AUR
   account registration is currently closed; Arch users build from the hosted
   PKGBUILD instead (`curl .../packaging/aur/PKGBUILD && makepkg -si` - see
   the README install section), and the package is submitted to AUR as
   `oma-bin` when registration reopens.
4. Run the tier-2 live gate (`mise run live-shell`) before tagging.

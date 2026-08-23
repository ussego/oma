# oma

SDK and CLI for building [Omarchy](https://omarchy.org) shell plugins - one
project, shared logic, multiple surfaces.

[![Latest release](https://shieldcn.dev/github/release/ussego/oma.svg?variant=secondary)](https://github.com/ussego/oma/releases/latest) [![GitHub stars](https://shieldcn.dev/github/stars/ussego/oma.svg?variant=secondary)](https://github.com/ussego/oma) [![License](https://shieldcn.dev/github/license/ussego/oma.svg?variant=secondary)](https://github.com/ussego/oma/blob/main/LICENSE) [![JSR @oma/runtime](https://shieldcn.dev/jsr/@oma/runtime.svg?variant=secondary)](https://jsr.io/@oma/runtime) [![Go 1.27](https://shieldcn.dev/badge/go-1.27-00ADD8.svg?variant=secondary&logo=go)](https://go.dev) [![Built for Omarchy](https://shieldcn.dev/badge/for-omarchy-9ece6a.svg?variant=secondary)](https://omarchy.org)

Plain JavaScript (TypeScript annotations accepted, stripped at build) is the
logic layer; QML is the UI. Reactive state declared in JS becomes live QML
properties through a generated bridge; plugin config persists automatically to
`~/.config/omarchy/<id>.json`. The CLI is a single static Go binary with esbuild
embedded - no Node, no Deno, no toolchain required on the user's machine.

## Install

**Any Linux distro** - curl installer:

```sh
curl -fsSL https://raw.githubusercontent.com/ussego/oma/main/install.sh | bash
```

**Go**:

```sh
go install github.com/ussego/oma@latest
```

**Arch Linux** - AUR registration is closed, build from the hosted PKGBUILD:

```sh
mkdir -p /tmp/oma-bin && cd /tmp/oma-bin
curl -fsSL -O https://raw.githubusercontent.com/ussego/oma/main/packaging/aur/PKGBUILD
makepkg -si && cd / && rm -rf /tmp/oma-bin
```

## Quickstart

```sh
oma create my-plugin # scaffold, directly: oma create my-plugin -s panel -a yourname
cd my-plugin
oma build        # bundle src/index.js + generate the QML bridge into ui/
oma install      # copy into ~/.config/omarchy/plugins/ and enable
oma restart      # install + restart the shell (alias: oma r)
```

Surfaces: `bar`, `bar-widget`, `menu`, `panel`, `overlay`, `service` - any
combination in one project. `oma status` shows build/install/launcher state at
a glance; `oma skills list` ships agent-facing deep dives.

Complete runnable projects live in [examples/](examples): **counter** (bar
widget + attached panel), **todo** (overlay) and **stopwatch** (bar widget +
attached panel exercising every runtime feature - state, derived, config, IPC
and actions).

## Editor types

Plugin code imports the runtime as `import { state, config } from
"@oma/runtime"` - builds alias it to the runtime embedded in the CLI, so
nothing needs to be installed. For editor intellisense and unit-testing
plugin logic, install the published lib (any package manager):

```sh
deno add jsr:@oma/runtime    # or: pnpm add jsr:@oma/runtime (10.9+)
npx jsr add @oma/runtime     # npm / bun - commit the generated .npmrc
```

## Agent skills

The repo publishes a discovery skill for the open agent skills ecosystem. It
doesn't contain the docs - it routes agents to the installed CLI, which serves
the always-current content (`oma skills list`, `oma skills get <name> [--full]`).

```sh
npx skills add ussego/oma     # project scope - or -g to install once globally
```

`oma create` prints this command at the end of scaffolding, and the skill
itself falls back to the curl installer when `oma` isn't on PATH yet.

## Plugin shape

```js
// src/index.js
import { config, state } from "@oma/runtime";

export const music = state({ playing: false, volume: 100 });

export function toggle() {
	music.playing = !music.playing;
}

// survives restarts via ~/.config/omarchy/<id>.json - no wiring needed
export const settings = config({ startOnOpen: false });
```

```qml
// ui/Panel.qml - bind to the generated bridge
MusicButton { active: logic.playing; onClicked: logic.toggle() }
```

Libraries come from `node_modules` (any package manager); esbuild resolves and
bundles them down to what Quickshell's QJSEngine parses (ES2016).

## Development

Building oma itself requires [Go 1.27+](https://go.dev) only - everything else
(esbuild, TUI deps) is vendored into the module.

```sh
# with mise (recommended - see mise.toml)
mise run build        # compile dist/oma, callable as `oma` inside the repo
mise run check        # gofmt + vet + hermetic tests
mise run live         # tier-1 offscreen verification against real Quickshell (safe)
mise run live-shell   # tier-2 full round trip - RESTARTS YOUR SHELL
mise run install-dev  # symlink dist/oma to ~/.local/bin/oma-dev

# or plain Go from the repo root
go build -trimpath -ldflags "-s -w -X main.cliVersion=dev" -o dist/oma .
go test ./...
```

Run `mise run live` after any change to the runtime, bridge template, or
bundler - hermetic tests can't catch QJSEngine-only breakage. Release builds
inject the version via `-X main.cliVersion=<tag>`; see
`.github/workflows/release.yml`. `go install` and local builds skip the
injection and fall back to the module version embedded by the toolchain
(`oma --version` reports the real tag, or `dev` outside a tagged checkout).

## Contributing

Bug reports, feature requests and pull requests are welcome - see
[CONTRIBUTING.md](CONTRIBUTING.md) for the setup, test conventions and style
rules, and [AGENTS.md](AGENTS.md) for the design spec.

## Support

oma is free and open source. If it saves you time,
[sponsor the project on GitHub](https://github.com/sponsors/ussego) ❤️

## License

MIT - see [LICENSE](LICENSE).

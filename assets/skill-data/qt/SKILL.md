---
name: qt
description: Use when hitting Qt/QJSEngine JavaScript engine limits — supported ES features, the class-field and object-spread restrictions, transpiling to ES2016 with esbuild, and ESM .mjs imports.
---

# Qt / QJSEngine

Quickshell 0.3 runs on Qt 6.11; its QJSEngine (the QsInterpreter) parses bundled
plugin JS. The support matrix below is verified against that runtime.

## Supported

`Proxy`, `WeakMap`, `Set`, `Map`, array spread `[...x]`, `Object.assign`,
`Object.entries`, optional chaining `?.`, nullish `??`, `Array.prototype.includes`,
`for...of`.

## Not supported

- **Object spread `{...x}`** — parse error in raw QJSEngine code (safe in
  bundles: esbuild lowers it).
- **`Object.fromEntries`** — not a function at runtime.
- **ES2022 class fields** (public or `#` private) — parse error. Assign instance
  state in the constructor, never as field initializers.

## Bundling

- oma bundles with esbuild at `target: "es2016"`, which lowers everything in
  the "Not supported" list above to engine-safe output. Runtime APIs are never
  polyfilled — the full constraint set lives in the oma skill's bridge
  reference.

## ESM modules

QJSEngine loads ESM via `import "file.mjs" as X`; named exports are exposed on the
namespace object. Plugin bundles are written as `index.mjs` for this reason.

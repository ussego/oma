// Quickshell's QJSEngine exposes no global object name (no globalThis, self,
// window, or global). Libraries that touch globalThis at module scope (e.g.
// zod's global config registry) would throw on load, so inject a module-scoped
// stand-in. esbuild rewrites every free `globalThis` in the bundle to this
// binding via the `inject` option. A plain object is enough: plugins run in
// one process, and the module scope persists for the bundle's lifetime.
export var globalThis = globalThis || {};

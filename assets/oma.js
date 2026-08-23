/* @ts-self-types="./oma.d.ts" */

/**
 * oma runtime - the shared core every Omarchy plugin imports.
 *
 * One source of truth for state; QML sees it through the generated bridge
 * QtObject (ui/&lt;Name&gt;.qml). Plain ES modules; esbuild lowers syntax to what
 * Quickshell's QJSEngine parses when `oma build` bundles your plugin.
 *
 * State, derived values and IPC events run in any modern JS engine, so plugin
 * logic can be unit-tested outside the shell. {@link config} persists through
 * the QML bridge and is inert without it; persistence survives surface churn
 * (panel hide/show, registry rescans) - writes buffer until a live bridge
 * exists and are never silently dropped. {@link snap} deep-clones state
 * values (unwrapping proxies) for crossing JS/QML boundaries.
 *
 * @example
 * ```js
 * import { state, derived } from "@oma/runtime";
 *
 * export const music = state({ playing: false, song: "", volume: 100 });
 * export const status = derived(() => (music.playing ? "Playing" : "Paused"));
 *
 * music.playing = true; // QML bindings react; status.value becomes "Playing"
 * ```
 *
 * @module
 */

/**
 * @typedef {() => void} Unsubscribe
 */

/**
 * @template T
 * @typedef {object} ValueState
 * @property {T} value
 * @property {(listener: () => void) => Unsubscribe} subscribe
 */

/**
 * @template {object} T
 * @typedef {T & { subscribe(listener: () => void): Unsubscribe }} ObjectState
 */

/** @type {{ run: () => void, deps: Set<object> } | null} */
let active = null;

class Cell {
	/**
	 * @param {unknown} value
	 */
	constructor(value) {
		this._value = value;
		this._listeners = new Set();
	}

	get value() {
		if (active) {
			active.deps.add(this);
			this._listeners.add(active.run);
		}
		return this._value;
	}

	set value(next) {
		if (Object.is(next, this._value)) return;
		this._value = next;
		notifyCell(this);
	}
}

/**
 * Reactive state. Primitives wrap as `{ value, subscribe }`; objects return a
 * deep-reactive proxy of the object itself with an added `subscribe`.
 *
 * @example
 * ```js
 * const count = state(0); // primitive: count.value += 1
 * const music = state({ playing: false }); // object: music.playing = true
 * ```
 *
 * @template T
 * @param {T} initial initial value, or an object literal for deep reactivity
 * @returns {T extends object ? ObjectState<T> : ValueState<T>}
 */
export function state(initial) {
	const cell = new Cell(initial);
	if (typeof initial === "object" && initial !== null && isPlain(initial)) {
		return objectProxy(cell, initial);
	}
	return {
		get value() {
			return cell.value;
		},
		set value(next) {
			cell.value = next;
		},
		subscribe(listener) {
			return subscribeCell(cell, listener);
		},
	};
}

function subscribeCell(cell, listener) {
	cell._listeners.add(listener);
	return () => cell._listeners.delete(listener);
}

function notifyCell(cell) {
	for (const listener of [...cell._listeners]) listener();
}

function isPlain(value) {
	const proto = Object.getPrototypeOf(value);
	return proto === Object.prototype || proto === null || Array.isArray(value);
}

// Map/Set/Date and other class instances aren't deep-wrapped (proxy receivers
// break their internal slots). Wrap them explicitly if a surface ever needs
// reactive Map/Set.
function objectProxy(cell, target) {
	const proxies = new WeakMap();
	const wrap = (value, root) => {
		if (value === null || typeof value !== "object") return value;
		if (!isPlain(value)) return value;
		const existing = proxies.get(value);
		if (existing) return existing;
		const proxy = new Proxy(value, {
			get(t, prop, receiver) {
				if (root && prop === "subscribe") {
					return (listener) => subscribeCell(cell, listener);
				}
				cell.value;
				return wrap(Reflect.get(t, prop, receiver), false);
			},
			set(t, prop, value, receiver) {
				const prev = Reflect.get(t, prop, receiver);
				const ok = Reflect.set(t, prop, value, receiver);
				if (!Object.is(prev, value)) notifyCell(cell);
				return ok;
			},
			deleteProperty(t, prop) {
				const had = Reflect.has(t, prop);
				const ok = Reflect.deleteProperty(t, prop);
				if (ok && had) notifyCell(cell);
				return ok;
			},
		});
		proxies.set(value, proxy);
		return proxy;
	};
	return wrap(target, true);
}

/**
 * Derived reactive value; recomputes lazily-tracked dependencies and only
 * notifies subscribers when the result changes.
 *
 * @example
 * ```js
 * const label = derived(() => (music.playing ? "Playing" : "Paused"));
 * label.value; // "Paused"
 * ```
 *
 * @template T
 * @param {() => T} fn
 * @returns {ValueState<T>}
 */
export function derived(fn) {
	const cell = new Cell(undefined);
	const effect = {
		run: () => {},
		deps: new Set(),
	};
	effect.run = () => {
		for (const dep of effect.deps) dep._listeners.delete(effect.run);
		effect.deps.clear();
		const prev = active;
		active = effect;
		try {
			cell.value = fn();
		} finally {
			active = prev;
		}
	};
	effect.run();
	return {
		get value() {
			return cell.value;
		},
		subscribe(listener) {
			return subscribeCell(cell, listener);
		},
	};
}

// All configs of one plugin share one persistence file, so set() writes a
// merged snapshot across every registered config. Keys are flat unless a
// config opts into a namespace (config("ns", {...})), which prefixes them.
const configs = [];
const readyCallbacks = [];
let seeded = false; // first __omaBind completed: stores now reflect disk
let lastSaved = null; // disk data from the first bind, for late configs
let sink = null; // newest live bridge's write-back channel
const sinks = []; // every live bridge channel, newest last (survives churn)
const pending = new Set(); // prefixed keys written before the first seed
let dirty = false; // store changed while no live sink existed (post-seed)
let maxDebounceMs = 200;

function schedulePersist() {
	if (!sink) {
		dirty = true; // buffered: flushed by __omaBind once a sink exists
		return;
	}
	dirty = false;
	const merged = {};
	for (const c of configs) {
		for (const [key, value] of c.store) merged[c.prefix + key] = value;
	}
	sink(merged);
}

// seedStore merges saved values over a config's defaults, honoring buffered
// pre-bind writes and the config's validate callback.
function seedStore(c, data) {
	for (const key of c.keys) {
		const k = c.prefix + key;
		if (pending.has(k)) continue; // pre-bind writes win over disk
		let raw = data[k];
		if (raw === undefined || raw === null) continue;
		if (c.validate) raw = c.validate(key, raw);
		if (raw !== undefined && raw !== null) c.store.set(key, raw);
	}
}

/**
 * Per-instance plugin config store. Every config joins a registry so the
 * generated bridge's bootstrap ({@link __omaBind}) can seed it from the
 * plugin's `~/.config/omarchy/<id>.json` and write changes back.
 *
 * Persistence is safe across surface churn (panel hide/show, registry
 * rescans): the bridge re-targets its write channel on every bind and the
 * runtime buffers writes while no bridge is alive, so nothing is ever lost.
 * Before the first bind (`config.ready` is false) reads see defaults and
 * writes are buffered, then replayed over the disk seed.
 *
 * @example
 * ```js
 * export const settings = config({ startOnOpen: false });
 *
 * export function toggleStart() {
 *   settings.set("startOnOpen", !settings.get("startOnOpen"));
 * }
 * ```
 *
 * Options:
 * - `validate(key, value)` - sanitize persisted input at seed time; return
 *   the value to store (or undefined/null to keep the default).
 * - `coerce(key, value)` - sanitize writes at set() time; return the value
 *   to store, or undefined/null to reject the write (old value kept). Keeps
 *   the persisted store and everything mirroring it inside the limits
 *   validate enforces on seed.
 * - `debounceMs` - override the 200ms write debounce for this plugin (the
 *   slowest config wins).
 * - Namespace form `config("ns", defaults)` prefixes every key with
 *   `ns.` in the shared settings file, ending cross-config collisions.
 *
 * @template {Record<string, unknown>} T
 * @param {string | T} arg1 namespace string, or the defaults object
 * @param {T | { validate?: (key: string, value: unknown) => unknown, coerce?: (key: string, value: unknown) => unknown, debounceMs?: number }} [arg2] defaults (namespace form) or options
 * @returns {{ get(key: keyof T): any, set(key: keyof T, value: any): void,
 *   subscribe(listener: (key: string, value: unknown) => void): Unsubscribe,
 *   onReady(cb: () => void): Unsubscribe, readonly ready: boolean }}
 */
export function config(arg1, arg2) {
	let prefix = "";
	let defaults = arg1;
	let options = arg2 || {};
	if (typeof arg1 === "string") {
		prefix = arg1 + ".";
		defaults = arg2;
		options = {};
	}
	const store = new Map(Object.entries(defaults));
	const listeners = new Set();
	const validate = typeof options.validate === "function" ? options.validate : null;
	const coerce = typeof options.coerce === "function" ? options.coerce : null;
	if (typeof options.debounceMs === "number" && options.debounceMs > 0) {
		maxDebounceMs = Math.max(maxDebounceMs, options.debounceMs);
	}
	configs.push({ keys: Object.keys(defaults), store, prefix, validate });
	// A config created after the first bind still gets its disk values (the
	// bridge data is remembered so late registration can't miss seeding).
	if (seeded && lastSaved) seedStore(configs[configs.length - 1], lastSaved);
	return {
		get(key) {
			return store.get(key);
		},
		set(key, value) {
			if (coerce) {
				value = coerce(key, value);
				if (value === undefined || value === null) return; // rejected - keep the old value
			}
			store.set(key, value);
			if (!seeded) pending.add(prefix + key);
			schedulePersist();
			for (const listener of [...listeners]) listener(key, value);
		},
		subscribe(listener) {
			listeners.add(listener);
			return () => listeners.delete(listener);
		},
		onReady(cb) {
			if (seeded) {
				cb();
				return () => {};
			}
			readyCallbacks.push(cb);
			return () => {
				const i = readyCallbacks.indexOf(cb);
				if (i >= 0) readyCallbacks.splice(i, 1);
			};
		},
		get ready() {
			return seeded;
		},
	};
}

/**
 * Deep plain clone that unwraps reactive proxies: arrays become arrays,
 * plain objects become plain objects, primitives and class instances pass
 * through untouched. The generated bridge calls this before assigning state
 * into QML properties, so proxied values never leak across the boundary
 * (a Proxy arriving in QML can mangle array shapes and break ListView
 * models). Use it yourself before crossing any other JS/QML boundary.
 *
 * @template T
 * @param {T} value
 * @returns {T}
 */
export function snap(value) {
	if (value === null || typeof value !== "object") return value;
	if (Array.isArray(value)) return value.map(snap);
	if (!isPlain(value)) return value;
	const out = {};
	for (const key of Reflect.ownKeys(value)) out[key] = snap(value[key]);
	return out;
}

/**
 * Called by the generated bridge once settings finished loading from
 * `~/.config/omarchy/<id>.json`: seeds every config with saved values over
 * its defaults and turns on write-through for later set() calls. Callable by
 * any number of bridge instances over the plugin's lifetime - the write
 * channel always re-targets the newest live instance, and the first bind
 * (and only the first) seeds the stores and fires `onReady` callbacks.
 * Returns a handle the caller passes to {@link __omaUnbind} when it dies.
 *
 * @internal bridge bootstrap - never call from plugin code
 *
 * @param {Record<string, unknown> | null} saved previously persisted values
 * @param {(data: Record<string, unknown>) => void} fn write-back channel
 * @returns {(data: Record<string, unknown>) => void} handle for {@link __omaUnbind}
 */
export function __omaBind(saved, fn) {
	sinks.push(fn);
	sink = fn; // newest live bridge wins as the write channel
	if (!seeded) {
		seeded = true;
		const data = saved || {};
		lastSaved = data;
		for (const c of configs) seedStore(c, data);
		for (const cb of [...readyCallbacks]) cb();
	}
	if (pending.size || dirty) {
		schedulePersist();
		pending.clear();
		dirty = false;
	}
	return fn;
}

/**
 * Releases a bridge's write channel. Called by the generated bridge's
 * Component.onDestruction so a dead instance can never receive writes;
 * pending writes buffer until the next bind. When other bridge instances are
 * still alive the channel falls back to the most recent one.
 *
 * @internal bridge bootstrap - never call from plugin code
 *
 * @param {(data: Record<string, unknown>) => void} handle from {@link __omaBind}
 * @returns {void}
 */
export function __omaUnbind(handle) {
	const i = sinks.indexOf(handle);
	if (i >= 0) sinks.splice(i, 1);
	if (sink === handle) sink = sinks.length ? sinks[sinks.length - 1] : null;
}

/**
 * Write debounce interval for the generated bridge's save Timer. Defaults to
 * 200ms; config({ debounceMs }) overrides it (the slowest config wins).
 *
 * @internal bridge bootstrap - never call from plugin code
 * @returns {number}
 */
export function __omaDebounceMs() {
	return maxDebounceMs;
}

// --- ipc: in-process event bus ---

// Surfaces share one shell process, so an in-process bus covers them all.
// Wire a cross-process transport (Quickshell.Io or a socket) if an external
// service ever needs it.

const handlers = new Map();

/**
 * Subscribe to an IPC event emitted anywhere in the plugin (surfaces share
 * one shell process).
 *
 * @example
 * ```js
 * const off = on("player:play", () => music.playing = true);
 * // ...later: off() to unsubscribe
 * ```
 *
 * @param {string} event
 * @param {(payload?: unknown) => void} handler
 * @returns {Unsubscribe}
 */
export function on(event, handler) {
	let set = handlers.get(event);
	if (!set) {
		set = new Set();
		handlers.set(event, set);
	}
	set.add(handler);
	return () => set.delete(handler);
}

/**
 * Emit an IPC event to every {@link on} subscriber of `event`.
 *
 * @example
 * ```js
 * emit("player:play", { track: 7 });
 * ```
 *
 * @param {string} event
 * @param {unknown} [payload]
 * @returns {void}
 */
export function emit(event, payload) {
	const set = handlers.get(event);
	if (!set) return;
	for (const handler of [...set]) handler(payload);
}

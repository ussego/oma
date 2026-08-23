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
 * the QML bridge and is inert without it.
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
// merged snapshot across every registered config. Keys are flat: give them
// unique names within a plugin (a shared key would overwrite between configs).
const configs = [];
let persistFn = null;
let bound = false;

function schedulePersist() {
	if (!persistFn) return;
	const merged = {};
	for (const c of configs) {
		for (const [key, value] of c.store) merged[key] = value;
	}
	persistFn(merged);
}

/**
 * Per-instance plugin config store. Every config joins a registry so the
 * generated bridge's bootstrap ({@link __omaBind}) can seed it from the
 * plugin's `~/.config/omarchy/<id>.json` and write changes back.
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
 * @template {Record<string, unknown>} T
 * @param {T} defaults keys must be unique within a plugin (configs share one file)
 * @returns {{ get(key: keyof T): any, set(key: keyof T, value: any): void,
 *   subscribe(listener: (key: string, value: unknown) => void): Unsubscribe }}
 */
export function config(defaults) {
	const store = new Map(Object.entries(defaults));
	const listeners = new Set();
	configs.push({ keys: Object.keys(defaults), store });
	return {
		get(key) {
			return store.get(key);
		},
		set(key, value) {
			store.set(key, value);
			schedulePersist();
			for (const listener of [...listeners]) listener(key, value);
		},
		subscribe(listener) {
			listeners.add(listener);
			return () => listeners.delete(listener);
		},
	};
}



/**
 * Called by the generated bridge once settings finished loading from
 * `~/.config/omarchy/<id>.json`: seeds every config with saved values over
 * its defaults and turns on write-through for later set() calls. Idempotent:
 * several surfaces may instantiate the bridge, but the JS module is shared,
 * so only the first bind wins.
 *
 * @internal bridge bootstrap - never call from plugin code
 *
 * @param {Record<string, unknown> | null} saved previously persisted values
 * @param {(data: Record<string, unknown>) => void} fn write-back channel
 * @returns {void}
 */
export function __omaBind(saved, fn) {
	if (bound) return;
	bound = true;
	persistFn = fn;
	const data = saved || {};
	for (const c of configs) {
		for (const key of c.keys) {
			if (data[key] !== undefined && data[key] !== null) c.store.set(key, data[key]);
		}
	}
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

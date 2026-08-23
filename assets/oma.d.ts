/**
 * Type declarations for @oma/runtime — the shared core every Omarchy plugin
 * imports. Keep in sync with cli/assets/oma.js (single implementation source).
 */

/** Function returned by subscribe/on calls; removes the listener. */
export type Unsubscribe = () => void;

/** Reactive primitive: read/write `.value`, react via `.subscribe()`. */
export interface ValueState<T> {
	value: T;
	subscribe(listener: () => void): Unsubscribe;
}

/** Deep-reactive object state: every field is proxied, plus `.subscribe()`. */
export interface ObjectState<T extends object> {
	subscribe(listener: () => void): Unsubscribe;
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
 */
export function state<T>(initial: T): T extends object ? ObjectState<T> : ValueState<T>;

/**
 * Derived reactive value; recomputes lazily-tracked dependencies and only
 * notifies subscribers when the result changes.
 *
 * @example
 * ```js
 * const label = derived(() => (music.playing ? "Playing" : "Paused"));
 * ```
 */
export function derived<T>(fn: () => T): ValueState<T>;

/** Persisted key/value store backed by ~/.config/omarchy/<id>.json. */
export interface ConfigStore<T extends Record<string, unknown>> {
	get(key: keyof T): any;
	set(key: keyof T, value: any): void;
	subscribe(listener: (key: string, value: unknown) => void): Unsubscribe;
}

/**
 * Per-instance plugin config store. Persists across restarts when the plugin
 * runs inside omarchy-shell; inert in plain node.
 *
 * @example
 * ```js
 * export const settings = config({ startOnOpen: false });
 * ```
 */
export function config<T extends Record<string, unknown>>(defaults: T): ConfigStore<T>;

/**
 * Subscribe to an IPC event emitted anywhere in the plugin (surfaces share
 * one shell process).
 */
export function on(event: string, handler: (payload?: unknown) => void): Unsubscribe;

/** Emit an IPC event to every `on` subscriber of `event`. */
export function emit(event: string, payload?: unknown): void;

/**
 * Called by the generated bridge once settings finished loading.
 * @internal bridge bootstrap - never call from plugin code
 */
export function __omaBind(
	saved: Record<string, unknown> | null,
	fn: (data: Record<string, unknown>) => void,
): void;

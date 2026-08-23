import { state, config } from "@oma/runtime";

// A counter that survives restarts: the value is mirrored into config() so
// it persists to ~/.config/omarchy/<id>.json.
export const counter = state({ step: 0 });
export const settings = config("counter", { step: 0 });

// Hydrate the reactive state once config() has seeded from disk; writes made
// before that are buffered and replayed, so this never clobbers saved data.
settings.onReady(() => {
	counter.step = settings.get("step");
});

export function inc() {
	counter.step += 1;
	settings.set("step", counter.step);
}

export function dec() {
	counter.step -= 1;
	settings.set("step", counter.step);
}

export function reset() {
	counter.step = 0;
	settings.set("step", 0);
}

// Scriptable check: omarchy-shell shell call <id> snapshot
export function snapshot() {
	return String(counter.step);
}

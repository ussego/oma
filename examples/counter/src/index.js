import { config, state } from "@oma/runtime";

// Every field becomes a live property on the generated bridge (ui/Counter.qml
// after "oma build"), so QML can bind to it and call these actions directly.
export const counter = state({
	count: 0,
	step: 1,
});

// Config survives restarts (~/.config/omarchy/examples.counter.json). Config
// stores are not bridged into QML - read them from actions like applySavedStep,
// which mirrors persisted values back into reactive state.
export const settings = config({ step: 1 });

export function applySavedStep() {
	const saved = Number(settings.get("step"));
	if (saved > 0) counter.step = saved;
}

export function inc() {
	counter.count += counter.step;
}

export function dec() {
	counter.count -= counter.step;
}

export function reset() {
	counter.count = 0;
}

export function setStep(v) {
	const n = Math.floor(Number(v));
	if (!(n > 0)) return;
	counter.step = n;
	settings.set("step", n); // written straight back to the settings file
}

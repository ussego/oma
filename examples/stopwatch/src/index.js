import { config, derived, emit, on, state } from "@oma/runtime";

// --- object state: every field becomes a reactive QML property on the
// generated bridge (ui/Stopwatch.qml after "oma build") ----------------------
export const timer = state({
	running: false,
	seconds: 0,
	display: "0:00",
	tickMs: 1000,
	lastEvent: "ready",
});

// --- primitive state: JS-side only, not bridged into QML --------------------
// Counted on every tick; QML reads it through the getTickCount() action.
const tickCount = state(0);

// --- derived: recomputes only when its dependencies change -------------------
// Derived values are JS-side too; mirror the value into a bridged field
// (timer.display) whenever the input changes, so QML stays in sync.
const formatted = derived(() => {
	const s = timer.seconds;
	return Math.floor(s / 60) + ":" + ("0" + (s % 60)).slice(-2);
});

// --- config: persists to ~/.config/omarchy/examples.stopwatch.json -----------
// Config stores are not bridged; reach them through actions.
export const settings = config({ title: "Stopwatch", tickMs: 1000 });

// --- actions: exported lowercase functions become bridge methods -------------

// Called by a QML Timer in the bar widget. The UI drives the loop because
// setInterval does not exist in QJSEngine module scope.
export function tick() {
	tickCount.value++;
	timer.seconds++;
	timer.display = formatted.value;
}

export function start() {
	if (timer.running) return;
	timer.tickMs = Number(settings.get("tickMs")) || 1000; // apply persisted preference
	timer.running = true;
	timer.lastEvent = "started";
}

export function stop() {
	if (!timer.running) return;
	timer.running = false;
	timer.lastEvent = "stopped";
}

export function toggle() {
	timer.running ? stop() : start();
}

export function reset() {
	timer.seconds = 0;
	timer.display = formatted.value;
	timer.lastEvent = "reset";
}

// --- IPC: emit()/on() is an in-process event bus shared by every surface -----
export function resetViaIpc() {
	emit("stopwatch:reset");
}

on("stopwatch:reset", () => {
	tickCount.value++;
	timer.lastEvent = "ipc: reset via emit() (#" + tickCount.value + ")";
	timer.seconds = 0;
	timer.display = formatted.value;
});

export function setTickMs(ms) {
	const v = Math.floor(Number(ms));
	if (v < 200) return;
	timer.tickMs = v;
	settings.set("tickMs", v); // written straight back to the settings file
	timer.lastEvent = "tick " + v + "ms";
}

export function getTitle() {
	return settings.get("title");
}

export function getTickCount() {
	return tickCount.value;
}

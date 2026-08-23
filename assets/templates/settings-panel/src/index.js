import { state, config } from "@oma/runtime";

// A settings panel: config values are mirrored into reactive state so QML
// bindings can read them directly (config() itself is methods-only on the
// bridge). Everything persists to ~/.config/omarchy/<id>.json.
export const ui = state({ enabled: false, volume: 80 });
export const settings = config("settings", { enabled: false, volume: 80 });

settings.onReady(() => {
	ui.enabled = settings.get("enabled") === true;
	ui.volume = Number(settings.get("volume")) || 80;
});

export function setEnabled(v) {
	ui.enabled = v === true;
	settings.set("enabled", ui.enabled);
}

export function setVolume(v) {
	ui.volume = Number(v);
	settings.set("volume", ui.volume);
}

// Scriptable check: omarchy-shell shell call <id> snapshot
export function snapshot() {
	return JSON.stringify({ enabled: ui.enabled, volume: ui.volume });
}

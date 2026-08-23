import { state } from "@oma/runtime";

// One todo per line. The bridge scan requires object-literal state with fixed
// keys (arrays are not allowed), so lists are stored as newline-separated
// strings - simple to split in QML and to splice in actions.
export const todos = state({
	open: "",
	done: "",
});

function lines(s) {
	return s ? s.split("\n") : [];
}

export function add(text) {
	const t = text.trim();
	if (!t) return;
	todos.open = todos.open ? todos.open + "\n" + t : t;
}

export function complete(text) {
	const rest = lines(todos.open).filter(function(t) { return t !== text });
	todos.open = rest.join("\n");
	todos.done = todos.done ? todos.done + "\n" + text : text;
}

export function reopen(text) {
	const rest = lines(todos.done).filter(function(t) { return t !== text });
	todos.done = rest.join("\n");
	todos.open = todos.open ? todos.open + "\n" + text : text;
}

export function clearDone() {
	todos.done = "";
}

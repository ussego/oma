import { state, config } from "@oma/runtime";

// One source of truth: the todo list lives here, QML sees it through the
// generated bridge (array fields arrive as plain snapshots - safe for
// ListView models), and config() persists it to ~/.config/omarchy/<id>.json.
export const todo = state({ items: [] });
export const settings = config("todo", { items: [] });

// Hydrate reactive state once the settings file has been read. Writes made
// before that are buffered by config() and replayed after seeding, so an
// early call can never clobber saved todos.
settings.onReady(() => {
	todo.items = Array.isArray(settings.get("items")) ? settings.get("items") : [];
});

let seq = 0;

function commit(items) {
	todo.items = items;
	settings.set("items", items);
}

export function addTodo(text) {
	const trimmed = String(text || "").trim();
	if (!trimmed) return;
	const item = { id: String(Date.now()) + "-" + ++seq, text: trimmed, done: false };
	commit([item].concat(todo.items));
}

export function toggleDone(id) {
	commit(
		todo.items.map(function (t) {
			return t.id === id ? { id: t.id, text: t.text, done: !t.done } : t;
		})
	);
}

export function removeTodo(id) {
	commit(
		todo.items.filter(function (t) {
			return t.id !== id;
		})
	);
}

export function clearDone() {
	commit(
		todo.items.filter(function (t) {
			return !t.done;
		})
	);
}

// Scriptable check: omarchy-shell shell call <id> snapshot
export function snapshot() {
	return JSON.stringify(todo.items);
}

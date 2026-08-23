// Runtime unit tests: plain ESM, run with `node --test jsr/` (or deno test).
// No shell, no QML - the persistence contract (bind/unbind/seeding/buffering)
// is pure JS and must hold before the live tiers run. Each case imports a
// fresh module instance (cache-busted URL) so module-level state never leaks
// between tests.
import { test } from "node:test";
import assert from "node:assert/strict";

let seq = 0;
async function freshRuntime() {
	return import(`../assets/oma.js?case=${++seq}`);
}

test("config seeds from disk on the first bind", async () => {
	const { config, __omaBind } = await freshRuntime();
	const s = config({ volume: 80 });
	__omaBind({ volume: 90 }, () => {});
	assert.equal(s.get("volume"), 90);
	assert.equal(s.ready, true);
});

test("second bind does not re-seed (seed once)", async () => {
	const { config, __omaBind } = await freshRuntime();
	const s = config({ volume: 80 });
	__omaBind({ volume: 90 }, () => {});
	__omaBind({ volume: 95 }, () => {});
	assert.equal(s.get("volume"), 90);
});

test("writes before the first bind are buffered, flushed, and win over disk", async () => {
	const { config, __omaBind } = await freshRuntime();
	const s = config({ items: [] });
	s.set("items", ["early"]);
	let last = null;
	__omaBind({ items: ["disk"] }, (d) => {
		last = d;
	});
	assert.equal(s.get("items")[0], "early"); // pre-bind write won the seed
	assert.deepEqual(last.items, ["early"]); // flush reached the sink
});

test("writes while no bridge is alive buffer and flush on the next bind", async () => {
	const { config, __omaBind, __omaUnbind } = await freshRuntime();
	const s = config({ v: 1 });
	const writes = [];
	const a = __omaBind({}, (d) => writes.push(["A", d.v]));
	s.set("v", 2); // via A
	__omaUnbind(a); // A dies
	s.set("v", 3); // buffered - no live sink
	assert.deepEqual(writes, [["A", 2]]);
	const b = __omaBind({}, (d) => writes.push(["B", d.v]));
	assert.deepEqual(writes, [["A", 2], ["B", 3]]); // flush reached B
	assert.equal(s.get("v"), 3);
});

test("unbind of a superseded sink does not kill a newer one", async () => {
	const { config, __omaBind, __omaUnbind } = await freshRuntime();
	const s = config({ v: 1 });
	const writes = [];
	const a = __omaBind({}, (d) => writes.push("A" + d.v));
	const b = __omaBind({}, (d) => writes.push("B" + d.v));
	__omaUnbind(a); // A dies while B lives
	s.set("v", 9);
	assert.deepEqual(writes, ["B9"]);
});

test("onReady fires once after the first bind; immediate when already ready", async () => {
	const { config, __omaBind } = await freshRuntime();
	const s = config({ v: 1 });
	let fired = 0;
	s.onReady(() => fired++);
	__omaBind({}, () => {});
	__omaBind({}, () => {});
	s.onReady(() => fired++); // already ready -> fires synchronously
	assert.equal(fired, 2);
	assert.equal(s.ready, true);
});

test("validate sanitizes persisted input at seed", async () => {
	const { config, __omaBind } = await freshRuntime();
	const s = config({ n: 1 }, { validate: (k, v) => (typeof v === "number" ? v : 42) });
	__omaBind({ n: "junk" }, () => {});
	assert.equal(s.get("n"), 42);
});

test("namespaced config reads and writes prefixed keys", async () => {
	const { config, __omaBind } = await freshRuntime();
	const ui = config("ui", { shown: false });
	let last = null;
	__omaBind({ "ui.shown": true }, (d) => {
		last = d;
	});
	assert.equal(ui.get("shown"), true);
	ui.set("shown", false);
	assert.deepEqual(last, { "ui.shown": false });
});

test("late config created after the first bind still seeds from disk", async () => {
	const { config, __omaBind } = await freshRuntime();
	__omaBind({ m: 7 }, () => {});
	const late = config({ m: 1, k: 3 });
	assert.equal(late.get("m"), 7);
	assert.equal(late.get("k"), 3);
});

test("debounceMs bumps the bridge interval (slowest config wins)", async () => {
	const { config, __omaDebounceMs } = await freshRuntime();
	assert.equal(__omaDebounceMs(), 200);
	config({ a: 1 }, { debounceMs: 700 });
	config({ b: 2 }, { debounceMs: 350 });
	assert.equal(__omaDebounceMs(), 700);
});

test("snap deep-clones proxied values into plain structures", async () => {
	const { state, snap } = await freshRuntime();
	const todo = state({ items: [{ id: 1, done: false }], count: 3 });
	const out = snap(todo.items);
	assert.ok(Array.isArray(out));
	assert.equal(out[0].constructor, Object);
	assert.equal(snap(todo.count), 3);
	assert.equal(snap(null), null);
	assert.equal(snap("x"), "x");
});

test("coerce sanitizes set() writes; undefined/null rejects", async () => {
	const { config } = await freshRuntime();
	const s = config({ volume: 80 }, { coerce: (k, v) => (k === "volume" ? Math.max(0, Math.min(100, v)) : v) });
	s.set("volume", 150);
	assert.equal(s.get("volume"), 100);
	s.set("volume", -5);
	assert.equal(s.get("volume"), 0);
	const r = config({ mode: "auto" }, {
		coerce: (k, v) => (k === "mode" && ["auto", "dark", "light"].includes(v) ? v : null),
	});
	r.set("mode", "neon");
	assert.equal(r.get("mode"), "auto"); // rejected - old value kept
	r.set("mode", "dark");
	assert.equal(r.get("mode"), "dark");
});

test("validate stays seed-only; coerce is the set-time hook", async () => {
	const { config, __omaBind } = await freshRuntime();
	const s = config({ level: 5 }, { validate: (k, v) => (k === "level" ? Math.min(10, v) : v) });
	__omaBind({ level: 99 }, () => {});
	assert.equal(s.get("level"), 10); // seed sanitized
	s.set("level", 99);
	assert.equal(s.get("level"), 99); // set() untouched by validate
});

test("object state deep mutations notify; subscribe only on the root", async () => {
	const { state } = await freshRuntime();
	const s = state({ items: [] });
	let n = 0;
	s.subscribe(() => n++);
	s.items.push({ x: 1 }); // deep mutation through the proxy
	assert.equal(n, 1);
	assert.equal(typeof s.subscribe, "function");
	assert.equal(s.items.subscribe, undefined); // nested objects don't expose it
});

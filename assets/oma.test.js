// Runtime behavior checks. Run with: node cli/assets/oma.test.js  (or bun)
import assert from "node:assert";
import { state, derived, config, on, emit, __omaBind } from "./oma.js";

// primitive state
const n = state(1);
assert.equal(n.value, 1);
let hits = 0;
const unsub = n.subscribe(() => hits++);
n.value = 2;
n.value = 2; // no change, no notify
assert.equal(hits, 1);
assert.equal(n.value, 2);
unsub();
n.value = 3;
assert.equal(hits, 1);

// object state: field mutation notifies
const music = state({ playing: false, song: "", meta: { volume: 10 } });
let seen = 0;
music.subscribe(() => seen++);
music.playing = true;
assert.equal(seen, 1);
music.song = "x";
assert.equal(seen, 2);
music.meta.volume = 42; // nested mutation bubbles to root subscribers
assert.equal(seen, 3);
assert.equal(music.meta.volume, 42);

// subscribe exists on the object itself
assert.equal(typeof music.subscribe, "function");

// equality guard
const arr = [1, 2];
music.list = arr;
const afterFirstSet = seen;
music.list = arr; // same ref, no notify
assert.equal(seen, afterFirstSet);

// property deletion notifies (any change, including deep mutations)
let delHits = 0;
music.subscribe(() => delHits++);
delete music.song;
assert.equal(delHits, 1);
assert.equal(music.song, undefined);
delete music.nope; // absent key: no notify
assert.equal(delHits, 1);

// array state is deep-reactive
const list = state([1, 2]);
let listHits = 0;
list.subscribe(() => listHits++);
list.push(3);
assert.equal(list.length, 3);
assert.ok(listHits >= 1);
list[0] = 9;
assert.equal(list[0], 9);

// non-plain objects (class instances) wrap as { value, subscribe }
const d = state(new Date(0));
assert.ok(d.value instanceof Date);
assert.equal(typeof d.subscribe, "function");

// derived with no dependencies never recomputes or notifies
const constant = derived(() => 42);
let constHits = 0;
constant.subscribe(() => constHits++);
assert.equal(constant.value, 42);
assert.equal(constHits, 0);

// a throwing derived fn propagates at construction
assert.throws(() => derived(() => { throw new Error("boom"); }));

// nested objects carry no subscribe (root-only)
assert.equal(typeof music.meta.subscribe, "undefined");

// derived tracks deps and skips equal results
const doubled = derived(() => music.meta.volume * 2);
assert.equal(doubled.value, 84);
let dHits = 0;
doubled.subscribe(() => dHits++);
music.meta.volume = 50;
assert.equal(doubled.value, 100);
assert.equal(dHits, 1);
music.playing = !music.playing; // unrelated write must not recompute
assert.equal(dHits, 1);

// config get/set/subscribe
const cfg = config({ volume: 80 });
assert.equal(cfg.get("volume"), 80);
let cfgKey = "";
cfg.subscribe((key) => {
	cfgKey = key;
});
cfg.set("volume", 50);
assert.equal(cfg.get("volume"), 50);
assert.equal(cfgKey, "volume");

// __omaBind seeds saved values over defaults; a write made before the first
// bind is a deliberate mutation: it wins over the disk seed and flushes.
let persisted = null;
__omaBind({ volume: 99 }, (data) => {
	persisted = data;
});
assert.equal(cfg.get("volume"), 50); // pre-bind write won over disk 99
assert.equal(persisted.volume, 50); // buffered write flushed to the sink
cfg.set("volume", 55);
assert.deepEqual(persisted, { volume: 55 });
// a later bind re-targets the sink (newest bridge wins) but does not re-seed
__omaBind({ volume: 1 }, (data) => {
	persisted = data;
});
assert.equal(cfg.get("volume"), 55);
// multiple configs persist as one merged file (namespaces end collisions)
const cfg2 = config({ label: "x", quality: "best" }); // joins registry after bind
cfg.set("volume", 60);
assert.deepEqual(persisted, { volume: 60, label: "x", quality: "best" });

// ipc
const got = [];
const off = on("player:play", (p) => got.push(p));
emit("player:play", 7);
off();
emit("player:play", 8);
assert.deepEqual(got, [7]);

// config unsubscribe removes the listener (registered last: configs share
// one persistence registry, so this must not disturb the merged-snapshot
// assertions above)
const c = config({ v: 1 });
let cHits = 0;
const unsubC = c.subscribe(() => cHits++);
unsubC();
c.set("v", 2);
assert.equal(cHits, 0);

// config coerce sanitizes set() writes; undefined/null rejects (old kept)
const bounded = config({ volume: 80 }, {
	coerce(key, value) {
		if (key === "volume") return Math.max(0, Math.min(100, value));
		return value;
	},
});
bounded.set("volume", 150);
assert.equal(bounded.get("volume"), 100);
bounded.set("volume", -5);
assert.equal(bounded.get("volume"), 0);
const rejecting = config({ mode: "auto" }, {
	coerce(key, value) {
		if (key === "mode" && ["auto", "dark", "light"].indexOf(value) === -1) return null;
		return value;
	},
});
rejecting.set("mode", "neon");
assert.equal(rejecting.get("mode"), "auto"); // rejected write kept the old value
rejecting.set("mode", "dark");
assert.equal(rejecting.get("mode"), "dark");
// validate stays seed-only - coerce is the set-time hook
const guarded = config({ level: 5 }, {
	validate(key, value) { return key === "level" ? Math.min(10, value) : value; },
});
guarded.set("level", 99);
assert.equal(guarded.get("level"), 99); // set() does not run validate

console.log("runtime OK");

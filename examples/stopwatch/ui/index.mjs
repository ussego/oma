// ../../../../.cache/oma/oma.js
var active = null;
var Cell = class {
  /**
   * @param {unknown} value
   */
  constructor(value) {
    this._value = value;
    this._listeners = /* @__PURE__ */ new Set();
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
};
function state(initial) {
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
    }
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
function objectProxy(cell, target) {
  const proxies = /* @__PURE__ */ new WeakMap();
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
      set(t, prop, value2, receiver) {
        const prev = Reflect.get(t, prop, receiver);
        const ok = Reflect.set(t, prop, value2, receiver);
        if (!Object.is(prev, value2)) notifyCell(cell);
        return ok;
      },
      deleteProperty(t, prop) {
        const had = Reflect.has(t, prop);
        const ok = Reflect.deleteProperty(t, prop);
        if (ok && had) notifyCell(cell);
        return ok;
      }
    });
    proxies.set(value, proxy);
    return proxy;
  };
  return wrap(target, true);
}
function derived(fn) {
  const cell = new Cell(void 0);
  const effect = {
    run: () => {
    },
    deps: /* @__PURE__ */ new Set()
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
    }
  };
}
var configs = [];
var readyCallbacks = [];
var seeded = false;
var lastSaved = null;
var sink = null;
var sinks = [];
var pending = /* @__PURE__ */ new Set();
var dirty = false;
var maxDebounceMs = 200;
function schedulePersist() {
  if (!sink) {
    dirty = true;
    return;
  }
  dirty = false;
  const merged = {};
  for (const c of configs) {
    for (const [key, value] of c.store) merged[c.prefix + key] = value;
  }
  sink(merged);
}
function seedStore(c, data) {
  for (const key of c.keys) {
    const k = c.prefix + key;
    if (pending.has(k)) continue;
    let raw = data[k];
    if (raw === void 0 || raw === null) continue;
    if (c.validate) raw = c.validate(key, raw);
    if (raw !== void 0 && raw !== null) c.store.set(key, raw);
  }
}
function config(arg1, arg2) {
  let prefix = "";
  let defaults = arg1;
  let options = arg2 || {};
  if (typeof arg1 === "string") {
    prefix = arg1 + ".";
    defaults = arg2;
    options = {};
  }
  const store = new Map(Object.entries(defaults));
  const listeners = /* @__PURE__ */ new Set();
  const validate = typeof options.validate === "function" ? options.validate : null;
  const coerce = typeof options.coerce === "function" ? options.coerce : null;
  if (typeof options.debounceMs === "number" && options.debounceMs > 0) {
    maxDebounceMs = Math.max(maxDebounceMs, options.debounceMs);
  }
  configs.push({ keys: Object.keys(defaults), store, prefix, validate });
  if (seeded && lastSaved) seedStore(configs[configs.length - 1], lastSaved);
  return {
    get(key) {
      return store.get(key);
    },
    set(key, value) {
      if (coerce) {
        value = coerce(key, value);
        if (value === void 0 || value === null) return;
      }
      store.set(key, value);
      if (!seeded) pending.add(prefix + key);
      schedulePersist();
      for (const listener of [...listeners]) listener(key, value);
    },
    subscribe(listener) {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    onReady(cb) {
      if (seeded) {
        cb();
        return () => {
        };
      }
      readyCallbacks.push(cb);
      return () => {
        const i = readyCallbacks.indexOf(cb);
        if (i >= 0) readyCallbacks.splice(i, 1);
      };
    },
    get ready() {
      return seeded;
    }
  };
}
function snap(value) {
  if (value === null || typeof value !== "object") return value;
  if (Array.isArray(value)) return value.map(snap);
  if (!isPlain(value)) return value;
  const out = {};
  for (const key of Reflect.ownKeys(value)) out[key] = snap(value[key]);
  return out;
}
function __omaBind(saved, fn) {
  sinks.push(fn);
  sink = fn;
  if (!seeded) {
    seeded = true;
    const data = saved || {};
    lastSaved = data;
    for (const c of configs) seedStore(c, data);
    for (const cb of [...readyCallbacks]) cb();
  }
  if (pending.size || dirty) {
    schedulePersist();
    pending.clear();
    dirty = false;
  }
  return fn;
}
function __omaUnbind(handle) {
  const i = sinks.indexOf(handle);
  if (i >= 0) sinks.splice(i, 1);
  if (sink === handle) sink = sinks.length ? sinks[sinks.length - 1] : null;
}
function __omaDebounceMs() {
  return maxDebounceMs;
}
var handlers = /* @__PURE__ */ new Map();
function on(event, handler) {
  let set = handlers.get(event);
  if (!set) {
    set = /* @__PURE__ */ new Set();
    handlers.set(event, set);
  }
  set.add(handler);
  return () => set.delete(handler);
}
function emit(event, payload) {
  const set = handlers.get(event);
  if (!set) return;
  for (const handler of [...set]) handler(payload);
}

// src/index.js
var timer = state({
  running: false,
  seconds: 0,
  display: "0:00",
  tickMs: 1e3,
  lastEvent: "ready"
});
var tickCount = state(0);
var formatted = derived(() => {
  const s = timer.seconds;
  return Math.floor(s / 60) + ":" + ("0" + s % 60).slice(-2);
});
var settings = config({ title: "Stopwatch", tickMs: 1e3 });
function tick() {
  tickCount.value++;
  timer.seconds++;
  timer.display = formatted.value;
}
function start() {
  if (timer.running) return;
  timer.tickMs = Number(settings.get("tickMs")) || 1e3;
  timer.running = true;
  timer.lastEvent = "started";
}
function stop() {
  if (!timer.running) return;
  timer.running = false;
  timer.lastEvent = "stopped";
}
function toggle() {
  timer.running ? stop() : start();
}
function reset() {
  timer.seconds = 0;
  timer.display = formatted.value;
  timer.lastEvent = "reset";
}
function resetViaIpc() {
  emit("stopwatch:reset");
}
on("stopwatch:reset", () => {
  tickCount.value++;
  timer.lastEvent = "ipc: reset via emit() (#" + tickCount.value + ")";
  timer.seconds = 0;
  timer.display = formatted.value;
});
function setTickMs(ms) {
  const v = Math.floor(Number(ms));
  if (v < 200) return;
  timer.tickMs = v;
  settings.set("tickMs", v);
  timer.lastEvent = "tick " + v + "ms";
}
function getTitle() {
  return settings.get("title");
}
function getTickCount() {
  return tickCount.value;
}
export {
  __omaBind as __omaBindRef,
  __omaDebounceMs as __omaDebounceMsRef,
  snap as __omaSnap,
  __omaUnbind as __omaUnbindRef,
  getTickCount,
  getTitle,
  reset,
  resetViaIpc,
  setTickMs,
  settings,
  start,
  stop,
  tick,
  timer,
  toggle
};

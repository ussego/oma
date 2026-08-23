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

// src/index.js
var counter = state({
  count: 0,
  step: 1
});
var settings = config({ step: 1 });
function applySavedStep() {
  const saved = Number(settings.get("step"));
  if (saved > 0) counter.step = saved;
}
function inc() {
  counter.count += counter.step;
}
function dec() {
  counter.count -= counter.step;
}
function reset() {
  counter.count = 0;
}
function setStep(v) {
  const n = Math.floor(Number(v));
  if (!(n > 0)) return;
  counter.step = n;
  settings.set("step", n);
}
export {
  __omaBind as __omaBindRef,
  __omaDebounceMs as __omaDebounceMsRef,
  snap as __omaSnap,
  __omaUnbind as __omaUnbindRef,
  applySavedStep,
  counter,
  dec,
  inc,
  reset,
  setStep,
  settings
};

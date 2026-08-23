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
var todos = state({
  open: "",
  done: ""
});
function lines(s) {
  return s ? s.split("\n") : [];
}
function add(text) {
  const t = text.trim();
  if (!t) return;
  todos.open = todos.open ? todos.open + "\n" + t : t;
}
function complete(text) {
  const rest = lines(todos.open).filter(function(t) {
    return t !== text;
  });
  todos.open = rest.join("\n");
  todos.done = todos.done ? todos.done + "\n" + text : text;
}
function reopen(text) {
  const rest = lines(todos.done).filter(function(t) {
    return t !== text;
  });
  todos.done = rest.join("\n");
  todos.open = todos.open ? todos.open + "\n" + text : text;
}
function clearDone() {
  todos.done = "";
}
export {
  __omaBind as __omaBindRef,
  __omaDebounceMs as __omaDebounceMsRef,
  snap as __omaSnap,
  __omaUnbind as __omaUnbindRef,
  add,
  clearDone,
  complete,
  reopen,
  todos
};

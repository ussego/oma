// ../../.cache/oma/oma.js
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
var persistFn = null;
var bound = false;
function schedulePersist() {
  if (!persistFn) return;
  const merged = {};
  for (const c of configs) {
    for (const [key, value] of c.store) merged[key] = value;
  }
  persistFn(merged);
}
function config(defaults) {
  const store = new Map(Object.entries(defaults));
  const listeners = /* @__PURE__ */ new Set();
  configs.push({ keys: Object.keys(defaults), store });
  return {
    get(key) {
      return store.get(key);
    },
    set(key, value) {
      store.set(key, value);
      schedulePersist();
      for (const listener of [...listeners]) listener(key, value);
    },
    subscribe(listener) {
      listeners.add(listener);
      return () => listeners.delete(listener);
    }
  };
}
function __omaBind(saved, fn) {
  if (bound) return;
  bound = true;
  persistFn = fn;
  const data = saved || {};
  for (const c of configs) {
    for (const key of c.keys) {
      if (data[key] !== void 0 && data[key] !== null) c.store.set(key, data[key]);
    }
  }
}

// examples/counter/src/index.js
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
  applySavedStep,
  counter,
  dec,
  inc,
  reset,
  setStep,
  settings
};

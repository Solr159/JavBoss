const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");

const source = fs.readFileSync(
  path.join(__dirname, "..", "content", "scrape-content.js"),
  "utf8",
);

test("stored JavDB assistance navigates without changing the URL hash", async () => {
  let replacedURL = "";
  let assistStyle = null;
  const windowListeners = new Map();
  const timers = [];
  const sentMessages = [];
  const sessionData = new Map();
  const assistRequest = {
    target: "series",
    code: "IPX-228",
    name: "中年オヤジ",
  };
  let receivedRequest = null;
  const location = {
    href: "https://javdb.com/search?q=IPX-228&f=all",
    replace(url) {
      replacedURL = url;
    },
  };
  const context = {
    JavBossJavDBParser: {
      assistedNavigationRequest: (request) => request || null,
      findAssistedNavigationURL: (_document, _url, request) => {
        receivedRequest = request;
        return "https://javdb.com/series/p32E";
      },
    },
    document: {
      readyState: "loading",
      createElement: () => ({
        remove() {
          this.removed = true;
        },
      }),
      documentElement: {
        setAttribute(name) {
          this.maskAttribute = name;
        },
        removeAttribute(name) {
          if (this.maskAttribute === name) this.maskAttribute = "";
        },
        appendChild(element) {
          assistStyle = element;
        },
      },
      getElementById: () => null,
    },
    location,
    chrome: {
      runtime: {
        onMessage: { addListener() {} },
        sendMessage: async (message) => {
          sentMessages.push(message);
          return message.type === "JAVBOSS_JAVDB_GET_ASSIST"
            ? { ok: true, request: assistRequest }
            : { relay: false };
        },
      },
    },
    sessionStorage: {
      getItem: (key) => sessionData.get(key) || "",
      setItem: (key, value) => sessionData.set(key, value),
      removeItem: (key) => sessionData.delete(key),
    },
    addEventListener(type, listener) {
      windowListeners.set(type, listener);
    },
    setTimeout(callback, delay) {
      timers.push({ callback, delay });
      return timers.length;
    },
    clearTimeout() {},
  };
  context.window = context;
  context.top = context;

  vm.runInNewContext(source, context);
  await new Promise((resolve) => setImmediate(resolve));

  assert.equal(replacedURL, "");
  assert.match(assistStyle.textContent, /background: #fff/);
  assert.match(assistStyle.textContent, /position: fixed/);
  assert.equal(
    context.document.documentElement.maskAttribute,
    "data-javboss-assisted-navigation",
  );
  assert.equal(
    timers.some((timer) => timer.delay === 15000),
    true,
  );
  assert.equal(typeof windowListeners.get("load"), "function");

  windowListeners.get("load")();
  assert.equal(replacedURL, "https://javdb.com/series/p32E");
  assert.notEqual(assistStyle.removed, true);
  assert.deepEqual(JSON.parse(JSON.stringify(receivedRequest)), assistRequest);
  assert.equal(
    sentMessages.some(
      (message) => message.type === "JAVBOSS_JAVDB_COMPLETE_ASSIST",
    ),
    false,
  );
  assert.deepEqual(
    JSON.parse(sessionData.get("javboss:javdb-assist-request")),
    assistRequest,
  );
});

test("the final assisted JavDB page asks the worker to reveal its tab", async () => {
  const sentMessages = [];
  const timers = [];
  let assistStyle = null;
  const assistRequest = { target: "movie", code: "IPX-228", name: "" };
  const sessionData = new Map([
    ["javboss:javdb-assist-request", JSON.stringify(assistRequest)],
  ]);
  const context = {
    JavBossJavDBParser: {
      assistedNavigationRequest: (request) => request || null,
      isAssistedNavigationTargetURL: () => true,
      findAssistedNavigationURL: () => "",
      parse: () => null,
    },
    document: {
      readyState: "loading",
      createElement: () => ({
        remove() {
          this.removed = true;
        },
      }),
      documentElement: {
        setAttribute(name) {
          this.maskAttribute = name;
        },
        removeAttribute(name) {
          if (this.maskAttribute === name) this.maskAttribute = "";
        },
        appendChild(element) {
          assistStyle = element;
        },
      },
      getElementById: () => null,
    },
    location: { href: "https://javdb.com/v/kKdRm" },
    chrome: {
      runtime: {
        onMessage: { addListener() {} },
        sendMessage: async (message) => {
          sentMessages.push(message);
          return message.type === "JAVBOSS_JAVDB_GET_ASSIST"
            ? { ok: true, request: assistRequest }
            : { ok: true, relay: false };
        },
      },
    },
    sessionStorage: {
      getItem: (key) => sessionData.get(key) || "",
      setItem: (key, value) => sessionData.set(key, value),
      removeItem: (key) => sessionData.delete(key),
    },
    addEventListener() {
      assert.fail("the final target must not wait for the load event");
    },
    setTimeout(callback, delay) {
      timers.push({ callback, delay });
      return timers.length;
    },
    clearTimeout() {},
  };
  context.window = context;
  context.top = context;

  vm.runInNewContext(source, context);
  await new Promise((resolve) => setImmediate(resolve));

  assert.equal(timers.length, 0);
  assert.equal(
    sentMessages.some(
      (message) => message.type === "JAVBOSS_JAVDB_COMPLETE_ASSIST",
    ),
    true,
  );
  assert.equal(assistStyle, null);
  assert.equal(context.document.documentElement.maskAttribute, undefined);
  assert.equal(sessionData.has("javboss:javdb-assist-request"), false);
});

test("a stalled assisted JavDB page becomes visible after the timeout", async () => {
  const sentMessages = [];
  const timers = [];
  let assistStyle = null;
  const sessionData = new Map();
  const context = {
    JavBossJavDBParser: {
      assistedNavigationRequest: (request) => request || null,
      findAssistedNavigationURL: () => "https://javdb.com/v/kKdRm",
      parse: () => null,
    },
    document: {
      readyState: "loading",
      createElement: () => ({
        remove() {
          this.removed = true;
        },
      }),
      documentElement: {
        setAttribute(name) {
          this.maskAttribute = name;
        },
        removeAttribute(name) {
          if (this.maskAttribute === name) this.maskAttribute = "";
        },
        appendChild(element) {
          assistStyle = element;
        },
      },
      getElementById: () => null,
    },
    location: {
      href: "https://javdb.com/search?q=IPX-228&f=all",
      replace() {
        assert.fail("a page completed by the timeout must not navigate later");
      },
    },
    chrome: {
      runtime: {
        onMessage: { addListener() {} },
        sendMessage: async (message) => {
          sentMessages.push(message);
          return message.type === "JAVBOSS_JAVDB_GET_ASSIST"
            ? {
                ok: true,
                request: { target: "movie", code: "IPX-228", name: "" },
              }
            : { ok: true, relay: false };
        },
      },
    },
    sessionStorage: {
      getItem: (key) => sessionData.get(key) || "",
      setItem: (key, value) => sessionData.set(key, value),
      removeItem: (key) => sessionData.delete(key),
    },
    addEventListener(type, listener) {
      if (type === "load") context.loadListener = listener;
    },
    setTimeout(callback, delay) {
      timers.push({ callback, delay });
      return timers.length;
    },
    clearTimeout() {},
  };
  context.window = context;
  context.top = context;

  vm.runInNewContext(source, context);
  await new Promise((resolve) => setImmediate(resolve));

  const completionTimer = timers.find((timer) => timer.delay === 15000);
  assert.ok(completionTimer);
  completionTimer.callback();
  context.loadListener();
  await new Promise((resolve) => setImmediate(resolve));

  assert.equal(
    sentMessages.filter(
      (message) => message.type === "JAVBOSS_JAVDB_COMPLETE_ASSIST",
    ).length,
    1,
  );
  assert.equal(assistStyle.removed, true);
  assert.equal(context.document.documentElement.maskAttribute, "");
  assert.equal(sessionData.has("javboss:javdb-assist-request"), false);
});

test("an ordinary JavDB page is revealed when there is no assist state", async () => {
  let assistStyle = null;
  const sessionData = new Map();
  const context = {
    JavBossJavDBParser: {
      assistedNavigationRequest: () => null,
      parse: () => null,
    },
    document: {
      readyState: "loading",
      createElement: () => ({
        remove() {
          this.removed = true;
        },
      }),
      documentElement: {
        setAttribute(name) {
          this.maskAttribute = name;
        },
        removeAttribute(name) {
          if (this.maskAttribute === name) this.maskAttribute = "";
        },
        appendChild(element) {
          assistStyle = element;
        },
      },
      getElementById: () => null,
    },
    location: { href: "https://javdb.com/v/kKdRm" },
    chrome: {
      runtime: {
        onMessage: { addListener() {} },
        sendMessage: async (message) =>
          message.type === "JAVBOSS_JAVDB_GET_ASSIST"
            ? { ok: true, request: null }
            : { ok: true, relay: false },
      },
    },
    sessionStorage: {
      getItem: (key) => sessionData.get(key) || "",
      setItem: (key, value) => sessionData.set(key, value),
      removeItem: (key) => sessionData.delete(key),
    },
    addEventListener() {},
    setTimeout: () => 1,
    clearTimeout() {},
  };
  context.window = context;
  context.top = context;

  vm.runInNewContext(source, context);
  await new Promise((resolve) => setImmediate(resolve));

  assert.equal(assistStyle.removed, true);
  assert.equal(context.document.documentElement.maskAttribute, "");
});

const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");

const source = fs.readFileSync(
  path.join(__dirname, "..", "service-worker.js"),
  "utf8",
);

const RELAY_PREFIX = "javboss:browser-relay:";
const SESSION_PREFIX = "javboss:browser-session:";
const SESSION_ID = "test-session-1234";

function plain(value) {
  return JSON.parse(JSON.stringify(value));
}

function createHarness() {
  const data = new Map([
    [`${RELAY_PREFIX}1`, { sessionId: SESSION_ID }],
    [`${SESSION_PREFIX}${SESSION_ID}`, { sessionId: SESSION_ID }],
  ]);
  const listeners = {};
  const sentMessages = [];

  const sessionStorage = {
    async get(keys) {
      if (keys === null) return Object.fromEntries(data);
      const requested = Array.isArray(keys) ? keys : [keys];
      return Object.fromEntries(
        requested
          .filter((key) => data.has(key))
          .map((key) => [key, data.get(key)]),
      );
    },
    async set(values) {
      for (const [key, value] of Object.entries(values)) data.set(key, value);
    },
    async remove(keys) {
      for (const key of Array.isArray(keys) ? keys : [keys]) data.delete(key);
    },
  };

  const chrome = {
    runtime: {
      onMessage: { addListener: (listener) => (listeners.message = listener) },
      onInstalled: {
        addListener: (listener) => (listeners.installed = listener),
      },
      sendMessage: async () => ({ ok: true }),
    },
    storage: { session: sessionStorage },
    tabs: {
      create: async () => ({ id: 10 }),
      update: async () => {},
      remove: async () => {},
      sendMessage: async (tabId, message) => {
        sentMessages.push({ tabId, message });
        return { ok: true };
      },
      onRemoved: { addListener: (listener) => (listeners.removed = listener) },
    },
  };

  vm.runInNewContext(source, { chrome, URL });

  async function send(message, tab) {
    return new Promise((resolve) => {
      const keepChannelOpen = listeners.message(
        message,
        { tab, url: tab.url },
        resolve,
      );
      assert.equal(keepChannelOpen, true);
    });
  }

  return { data, listeners, send, sentMessages };
}

test("a manually created tab cannot inherit from its opener", async () => {
  const harness = createHarness();
  const response = await harness.send(
    { type: "JAVBOSS_JAVBUS_IS_RELAY", sessionId: "" },
    { id: 2, openerTabId: 1, url: "https://www.javbus.com/ABC-123" },
  );

  assert.deepEqual(plain(response), { ok: true, relay: false });
  assert.equal(harness.data.has(`${RELAY_PREFIX}2`), false);
});

test("the bridge can open an allowed JavLibrary URL", async () => {
  const harness = createHarness();
  const response = await harness.send(
    {
      type: "JAVBOSS_JAVBUS_OPEN_RELAY",
      sessionId: SESSION_ID,
      url: "https://www.javlibrary.com/tw/vl_searchbyid.php?keyword=OFJE-282",
    },
    {
      id: 1,
      windowId: 5,
      url: "chrome-extension://iikdjhkpjihfkehccfmkpkdmenmbaacn/bridge.html",
    },
  );

  assert.deepEqual(plain(response), { ok: true });
  assert.deepEqual(plain(harness.data.get(`${RELAY_PREFIX}10`)), {
    sessionId: SESSION_ID,
  });
});

test("the bridge can open an allowed JavDB search URL", async () => {
  const harness = createHarness();
  const response = await harness.send(
    {
      type: "JAVBOSS_JAVBUS_OPEN_RELAY",
      sessionId: SESSION_ID,
      url: "https://javdb.com/search?q=OFJE-282&f=all",
    },
    {
      id: 1,
      windowId: 5,
      url: "chrome-extension://iikdjhkpjihfkehccfmkpkdmenmbaacn/bridge.html",
    },
  );

  assert.deepEqual(plain(response), { ok: true });
  assert.deepEqual(plain(harness.data.get(`${RELAY_PREFIX}10`)), {
    sessionId: SESSION_ID,
  });
});

test("the bridge can open an allowed AVSOX search URL", async () => {
  const harness = createHarness();
  const response = await harness.send(
    {
      type: "JAVBOSS_JAVBUS_OPEN_RELAY",
      sessionId: SESSION_ID,
      url: "https://avsox.click/tw/search/030919_047",
    },
    {
      id: 1,
      windowId: 5,
      url: "chrome-extension://iikdjhkpjihfkehccfmkpkdmenmbaacn/bridge.html",
    },
  );

  assert.deepEqual(plain(response), { ok: true });
  assert.deepEqual(plain(harness.data.get(`${RELAY_PREFIX}10`)), {
    sessionId: SESSION_ID,
  });
});

test("a tab with the temporary marker can claim the scrape session", async () => {
  const harness = createHarness();
  const response = await harness.send(
    { type: "JAVBOSS_JAVBUS_IS_RELAY", sessionId: SESSION_ID },
    { id: 2, url: "https://www.javbus.com/search/ABC-123" },
  );

  assert.deepEqual(plain(response), {
    ok: true,
    relay: true,
    sessionId: SESSION_ID,
  });
  assert.deepEqual(plain(harness.data.get(`${RELAY_PREFIX}2`)), {
    sessionId: SESSION_ID,
  });
});

const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");

const source = fs.readFileSync(path.join(__dirname, "..", "bridge.js"), "utf8");

function createBridge() {
  const listeners = {};
  const runtimeMessages = [];
  const parentMessages = [];
  let receiveRuntimeMessage;
  const parent = {
    postMessage(message, targetOrigin) {
      parentMessages.push({ message, targetOrigin });
    },
  };
  const window = {
    parent,
    addEventListener(type, listener) {
      listeners[type] = listener;
    },
  };
  const chrome = {
    runtime: {
      onMessage: {
        addListener(listener) {
          receiveRuntimeMessage = listener;
        },
      },
      async sendMessage(message) {
        runtimeMessages.push(message);
        return { ok: true };
      },
    },
  };

  vm.runInNewContext(source, { chrome, URL, window });
  return {
    listeners,
    runtimeMessages,
    parentMessages,
    parent,
    receiveRuntimeMessage,
  };
}

test("the bridge accepts HTTP and HTTPS pages regardless of hostname", () => {
  for (const origin of [
    "http://localhost:17654",
    "http://192.168.9.200:8655",
    "http://jb.example.com",
    "https://jb.example.com",
    "https://jb.example.com:8443",
    "http://[::1]:17654",
  ]) {
    const { listeners, parent, parentMessages } = createBridge();
    listeners.message({
      source: parent,
      origin,
      data: {
        type: "JAVBOSS_EXTENSION_CONNECT",
        sessionId: "test-session-1234",
      },
    });
    assert.equal(parentMessages.length, 1, origin);
    assert.equal(parentMessages[0].message.type, "JAVBOSS_EXTENSION_READY");
    assert.equal(parentMessages[0].message.version, 1);
    assert.equal(parentMessages[0].message.sessionId, "test-session-1234");
    assert.equal(parentMessages[0].targetOrigin, origin);
  }
});

test("the bridge rejects invalid origins, non-parent windows and invalid sessions", () => {
  for (const overrides of [
    { origin: "null" },
    { origin: "" },
    { origin: "not a URL" },
    { origin: "file:///tmp/page.html" },
    { origin: "data:text/html,test" },
    { origin: "ftp://jb.example.com" },
    { source: {} },
    { data: { type: "JAVBOSS_EXTENSION_CONNECT", sessionId: "bad" } },
  ]) {
    const { listeners, parent, parentMessages } = createBridge();
    listeners.message({
      source: parent,
      origin: "https://jb.example.com",
      data: {
        type: "JAVBOSS_EXTENSION_CONNECT",
        sessionId: "test-session-1234",
      },
      ...overrides,
    });
    assert.equal(parentMessages.length, 0, JSON.stringify(overrides));
  }
});

test("the bridge forwards clean JavDB assistance to the service worker", async () => {
  const { listeners, parent, runtimeMessages } = createBridge();
  const sessionId = "test-session-1234";
  listeners.message({
    source: parent,
    origin: "https://jb.example.com",
    data: { type: "JAVBOSS_EXTENSION_CONNECT", sessionId },
  });
  listeners.message({
    source: parent,
    origin: "https://jb.example.com",
    data: {
      type: "JAVBOSS_JAVDB_OPEN",
      sessionId,
      url: "https://javdb.com/search?q=ADN-429&f=all",
      fallbackUrl:
        "https://javdb.com/search?f=actor&q=%E5%B2%AC%E3%81%AA%E3%81%AA%E3%81%BF",
      request: { target: "idol", code: "ADN-429", name: "岬ななみ" },
    },
  });
  await new Promise((resolve) => setImmediate(resolve));

  assert.deepEqual(JSON.parse(JSON.stringify(runtimeMessages)), [
    {
      type: "JAVBOSS_JAVDB_OPEN_ASSIST",
      sessionId,
      url: "https://javdb.com/search?q=ADN-429&f=all",
      fallbackUrl:
        "https://javdb.com/search?f=actor&q=%E5%B2%AC%E3%81%AA%E3%81%AA%E3%81%BF",
      request: { target: "idol", code: "ADN-429", name: "岬ななみ" },
    },
  ]);
});

test("the bridge isolates commands and metadata by origin, window and session", async () => {
  const {
    listeners,
    parent,
    parentMessages,
    runtimeMessages,
    receiveRuntimeMessage,
  } = createBridge();
  const origin = "https://jb.example.com";
  const sessionId = "test-session-1234";
  listeners.message({
    source: parent,
    origin,
    data: { type: "JAVBOSS_EXTENSION_CONNECT", sessionId },
  });
  const data = {
    type: "JAVBOSS_SCRAPE_OPEN",
    sessionId,
    url: "https://www.javbus.com/ADN-429",
  };
  for (const overrides of [
    { source: {} },
    { origin: "https://other.example.com" },
    { origin: "http://jb.example.com" },
    { origin: "https://jb.example.com:8443" },
    { data: { ...data, sessionId: "another-session-1234" } },
    { data: { ...data, type: "JAVBOSS_DOWNLOAD_MAGNET" } },
  ]) {
    listeners.message({ source: parent, origin, data, ...overrides });
  }
  assert.equal(runtimeMessages.length, 0);
  listeners.message({ source: parent, origin, data });
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(runtimeMessages.length, 1);
  assert.equal(runtimeMessages[0].type, "JAVBOSS_SCRAPE_OPEN_RELAY");

  const metadata = {
    type: "JAVBOSS_SCRAPE_BRIDGE_METADATA",
    sessionId,
    payload: { code: "ADN-429" },
  };
  const before = parentMessages.length;
  receiveRuntimeMessage(
    { ...metadata, sessionId: "another-session-1234" },
    {},
    () => assert.fail("unrelated session must not be acknowledged"),
  );
  assert.equal(parentMessages.length, before);
  receiveRuntimeMessage(metadata, {}, () => {});
  assert.equal(parentMessages.length, before + 1);
  assert.equal(parentMessages.at(-1).message.type, "JAVBOSS_SCRAPE_METADATA");
  assert.equal(parentMessages.at(-1).message.payload.code, "ADN-429");
  assert.ok(parentMessages.every((entry) => entry.targetOrigin === origin));
});

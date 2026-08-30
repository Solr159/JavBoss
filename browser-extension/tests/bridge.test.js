const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");

const source = fs.readFileSync(path.join(__dirname, "..", "bridge.js"), "utf8");

test("the bridge forwards clean JavDB assistance to the service worker", async () => {
  const listeners = {};
  const runtimeMessages = [];
  const parent = { postMessage() {} };
  const window = {
    parent,
    addEventListener(type, listener) {
      listeners[type] = listener;
    },
  };
  const chrome = {
    runtime: {
      onMessage: { addListener() {} },
      async sendMessage(message) {
        runtimeMessages.push(message);
        return { ok: true };
      },
    },
  };

  vm.runInNewContext(source, { chrome, URL, window });
  const sessionId = "test-session-1234";
  listeners.message({
    source: parent,
    origin: "http://localhost:17654",
    data: { type: "JAVBOSS_EXTENSION_CONNECT", sessionId },
  });
  listeners.message({
    source: parent,
    origin: "http://localhost:17654",
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

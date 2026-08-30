const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");

const source = fs.readFileSync(path.join(__dirname, "..", "popup.js"), "utf8");

function createHarness({ magnetSettings = null, javDBSettings = null } = {}) {
  const elements = new Map();
  for (const id of [
    "server-url",
    "enabled",
    "javdb-auto-redirect",
    "save",
    "status",
  ]) {
    const listeners = {};
    elements.set(id, {
      checked: false,
      classList: { toggle() {} },
      disabled: false,
      value: "",
      textContent: "",
      addEventListener(type, listener) {
        listeners[type] = listener;
      },
      listeners,
    });
  }
  const storedValues = [];
  const permissionRequests = [];
  const chrome = {
    permissions: {
      async request(request) {
        permissionRequests.push(request);
        return true;
      },
    },
    storage: {
      local: {
        async get(keys) {
          const stored = {};
          if (magnetSettings) {
            stored["javboss:magnet-download-settings"] = magnetSettings;
          }
          if (javDBSettings) {
            stored["javboss:javdb-settings"] = javDBSettings;
          }
          const requested = new Set(Array.isArray(keys) ? keys : [keys]);
          return Object.fromEntries(
            Object.entries(stored).filter(([key]) => requested.has(key)),
          );
        },
        async set(value) {
          storedValues.push(value);
        },
      },
    },
  };
  const document = {
    getElementById(id) {
      return elements.get(id);
    },
  };
  vm.runInNewContext(source, { chrome, document, URL });
  return { elements, permissionRequests, storedValues };
}

test("the popup starts with magnet downloads disabled and JavDB redirects enabled", async () => {
  const harness = createHarness();

  await new Promise((resolve) => setImmediate(resolve));

  assert.equal(harness.elements.get("server-url").value, "");
  assert.equal(harness.elements.get("enabled").checked, false);
  assert.equal(harness.elements.get("javdb-auto-redirect").checked, true);
});

test("the popup restores a disabled JavDB auto redirect setting", async () => {
  const harness = createHarness({
    javDBSettings: { autoRedirect: false },
  });

  await new Promise((resolve) => setImmediate(resolve));

  assert.equal(harness.elements.get("javdb-auto-redirect").checked, false);
});

test("enabling saves the server URL after requesting host access", async () => {
  const harness = createHarness();
  await new Promise((resolve) => setImmediate(resolve));
  harness.elements.get("server-url").value =
    " http://192.168.1.20:17654/javboss/ ";
  harness.elements.get("enabled").checked = true;

  await harness.elements.get("save").listeners.click();

  assert.deepEqual(JSON.parse(JSON.stringify(harness.permissionRequests)), [
    { origins: ["http://192.168.1.20/*"] },
  ]);
  assert.deepEqual(JSON.parse(JSON.stringify(harness.storedValues)), [
    {
      "javboss:magnet-download-settings": {
        enabled: true,
        serverUrl: "http://192.168.1.20:17654/javboss",
      },
      "javboss:javdb-settings": {
        autoRedirect: true,
      },
    },
  ]);
  assert.equal(harness.elements.get("status").textContent, "设置已保存");
});

test("JavDB auto redirect can be disabled without magnet settings", async () => {
  const harness = createHarness();
  await new Promise((resolve) => setImmediate(resolve));
  harness.elements.get("javdb-auto-redirect").checked = false;

  await harness.elements.get("save").listeners.click();

  assert.deepEqual(JSON.parse(JSON.stringify(harness.permissionRequests)), []);
  assert.deepEqual(JSON.parse(JSON.stringify(harness.storedValues)), [
    {
      "javboss:magnet-download-settings": {
        enabled: false,
        serverUrl: "",
      },
      "javboss:javdb-settings": {
        autoRedirect: false,
      },
    },
  ]);
  assert.equal(harness.elements.get("status").textContent, "设置已保存");
});

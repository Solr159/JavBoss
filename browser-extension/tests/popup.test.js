const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");

const source = fs.readFileSync(path.join(__dirname, "..", "popup.js"), "utf8");

function createHarness(settings = null) {
  const elements = new Map();
  for (const id of ["server-url", "enabled", "save", "status"]) {
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
        async get(key) {
          return settings ? { [key]: settings } : {};
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

test("the popup starts with magnet downloads disabled", async () => {
  const harness = createHarness();

  await new Promise((resolve) => setImmediate(resolve));

  assert.equal(harness.elements.get("server-url").value, "");
  assert.equal(harness.elements.get("enabled").checked, false);
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
    },
  ]);
  assert.equal(harness.elements.get("status").textContent, "已启用并保存");
});

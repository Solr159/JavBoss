const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs/promises");
const { existsSync } = require("node:fs");
const path = require("node:path");
const os = require("node:os");
const http = require("node:http");

const EXTENSION_ID = "iikdjhkpjihfkehccfmkpkdmenmbaacn";
const CONNECTION_KEY = "javboss:connection-settings";
const TOKEN = "jbe_" + "a".repeat(43);
const chromePath = process.env.CHROME_BIN || "/usr/bin/google-chrome";
const browserUnavailable =
  !existsSync(chromePath) || typeof WebSocket === "undefined";

async function openExtension(t, mode) {
  const extensionDir = await fs.mkdtemp(
    path.join(os.tmpdir(), "javboss-storage-test-"),
  );
  t.after(() => fs.rm(extensionDir, { recursive: true, force: true }));
  await fs.cp(path.join(__dirname, ".."), extensionDir, {
    recursive: true,
    filter: (source) => !source.includes(`${path.sep}tests`),
  });
  const storagePath = path.join(extensionDir, "settings-storage.js");
  const patches = {
    missing: "chrome.storage.local.setAccessLevel = undefined;",
    throws:
      'chrome.storage.local.setAccessLevel = () => { throw new Error("unsupported"); };',
    rejects:
      'chrome.storage.local.setAccessLevel = () => Promise.reject(new Error("unsupported"));',
    supported: "",
  };
  await fs.writeFile(
    storagePath,
    patches[mode] + "\n" + (await fs.readFile(storagePath, "utf8")),
  );

  // The temporary extension gets a local probe only; production host permissions
  // and content scripts are unchanged. Read from the actual untrusted context.
  const manifestPath = path.join(extensionDir, "manifest.json");
  const manifest = JSON.parse(await fs.readFile(manifestPath, "utf8"));
  manifest.host_permissions.push("http://127.0.0.1/*");
  manifest.content_scripts.push({
    matches: ["http://127.0.0.1/*"],
    js: ["credential-probe.js"],
  });
  await fs.writeFile(manifestPath, JSON.stringify(manifest));
  await fs.writeFile(
    path.join(extensionDir, "credential-probe.js"),
    `
    (async () => {
      let local;
      try { local = await chrome.storage.local.get(${JSON.stringify(CONNECTION_KEY)}); }
      catch { local = {}; }
      const databases = await indexedDB.databases();
      const flags = await chrome.runtime.sendMessage({type: "JAVBOSS_MAGNET_SETTINGS"});
      document.documentElement.dataset.credentialProbe = JSON.stringify({local, databases, flags});
    })();
  `,
  );

  const requests = [];
  const server = http.createServer((request, response) => {
    requests.push({
      url: request.url,
      authorization: request.headers.authorization,
    });
    if (request.url.startsWith("/extension/")) {
      response.setHeader("Content-Type", "application/json");
      response.end(JSON.stringify({ authenticated: true }));
    } else {
      response.setHeader("Content-Type", "text/html");
      response.end("<!doctype html><title>Credential isolation probe</title>");
    }
  });
  await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
  t.after(() => new Promise((resolve) => server.close(resolve)));
  const serverUrl = `http://127.0.0.1:${server.address().port}`;
  const { startChrome } = await import("../../web/tests/helpers/chrome.js");
  const { port } = await startChrome(t, {
    args: [
      "--disable-features=DisableLoadExtensionCommandLineSwitch",
      `--load-extension=${extensionDir}`,
    ],
  });
  const target = await (
    await fetch(`http://127.0.0.1:${port}/json/new?about:blank`, {
      method: "PUT",
    })
  ).json();
  const socket = new WebSocket(target.webSocketDebuggerUrl);
  await new Promise((resolve) =>
    socket.addEventListener("open", resolve, { once: true }),
  );
  t.after(() => socket.close());
  let sequence = 0;
  const pending = new Map();
  socket.addEventListener("message", ({ data }) => {
    const message = JSON.parse(data);
    if (pending.has(message.id)) {
      pending.get(message.id)(message);
      pending.delete(message.id);
    }
  });
  const command = (method, params = {}) =>
    new Promise((resolve) => {
      const id = ++sequence;
      pending.set(id, resolve);
      socket.send(JSON.stringify({ id, method, params }));
    });
  const evaluate = async (expression) => {
    const reply = await command("Runtime.evaluate", {
      expression,
      returnByValue: true,
      awaitPromise: true,
      userGesture: true,
    });
    assert.ok(
      !reply.error && !reply.result?.exceptionDetails,
      JSON.stringify(reply),
    );
    return reply.result.result.value;
  };
  const waitFor = async (expression) => {
    for (let i = 0; i < 100; i++) {
      if (await evaluate(`(async () => Boolean(${expression}))()`)) return;
      await new Promise((resolve) => setTimeout(resolve, 50));
    }
    assert.fail(`Timed out: ${expression}`);
  };
  await command("Page.enable");
  const openPopup = async () => {
    const previous = await evaluate("performance.timeOrigin");
    await command("Page.navigate", {
      url: `chrome-extension://${EXTENSION_ID}/popup.html`,
    });
    await waitFor(
      `performance.timeOrigin !== ${previous} && typeof JavBossSettings !== 'undefined'`,
    );
    await waitFor(
      `!document.querySelector('#ownership-enabled').disabled || document.querySelector('#status').textContent`,
    );
  };
  await openPopup();
  return { evaluate, command, waitFor, openPopup, serverUrl, requests };
}

for (const mode of ["missing", "throws", "rejects", "supported"]) {
  test(
    `extension credentials persist and remain private when setAccessLevel is ${mode}`,
    { skip: browserUnavailable, timeout: 60000 },
    async (t) => {
      const { evaluate, command, waitFor, openPopup, serverUrl, requests } =
        await openExtension(t, mode);
      assert.equal(await evaluate("JavBossSettings.ready"), true);
      // Model an upgrade from the previous release, then let a new popup migrate.
      const connection = { serverUrl, apiToken: TOKEN };
      await evaluate(
        `chrome.storage.local.set(${JSON.stringify({ [CONNECTION_KEY]: connection })})`,
      );
      await openPopup();
      assert.equal(
        await evaluate("document.querySelector('#server-token').value"),
        TOKEN,
      );
      assert.deepEqual(
        await evaluate(`chrome.storage.local.get('${CONNECTION_KEY}')`),
        {},
      );

      // A second migration must not replace the already saved connection.
      await evaluate(
        `chrome.storage.local.set({'${CONNECTION_KEY}': {serverUrl:'https://stale.example',apiToken:'stale'}})`,
      );
      await openPopup();
      assert.equal(
        await evaluate("document.querySelector('#server-url').value"),
        serverUrl,
      );
      assert.equal(
        await evaluate("document.querySelector('#server-token').value"),
        TOKEN,
      );
      await evaluate("document.querySelector('#test-connection').click()");
      await waitFor(
        "document.querySelector('#connection-status').textContent.includes('连接成功')",
      );
      assert.equal(
        requests.find((request) => request.url === "/extension/status")
          ?.authorization,
        `Bearer ${TOKEN}`,
      );

      const nextToken = "jbe_" + "b".repeat(43);
      await evaluate(`{
        const input = document.querySelector('#server-token');
        input.value = '${nextToken}';
        input.dispatchEvent(new Event('input', {bubbles:true}));
      }`);
      await waitFor(
        `(await JavBossSettings.get('${CONNECTION_KEY}'))['${CONNECTION_KEY}']?.apiToken === '${nextToken}'`,
      );
      await openPopup();
      assert.equal(
        await evaluate("document.querySelector('#server-token').value"),
        nextToken,
      );
      const local = await evaluate("chrome.storage.local.get(null)");
      assert.equal(local[CONNECTION_KEY], undefined);
      assert.equal(JSON.stringify(local).includes(nextToken), false);
      assert.equal(typeof local["javboss:connection-changed"], "string");

      // Wake the real service worker and check that it reads the same private DB.
      await evaluate(
        "chrome.storage.local.set({'javboss:magnet-download-settings':{enabled:true}})",
      );
      await command("ServiceWorker.enable");
      const stopped = await command("ServiceWorker.stopAllWorkers");
      assert.equal(stopped.error, undefined);
      assert.deepEqual(
        await evaluate(`chrome.runtime.sendMessage({
        type:'JAVBOSS_DOWNLOAD_MAGNET', magnetUrl:'magnet:?xt=urn:btih:0123456789ABCDEF0123456789ABCDEF01234567'
      })`),
        { ok: true },
      );
      assert.equal(
        requests.find((request) => request.url === "/extension/downloads")
          ?.authorization,
        `Bearer ${nextToken}`,
      );

      await command("Page.navigate", { url: serverUrl });
      await waitFor("document.documentElement?.dataset.credentialProbe");
      assert.deepEqual(
        await evaluate(
          "JSON.parse(document.documentElement.dataset.credentialProbe)",
        ),
        {
          local: {},
          databases: [],
          flags: { enabled: true },
        },
      );
    },
  );
}

test(
  "failed IndexedDB transactions never delete legacy credentials or fall back to public storage",
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { evaluate, command, waitFor, openPopup, serverUrl } =
      await openExtension(t, "missing");
    const connection = { serverUrl, apiToken: TOKEN };
    await evaluate(
      `chrome.storage.local.set(${JSON.stringify({ [CONNECTION_KEY]: connection })})`,
    );
    const { result: injected } = await command(
      "Page.addScriptToEvaluateOnNewDocument",
      {
        source: `const originalPut = IDBObjectStore.prototype.put;
        IDBObjectStore.prototype.put = function(...args) {
          const request = originalPut.apply(this, args);
          this.transaction.abort();
          return request;
        };`,
      },
    );
    await openPopup();
    assert.equal(await evaluate("JavBossSettings.ready"), false);
    assert.deepEqual(
      await evaluate(`chrome.storage.local.get('${CONNECTION_KEY}')`),
      { [CONNECTION_KEY]: connection },
    );
    assert.match(
      await evaluate("document.querySelector('#status').textContent"),
      /凭据存储/,
    );
    await command("Page.removeScriptToEvaluateOnNewDocument", {
      identifier: injected.identifier,
    });
    await openPopup();
    assert.equal(await evaluate("JavBossSettings.ready"), true);
    assert.deepEqual(
      await evaluate(`chrome.storage.local.get('${CONNECTION_KEY}')`),
      {},
    );

    await evaluate(`{
      const put = IDBObjectStore.prototype.put;
      IDBObjectStore.prototype.put = function(...args) {
        const request = put.apply(this, args); this.transaction.abort(); return request;
      };
      const input = document.querySelector('#server-token');
      input.value = 'jbe_new'; input.dispatchEvent(new Event('input', {bubbles:true}));
    }`);
    await waitFor(
      "document.querySelector('#status').textContent.includes('凭据存储')",
    );
    assert.deepEqual(
      await evaluate(`chrome.storage.local.get('${CONNECTION_KEY}')`),
      {},
    );
    await openPopup();
    assert.equal(
      await evaluate("document.querySelector('#server-token').value"),
      TOKEN,
    );
  },
);

test(
  "unavailable IndexedDB reports an error without breaking popup initialization",
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { evaluate, command, waitFor, openPopup, serverUrl, requests } =
      await openExtension(t, "missing");
    await command("Page.addScriptToEvaluateOnNewDocument", {
      source:
        'Object.defineProperty(globalThis, "indexedDB", {value: undefined});',
    });
    await openPopup();
    assert.equal(await evaluate("JavBossSettings.ready"), false);
    assert.match(
      await evaluate("document.querySelector('#status').textContent"),
      /凭据存储/,
    );
    await evaluate(`{
      document.querySelector('#server-url').value = '${serverUrl}';
      document.querySelector('#server-token').value = '${TOKEN}';
      document.querySelector('#test-connection').click();
    }`);
    await waitFor(
      "document.querySelector('#connection-status').textContent.includes('凭据存储')",
    );
    assert.equal(
      requests.some((request) => request.authorization),
      false,
    );
    assert.deepEqual(
      await evaluate(`chrome.storage.local.get('${CONNECTION_KEY}')`),
      {},
    );
  },
);

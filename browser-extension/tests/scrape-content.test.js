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
  let blankStyle = null;
  const windowListeners = new Map();
  const timers = [];
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
  const deterministicMath = Object.create(Math);
  deterministicMath.random = () => 0.5;
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
        setAttribute() {},
        remove() {},
      }),
      documentElement: {
        appendChild(element) {
          blankStyle = element;
        },
      },
      getElementById: () => null,
    },
    location,
    Math: deterministicMath,
    chrome: {
      runtime: {
        onMessage: { addListener() {} },
        sendMessage: async (message) =>
          message.type === "JAVBOSS_JAVDB_GET_ASSIST"
            ? { ok: true, request: assistRequest }
            : { relay: false },
      },
    },
    sessionStorage: {
      getItem: () => "",
      removeItem() {},
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
  assert.equal(
    timers.some((timer) => timer.delay >= 300 && timer.delay <= 600),
    false,
  );
  assert.equal(typeof windowListeners.get("load"), "function");

  windowListeners.get("load")();
  assert.equal(replacedURL, "");
  const navigationTimer = timers.find(
    (timer) => timer.delay >= 300 && timer.delay <= 600,
  );
  assert.ok(navigationTimer);
  assert.equal(navigationTimer.delay, 450);
  navigationTimer.callback();

  assert.equal(replacedURL, "https://javdb.com/series/p32E");
  assert.deepEqual(JSON.parse(JSON.stringify(receivedRequest)), assistRequest);
  assert.match(blankStyle.textContent, /visibility: hidden/);
});

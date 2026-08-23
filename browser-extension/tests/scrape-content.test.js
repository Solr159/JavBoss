const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");

const source = fs.readFileSync(
  path.join(__dirname, "..", "content", "scrape-content.js"),
  "utf8",
);

test("an assisted JavDB navigation waits for load and a random delay", () => {
  let replacedURL = "";
  let blankStyle = null;
  const windowListeners = new Map();
  const timers = [];
  const location = {
    href: "https://javdb.com/search?f=series&q=exact#javboss=direct&target=series&code=IPX-228",
    replace(url) {
      replacedURL = url;
    },
  };
  const deterministicMath = Object.create(Math);
  deterministicMath.random = () => 0.5;
  const context = {
    JavBossJavDBParser: {
      findAssistedNavigationURL: () => "https://javdb.com/series/p32E",
      isAssistedNavigationURL: () => true,
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
        sendMessage: async () => ({ relay: false }),
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
  assert.match(blankStyle.textContent, /visibility: hidden/);
});

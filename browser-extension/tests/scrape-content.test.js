const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");

const source = fs.readFileSync(
  path.join(__dirname, "..", "content", "scrape-content.js"),
  "utf8",
);

test("an assisted JavDB navigation replaces the current page", () => {
  let replacedURL = "";
  let blankStyle = null;
  const location = {
    href:
      "https://javdb.com/search?f=series&q=exact#javboss=direct&target=series&code=IPX-228",
    replace(url) {
      replacedURL = url;
    },
  };
  const context = {
    JavBossJavDBParser: {
      findAssistedNavigationURL: () => "https://javdb.com/series/p32E",
      isAssistedNavigationURL: () => true,
    },
    document: {
      readyState: "complete",
      createElement: () => ({
        setAttribute() {},
        remove() {},
      }),
      documentElement: {
        appendChild(element) {
          blankStyle = element;
        },
      },
    },
    location,
    setTimeout: () => 1,
    clearTimeout() {},
  };
  context.window = context;
  context.top = context;

  vm.runInNewContext(source, context);

  assert.equal(replacedURL, "https://javdb.com/series/p32E");
  assert.match(blankStyle.textContent, /visibility: hidden/);
});

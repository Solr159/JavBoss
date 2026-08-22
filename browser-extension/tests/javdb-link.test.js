const test = require("node:test");
const assert = require("node:assert/strict");
const path = require("node:path");
const { pathToFileURL } = require("node:url");

const moduleURL = pathToFileURL(
  path.join(__dirname, "..", "..", "web", "src", "utils", "javdb.js"),
).href;

test("disabled assistant preserves the original JavDB URL", async () => {
  const { resolveJavDBOpenURL } = await import(moduleURL);
  const fallback = "https://javdb.com/search?f=actor&q=%E5%B2%AC%E3%81%AA%E3%81%AA%E3%81%BF";

  assert.equal(
    resolveJavDBOpenURL(
      fallback,
      { target: "idol", code: "IPX-228", name: "岬ななみ" },
      false,
    ),
    fallback,
  );
});

test("connected assistant opens the marked code search directly", async () => {
  const { resolveJavDBOpenURL } = await import(moduleURL);
  const fallback = "https://javdb.com/search?f=actor&q=%E5%B2%AC%E3%81%AA%E3%81%AA%E3%81%BF";
  const result = new URL(
    resolveJavDBOpenURL(
      fallback,
      { target: "idol", code: "IPX-228", name: "岬ななみ" },
      true,
    ),
  );

  assert.equal(result.pathname, "/search");
  assert.equal(result.searchParams.get("q"), "IPX-228");
  assert.equal(result.searchParams.get("f"), "all");
  const marker = new URLSearchParams(result.hash.slice(1));
  assert.equal(marker.get("javboss"), "direct");
  assert.equal(marker.get("target"), "idol");
  assert.equal(marker.get("name"), "岬ななみ");
});

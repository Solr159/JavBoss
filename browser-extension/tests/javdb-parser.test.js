const test = require("node:test");
const assert = require("node:assert/strict");

const parser = require("../content/javdb-parser.js");

test("normalizeLabel handles JavDB field punctuation", () => {
  assert.equal(parser.normalizeLabel("  番號: "), "番號");
});

test("parse maps the JavDB detail fields used by the supplied sample", () => {
  const node = (textContent, attributes = {}, children = new Map()) => ({
    textContent,
    getAttribute: (name) => attributes[name] || null,
    querySelector: (selector) => children.get(selector)?.[0] || null,
    querySelectorAll: (selector) => children.get(selector) || [],
  });
  const block = (label, value, links = [], selectors = new Map()) =>
    node(
      "",
      {},
      new Map([
        ["strong", [node(label)]],
        [".value", [node(value)]],
        [".value a", links.map((link) => node(link.text, { href: link.href }))],
        [
          '.value a[href*="/tags/uncensored"]',
          links
            .filter((link) => link.href.includes("/tags/uncensored"))
            .map((link) => node(link.text, { href: link.href })),
        ],
        ...selectors,
      ]),
    );
  const blocks = [
    block("番號:", "082226_01"),
    block("日期:", "2026-08-22"),
    block("時長:", "51 分鍾"),
    block("片商:", "10musume", [{ text: "10musume", href: "/makers/mMr" }]),
    block("類別:", "素人, 白虎", [
      { text: "素人", href: "/tags?c3=88" },
      { text: "白虎", href: "/tags?c3=74" },
    ]),
    block("演員:", "白花まなみ", [
      { text: "白花まなみ", href: "/actors/76Wdb" },
    ]),
  ];
  const singles = new Map([
    [
      ".video-detail .current-title",
      node("リフレの面接に来た娘に手とり足とり実践指導"),
    ],
    [".video-detail > h2 strong:not(.current-title)", node("082226_01 無碼")],
    [
      ".video-cover",
      node("", { src: "https://c0.jdbstatic.com/covers/r3/r37E2D.jpg" }),
    ],
  ]);
  const document = {
    querySelector: (selector) => singles.get(selector) || null,
    querySelectorAll: (selector) =>
      selector === ".movie-panel-info .panel-block" ? blocks : [],
  };

  for (const detailBlock of blocks) {
    const original = detailBlock.querySelectorAll;
    detailBlock.querySelectorAll = (selector) => {
      if (selector.startsWith('.value a[href^="')) {
        const prefix = selector.match(/href\^="([^"]+)/)?.[1] || "";
        return original(".value a").filter((link) =>
          link.getAttribute("href").startsWith(prefix),
        );
      }
      return original(selector);
    };
  }

  assert.deepEqual(parser.parse(document, "https://javdb.com/v/r37E2D"), {
    code: "082226_01",
    title: "リフレの面接に来た娘に手とり足とり実践指導",
    studio: "10musume",
    series: "",
    release_date: "2026-08-22",
    duration_min: 51,
    tags: ["素人", "白虎"],
    actors: ["白花まなみ"],
    cover_url: "https://c0.jdbstatic.com/covers/r3/r37E2D.jpg",
    is_uncensored: true,
    source_name: "JavDB",
    source_url: "https://javdb.com/v/r37E2D",
  });
});

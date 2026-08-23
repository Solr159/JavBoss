const test = require("node:test");
const assert = require("node:assert/strict");

const parser = require("../content/javdb-parser.js");

test("normalizeLabel handles JavDB field punctuation", () => {
  assert.equal(parser.normalizeLabel("  番號: "), "番號");
});

function movieResult(code, href) {
  const link = {
    getAttribute: (name) => (name === "href" ? href : null),
  };
  return {
    querySelector: (selector) =>
      selector === ".video-title strong"
        ? { textContent: code }
        : selector === "a[href]"
          ? link
          : null,
  };
}

function movieSearchDocument(items) {
  return {
    querySelectorAll: (selector) =>
      selector === ".movie-list .item" ? items : [],
  };
}

function assistURL(
  path,
  { target = "series", code = "IPX-228", name = "" } = {},
) {
  const url = new URL(path, "https://javdb.com");
  const marker = new URLSearchParams({ javboss: "direct", target, code });
  if (name) marker.set("name", name);
  url.hash = marker.toString();
  return url.href;
}

function detailLink(name, href, gender = "", symbolText = "") {
  return {
    textContent: name,
    getAttribute: (attribute) => (attribute === "href" ? href : null),
    nextElementSibling:
      gender || symbolText
        ? {
            textContent: symbolText,
            getAttribute: (attribute) =>
              attribute === "class" ? `symbol ${gender}`.trim() : null,
          }
        : null,
  };
}

function detailBlock(label, links) {
  return {
    querySelector: (selector) =>
      selector === "strong" ? { textContent: label } : null,
    querySelectorAll: (selector) =>
      selector === ".value a[href]" ? links : [],
  };
}

function detailDocument(blocks) {
  return {
    querySelectorAll: (selector) =>
      selector === ".movie-panel-info .panel-block" ? blocks : [],
  };
}

test("marked entity search starts with a code search", () => {
  const sourceURL = assistURL(
    "/search?f=series&q=%E6%8B%85%E4%BB%BB%E3%81%8B%E3%82%89%E3%81%93%E3%81%93%E3%81%B8%E6%9D%A5%E3%82%8B%E3%82%88%E3%81%86%E3%81%AB%E8%A8%80%E3%82%8F%E3%82%8C%E3%81%BE%E3%81%97%E3%81%9F%E3%80%82",
    { target: "series", code: "IPX-228", name: "中年オヤジ" },
  );

  const result = new URL(
    parser.findAssistedNavigationURL(movieSearchDocument([]), sourceURL),
  );
  assert.equal(result.pathname, "/search");
  assert.equal(result.searchParams.get("q"), "IPX-228");
  assert.equal(result.searchParams.get("f"), "all");
  assert.equal(result.hash, new URL(sourceURL).hash);
});

test("code search resolves the unique exact movie and carries the marker", () => {
  const document = movieSearchDocument([
    movieResult("IPX-228", "/v/kKdRm"),
    movieResult("IPX-128", "/v/zKmWJ"),
  ]);
  const sourceURL = assistURL("/search?q=ipx-228&f=all", {
    target: "series",
    code: "IPX-228",
    name: "中年オヤジ",
  });

  assert.equal(
    parser.findAssistedNavigationURL(document, sourceURL),
    assistURL("/v/kKdRm", {
      target: "series",
      code: "IPX-228",
      name: "中年オヤジ",
    }),
  );
});

test("code search leaves ambiguous exact movie results visible", () => {
  const document = movieSearchDocument([
    movieResult("IPX-228", "/v/first"),
    movieResult("ipx228", "/v/second"),
  ]);

  assert.equal(
    parser.findAssistedNavigationURL(
      document,
      assistURL("/search?q=IPX-228&f=all"),
    ),
    "",
  );
});

test("movie detail resolves idol, series, and studio links", () => {
  const document = detailDocument([
    detailBlock("演員:", [
      detailLink("別の女優", "/actors/first"),
      detailLink("岬ななみ", "/actors/QNen"),
      detailLink("男優", "/actors/male", "male"),
    ]),
    detailBlock("系列:", [detailLink("中年オヤジ", "/series/w54b")]),
    detailBlock("片商:", [
      detailLink("アイデアポケット", "/makers/ZXX?f=download#results"),
    ]),
  ]);

  const cases = [
    ["idol", "岬ななみ", "https://javdb.com/actors/QNen"],
    ["series", "中年オヤジ", "https://javdb.com/series/w54b"],
    ["studio", "アイデアポケット", "https://javdb.com/makers/ZXX"],
  ];
  for (const [target, name, expected] of cases) {
    assert.equal(
      parser.findAssistedNavigationURL(
        document,
        assistURL("/v/kKdRm", { target, name }),
      ),
      expected,
    );
  }
});

test("movie detail excludes male actors marked only by symbol text", () => {
  const document = detailDocument([
    detailBlock("演員:", [
      detailLink("岬ななみ", "/actors/QNen", "female"),
      detailLink("男優", "/actors/male", "", "♂"),
    ]),
  ]);

  assert.equal(
    parser.findAssistedNavigationURL(
      document,
      assistURL("/v/kKdRm", { target: "idol" }),
    ),
    "https://javdb.com/actors/QNen",
  );
});

test("movie target stays on the resolved movie detail page", () => {
  assert.equal(
    parser.findAssistedNavigationURL(
      detailDocument([]),
      assistURL("/v/kKdRm", { target: "movie" }),
    ),
    "",
  );
});

test("ordinary unmarked JavDB pages do not use assisted navigation", () => {
  assert.equal(
    parser.findAssistedNavigationURL(
      movieSearchDocument([movieResult("IPX-228", "/v/kKdRm")]),
      "https://javdb.com/search?q=IPX-228&f=all",
    ),
    "",
  );
});

test("parse maps the JavDB detail fields used by the supplied sample", () => {
  const node = (textContent, attributes = {}, children = new Map()) => ({
    textContent,
    getAttribute: (name) => attributes[name] || null,
    querySelector: (selector) => children.get(selector)?.[0] || null,
    querySelectorAll: (selector) => children.get(selector) || [],
  });
  const block = (label, value, links = [], selectors = new Map()) => {
    const linkNodes = links.map((link) => {
      const linkNode = node(link.text, { href: link.href });
      if (link.gender || link.symbolText) {
        linkNode.nextElementSibling = node(link.symbolText || "", {
          class: `symbol ${link.gender || ""}`.trim(),
        });
      }
      return linkNode;
    });
    return node(
      "",
      {},
      new Map([
        ["strong", [node(label)]],
        [".value", [node(value)]],
        [".value a", linkNodes],
        [
          '.value a[href*="/tags/uncensored"]',
          linkNodes.filter((_, index) =>
            links[index].href.includes("/tags/uncensored"),
          ),
        ],
        ...selectors,
      ]),
    );
  };
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
      { text: "白花まなみ", href: "/actors/76Wdb", gender: "female" },
      { text: "藍井優太", href: "/actors/Ddd8", gender: "male" },
      { text: "松山伸也", href: "/actors/YnEYz", gender: "male" },
      { text: "純文字男優", href: "/actors/text-male", symbolText: "♂" },
    ]),
  ];
  const singles = new Map([
    [
      ".video-detail .current-title",
      node("リフレの面接に来た娘に手とり足とり実践指導"),
    ],
    [
      ".video-detail .origin-title",
      node("リフレの面接に来た娘に手取り足取り実践指導"),
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
    title: "リフレの面接に来た娘に手取り足取り実践指導",
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

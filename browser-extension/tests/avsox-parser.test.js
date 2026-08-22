const test = require("node:test");
const assert = require("node:assert/strict");

const parser = require("../content/avsox-parser.js");

test("cleanTitle removes the AVSOX code prefix and site suffix", () => {
  assert.equal(
    parser.cleanTitle("030919_047 Fixture AVSOX title - AVSOX", "030919_047"),
    "Fixture AVSOX title",
  );
});

test("parse supports the current AVSOX detail layout", () => {
  const node = (textContent, options = {}) => ({
    textContent,
    getAttribute: (name) => options.attributes?.[name] || null,
    querySelector: (selector) => options.singles?.get(selector) || null,
    querySelectorAll: (selector) => options.lists?.get(selector) || [],
  });
  const detailRow = (label, value, links = []) =>
    node("", {
      singles: new Map([
        [".detail-label", node(label)],
        [".detail-value", node(value)],
      ]),
      lists: new Map([[".detail-link", links.map((link) => node(link))]]),
    });
  const rows = [
    detailRow("識別碼:", "CW3D2DBD-10"),
    detailRow("發行時間:", "2012-02-24"),
    detailRow("長度:", "100分鐘"),
    detailRow("製作商:", "キャットウォーク"),
    detailRow("系列:", "3D キャットウォーク ポイズン (Blu-ray)"),
    detailRow("類別:", "", ["店長推薦作品", "美女", "素人"]),
  ];
  const singles = new Map([
    [
      ".movie-detail > h1",
      node(
        "CW3D2DBD-10 3D キャットウォーク ポイズン 10 - 3Dソープランドへようこそ -",
      ),
    ],
    [
      ".movie-detail .poster-image img",
      node("", {
        attributes: {
          src: "https://file.netcdn.space/storage/ave/archive/bigcover/dvd1cw3d2dbd-10.jpg",
        },
      }),
    ],
  ]);
  const lists = new Map([
    [".movie-detail .detail-row", rows],
    [".actresses .actress-name", [node("Kyoko Maki")]],
  ]);
  const document = {
    title: "AVSOX current fixture",
    querySelector: (selector) => singles.get(selector) || null,
    querySelectorAll: (selector) => lists.get(selector) || [],
  };

  assert.deepEqual(
    parser.parse(document, "https://avsox.click/tw/movies/example"),
    {
      code: "CW3D2DBD-10",
      title: "3D キャットウォーク ポイズン 10 - 3Dソープランドへようこそ -",
      studio: "キャットウォーク",
      series: "3D キャットウォーク ポイズン (Blu-ray)",
      release_date: "2012-02-24",
      duration_min: 100,
      tags: ["店長推薦作品", "美女", "素人"],
      actors: ["Kyoko Maki"],
      cover_url:
        "https://file.netcdn.space/storage/ave/archive/bigcover/dvd1cw3d2dbd-10.jpg",
      is_uncensored: true,
      source_name: "AVSOX",
      source_url: "https://avsox.click/tw/movies/example",
    },
  );
});

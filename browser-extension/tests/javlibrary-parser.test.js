const test = require("node:test");
const assert = require("node:assert/strict");

const parser = require("../content/javlibrary-parser.js");

test("cleanTitle removes the JavLibrary code prefix", () => {
  assert.equal(
    parser.cleanTitle("OFJE-282 Fixture JavLibrary title", "OFJE-282"),
    "Fixture JavLibrary title",
  );
});

test("cleanText collapses JavLibrary page whitespace", () => {
  assert.equal(parser.cleanText("  S1\n NO.1   STYLE "), "S1 NO.1 STYLE");
});

test("parse maps the JavLibrary detail fields used by the supplied sample", () => {
  const node = (textContent, attributes = {}) => ({
    textContent,
    getAttribute: (name) => attributes[name] || null,
  });
  const singles = new Map([
    ["#video_id .text", node("OFJE-282")],
    ["#video_title h3 a", node("OFJE-282 Fixture title")],
    ["#video_maker .text a", node("S1 NO.1 STYLE")],
    ["#video_date .text", node("2020-10-07")],
    ["#video_length .text", node("480")],
    [
      "#video_jacket_img",
      node("", {
        src: "https://pics.dmm.co.jp/mono/movie/adult/ofje282/ofje282pl.jpg",
      }),
    ],
  ]);
  const lists = new Map([
    ["#video_genres .genre a", [node("單體作品"), node("制服")]],
    ["#video_cast .cast .star a", [node("架乃ゆら")]],
  ]);
  const document = {
    querySelector: (selector) => singles.get(selector) || null,
    querySelectorAll: (selector) => lists.get(selector) || [],
  };

  assert.deepEqual(
    parser.parse(document, "https://www.javlibrary.com/tw/javmezbb74.html"),
    {
      code: "OFJE-282",
      title: "Fixture title",
      studio: "S1 NO.1 STYLE",
      series: "",
      release_date: "2020-10-07",
      duration_min: 480,
      tags: ["單體作品", "制服"],
      actors: ["架乃ゆら"],
      cover_url:
        "https://pics.dmm.co.jp/mono/movie/adult/ofje282/ofje282pl.jpg",
      is_uncensored: false,
      source_name: "JavLibrary",
      source_url: "https://www.javlibrary.com/tw/javmezbb74.html",
    },
  );
});

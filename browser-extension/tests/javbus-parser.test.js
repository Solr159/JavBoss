const test = require("node:test");
const assert = require("node:assert/strict");

const parser = require("../content/javbus-parser.js");

test("cleanTitle removes a JavBus suffix and leading code", () => {
  assert.equal(
    parser.cleanTitle("IPX-001  Test title - JavBus", "IPX-001"),
    "Test title",
  );
  assert.equal(parser.cleanTitle("IPX001 Test title", "IPX001"), "Test title");
  assert.equal(
    parser.cleanTitle("051526-001 Numeric code title - JavBus", "051526-001"),
    "Numeric code title",
  );
});

test("normalizeLabel handles full-width punctuation and whitespace", () => {
  assert.equal(parser.normalizeLabel(" 發行日期： "), "發行日期");
  assert.equal(parser.normalizeLabel("Studio:"), "studio");
});

test("cleanText collapses page whitespace", () => {
  assert.equal(parser.cleanText("  one\n\t two  "), "one two");
});

function detailField(label, value, plainText = false) {
  return {
    textContent: label,
    nextElementSibling: plainText ? null : { textContent: value },
    parentElement: { textContent: `${label} ${value}` },
  };
}

function detailDocument(fields) {
  return {
    querySelector: (selector) =>
      selector === "h3" ? { textContent: "ABC-001 Fixture title" } : null,
    querySelectorAll: (selector) =>
      selector === "span" ? [detailField("識別碼:", "ABC-001"), ...fields] : [],
  };
}

for (const label of ["發行商:", "发行商：", "レーベル:", "Label:"]) {
  for (const publisherFirst of [false, true]) {
    test(`parse uses ${label} as studio with publisher first=${publisherFirst}`, () => {
      const fields = [
        detailField("製作商:", "エスワン ナンバーワンスタイル"),
        detailField("制作商:", "Other Producer"),
        detailField("片商:", "Other Company"),
        detailField("メーカー:", "Other Maker"),
        detailField("Studio:", "Other Studio"),
        detailField("Maker:", "Another Maker"),
      ];
      const publisher = detailField(label, " S1 NO.1 STYLE ");
      if (publisherFirst) fields.unshift(publisher);
      else fields.push(publisher);

      const result = parser.parse(
        detailDocument(fields),
        "https://www.javbus.com/ABC-001",
      );
      assert.equal(result.studio, "S1 NO.1 STYLE");
    });
  }
}

test("parse supports a plain-text publisher", () => {
  const result = parser.parse(
    detailDocument([detailField("發行商:", "S1 NO.1 STYLE", true)]),
    "https://www.javbus.com/ABC-001",
  );
  assert.equal(result.studio, "S1 NO.1 STYLE");
});

for (const publisherFields of [[], [detailField("發行商:", "")]]) {
  test(`parse leaves studio empty when publisher is ${publisherFields.length ? "empty" : "missing"}`, () => {
    const result = parser.parse(
      detailDocument([
        detailField("製作商:", "Other Producer"),
        detailField("制作商:", "Other Producer"),
        detailField("片商:", "Other Company"),
        detailField("メーカー:", "Other Maker"),
        detailField("Studio:", "Other Studio"),
        detailField("Maker:", "Another Maker"),
        ...publisherFields,
      ]),
      "https://www.javbus.com/ABC-001",
    );
    assert.equal(result.studio, "");
  });
}

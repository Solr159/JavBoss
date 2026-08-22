const test = require("node:test");
const assert = require("node:assert/strict");

const parser = require("../content/javbus-parser.js");

test("cleanTitle removes a JavBus suffix and leading code", () => {
  assert.equal(parser.cleanTitle("IPX-001  Test title - JavBus"), "Test title");
  assert.equal(parser.cleanTitle("IPX001 Test title"), "Test title");
});

test("normalizeLabel handles full-width punctuation and whitespace", () => {
  assert.equal(parser.normalizeLabel(" 發行日期： "), "發行日期");
  assert.equal(parser.normalizeLabel("Studio:"), "studio");
});

test("cleanText collapses page whitespace", () => {
  assert.equal(parser.cleanText("  one\n\t two  "), "one two");
});

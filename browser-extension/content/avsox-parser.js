(function exposeAvsoxParser(root, factory) {
  const parser = factory();
  root.JavBossAvsoxParser = parser;
  if (typeof module === "object" && module.exports) module.exports = parser;
})(
  typeof globalThis !== "undefined" ? globalThis : this,
  function createAvsoxParser() {
    const DATE_PATTERN = /\d{4}-\d{2}-\d{2}/;
    const DURATION_PATTERN = /(\d{1,4})\s*(?:分鐘|分钟|分|分間|min)?/i;

    function cleanText(value) {
      return String(value || "")
        .replace(/\s+/g, " ")
        .trim();
    }

    function normalizeLabel(value) {
      return cleanText(value)
        .replace(/[:：]\s*$/, "")
        .toLowerCase();
    }

    function cleanTitle(value, code) {
      let title = cleanText(value).replace(/\s*-\s*avsox\s*$/i, "");
      const normalizedCode = cleanText(code);
      if (
        normalizedCode &&
        title.toUpperCase().startsWith(normalizedCode.toUpperCase())
      ) {
        title = title.slice(normalizedCode.length).trim();
      }
      return title;
    }

    function uniqueTexts(elements) {
      const seen = new Set();
      const values = [];
      for (const element of elements) {
        const value = cleanText(element.textContent);
        if (!value || seen.has(value)) continue;
        seen.add(value);
        values.push(value);
      }
      return values;
    }

    function fieldRow(document, labels) {
      const expected = new Set(labels.map(normalizeLabel));
      return [...document.querySelectorAll(".movie-detail .detail-row")].find(
        (row) =>
          expected.has(
            normalizeLabel(row.querySelector(".detail-label")?.textContent),
          ),
      );
    }

    function fieldValue(document, labels) {
      const value = cleanText(
        fieldRow(document, labels)?.querySelector(".detail-value")?.textContent,
      );
      return value === "-" ? "" : value;
    }

    function resolvedURL(document, pageURL, selector, attribute) {
      const value = document.querySelector(selector)?.getAttribute(attribute);
      if (!value) return "";
      try {
        const resolved = new URL(value, pageURL);
        return resolved.protocol === "http:" || resolved.protocol === "https:"
          ? resolved.href
          : "";
      } catch {
        return "";
      }
    }

    function parse(document, pageURL) {
      const code = fieldValue(document, [
        "識別碼",
        "识别码",
        "番號",
        "番号",
        "id",
      ]).toUpperCase();
      const rawTitle = cleanText(
        document.querySelector(".movie-detail > h1")?.textContent ||
          document.title,
      );
      const title = cleanTitle(rawTitle, code);
      if (!code || !title) return null;

      const dateText = fieldValue(document, [
        "發行日期",
        "发行日期",
        "發行時間",
        "发行时间",
        "発売日",
        "release date",
      ]);
      const duration = Number.parseInt(
        fieldValue(document, [
          "長度",
          "长度",
          "時長",
          "时长",
          "収録時間",
          "duration",
          "runtime",
        ]).match(DURATION_PATTERN)?.[1] || "",
        10,
      );

      const tags = uniqueTexts(
        fieldRow(document, [
          "類別",
          "类别",
          "ジャンル",
          "genre",
        ])?.querySelectorAll(".detail-link") || [],
      );
      const actors = uniqueTexts(
        document.querySelectorAll(".actresses .actress-name"),
      );

      return {
        code,
        title,
        studio: fieldValue(document, [
          "製作商",
          "制作商",
          "片商",
          "studio",
          "メーカー",
        ]),
        series: fieldValue(document, ["系列", "series", "シリーズ"]),
        release_date: dateText.match(DATE_PATTERN)?.[0] || "",
        duration_min:
          Number.isFinite(duration) && duration >= 0 ? duration : null,
        tags,
        actors,
        cover_url: resolvedURL(
          document,
          pageURL,
          ".movie-detail .poster-image img",
          "src",
        ),
        is_uncensored: true,
        source_name: "AVSOX",
        source_url: String(pageURL || ""),
      };
    }

    return { cleanText, normalizeLabel, cleanTitle, parse };
  },
);

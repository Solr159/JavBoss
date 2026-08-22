(function exposeJavDBParser(root, factory) {
  const parser = factory();
  root.JavBossJavDBParser = parser;
  if (typeof module === "object" && module.exports) module.exports = parser;
})(
  typeof globalThis !== "undefined" ? globalThis : this,
  function createJavDBParser() {
    function cleanText(value) {
      return String(value || "")
        .replace(/\s+/g, " ")
        .trim();
    }

    function normalizeLabel(value) {
      return cleanText(value).replace(/[：:]/g, "").toLowerCase();
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

    function fieldBlock(document, labels) {
      const expected = new Set(labels.map(normalizeLabel));
      return [
        ...document.querySelectorAll(".movie-panel-info .panel-block"),
      ].find((block) =>
        expected.has(
          normalizeLabel(block.querySelector("strong")?.textContent),
        ),
      );
    }

    function fieldText(document, labels) {
      return cleanText(
        fieldBlock(document, labels)?.querySelector(".value")?.textContent,
      );
    }

    function fieldLinks(document, labels, hrefPrefix = "") {
      const block = fieldBlock(document, labels);
      if (!block) return [];
      const selector = hrefPrefix
        ? `.value a[href^="${hrefPrefix}"]`
        : ".value a";
      return uniqueTexts(block.querySelectorAll(selector));
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
      const code = fieldText(document, ["番號", "番号"]).toUpperCase();
      const title = cleanText(
        document.querySelector(".video-detail .current-title")?.textContent,
      );
      if (!code || !title) return null;

      const dateText = fieldText(document, ["日期", "発売日"]);
      const duration = Number.parseInt(
        fieldText(document, ["時長", "时长", "収録時間"]),
        10,
      );
      const tags = fieldLinks(document, ["類別", "类别", "ジャンル"], "/tags");
      const headingMarker = cleanText(
        document.querySelector(".video-detail > h2 strong:not(.current-title)")
          ?.textContent,
      );
      const tagBlock = fieldBlock(document, ["類別", "类别", "ジャンル"]);

      return {
        code,
        title,
        studio: fieldLinks(document, ["片商", "メーカー", "Maker"])[0] || "",
        series: fieldLinks(document, ["系列", "シリーズ", "Series"])[0] || "",
        release_date:
          dateText
            .match(/\d{4}[-/]\d{2}[-/]\d{2}/)?.[0]
            ?.replaceAll("/", "-") || "",
        duration_min:
          Number.isFinite(duration) && duration >= 0 ? duration : null,
        tags,
        actors: fieldLinks(document, ["演員", "演员", "出演者"], "/actors/"),
        cover_url:
          resolvedURL(document, pageURL, ".video-cover", "src") ||
          resolvedURL(document, pageURL, ".column-video-cover a", "href"),
        is_uncensored:
          /(?:無碼|无码|uncensored|無修正)/i.test(headingMarker) ||
          Boolean(
            tagBlock?.querySelector('.value a[href*="/tags/uncensored"]'),
          ),
        source_name: "JavDB",
        source_url: String(pageURL || ""),
      };
    }

    return { cleanText, normalizeLabel, parse };
  },
);

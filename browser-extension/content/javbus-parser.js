(function exposeJavBusParser(root, factory) {
  const parser = factory();
  root.JavBossJavBusParser = parser;
  if (typeof module === "object" && module.exports) module.exports = parser;
})(
  typeof globalThis !== "undefined" ? globalThis : this,
  function createJavBusParser() {
    const DATE_PATTERN = /\d{4}-\d{2}-\d{2}/;
    const DURATION_PATTERN = /(\d{1,4})\s*(?:分鐘|分钟|分|分間|min)?/i;

    function cleanText(value) {
      return String(value || "")
        .replace(/\s+/g, " ")
        .trim();
    }

    function normalizeLabel(value) {
      return cleanText(value)
        .toLowerCase()
        .replace(/[:：]\s*$/, "")
        .trim();
    }

    function cleanTitle(value, code) {
      let title = cleanText(value).replace(/\s*-\s*javbus\s*$/i, "");
      const normalizedCode = cleanText(code);
      if (
        normalizedCode &&
        title.toUpperCase().startsWith(normalizedCode.toUpperCase())
      ) {
        title = title.slice(normalizedCode.length).trim();
      }
      return title;
    }

    function fieldValue(document, labels) {
      const wanted = labels.map(normalizeLabel);
      for (const label of document.querySelectorAll("span")) {
        const labelText = normalizeLabel(label.textContent);
        if (
          !wanted.some(
            (candidate) =>
              labelText === candidate ||
              (candidate.length > 2 && labelText.includes(candidate)),
          )
        ) {
          continue;
        }
        const siblingValue = cleanText(label.nextElementSibling?.textContent);
        if (siblingValue) return siblingValue;
        const line = cleanText(label.parentElement?.textContent);
        const ownText = cleanText(label.textContent);
        const value = cleanText(
          line.slice(line.indexOf(ownText) + ownText.length),
        );
        if (value) return value;
      }
      return "";
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

    function firstResolvedURL(document, pageURL, selectors) {
      for (const [selector, attribute] of selectors) {
        const value = document.querySelector(selector)?.getAttribute(attribute);
        if (!value) continue;
        try {
          const resolved = new URL(value, pageURL);
          if (resolved.protocol === "http:" || resolved.protocol === "https:")
            return resolved.href;
        } catch {
          // Try the next candidate.
        }
      }
      return "";
    }

    function parseDate(document) {
      const value = fieldValue(document, [
        "發行日期",
        "发行日期",
        "発売日",
        "release date",
        "release",
      ]);
      return value.match(DATE_PATTERN)?.[0] || "";
    }

    function parseDuration(document) {
      const value = fieldValue(document, [
        "長度",
        "长度",
        "時長",
        "时长",
        "時間",
        "length",
        "duration",
      ]);
      const parsed = Number.parseInt(
        value.match(DURATION_PATTERN)?.[1] || "",
        10,
      );
      return Number.isFinite(parsed) && parsed >= 0 ? parsed : null;
    }

    function parse(document, pageURL) {
      const movieSection = document.querySelector("div.movie.row") || document;
      const code = fieldValue(document, [
        "識別碼",
        "识别码",
        "番號",
        "番号",
        "id",
      ]).toUpperCase();
      const rawTitle = cleanText(
        document.querySelector("h3")?.textContent || document.title,
      );
      const title = cleanTitle(rawTitle, code);
      const tags = uniqueTexts(
        [...movieSection.querySelectorAll("span.genre a")].filter(
          (link) => !String(link.getAttribute("href") || "").includes("/star/"),
        ),
      );
      const actors = uniqueTexts(
        movieSection.querySelectorAll('a[href*="/star/"]'),
      );
      if (!code || (!title && tags.length === 0 && actors.length === 0))
        return null;

      const activeSection = cleanText(
        document.querySelector("li.active a")?.textContent,
      ).toLowerCase();
      const activeHref = String(
        document.querySelector("li.active a")?.getAttribute("href") || "",
      )
        .toLowerCase()
        .trim();
      return {
        code,
        title,
        studio: fieldValue(document, [
          "製作商",
          "制作商",
          "片商",
          "メーカー",
          "studio",
          "maker",
        ]),
        series: fieldValue(document, ["系列", "シリーズ", "series"]),
        release_date: parseDate(document),
        duration_min: parseDuration(document),
        tags,
        actors,
        cover_url: firstResolvedURL(document, pageURL, [
          ['meta[property="og:image"]', "content"],
          ["a.bigImage", "href"],
          ["img.cover", "src"],
          [".bigImage img", "src"],
        ]),
        is_uncensored:
          activeHref.includes("/uncensored") ||
          activeSection.includes("無碼") ||
          activeSection.includes("无码") ||
          activeSection.includes("uncensored"),
        source_name: "JavBus",
        source_url: String(pageURL || ""),
      };
    }

    return { cleanText, normalizeLabel, cleanTitle, parse };
  },
);

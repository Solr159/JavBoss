(function exposeJavLibraryParser(root, factory) {
  const parser = factory();
  root.JavBossJavLibraryParser = parser;
  if (typeof module === "object" && module.exports) module.exports = parser;
})(
  typeof globalThis !== "undefined" ? globalThis : this,
  function createJavLibraryParser() {
    function cleanText(value) {
      return String(value || "")
        .replace(/\s+/g, " ")
        .trim();
    }

    function cleanTitle(value, code) {
      const title = cleanText(value);
      const normalizedCode = cleanText(code);
      if (!normalizedCode) return title;
      return title.toUpperCase().startsWith(normalizedCode.toUpperCase())
        ? title.slice(normalizedCode.length).trim()
        : title;
    }

    function text(document, selector) {
      return cleanText(document.querySelector(selector)?.textContent);
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
      const code = text(document, "#video_id .text").toUpperCase();
      const rawTitle = text(document, "#video_title h3 a");
      if (!code || !rawTitle) return null;

      const tags = uniqueTexts(
        document.querySelectorAll("#video_genres .genre a"),
      );
      const actors = uniqueTexts(
        document.querySelectorAll("#video_cast .cast .star a"),
      );
      const rawDuration = Number.parseInt(
        text(document, "#video_length .text"),
        10,
      );
      const result = {
        code,
        title: cleanTitle(rawTitle, code),
        studio:
          text(document, "#video_maker .text a") ||
          text(document, "#video_label .text a"),
        series: "",
        release_date:
          text(document, "#video_date .text").match(/\d{4}-\d{2}-\d{2}/)?.[0] ||
          "",
        duration_min:
          Number.isFinite(rawDuration) && rawDuration >= 0 ? rawDuration : null,
        tags,
        actors,
        cover_url: resolvedURL(document, pageURL, "#video_jacket_img", "src"),
        is_uncensored: tags.some((tag) =>
          /(?:無碼|无码|uncensored|無修正|无码流出)/i.test(tag),
        ),
        source_name: "JavLibrary",
        source_url: String(pageURL || ""),
      };
      return result;
    }

    return { cleanText, cleanTitle, parse };
  },
);

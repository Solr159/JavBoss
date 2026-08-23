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

    function normalizeCode(value) {
      return cleanText(value).replace(/[^a-z0-9]/gi, "").toUpperCase();
    }

    function assistRequest(pageURL) {
      let url;
      try {
        url = new URL(String(pageURL || ""));
      } catch {
        return null;
      }

      const params = new URLSearchParams(url.hash.replace(/^#/, ""));
      const target = cleanText(params.get("target"));
      const code = cleanText(params.get("code"));
      if (
        params.get("javboss") !== "direct" ||
        !["movie", "idol", "series", "studio"].includes(target) ||
        !code
      ) {
        return null;
      }
      return { code, name: cleanText(params.get("name")), target, url };
    }

    function isAssistedNavigationURL(pageURL) {
      return Boolean(assistRequest(pageURL));
    }

    function assistHash(request) {
      const params = new URLSearchParams();
      params.set("javboss", "direct");
      params.set("target", request.target);
      params.set("code", request.code);
      if (request.name) params.set("name", request.name);
      return params.toString();
    }

    function assistedCodeSearchURL(request) {
      const searchURL = new URL("/search", request.url.origin);
      searchURL.searchParams.set("q", request.code);
      searchURL.searchParams.set("f", "all");
      searchURL.hash = assistHash(request);
      return searchURL.href;
    }

    function exactMovieResultURL(document, request) {
      const wantCode = normalizeCode(request.code);
      if (!wantCode) return "";

      const matches = new Set();
      for (const item of document.querySelectorAll(".movie-list .item")) {
        const code = normalizeCode(
          item.querySelector(".video-title strong")?.textContent,
        );
        if (code !== wantCode) continue;

        const href = item.querySelector("a[href]")?.getAttribute("href");
        if (!href) continue;
        try {
          const resolved = new URL(href, request.url);
          if (
            resolved.origin === request.url.origin &&
            /^\/v\/[^/]+\/?$/.test(resolved.pathname)
          ) {
            resolved.hash = assistHash(request);
            matches.add(resolved.href);
          }
        } catch {
          // Ignore malformed result links and leave the search page visible.
        }
      }
      return matches.size === 1 ? [...matches][0] : "";
    }

    function detailTargetConfig(target) {
      switch (target) {
        case "idol":
          return {
            labels: ["演員", "演员", "出演者"],
            path: /^\/actors\/[^/]+\/?$/,
          };
        case "series":
          return {
            labels: ["系列", "シリーズ", "Series"],
            path: /^\/series\/[^/]+\/?$/,
          };
        case "studio":
          return {
            labels: ["片商", "メーカー", "Maker"],
            path: /^\/makers\/[^/]+\/?$/,
          };
        default:
          return null;
      }
    }

    function detailTargetURL(document, request) {
      const config = detailTargetConfig(request.target);
      const block = config ? fieldBlock(document, config.labels) : null;
      if (!block) return "";

      const candidates = [];
      const exact = [];
      const seen = new Set();
      for (const link of block.querySelectorAll(".value a[href]")) {
        const name = cleanText(link.textContent);
        const href = link.getAttribute("href");
        if (!href) continue;
        try {
          const resolved = new URL(href, request.url);
          if (
            resolved.origin !== request.url.origin ||
            !config.path.test(resolved.pathname)
          ) {
            continue;
          }
          resolved.search = "";
          resolved.hash = "";
          if (seen.has(resolved.href)) continue;
          seen.add(resolved.href);
          candidates.push(resolved.href);
          if (request.name && name === request.name) exact.push(resolved.href);
        } catch {
          // Ignore malformed detail links and keep the movie page visible.
        }
      }

      if (exact.length === 1) return exact[0];
      return candidates.length === 1 ? candidates[0] : "";
    }

    function findAssistedNavigationURL(document, pageURL) {
      const request = assistRequest(pageURL);
      if (!request) return "";

      if (request.url.pathname === "/search") {
        const currentCode = normalizeCode(request.url.searchParams.get("q"));
        const currentType = request.url.searchParams.get("f") || "all";
        if (
          currentType !== "all" ||
          currentCode !== normalizeCode(request.code)
        ) {
          return assistedCodeSearchURL(request);
        }
        return exactMovieResultURL(document, request);
      }

      if (/^\/v\/[^/]+\/?$/.test(request.url.pathname)) {
        return request.target === "movie"
          ? ""
          : detailTargetURL(document, request);
      }
      return "";
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

    return {
      cleanText,
      findAssistedNavigationURL,
      isAssistedNavigationURL,
      normalizeCode,
      normalizeLabel,
      parse,
    };
  },
);

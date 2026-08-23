(() => {
  const provider = globalThis.JavBossAvsoxParser
    ? { name: "AVSOX", parser: globalThis.JavBossAvsoxParser }
    : globalThis.JavBossJavDBParser
      ? { name: "JavDB", parser: globalThis.JavBossJavDBParser }
      : globalThis.JavBossJavLibraryParser
        ? { name: "JavLibrary", parser: globalThis.JavBossJavLibraryParser }
        : { name: "JavBus", parser: globalThis.JavBossJavBusParser };
  if (!provider.parser) return;
  const parser = provider.parser;

  if (
    provider.name === "JavDB" &&
    window.top === window &&
    parser.isAssistedNavigationURL?.(location.href)
  ) {
    const blankStyle = document.createElement("style");
    blankStyle.textContent =
      "html { visibility: hidden !important; background: #fff !important; }";
    blankStyle.setAttribute("data-javboss-assisted-navigation", "");
    document.documentElement.appendChild(blankStyle);

    let revealTimer = window.setTimeout(() => blankStyle.remove(), 15000);
    const revealPage = () => {
      window.clearTimeout(revealTimer);
      revealTimer = 0;
      blankStyle.remove();
    };
    const navigateOrReveal = () => {
      const directURL = parser.findAssistedNavigationURL?.(
        document,
        location.href,
      );
      if (directURL && directURL !== location.href) {
        location.replace(directURL);
        return true;
      }
      if (document.readyState === "complete") revealPage();
      else window.addEventListener("load", revealPage, { once: true });
      return false;
    };

    if (document.readyState === "loading") {
      document.addEventListener("DOMContentLoaded", navigateOrReveal, {
        once: true,
      });
    } else if (navigateOrReveal()) {
      return;
    }
  }

  const BUTTON_ID = "javboss-browser-scrape-fill-button";
  const CONTROLS_ID = "javboss-browser-scrape-controls";
  const SESSION_STORAGE_KEY = "javboss:browser-scrape-session";
  let refreshTimer = 0;
  let relayEnabled = false;

  function storedRelaySessionID() {
    try {
      return window.sessionStorage.getItem(SESSION_STORAGE_KEY) || "";
    } catch {
      return "";
    }
  }

  function storeRelaySessionID(sessionId) {
    try {
      if (sessionId) {
        window.sessionStorage.setItem(SESSION_STORAGE_KEY, sessionId);
      } else {
        window.sessionStorage.removeItem(SESSION_STORAGE_KEY);
      }
    } catch {
      // The tab mapping still works when sessionStorage is unavailable.
    }
  }

  function disableRelayUI() {
    relayEnabled = false;
    storeRelaySessionID("");
    document.getElementById(CONTROLS_ID)?.remove();
  }

  async function submitMetadata() {
    const payload = parser.parse(document, location.href);
    if (!payload) {
      window.alert(
        `当前页面没有识别到 ${provider.name} 作品信息，请先打开作品详情页。`,
      );
      return;
    }

    const response = await chrome.runtime.sendMessage({
      type: "JAVBOSS_JAVBUS_SUBMIT_RELAY",
      payload,
    });
    if (!response?.ok) {
      window.alert(
        `无法回填到 JavBoss：${response?.error || "未找到手动刮削窗口"}`,
      );
      return;
    }
    const button = document.getElementById(BUTTON_ID);
    if (button) {
      button.textContent = "已回填，可继续浏览";
      window.setTimeout(() => {
        if (relayEnabled && button.isConnected) {
          button.textContent = "回填到 JavBoss";
        }
      }, 1500);
    }
  }

  function ensureFillButton() {
    const metadata = parser.parse(document, location.href);
    const existing = document.getElementById(CONTROLS_ID);
    if (!relayEnabled || !metadata) {
      existing?.remove();
      return;
    }
    if (existing) return;

    const controls = document.createElement("div");
    controls.id = CONTROLS_ID;
    Object.assign(controls.style, {
      position: "fixed",
      right: "20px",
      bottom: "20px",
      zIndex: "2147483647",
      display: "flex",
      gap: "6px",
      alignItems: "center",
    });

    const button = document.createElement("button");
    button.id = BUTTON_ID;
    button.type = "button";
    button.textContent = "回填到 JavBoss";
    button.setAttribute(
      "aria-label",
      `提取当前 ${provider.name} 作品信息并回填到 JavBoss`,
    );
    Object.assign(button.style, {
      border: "1px solid #1d4ed8",
      borderRadius: "8px",
      padding: "10px 16px",
      background: "#2563eb",
      color: "#fff",
      font: "600 14px/1.2 system-ui, sans-serif",
      boxShadow: "0 8px 24px rgba(15, 23, 42, .3)",
      cursor: "pointer",
    });
    button.addEventListener("click", () => void submitMetadata());

    controls.appendChild(button);
    document.documentElement.appendChild(controls);
  }

  chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
    if (message?.type !== "JAVBOSS_JAVBUS_DISABLE_RELAY") return false;
    disableRelayUI();
    sendResponse({ ok: true });
    return false;
  });

  chrome.runtime
    .sendMessage({
      type: "JAVBOSS_JAVBUS_IS_RELAY",
      sessionId: storedRelaySessionID(),
    })
    .then((response) => {
      if (!response?.relay || !response.sessionId) {
        storeRelaySessionID("");
        return;
      }
      relayEnabled = true;
      storeRelaySessionID(response.sessionId);
      ensureFillButton();
      const observer = new MutationObserver(() => {
        window.clearTimeout(refreshTimer);
        refreshTimer = window.setTimeout(ensureFillButton, 150);
      });
      observer.observe(document.documentElement, {
        childList: true,
        subtree: true,
      });
    });
})();

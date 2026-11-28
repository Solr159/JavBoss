(() => {
  const BRIDGE_VERSION = 1;
  const SESSION_PATTERN = /^[a-zA-Z0-9_-]{8,128}$/;
  let parentOrigin = "";
  let sessionId = "";

  function validSessionID(value) {
    const candidate = String(value || "");
    return SESSION_PATTERN.test(candidate) ? candidate : "";
  }

  function isHttpOrigin(value) {
    try {
      const parsed = new URL(String(value || ""));
      return parsed.protocol === "http:" || parsed.protocol === "https:";
    } catch {
      return false;
    }
  }

  function postToParent(message) {
    if (!parentOrigin || window.parent === window) return;
    window.parent.postMessage(
      { ...message, version: BRIDGE_VERSION, sessionId },
      parentOrigin,
    );
  }

  window.addEventListener("message", (event) => {
    if (event.source !== window.parent) return;
    const message = event.data;
    if (message?.type === "JAVBOSS_EXTENSION_CONNECT") {
      const requestedSessionId = validSessionID(message.sessionId);
      if (!requestedSessionId || !isHttpOrigin(event.origin)) return;
      parentOrigin = event.origin;
      sessionId = requestedSessionId;
      postToParent({ type: "JAVBOSS_EXTENSION_READY" });
      return;
    }
    const openMessageTypes = new Set([
      "JAVBOSS_SCRAPE_OPEN",
      "JAVBOSS_JAVDB_OPEN",
    ]);
    if (
      !parentOrigin ||
      event.origin !== parentOrigin ||
      !openMessageTypes.has(message?.type) ||
      validSessionID(message.sessionId) !== sessionId
    ) {
      return;
    }

    const runtimeMessage =
      message.type === "JAVBOSS_JAVDB_OPEN"
        ? {
            type: "JAVBOSS_JAVDB_OPEN_ASSIST",
            sessionId,
            url: String(message.url || ""),
            fallbackUrl: String(message.fallbackUrl || ""),
            request: message.request,
          }
        : {
            type: "JAVBOSS_SCRAPE_OPEN_RELAY",
            sessionId,
            url: String(message.url || ""),
          };
    const statusType =
      message.type === "JAVBOSS_JAVDB_OPEN"
        ? "JAVBOSS_JAVDB_OPEN_STATUS"
        : "JAVBOSS_SCRAPE_OPEN_STATUS";
    chrome.runtime
      .sendMessage(runtimeMessage)
      .then((response) => {
        postToParent({
          type: statusType,
          ok: Boolean(response?.ok),
          error: String(response?.error || ""),
        });
      })
      .catch((error) => {
        postToParent({
          type: statusType,
          ok: false,
          error: String(error?.message || error || "extension error"),
        });
      });
  });

  chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
    if (
      message?.type !== "JAVBOSS_SCRAPE_BRIDGE_METADATA" ||
      validSessionID(message.sessionId) !== sessionId
    ) {
      return;
    }
    postToParent({
      type: "JAVBOSS_SCRAPE_METADATA",
      payload: message.payload,
    });
    sendResponse({ ok: true });
    return false;
  });
})();

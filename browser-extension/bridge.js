(() => {
  const BRIDGE_VERSION = 1;
  const SESSION_PATTERN = /^[a-zA-Z0-9_-]{8,128}$/;
  let parentOrigin = "";
  let sessionId = "";

  function validSessionID(value) {
    const candidate = String(value || "");
    return SESSION_PATTERN.test(candidate) ? candidate : "";
  }

  function trustedJavBossOrigin(value) {
    let parsed;
    try {
      parsed = new URL(String(value || ""));
    } catch {
      return false;
    }
    if (parsed.protocol !== "http:" && parsed.protocol !== "https:")
      return false;

    const host = parsed.hostname.toLowerCase();
    if (
      host === "localhost" ||
      host.endsWith(".localhost") ||
      host === "127.0.0.1" ||
      host === "[::1]" ||
      host.endsWith(".local") ||
      host === "host.docker.internal" ||
      !host.includes(".") ||
      host.startsWith("10.") ||
      host.startsWith("192.168.")
    ) {
      return true;
    }
    const octets = host.split(".").map((part) => Number.parseInt(part, 10));
    return (
      octets.length === 4 &&
      octets.every(
        (part) => Number.isInteger(part) && part >= 0 && part <= 255,
      ) &&
      ((octets[0] === 172 && octets[1] >= 16 && octets[1] <= 31) ||
        (octets[0] === 100 && octets[1] >= 64 && octets[1] <= 127))
    );
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
      if (!requestedSessionId || !trustedJavBossOrigin(event.origin)) return;
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

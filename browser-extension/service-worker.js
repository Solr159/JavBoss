const SCRAPE_ORIGINS = new Set([
  "https://www.javbus.com",
  "https://www.javlibrary.com",
  "https://javdb.com",
  "https://avsox.click",
]);
const RELAY_KEY_PREFIX = "javboss:browser-relay:";
const RELAY_SESSION_KEY_PREFIX = "javboss:browser-session:";
const LEGACY_RELAY_KEY_PREFIX = "javboss:javbus-relay:";
const LEGACY_RELAY_SESSION_KEY_PREFIX = "javboss:javbus-session:";

function relayKey(tabId) {
  return `${RELAY_KEY_PREFIX}${tabId}`;
}

function relaySessionKey(sessionId) {
  return `${RELAY_SESSION_KEY_PREFIX}${sessionId}`;
}

function validScrapeURL(value) {
  try {
    const parsed = new URL(String(value || ""));
    return SCRAPE_ORIGINS.has(parsed.origin) ? parsed.href : "";
  } catch {
    return "";
  }
}

function validSessionID(value) {
  const sessionId = String(value || "");
  return /^[a-zA-Z0-9_-]{8,128}$/.test(sessionId) ? sessionId : "";
}

async function openRelayTab(message, sender) {
  const url = validScrapeURL(message?.url);
  const sessionId = validSessionID(message?.sessionId);
  let senderURL;
  try {
    senderURL = new URL(String(sender.url || ""));
  } catch {
    senderURL = null;
  }
  if (
    !url ||
    !sessionId ||
    senderURL?.protocol !== "chrome-extension:" ||
    senderURL.pathname !== "/bridge.html"
  ) {
    return { ok: false, error: "invalid relay request" };
  }

  const createProperties = {
    url: "about:blank",
    active: true,
  };
  if (Number.isInteger(sender.tab?.windowId)) {
    createProperties.windowId = sender.tab.windowId;
  }
  const relayTab = await chrome.tabs.create(createProperties);
  const relayTabId = relayTab?.id;
  if (!Number.isInteger(relayTabId)) {
    return { ok: false, error: "failed to create metadata tab" };
  }

  await chrome.storage.session.set({
    [relayKey(relayTabId)]: {
      sessionId,
    },
    [relaySessionKey(sessionId)]: {
      sessionId,
    },
  });
  await chrome.tabs.update(relayTabId, { url });
  return { ok: true };
}

async function isRelayTab(message, sender) {
  const tabId = sender.tab?.id;
  if (!Number.isInteger(tabId)) return { ok: true, relay: false };
  const key = relayKey(tabId);
  const stored = await chrome.storage.session.get(key);
  const currentSessionId = validSessionID(stored[key]?.sessionId);
  if (currentSessionId) {
    const active = await chrome.storage.session.get(
      relaySessionKey(currentSessionId),
    );
    if (active[relaySessionKey(currentSessionId)]) {
      return { ok: true, relay: true, sessionId: currentSessionId };
    }
    await chrome.storage.session.remove(key);
  }

  const inheritedSessionId = validSessionID(message?.sessionId);
  if (!inheritedSessionId) return { ok: true, relay: false };
  const active = await chrome.storage.session.get(
    relaySessionKey(inheritedSessionId),
  );
  if (!active[relaySessionKey(inheritedSessionId)]) {
    return { ok: true, relay: false };
  }
  await chrome.storage.session.set({
    [key]: { sessionId: inheritedSessionId },
  });
  return { ok: true, relay: true, sessionId: inheritedSessionId };
}

async function invalidateRelaySession(sessionId) {
  const validID = validSessionID(sessionId);
  if (!validID) return;
  const stored = await chrome.storage.session.get(null);
  const keys = [relaySessionKey(validID)];
  const tabIds = [];
  for (const [key, value] of Object.entries(stored)) {
    if (!key.startsWith(RELAY_KEY_PREFIX) || value?.sessionId !== validID) {
      continue;
    }
    keys.push(key);
    const tabId = Number.parseInt(key.slice(RELAY_KEY_PREFIX.length), 10);
    if (Number.isInteger(tabId)) tabIds.push(tabId);
  }
  await chrome.storage.session.remove(keys);
  await Promise.all(
    tabIds.map((tabId) =>
      chrome.tabs
        .sendMessage(
          tabId,
          { type: "JAVBOSS_JAVBUS_DISABLE_RELAY" },
          { frameId: 0 },
        )
        .catch(() => {}),
    ),
  );
}

async function relayMetadata(message, sender) {
  const relayTabId = sender.tab?.id;
  if (!Number.isInteger(relayTabId))
    return { ok: false, error: "missing sender tab" };

  const key = relayKey(relayTabId);
  const stored = await chrome.storage.session.get(key);
  const relay = stored[key];
  if (!relay)
    return { ok: false, error: "this page was not opened from JavBoss" };
  const active = await chrome.storage.session.get(
    relaySessionKey(relay.sessionId),
  );
  if (!active[relaySessionKey(relay.sessionId)]) {
    await chrome.storage.session.remove(key);
    return { ok: false, error: "the JavBoss scrape session has expired" };
  }

  try {
    const response = await chrome.runtime.sendMessage({
      type: "JAVBOSS_JAVBUS_BRIDGE_METADATA",
      sessionId: relay.sessionId,
      payload: message?.payload,
    });
    if (!response?.ok)
      throw new Error("matching extension bridge was not found");
  } catch {
    await invalidateRelaySession(relay.sessionId);
    return {
      ok: false,
      error: "the JavBoss extension bridge is no longer open",
    };
  }
  return { ok: true };
}

chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  let operation;
  if (message?.type === "JAVBOSS_JAVBUS_OPEN_RELAY") {
    operation = openRelayTab(message, sender);
  } else if (message?.type === "JAVBOSS_JAVBUS_IS_RELAY") {
    operation = isRelayTab(message, sender);
  } else if (message?.type === "JAVBOSS_JAVBUS_SUBMIT_RELAY") {
    operation = relayMetadata(message, sender);
  } else {
    return false;
  }

  operation.then(sendResponse).catch((error) => {
    sendResponse({
      ok: false,
      error: String(error?.message || error || "extension error"),
    });
  });
  return true;
});

chrome.tabs.onRemoved.addListener((tabId) => {
  chrome.storage.session.remove(relayKey(tabId)).catch(() => {});
});

chrome.runtime.onInstalled.addListener(() => {
  chrome.storage.session
    .get(null)
    .then((stored) =>
      chrome.storage.session.remove(
        Object.keys(stored).filter(
          (key) =>
            key.startsWith(RELAY_KEY_PREFIX) ||
            key.startsWith(RELAY_SESSION_KEY_PREFIX) ||
            key.startsWith(LEGACY_RELAY_KEY_PREFIX) ||
            key.startsWith(LEGACY_RELAY_SESSION_KEY_PREFIX),
        ),
      ),
    )
    .catch(() => {});
});

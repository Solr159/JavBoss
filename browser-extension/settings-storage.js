// Loaded only by extension pages and the service worker, never by content scripts.
globalThis.JavBossSettings = (() => {
  const CONNECTION_KEY = "javboss:connection-settings";
  const CONNECTION_CHANGED_KEY = "javboss:connection-changed";
  const STORE = "settings";
  const STORAGE_ERROR = "无法访问扩展凭据存储，请重新加载扩展或更换浏览器";
  let database;

  function connectionValue(value) {
    return {
      serverUrl: String(value?.serverUrl || ""),
      apiToken: String(value?.apiToken || ""),
    };
  }

  function openDatabase() {
    return new Promise((resolve, reject) => {
      const request = indexedDB.open("javboss-private-settings", 1);
      let blocked = false;
      request.onupgradeneeded = () => request.result.createObjectStore(STORE);
      request.onerror = () => reject(new Error(STORAGE_ERROR));
      request.onblocked = () => {
        blocked = true;
        reject(new Error(STORAGE_ERROR));
      };
      request.onsuccess = () => {
        const db = request.result;
        if (blocked) {
          db.close();
          return;
        }
        db.onversionchange = () => db.close();
        resolve(db);
      };
    });
  }

  // A successful request can still be rolled back. Report success only on commit.
  function transact(mode, operation) {
    return new Promise((resolve, reject) => {
      const transaction = database.transaction(STORE, mode);
      const request = operation(transaction.objectStore(STORE));
      transaction.oncomplete = () => resolve(request.result);
      transaction.onabort = transaction.onerror = () =>
        reject(new Error(STORAGE_ERROR));
    });
  }

  async function initialize() {
    // Best effort for legacy data and preferences. Credentials no longer depend
    // on this optional extension API for isolation from content scripts.
    try {
      if (typeof chrome.storage.local.setAccessLevel === "function") {
        await chrome.storage.local.setAccessLevel({
          accessLevel: "TRUSTED_CONTEXTS",
        });
      }
    } catch {
      // Some Chromium derivatives omit, throw from, or reject this API.
    }

    database = await openDatabase();
    const legacy = (await chrome.storage.local.get(CONNECTION_KEY))[
      CONNECTION_KEY
    ];
    if (legacy !== undefined) {
      // Popup and worker can start together. The read/write transaction ensures
      // a second migration cannot overwrite an existing or newly edited pair.
      await transact("readwrite", (store) => {
        const request = store.get(CONNECTION_KEY);
        request.onsuccess = () => {
          if (request.result === undefined) {
            store.put(connectionValue(legacy), CONNECTION_KEY);
          }
        };
        return request;
      });
      // Keep the legacy copy if the transaction fails; never delete before commit.
      await chrome.storage.local.remove(CONNECTION_KEY);
    }
    return true;
  }

  // Catch initialization failures without preventing event listener registration.
  const ready = initialize().catch(() => {
    database?.close();
    return false;
  });

  async function requireReady() {
    if (!(await ready)) throw new Error(STORAGE_ERROR);
  }

  async function get(keys) {
    await requireReady();
    const requested = Array.isArray(keys) ? keys : [keys];
    const publicKeys = requested.filter((key) => key !== CONNECTION_KEY);
    const values = publicKeys.length
      ? await chrome.storage.local.get(publicKeys)
      : {};
    if (requested.includes(CONNECTION_KEY)) {
      values[CONNECTION_KEY] =
        (await transact("readonly", (store) => store.get(CONNECTION_KEY))) ||
        {};
    }
    return values;
  }

  async function set(values) {
    await requireReady();
    const publicValues = { ...values };
    if (Object.prototype.hasOwnProperty.call(values, CONNECTION_KEY)) {
      delete publicValues[CONNECTION_KEY];
      const connection = connectionValue(values[CONNECTION_KEY]);
      await transact("readwrite", (store) =>
        store.put(connection, CONNECTION_KEY),
      );
      // Preserve storage.onChanged notifications without publishing credentials.
      publicValues[CONNECTION_CHANGED_KEY] = crypto.randomUUID();
    }
    await chrome.storage.local.set(publicValues);
  }

  return { ready, get, set };
})();

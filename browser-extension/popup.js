(() => {
  const MAGNET_DOWNLOAD_SETTINGS_KEY = "javboss:magnet-download-settings";
  const JAVDB_SETTINGS_KEY = "javboss:javdb-settings";
  const serverInput = document.getElementById("server-url");
  const enabledInput = document.getElementById("enabled");
  const javDBAutoRedirectInput = document.getElementById("javdb-auto-redirect");
  const saveButton = document.getElementById("save");
  const status = document.getElementById("status");

  function normalizedServerURL(value) {
    const candidate = String(value || "").trim();
    if (!candidate) return "";
    try {
      const parsed = new URL(candidate);
      if (
        !["http:", "https:"].includes(parsed.protocol) ||
        !parsed.hostname ||
        parsed.username ||
        parsed.password
      ) {
        return "";
      }
      parsed.search = "";
      parsed.hash = "";
      parsed.pathname = parsed.pathname.replace(/\/+$/, "");
      return parsed.href.replace(/\/$/, "");
    } catch {
      return "";
    }
  }

  function hostPermission(serverUrl) {
    const parsed = new URL(serverUrl);
    return `${parsed.protocol}//${parsed.hostname}/*`;
  }

  function showStatus(message, failed = false) {
    status.textContent = message;
    status.classList.toggle("error", failed);
  }

  async function loadSettings() {
    const stored = await chrome.storage.local.get([
      MAGNET_DOWNLOAD_SETTINGS_KEY,
      JAVDB_SETTINGS_KEY,
    ]);
    const magnetSettings = stored[MAGNET_DOWNLOAD_SETTINGS_KEY] || {};
    const javDBSettings = stored[JAVDB_SETTINGS_KEY] || {};
    serverInput.value = String(magnetSettings.serverUrl || "");
    enabledInput.checked = magnetSettings.enabled === true;
    javDBAutoRedirectInput.checked = javDBSettings.autoRedirect !== false;
  }

  async function saveSettings() {
    const serverUrl = normalizedServerURL(serverInput.value);
    if (enabledInput.checked && !serverUrl) {
      showStatus("请输入有效的 HTTP 或 HTTPS Server 地址", true);
      return;
    }

    saveButton.disabled = true;
    showStatus("");
    try {
      if (enabledInput.checked) {
        const granted = await chrome.permissions.request({
          origins: [hostPermission(serverUrl)],
        });
        if (!granted) {
          throw new Error("需要授权访问该 JavBoss Server 才能启用");
        }
      }
      await chrome.storage.local.set({
        [MAGNET_DOWNLOAD_SETTINGS_KEY]: {
          enabled: enabledInput.checked,
          serverUrl,
        },
        [JAVDB_SETTINGS_KEY]: {
          autoRedirect: javDBAutoRedirectInput.checked,
        },
      });
      serverInput.value = serverUrl;
      showStatus("设置已保存");
    } catch (error) {
      showStatus(String(error?.message || error || "保存失败"), true);
    } finally {
      saveButton.disabled = false;
    }
  }

  saveButton.addEventListener("click", saveSettings);
  loadSettings().catch((error) => {
    showStatus(String(error?.message || error || "读取设置失败"), true);
  });
})();

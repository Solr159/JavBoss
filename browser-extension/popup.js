(() => {
  const SETTINGS_KEY = "javboss:magnet-download-settings";
  const serverInput = document.getElementById("server-url");
  const enabledInput = document.getElementById("enabled");
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
    const stored = await chrome.storage.local.get(SETTINGS_KEY);
    const settings = stored[SETTINGS_KEY] || {};
    serverInput.value = String(settings.serverUrl || "");
    enabledInput.checked = settings.enabled === true;
  }

  async function saveSettings() {
    const serverUrl = normalizedServerURL(serverInput.value);
    if (!serverUrl) {
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
        [SETTINGS_KEY]: {
          enabled: enabledInput.checked,
          serverUrl,
        },
      });
      serverInput.value = serverUrl;
      showStatus(enabledInput.checked ? "已启用并保存" : "已关闭并保存");
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

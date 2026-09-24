(function () {
  "use strict";

  const config = window.LINKDESK_CONFIG || {};
  const apiBaseUrl = String(config.API_BASE_URL || "http://localhost:8080").replace(/\/$/, "");
  const pageSize = Number(config.PAGE_SIZE || 100);
  const storageKeys = {
    token: "linkdesk_token",
    user: "linkdesk_user"
  };

  const state = {
    token: localStorage.getItem(storageKeys.token) || "",
    user: readStoredUser(),
    links: [],
    total: 0,
    isLoadingLinks: false,
    deleteTarget: null
  };

  const elements = {};

  document.addEventListener("DOMContentLoaded", init);

  function init() {
    cacheElements();
    bindEvents();
    setApiBaseText();
    setTodayLabel();

    if (state.token) {
      enterDashboard();
      verifySession();
    } else {
      showAuthView();
    }
  }

  function cacheElements() {
    elements.authView = document.querySelector("#auth-view");
    elements.dashboardView = document.querySelector("#dashboard-view");
    elements.mainContent = document.querySelector("#main-content");
    elements.loginForm = document.querySelector("#login-form");
    elements.registerForm = document.querySelector("#register-form");
    elements.createForm = document.querySelector("#create-link-form");
    elements.linksBody = document.querySelector("#links-body");
    elements.linksLoading = document.querySelector("#links-loading");
    elements.linksEmpty = document.querySelector("#links-empty");
    elements.linksTableWrap = document.querySelector("#links-table-wrap");
    elements.linkSearch = document.querySelector("#link-search");
    elements.deleteDialog = document.querySelector("#delete-dialog");
    elements.toastRegion = document.querySelector("#toast-region");
    elements.userName = document.querySelector("#user-name");
    elements.userAvatar = document.querySelector("#user-avatar");
    elements.apiStatusDot = document.querySelector("#api-status-dot");
    elements.apiStatusText = document.querySelector("#api-status-text");
    elements.metricTotal = document.querySelector("#metric-total");
    elements.metricClicks = document.querySelector("#metric-clicks");
    elements.metricApi = document.querySelector("#metric-api");
    elements.footerApiBase = document.querySelector("#footer-api-base");
  }

  function bindEvents() {
    document.querySelectorAll("[data-auth-tab]").forEach(function (tab) {
      tab.addEventListener("click", function () {
        switchAuthMode(tab.dataset.authTab);
      });
    });

    document.querySelectorAll("[data-password-toggle]").forEach(function (button) {
      button.addEventListener("click", function () {
        const input = document.getElementById(button.dataset.passwordToggle);
        const isPassword = input.type === "password";
        input.type = isPassword ? "text" : "password";
        button.textContent = isPassword ? "隐藏" : "显示";
      });
    });

    elements.loginForm.addEventListener("submit", handleLogin);
    elements.registerForm.addEventListener("submit", handleRegister);
    elements.createForm.addEventListener("submit", handleCreateLink);
    document.querySelector("#logout-button").addEventListener("click", handleLogout);
    document.querySelector("#refresh-button").addEventListener("click", function () {
      loadLinks({ announce: true });
    });
    document.querySelector("#focus-create-button").addEventListener("click", focusCreateForm);
    document.querySelector("#empty-create-button").addEventListener("click", focusCreateForm);
    elements.linkSearch.addEventListener("input", renderLinks);
    elements.linksBody.addEventListener("click", handleLinkAction);

    document.querySelector("#cancel-delete-button").addEventListener("click", closeDeleteDialog);
    document.querySelector("#confirm-delete-button").addEventListener("click", confirmDelete);
    elements.deleteDialog.addEventListener("cancel", function (event) {
      event.preventDefault();
      closeDeleteDialog();
    });

    ["login-username", "login-password", "register-username", "register-password", "original-url"].forEach(function (id) {
      const input = document.getElementById(id);
      input.addEventListener("blur", function () {
        validateField(input);
      });
      input.addEventListener("input", function () {
        clearFieldError(input);
      });
    });
  }

  async function handleLogin(event) {
    event.preventDefault();
    clearFormState(elements.loginForm, "login");

    const username = document.querySelector("#login-username");
    const password = document.querySelector("#login-password");
    if (!validateCredentials(username, password)) {
      focusFirstError(elements.loginForm);
      return;
    }

    const submitButton = event.submitter || elements.loginForm.querySelector("button[type=submit]");
    setButtonLoading(submitButton, true, "登录中");

    try {
      const response = await apiRequest("/api/v1/auth/login", {
        method: "POST",
        body: JSON.stringify({ username: username.value.trim(), password: password.value })
      });

      if (!response || !response.token) {
        throw new ApiError(500, { code: "INVALID_RESPONSE", message: "登录响应缺少 token" });
      }

      saveSession(response.token, response.user);
      showToast("登录成功，正在打开工作台");
      enterDashboard();
      await loadDashboard();
    } catch (error) {
      showFormError("login", getFriendlyError(error, "登录失败，请检查后端服务和账号密码"));
    } finally {
      setButtonLoading(submitButton, false);
    }
  }

  async function handleRegister(event) {
    event.preventDefault();
    clearFormState(elements.registerForm, "register");

    const username = document.querySelector("#register-username");
    const password = document.querySelector("#register-password");
    if (!validateCredentials(username, password)) {
      focusFirstError(elements.registerForm);
      return;
    }

    const submitButton = event.submitter || elements.registerForm.querySelector("button[type=submit]");
    setButtonLoading(submitButton, true, "创建中");

    try {
      await apiRequest("/api/v1/auth/register", {
        method: "POST",
        body: JSON.stringify({ username: username.value.trim(), password: password.value })
      });

      document.querySelector("#login-username").value = username.value.trim();
      document.querySelector("#login-password").value = "";
      switchAuthMode("login");
      showFormSuccess("login", "注册成功，请输入密码登录");
      showToast("账号创建成功");
    } catch (error) {
      showFormError("register", getFriendlyError(error, "注册失败，请检查后端服务"));
    } finally {
      setButtonLoading(submitButton, false);
    }
  }

  async function handleCreateLink(event) {
    event.preventDefault();
    clearFormState(elements.createForm, "create");

    const urlInput = document.querySelector("#original-url");
    if (!validateUrl(urlInput)) {
      focusFirstError(elements.createForm);
      return;
    }

    const submitButton = event.submitter || elements.createForm.querySelector("button[type=submit]");
    setButtonLoading(submitButton, true, "生成中");

    try {
      await apiRequest("/api/v1/links", {
        method: "POST",
        body: JSON.stringify({ url: urlInput.value.trim() })
      });

      elements.createForm.reset();
      showFormSuccess("create", "短链接已创建，列表已更新");
      showToast("短链接创建成功");
      await loadLinks();
    } catch (error) {
      const message = getFriendlyError(error, "创建失败，请稍后重试");
      showFormError("create", message);
      if (error.status === 401) {
        handleUnauthorized();
      }
    } finally {
      setButtonLoading(submitButton, false);
    }
  }

  async function handleLinkAction(event) {
    const actionButton = event.target.closest("[data-link-action]");
    if (!actionButton) return;

    const link = state.links.find(function (item) {
      return String(item.id) === String(actionButton.dataset.linkId);
    });
    if (!link) return;

    const action = actionButton.dataset.linkAction;
    if (action === "copy") {
      await copyText(link.short_url);
    }
    if (action === "delete") {
      openDeleteDialog(link);
    }
  }

  async function confirmDelete() {
    if (!state.deleteTarget) return;

    const button = document.querySelector("#confirm-delete-button");
    setButtonLoading(button, true, "删除中");

    try {
      await apiRequest("/api/v1/links/" + encodeURIComponent(state.deleteTarget.id), {
        method: "DELETE"
      });

      closeDeleteDialog();
      showToast("短链接已删除");
      await loadLinks();
    } catch (error) {
      showToast(getFriendlyError(error, "删除失败，请稍后重试"), "error");
      if (error.status === 401) {
        handleUnauthorized();
      }
    } finally {
      setButtonLoading(button, false);
    }
  }

  async function verifySession() {
    try {
      const user = await apiRequest("/api/v1/me");
      state.user = user;
      saveStoredUser(user);
      await loadDashboard();
    } catch (error) {
      if (error.status === 401) {
        handleUnauthorized("登录状态已失效，请重新登录");
      } else {
        showToast(getFriendlyError(error, "无法确认登录状态"), "error");
        await loadDashboard({ skipMe: true });
      }
    }
  }

  async function loadDashboard(options) {
    const settings = options || {};
    updateUserDisplay();
    checkHealth();
    if (!settings.skipMe && !state.user) {
      try {
        state.user = await apiRequest("/api/v1/me");
        saveStoredUser(state.user);
        updateUserDisplay();
      } catch (error) {
        if (error.status === 401) handleUnauthorized();
      }
    }
    await loadLinks();
  }

  async function checkHealth() {
    setApiStatus("检查中", "pending");
    try {
      await apiRequest("/health", { skipAuth: true });
      setApiStatus("在线", "online");
    } catch (error) {
      setApiStatus("离线", "offline");
    }
  }

  async function loadLinks(options) {
    const settings = options || {};
    if (!state.token || state.isLoadingLinks) return;

    state.isLoadingLinks = true;
    showLinkState("loading");

    try {
      const response = await apiRequest("/api/v1/links?page=1&page_size=" + pageSize);
      state.links = Array.isArray(response) ? response : (response.items || []);
      state.total = Array.isArray(response) ? state.links.length : Number(response.total || state.links.length);

      state.isLoadingLinks = false;

      renderStats();
      renderLinks();
      if (settings.announce) showToast("列表已刷新");
    } catch (error) {
      state.links = [];
      state.total = 0;
      renderStats();
      showLinkState("error", getFriendlyError(error, "读取列表失败，请确认后端接口已启动"));
      if (error.status === 401) handleUnauthorized();
    } finally {
      state.isLoadingLinks = false;
    }
  }

  function renderLinks() {
    if (state.isLoadingLinks) return;

    const query = elements.linkSearch.value.trim().toLowerCase();
    const filtered = state.links.filter(function (link) {
      return !query || [link.original_url, link.code, link.short_url].some(function (value) {
        return String(value || "").toLowerCase().includes(query);
      });
    });

    if (!state.links.length) {
      showLinkState("empty");
      return;
    }

    if (!filtered.length) {
      showLinkState("no-match");
      return;
    }

    elements.linksLoading.hidden = true;
    elements.linksEmpty.hidden = true;
    elements.linksTableWrap.hidden = false;
    elements.linksBody.innerHTML = filtered.map(renderLinkRow).join("");
  }

  function renderLinkRow(link) {
    const originalUrl = String(link.original_url || "");
    const shortUrl = String(link.short_url || buildShortUrl(link.code));
    const id = escapeHtml(link.id);
    const safeOriginalUrl = escapeHtml(originalUrl);
    const safeShortUrl = escapeHtml(shortUrl);
    const safeCode = escapeHtml(link.code || "-");
    const safeDate = escapeHtml(formatDate(link.created_at));
    const clickCount = formatNumber(link.click_count || 0);

    return [
      "<tr>",
      "<td class=\"url-cell\" title=\"" + safeOriginalUrl + "\"><strong>" + safeOriginalUrl + "</strong><span>" + safeShortUrl + "</span></td>",
      "<td class=\"code-cell\">" + safeCode + "</td>",
      "<td class=\"click-cell\">" + clickCount + "</td>",
      "<td class=\"date-cell\">" + safeDate + "</td>",
      "<td><div class=\"row-actions\">",
      "<a class=\"row-action\" href=\"" + safeShortUrl + "\" target=\"_blank\" rel=\"noopener\">访问</a>",
      "<button class=\"row-action\" type=\"button\" data-link-action=\"copy\" data-link-id=\"" + id + "\">复制</button>",
      "<button class=\"row-action danger\" type=\"button\" data-link-action=\"delete\" data-link-id=\"" + id + "\">删除</button>",
      "</div></td>",
      "</tr>"
    ].join("");
  }

  function renderStats() {
    elements.metricTotal.textContent = formatNumber(state.total);
    const visibleClicks = state.links.reduce(function (total, link) {
      return total + Number(link.click_count || 0);
    }, 0);
    elements.metricClicks.textContent = formatNumber(visibleClicks);
  }

  function showLinkState(type, message) {
    elements.linksLoading.hidden = type !== "loading";
    elements.linksTableWrap.hidden = type !== "table";
    elements.linksEmpty.hidden = !["empty", "no-match", "error"].includes(type);

    if (type === "empty") {
      document.querySelector("#empty-title").textContent = "还没有短链接";
      document.querySelector("#empty-copy").textContent = "从上方创建第一条链接，它会出现在这里。";
      document.querySelector("#empty-create-button").hidden = false;
    }
    if (type === "no-match") {
      document.querySelector("#empty-title").textContent = "没有匹配的链接";
      document.querySelector("#empty-copy").textContent = "换一个 URL 或短码关键词再试一次。";
      document.querySelector("#empty-create-button").hidden = true;
    }
    if (type === "error") {
      document.querySelector("#empty-title").textContent = "列表暂时无法读取";
      document.querySelector("#empty-copy").textContent = message || "请检查后端状态后重试。";
      document.querySelector("#empty-create-button").hidden = false;
      document.querySelector("#empty-create-button").textContent = "重新加载";
      document.querySelector("#empty-create-button").onclick = function () {
        document.querySelector("#empty-create-button").textContent = "创建第一条";
        loadLinks();
      };
    }
  }

  function switchAuthMode(mode) {
    const isLogin = mode === "login";
    document.querySelectorAll("[data-auth-tab]").forEach(function (tab) {
      const active = tab.dataset.authTab === mode;
      tab.classList.toggle("is-active", active);
      tab.setAttribute("aria-selected", String(active));
    });
    elements.loginForm.hidden = !isLogin;
    elements.registerForm.hidden = isLogin;
    document.querySelector("#auth-title").textContent = isLogin ? "进入你的工作台" : "创建你的账号";
    document.querySelector(".auth-subtitle").textContent = isLogin ? "使用后端接口创建你的第一条短链接。" : "注册后即可开始管理自己的短链接。";
    if (isLogin) document.querySelector("#login-username").focus();
    else document.querySelector("#register-username").focus();
  }

  function showAuthView() {
    elements.authView.hidden = false;
    elements.dashboardView.hidden = true;
  }

  function enterDashboard() {
    elements.authView.hidden = true;
    elements.dashboardView.hidden = false;
    updateUserDisplay();
  }

  function handleLogout() {
    clearSession();
    showAuthView();
    switchAuthMode("login");
    elements.loginForm.reset();
    showToast("你已退出登录");
  }

  function handleUnauthorized(message) {
    clearSession();
    showAuthView();
    switchAuthMode("login");
    showFormError("login", message || "登录状态已失效，请重新登录");
  }

  function saveSession(token, user) {
    state.token = token;
    state.user = user || null;
    localStorage.setItem(storageKeys.token, token);
    if (user) saveStoredUser(user);
  }

  function clearSession() {
    state.token = "";
    state.user = null;
    state.links = [];
    state.total = 0;
    localStorage.removeItem(storageKeys.token);
    localStorage.removeItem(storageKeys.user);
  }

  function updateUserDisplay() {
    const username = state.user && state.user.username ? state.user.username : "用户";
    elements.userName.textContent = username;
    elements.userAvatar.textContent = username.slice(0, 1).toUpperCase();
  }

  function setApiStatus(text, status) {
    elements.apiStatusText.textContent = text;
    elements.metricApi.textContent = text;
    elements.apiStatusDot.classList.toggle("is-online", status === "online");
    elements.apiStatusDot.classList.toggle("is-offline", status === "offline");
  }

  function setTodayLabel() {
    const formatted = new Intl.DateTimeFormat("zh-CN", {
      year: "numeric",
      month: "long",
      day: "numeric",
      weekday: "long"
    }).format(new Date());
    document.querySelector("#today-label").textContent = formatted;
  }

  function setApiBaseText() {
    elements.footerApiBase.textContent = apiBaseUrl;
  }

  function focusCreateForm() {
    document.querySelector("#create-panel").scrollIntoView({ behavior: "smooth", block: "center" });
    window.setTimeout(function () {
      document.querySelector("#original-url").focus();
    }, 160);
  }

  function openDeleteDialog(link) {
    state.deleteTarget = link;
    if (typeof elements.deleteDialog.showModal === "function") {
      elements.deleteDialog.showModal();
      document.querySelector("#cancel-delete-button").focus();
    } else if (window.confirm("确定删除这条短链接吗？")) {
      confirmDelete();
    }
  }

  function closeDeleteDialog() {
    state.deleteTarget = null;
    if (elements.deleteDialog.open) elements.deleteDialog.close();
  }

  async function copyText(text) {
    try {
      if (navigator.clipboard && window.isSecureContext) {
        await navigator.clipboard.writeText(text);
      } else {
        const textarea = document.createElement("textarea");
        textarea.value = text;
        textarea.style.position = "fixed";
        textarea.style.opacity = "0";
        document.body.appendChild(textarea);
        textarea.focus();
        textarea.select();
        document.execCommand("copy");
        textarea.remove();
      }
      showToast("短链接已复制");
    } catch (error) {
      showToast("复制失败，请手动复制短链接", "error");
    }
  }

  async function apiRequest(path, options) {
    const settings = options || {};
    const headers = new Headers(settings.headers || {});
    headers.set("Accept", "application/json");
    if (settings.body && !headers.has("Content-Type")) {
      headers.set("Content-Type", "application/json");
    }
    if (state.token && !settings.skipAuth) {
      headers.set("Authorization", "Bearer " + state.token);
    }

    let response;
    try {
      response = await fetch(apiBaseUrl + path, {
        ...settings,
        headers,
        signal: AbortSignal.timeout ? AbortSignal.timeout(12000) : undefined
      });
    } catch (error) {
      throw new ApiError(0, { code: "NETWORK_ERROR", message: "无法连接后端，请确认服务地址和服务状态" });
    }

    const payload = await parseResponse(response);
    if (!response.ok) {
      throw new ApiError(response.status, payload);
    }
    return payload;
  }

  async function parseResponse(response) {
    if (response.status === 204) return null;
    const text = await response.text();
    if (!text) return null;
    try {
      return JSON.parse(text);
    } catch (error) {
      return { message: text };
    }
  }

  function getFriendlyError(error, fallback) {
    if (!error) return fallback;
    if (error.status === 0) return "无法连接后端，请确认后端已启动，并检查 frontend/config.js 中的 API 地址";
    const known = {
      INVALID_CREDENTIALS: "用户名或密码错误",
      USERNAME_EXISTS: "用户名已存在，请换一个用户名",
      INVALID_URL: "请输入以 http:// 或 https:// 开头的有效地址",
      DAILY_QUOTA_EXCEEDED: "今日创建次数已达上限，请明天再试",
      UNAUTHORIZED: "登录状态已失效，请重新登录",
      NOT_FOUND: "请求的资源不存在",
      VALIDATION_FAILED: "提交内容没有通过校验"
    };
    return known[error.data && error.data.code] || (error.data && error.data.message) || fallback;
  }

  function validateCredentials(usernameInput, passwordInput) {
    let valid = true;
    if (!usernameInput.value.trim() || usernameInput.value.trim().length < 3) {
      setFieldError(usernameInput, "用户名至少需要 3 个字符");
      valid = false;
    }
    if (!passwordInput.value || passwordInput.value.length < 6) {
      setFieldError(passwordInput, "密码至少需要 6 个字符");
      valid = false;
    }
    return valid;
  }

  function validateUrl(input) {
    try {
      const url = new URL(input.value.trim());
      if (!["http:", "https:"].includes(url.protocol)) throw new Error("invalid protocol");
      clearFieldError(input);
      return true;
    } catch (error) {
      setFieldError(input, "请输入以 http:// 或 https:// 开头的有效地址");
      return false;
    }
  }

  function validateField(input) {
    if (!input.value.trim()) {
      setFieldError(input, "此项不能为空");
      return false;
    }
    if (input.type === "url") return validateUrl(input);
    if (input.name === "username" && input.value.trim().length < 3) {
      setFieldError(input, "用户名至少需要 3 个字符");
      return false;
    }
    if (input.name === "password" && input.value.length < 6) {
      setFieldError(input, "密码至少需要 6 个字符");
      return false;
    }
    clearFieldError(input);
    return true;
  }

  function setFieldError(input, message) {
    input.classList.add("has-error");
    const error = document.querySelector('[data-error-for="' + input.id + '"]');
    if (error) error.textContent = message;
  }

  function clearFieldError(input) {
    input.classList.remove("has-error");
    const error = document.querySelector('[data-error-for="' + input.id + '"]');
    if (error) error.textContent = "";
  }

  function clearFormState(form, name) {
    form.querySelectorAll(".field-error").forEach(function (item) {
      item.textContent = "";
    });
    form.querySelectorAll("input").forEach(function (input) {
      input.classList.remove("has-error");
    });
    const message = document.querySelector('[data-form-message="' + name + '"]');
    message.textContent = "";
    message.classList.remove("is-success");
  }

  function showFormError(name, message) {
    const target = document.querySelector('[data-form-message="' + name + '"]');
    target.textContent = message;
    target.classList.remove("is-success");
  }

  function showFormSuccess(name, message) {
    const target = document.querySelector('[data-form-message="' + name + '"]');
    target.textContent = message;
    target.classList.add("is-success");
  }

  function focusFirstError(form) {
    const input = form.querySelector(".has-error");
    if (input) input.focus();
  }

  function setButtonLoading(button, loading, label) {
    if (!button) return;
    button.disabled = loading;
    button.classList.toggle("is-loading", loading);
    const labelNode = button.querySelector("[data-submit-label]");
    if (labelNode && label) {
      if (loading) {
        button.dataset.defaultLabel = labelNode.textContent;
        labelNode.textContent = label;
      } else {
        labelNode.textContent = button.dataset.defaultLabel || labelNode.textContent;
      }
    }
  }

  function showToast(message, type) {
    const toast = document.createElement("div");
    toast.className = "toast" + (type ? " is-" + type : "");
    toast.setAttribute("role", type === "error" ? "alert" : "status");
    toast.textContent = message;
    elements.toastRegion.appendChild(toast);
    window.setTimeout(function () {
      toast.remove();
    }, 4200);
  }

  function formatNumber(value) {
    return new Intl.NumberFormat("zh-CN").format(Number(value || 0));
  }

  function formatDate(value) {
    if (!value) return "-";
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return String(value);
    return new Intl.DateTimeFormat("zh-CN", {
      year: "numeric",
      month: "2-digit",
      day: "2-digit"
    }).format(date);
  }

  function buildShortUrl(code) {
    return apiBaseUrl + "/r/" + encodeURIComponent(code || "");
  }

  function escapeHtml(value) {
    return String(value == null ? "" : value)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/\"/g, "&quot;")
      .replace(/'/g, "&#039;");
  }

  function readStoredUser() {
    try {
      return JSON.parse(localStorage.getItem(storageKeys.user) || "null");
    } catch (error) {
      return null;
    }
  }

  function saveStoredUser(user) {
    if (user) localStorage.setItem(storageKeys.user, JSON.stringify(user));
  }

  function ApiError(status, data) {
    this.name = "ApiError";
    this.status = status;
    this.data = data || {};
    this.message = this.data.message || "API request failed";
  }

  ApiError.prototype = Object.create(Error.prototype);
  ApiError.prototype.constructor = ApiError;
})();

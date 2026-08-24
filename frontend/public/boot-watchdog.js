/*
 * Loaded as a classic script before the app's module graph, so it still runs
 * when that graph fails. A module that 404s - the dev server re-optimising
 * dependencies mid-load is the usual cause - leaves a frameless window painted
 * in nothing but its background colour: no title bar to close, no console to
 * open, no clue what went wrong. Retry once, then put the failure on screen.
 *
 * Runs before i18n exists, so its text is English only.
 */
(function () {
  "use strict";

  var BOOT_TIMEOUT_MS = 12000;
  var RETRY_KEY = "tcno:boot-retried";

  var failures = [];
  var timer = null;
  var booted = false;
  var panel = null;

  function describe(value) {
    if (value === null || value === undefined) return "unknown error";
    if (typeof value === "string") return value;
    if (value.stack) return String(value.stack).split("\n").slice(0, 4).join("\n");
    if (value.message) return String(value.message);
    try {
      return JSON.stringify(value);
    } catch (err) {
      return String(value);
    }
  }

  function record(stage, detail) {
    if (failures.length < 8) failures.push(stage + ": " + detail);
  }

  function retried() {
    try {
      return window.sessionStorage.getItem(RETRY_KEY) === "1";
    } catch (err) {
      return false;
    }
  }

  function setRetried(value) {
    try {
      if (value) window.sessionStorage.setItem(RETRY_KEY, "1");
      else window.sessionStorage.removeItem(RETRY_KEY);
      return retried() === value;
    } catch (err) {
      return false;
    }
  }

  /**
   * A script that failed to fetch usually arrives on the next attempt. The
   * reload is spent only once, and only if the marker that says so survives -
   * without storage there is nothing to stop the second attempt reloading again.
   */
  function retryOnce() {
    if (booted || panel || retried()) return;
    if (!setRetried(true)) return;
    window.location.reload();
  }

  function mounted() {
    var root = document.getElementById("app");
    return !!root && root.childElementCount > 0;
  }

  function stopTimer() {
    if (timer !== null) {
      window.clearTimeout(timer);
      timer = null;
    }
  }

  function makeButton(label, primary) {
    var el = document.createElement("button");
    el.type = "button";
    el.textContent = label;
    el.style.cssText =
      "font:inherit;padding:7px 16px;border-radius:6px;cursor:pointer;" +
      (primary
        ? "border:0;background:#3a76d8;color:#fff;"
        : "border:1px solid #46597c;background:transparent;color:#c9d6ea;");
    return el;
  }

  function show() {
    if (panel) return;
    stopTimer();

    panel = document.createElement("div");
    panel.setAttribute("data-tcno-boot-error", "");
    panel.style.cssText =
      "position:fixed;inset:0;z-index:2147483647;display:flex;align-items:center;" +
      "justify-content:center;padding:24px;background:#1b2636;color:#e6ecf5;" +
      "font:14px/1.55 'Segoe UI',system-ui,sans-serif;";

    // The real title bar never mounted, so the window has nothing to drag by.
    var dragStrip = document.createElement("div");
    dragStrip.style.cssText = "position:fixed;top:0;left:0;right:0;height:32px;--wails-draggable:drag;";

    var card = document.createElement("div");
    card.style.cssText =
      "width:100%;max-width:560px;background:#22304a;border:1px solid #37496b;" +
      "border-radius:8px;padding:20px 22px;box-shadow:0 12px 32px rgba(0,0,0,.45);";

    var title = document.createElement("h1");
    title.textContent = "The interface didn't load";
    title.style.cssText = "margin:0 0 8px;font-size:18px;font-weight:600;";

    var body = document.createElement("p");
    body.textContent =
      "TcNo Account Switcher started, but its window never rendered. " +
      "Reloading usually fixes it. If it doesn't, close the app and open it again.";
    body.style.cssText = "margin:0 0 16px;color:#b8c6dd;";

    var details = document.createElement("pre");
    details.textContent = failures.join("\n") || "No error was reported.";
    details.style.cssText =
      "margin:0 0 16px;padding:10px 12px;max-height:180px;overflow:auto;" +
      "background:#1a2438;border:1px solid #33445f;border-radius:6px;" +
      "font:12px/1.45 Consolas,monospace;color:#9fb2ce;white-space:pre-wrap;word-break:break-word;";

    var actions = document.createElement("div");
    actions.style.cssText = "display:flex;gap:10px;";

    var reload = makeButton("Reload", true);
    reload.addEventListener("click", function () {
      setRetried(false);
      window.location.reload();
    });

    var copy = makeButton("Copy details", false);
    copy.addEventListener("click", function () {
      var text = failures.join("\n");
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text).catch(function () {});
      }
      copy.textContent = "Copied";
      window.setTimeout(function () {
        copy.textContent = "Copy details";
      }, 1500);
    });

    actions.appendChild(reload);
    actions.appendChild(copy);
    card.appendChild(title);
    card.appendChild(body);
    card.appendChild(details);
    card.appendChild(actions);
    panel.appendChild(dragStrip);
    panel.appendChild(card);
    (document.body || document.documentElement).appendChild(panel);
  }

  window.__tcnoBoot = {
    ready: function () {
      booted = true;
      setRetried(false);
      stopTimer();
      if (panel) {
        panel.remove();
        panel = null;
      }
    },
    fail: function (stage, error) {
      record(stage, describe(error));
      show();
    },
  };

  window.addEventListener(
    "error",
    function (event) {
      if (booted) return;
      var target = event.target;
      if (target && target !== window && target.tagName) {
        record("load", (target.src || target.href || target.tagName) + " failed to load");
        // Only a missing script can leave the window empty; a stylesheet or an
        // image is not worth a reload.
        if (target.tagName === "SCRIPT") retryOnce();
        return;
      }
      record("error", describe(event.error || event.message));
    },
    true,
  );

  window.addEventListener("unhandledrejection", function (event) {
    if (booted) return;
    var text = describe(event.reason);
    record("unhandled", text);
    if (/dynamically imported module|module script failed/i.test(text)) {
      retryOnce();
    }
  });

  timer = window.setTimeout(function () {
    timer = null;
    if (booted || mounted()) return;
    record("boot", "nothing rendered within " + Math.round(BOOT_TIMEOUT_MS / 1000) + " seconds");
    show();
  }, BOOT_TIMEOUT_MS);
})();

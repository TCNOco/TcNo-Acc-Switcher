/*
PreviewCss contrast sampler for http://127.0.0.1:5183/#/preview-css.

Usage:
1. Open the PreviewCss route in a browser with the app running.
2. Paste this whole file into DevTools Console.
3. Run one of:
   await PreviewCssContrastSampler.run()
   await PreviewCssContrastSampler.runAllThemes()
   await PreviewCssContrastSampler.runStateMatrix()
   await PreviewCssContrastSampler.runStateMatrixAllThemes()
   await PreviewCssContrastSampler.runStateMatrixAllThemesAtZoom(2)

The sampler reads computed styles from live DOM elements and adds no package
dependencies. It prints a console table, JSON, and Markdown. It also stores
the last run in window.PreviewCssContrastSampler.last.

Current limitation:
runAllThemes clicks the custom theme dropdown by visible labels. If Svelte
rerenders the picker while the menu is closed, or translated labels differ
from the collected labels, select each theme manually and run:
await PreviewCssContrastSampler.run({ prepare: true })
*/

(() => {
  "use strict";

  const UI_THRESHOLD = 3;
  const NORMAL_TEXT_THRESHOLD = 4.5;
  const LARGE_TEXT_THRESHOLD = 3;
  const WAIT_MS = 80;
  const ROUTE = "#/preview-css";
  const FORCED_STATE_CLASS_PREFIX = "pcs-force-";

  const SAMPLES = [
    {
      section: "top",
      label: "Preview header",
      selector: ".preview-css-page > .SettingsHeader:first-of-type",
    },
    {
      section: "top",
      label: "Preview intro text",
      selector: ".preview-css-intro",
    },
    {
      section: "top",
      label: "Theme dropdown toggle",
      selector: ".theme-control-group > .rowDropdown:first-child .dropdown-toggle",
      focus: true,
    },
    {
      section: "top",
      label: "Theme dropdown item",
      selector: ".theme-control-group > .rowDropdown:first-child .dropdown-item:first-child",
      state: "open",
      focus: true,
    },
    {
      section: "top",
      label: "Accent dropdown toggle",
      selector: ".accent-dropdown .accent-toggle",
      focus: true,
    },
    {
      section: "top",
      label: "Accent dropdown chip",
      selector: ".accent-dropdown .accent-toggle .theme-accent-chip",
    },
    {
      section: "top",
      label: "Accent dropdown item",
      selector: ".accent-dropdown-menu > li:first-child .accent-dropdown-item",
      state: "open",
      prepare: "accentDropdown",
      cleanup: "accentDropdown",
      focus: true,
    },
    {
      section: "top",
      label: "Accent dropdown item chip",
      selector: ".accent-dropdown-menu > li:first-child .theme-accent-chip",
      state: "open",
      prepare: "accentDropdown",
      cleanup: "accentDropdown",
    },
    {
      section: "top",
      label: "Platform tile container",
      selector: ".preview_program_main .platform_list_item:first-child .platform_tile_ctx",
      state: "draggable",
    },
    {
      section: "top",
      label: "Platform tile text",
      selector: ".preview_program_main .platform_list_item:first-child .fgText p",
    },
    {
      section: "top",
      label: "Platform selected edge tile",
      selector: ".preview-platform-edge--selected",
      state: "selected",
      uiOnly: true,
    },
    {
      section: "top",
      label: "Platform selected edge badge",
      selector: ".preview-platform-edge--selected .preview-platform-state-badge",
      state: "selected",
    },
    {
      section: "top",
      label: "Platform current edge tile",
      selector: ".preview-platform-edge--current",
      state: "current",
      uiOnly: true,
    },
    {
      section: "top",
      label: "Platform disabled edge tile",
      selector: ".preview-platform-edge--disabled",
      state: "disabled",
      uiOnly: true,
    },
    {
      section: "top",
      label: "Platform hover edge tile",
      selector: ".preview-platform-edge--hover",
      state: "hover",
      uiOnly: true,
    },
    {
      section: "top",
      label: "Platform drop target edge tile",
      selector: ".preview-platform-edge--drop-target",
      state: "drop-target",
      uiOnly: true,
    },
    {
      section: "top",
      label: "Platform actionbar status",
      selector: ".preview_fake_actionbar .actionbar__status",
    },
    {
      section: "top",
      label: "Platform manage button",
      selector: ".preview_fake_actionbar .btnicontext",
      focus: true,
    },
    {
      section: "top",
      label: "Platform square icon button",
      selector: ".preview_fake_actionbar .actionbar__actions > button.square",
      focus: true,
    },
    {
      section: "top",
      label: "Platform square icon glyph",
      selector: ".preview_fake_actionbar .actionbar__actions > button.square svg",
    },
    {
      section: "top",
      label: "Selected account row container",
      selector: ".preview_accounts_wrap input.acc:checked + label.acc",
      state: "selected",
    },
    {
      section: "top",
      label: "Current account row container",
      selector: ".preview_accounts_wrap label.acc.currentAcc",
      state: "current",
    },
    {
      section: "top",
      label: "Current account name",
      selector: ".preview_accounts_wrap label.acc.currentAcc h6",
      state: "current",
    },
    {
      section: "top",
      label: "Normal account Steam ID",
      selector: ".preview_accounts_wrap label.acc:not(.currentAcc) .steamId",
    },
    {
      section: "top",
      label: "Disabled account edge row",
      selector: ".preview-account-edge--disabled",
      state: "disabled",
    },
    {
      section: "top",
      label: "Broken account edge row",
      selector: ".preview-account-edge--broken",
      state: "broken-data",
    },
    {
      section: "top",
      label: "Broken account repair badge",
      selector: ".preview-account-edge--broken .acc_broken_badge",
      state: "broken-data",
    },
    {
      section: "top",
      label: "Hover account edge row",
      selector: ".preview-account-edge--hover",
      state: "hover",
    },
    {
      section: "top",
      label: "Account edge drop target row",
      selector: ".preview-account-edge--drop-target",
      state: "drop-target",
    },
    {
      section: "top",
      label: "VAC account image ring",
      selector: ".preview_accounts_wrap img.status_vac",
      state: "vac",
    },
    {
      section: "top",
      label: "Limited account image ring",
      selector: ".preview_accounts_wrap img.status_limited",
      state: "limited",
    },
    {
      section: "top",
      label: "Account actionbar input",
      selector: ".preview_accounts_actionbar #pvCurrentStatus",
    },
    {
      section: "top",
      label: "Add account button",
      selector: ".preview_accounts_actionbar #btnAddNew",
      focus: true,
    },
    {
      section: "top",
      label: "Add account button icon",
      selector: ".preview_accounts_actionbar #btnAddNew svg",
    },
    {
      section: "top",
      label: "Login button",
      selector: ".preview_accounts_actionbar .actionbar__login",
      focus: true,
    },
    {
      section: "top",
      label: "Login button icon",
      selector: ".preview_accounts_actionbar .actionbar__login svg",
    },
    {
      section: "top",
      label: "Settings square icon button",
      selector: "#pvSettingsButton",
      focus: true,
    },
    {
      section: "top",
      label: "Settings square icon glyph",
      selector: "#pvSettingsButton svg",
    },
    {
      section: "top",
      label: "Info square icon button",
      selector: "#pvInfoButton",
      focus: true,
    },
    {
      section: "top",
      label: "Info square icon glyph",
      selector: "#pvInfoButton svg",
    },
    {
      section: "top",
      label: "Shortcut dropdown button",
      selector: "#shortcutDropdownBtn",
      state: "open",
      focus: true,
    },
    {
      section: "top",
      label: "Pinned shortcut icon button",
      selector: ".preview_shortcut_bar .shortcuts .shortcutDndCell:first-child button.HasContextMenu",
      state: "pinned",
      focus: true,
    },
    {
      section: "top",
      label: "Shortcut folder button",
      selector: "#shortcutDropdown #btnOpenShortcutFolder",
      state: "open",
      focus: true,
    },
    {
      section: "top",
      label: "Launch icon-only button",
      selector: "#btnStartPlat",
      focus: true,
    },
    {
      section: "top",
      label: "Account image overlay surface",
      selector: ".acc-img-overlay__panel",
      state: "drop-target",
    },
    {
      section: "top",
      label: "Account image dropzone",
      selector: ".acc-img-overlay__dropzone",
      state: "drop-target",
    },
    {
      section: "top",
      label: "Profile overlay title",
      selector: ".acc-img-overlay__title",
    },
    {
      section: "top",
      label: "Drop overlay CTA",
      selector: ".acc-img-overlay__cta",
      state: "drop-target",
    },
    {
      section: "top",
      label: "File drop overlay surface",
      selector: ".fileDropOverlay__inner",
      state: "drop-target",
    },
    {
      section: "top",
      label: "Account row drop receiver",
      selector: ".preview-overlay-drop-cell--accrow .acc--drop-target",
      state: "drag-hover",
    },
    {
      section: "top",
      label: "Account drop row label",
      selector: ".acc_profile_drop_overlay__label",
      state: "drop-target",
    },
    {
      section: "bottom",
      label: "Success toast title",
      selector: ".toast--success .toast__title",
      state: "success",
    },
    {
      section: "bottom",
      label: "Success toast surface",
      selector: ".toast--success.preview-static-toast",
      state: "success",
    },
    {
      section: "bottom",
      label: "Success toast icon",
      selector: ".toast--success .toast__icon",
      state: "success",
    },
    {
      section: "bottom",
      label: "Success toast message",
      selector: ".toast--success .toast__message",
      state: "success",
    },
    {
      section: "bottom",
      label: "Success toast close button",
      selector: ".toast--success .toast__close",
      state: "success",
      focus: true,
    },
    {
      section: "bottom",
      label: "Warning toast surface",
      selector: ".toast--warning.preview-static-toast",
      state: "warning",
    },
    {
      section: "bottom",
      label: "Warning toast icon",
      selector: ".toast--warning .toast__icon",
      state: "warning",
    },
    {
      section: "bottom",
      label: "Warning toast title",
      selector: ".toast--warning .toast__title",
      state: "warning",
    },
    {
      section: "bottom",
      label: "Warning toast message",
      selector: ".toast--warning .toast__message",
      state: "warning",
    },
    {
      section: "bottom",
      label: "Warning toast close button",
      selector: ".toast--warning .toast__close",
      state: "warning",
      focus: true,
    },
    {
      section: "bottom",
      label: "Error toast surface",
      selector: ".toast--error.preview-static-toast",
      state: "error",
    },
    {
      section: "bottom",
      label: "Error toast icon",
      selector: ".toast--error .toast__icon",
      state: "error",
    },
    {
      section: "bottom",
      label: "Error toast message",
      selector: ".toast--error .toast__message",
      state: "error",
    },
    {
      section: "bottom",
      label: "Error toast close button",
      selector: ".toast--error .toast__close",
      state: "error",
      focus: true,
    },
    {
      section: "bottom",
      label: "Info toast surface",
      selector: ".toast--info.preview-static-toast",
      state: "info",
    },
    {
      section: "bottom",
      label: "Info toast icon",
      selector: ".toast--info .toast__icon",
      state: "info",
    },
    {
      section: "bottom",
      label: "Info toast title",
      selector: ".toast--info .toast__title",
      state: "info",
    },
    {
      section: "bottom",
      label: "Info toast message",
      selector: ".toast--info .toast__message",
      state: "info",
    },
    {
      section: "bottom",
      label: "Info toast close button",
      selector: ".toast--info .toast__close",
      state: "info",
      focus: true,
    },
    {
      section: "bottom",
      label: "Permanent toast label",
      selector: ".toastPermanentRow span:first-of-type",
    },
    {
      section: "bottom",
      label: "Context menu surface",
      selector: "ul.ctx-menu-root.contextmenu",
      state: "open",
      prepare: "contextMenu",
      cleanup: "contextMenu",
      focus: true,
    },
    {
      section: "bottom",
      label: "Context menu first item",
      selector: "ul.ctx-menu-root.contextmenu > li:first-child .ctx-menu__btn",
      state: "open",
      prepare: "contextMenu",
      cleanup: "contextMenu",
      focus: true,
    },
    {
      section: "bottom",
      label: "Live toast button",
      selector: ".modalTestPanel .modalTestButtons .btnicontext:first-child",
      focus: true,
    },
    {
      section: "bottom",
      label: "Disabled preview button",
      selector: "#pvDisabledButton",
      state: "disabled",
    },
    {
      section: "bottom",
      label: "Modal output container",
      selector: ".modalTestOutput",
    },
    {
      section: "bottom",
      label: "Prompt test button",
      selector: ".modalTestButtons button:nth-of-type(5)",
      focus: true,
    },
    {
      section: "bottom",
      label: "Live modal surface",
      selector: ".modalBG .modalFG",
      state: "prompt-open",
      prepare: "promptModal",
    },
    {
      section: "bottom",
      label: "Live modal body",
      selector: ".modalFG .modal-scroll",
      state: "prompt-open",
      prepare: "promptModal",
    },
    {
      section: "bottom",
      label: "Live modal primary button",
      selector: ".modalFG .modal-inline-actions .btnicontext.modal-primary",
      state: "prompt-open",
      prepare: "promptModal",
      focus: true,
    },
    {
      section: "bottom",
      label: "Live modal close button",
      selector: ".modalFG .win-btn-close",
      state: "prompt-open",
      prepare: "promptModal",
      focus: true,
    },
    {
      section: "bottom",
      label: "Live modal text input",
      selector: ".modalFG .modal-input",
      state: "prompt-open",
      prepare: "promptModal",
      cleanup: "promptModal",
      focus: true,
    },
    {
      section: "bottom",
      label: "Settings image expiry label",
      selector: ".settingsCol .form-text span",
    },
    {
      section: "bottom",
      label: "Settings number input",
      selector: "#pvImageExpiry",
      focus: true,
    },
    {
      section: "bottom",
      label: "Settings streamer label",
      selector: "label[for='pvStreamer']:not(.form-check-label)",
    },
    {
      section: "bottom",
      label: "Settings text input",
      selector: "#pvText",
      focus: true,
    },
    {
      section: "bottom",
      label: "Settings placeholder input",
      selector: "#pvPlaceholderText",
      focus: true,
    },
    {
      section: "bottom",
      label: "Settings placeholder text",
      selector: "#pvPlaceholderText",
      state: "placeholder",
      placeholder: true,
    },
    {
      section: "bottom",
      label: "Settings invalid input",
      selector: "#pvInvalidText",
      state: "invalid",
      focus: true,
    },
    {
      section: "bottom",
      label: "Settings disabled text input",
      selector: "#pvDisabledText",
      state: "disabled",
    },
    {
      section: "bottom",
      label: "Settings dropdown toggle",
      selector: ".settingsCol .dropdown-toggle",
      state: "open",
      focus: true,
    },
    {
      section: "bottom",
      label: "Settings dropdown chevron",
      selector: ".settingsCol .dropdown-toggle .caret",
      state: "open",
    },
    {
      section: "bottom",
      label: "Settings dropdown item",
      selector: ".settingsCol .dropdown-item:first-child",
      state: "open",
      focus: true,
    },
    {
      section: "bottom",
      label: "Settings checkbox control",
      selector: "#pvTray + .form-check-label",
      state: "disabled",
    },
    {
      section: "bottom",
      label: "Disabled settings checkbox input",
      selector: "#pvTray",
      state: "disabled",
    },
  ];

  const STATE_SAMPLES = [
    { section: "top", label: "Theme dropdown toggle", selector: ".theme-control-group > .rowDropdown:first-child .dropdown-toggle", state: "hover", probe: "hover" },
    { section: "top", label: "Theme dropdown toggle", selector: ".theme-control-group > .rowDropdown:first-child .dropdown-toggle", state: "active", probe: "active" },
    { section: "top", label: "Accent dropdown toggle", selector: ".accent-dropdown .accent-toggle", state: "hover", probe: "hover" },
    { section: "top", label: "Accent dropdown toggle", selector: ".accent-dropdown .accent-toggle", state: "active", probe: "active" },
    { section: "top", label: "Platform tile container", selector: ".preview_program_main .platform_list_item:first-child .platform_tile_ctx", state: "hover", probe: "hover" },
    { section: "top", label: "Platform tile container", selector: ".preview_program_main .platform_list_item:first-child .platform_tile_ctx", state: "active", probe: "active" },
    { section: "top", label: "Platform selected edge tile", selector: ".preview-platform-edge--selected", state: "selected", probe: "classAttributeState", className: "preview-platform-edge--selected", attr: "aria-selected", value: "true", offValue: "false" },
    { section: "top", label: "Platform current edge tile", selector: ".preview-platform-edge--current", state: "current", probe: "classAttributeState", className: "preview-platform-edge--current", attr: "aria-current", value: "page" },
    { section: "top", label: "Platform disabled edge tile", selector: ".preview-platform-edge--disabled", state: "disabled", probe: "classAttributeState", className: "preview-platform-edge--disabled", attr: "aria-disabled", value: "true" },
    { section: "top", label: "Platform hover edge tile", selector: ".preview-platform-edge--hover", state: "hover", probe: "classAttributeState", className: "preview-platform-edge--hover" },
    { section: "top", label: "Platform drop target edge tile", selector: ".preview-platform-edge--drop-target", state: "drop-target", probe: "classAttributeState", className: "preview-platform-edge--drop-target" },
    { section: "top", label: "Platform manage button", selector: ".preview_fake_actionbar .btnicontext", state: "hover", probe: "hover" },
    { section: "top", label: "Platform manage button", selector: ".preview_fake_actionbar .btnicontext", state: "active", probe: "active" },
    { section: "top", label: "Platform square icon button", selector: ".preview_fake_actionbar .actionbar__actions > button.square", state: "hover", probe: "hover" },
    { section: "top", label: "Platform square icon button", selector: ".preview_fake_actionbar .actionbar__actions > button.square", state: "active", probe: "active" },
    { section: "top", label: "Add account button", selector: "#btnAddNew", state: "hover", probe: "hover" },
    { section: "top", label: "Add account button", selector: "#btnAddNew", state: "active", probe: "active" },
    { section: "top", label: "Shortcut dropdown button", selector: "#shortcutDropdownBtn", state: "hover", probe: "hover" },
    { section: "top", label: "Shortcut dropdown button", selector: "#shortcutDropdownBtn", state: "active", probe: "active" },
    { section: "top", label: "Pinned shortcut icon button", selector: ".preview_shortcut_bar button.HasContextMenu", state: "hover", probe: "hover" },
    { section: "top", label: "Pinned shortcut icon button", selector: ".preview_shortcut_bar button.HasContextMenu", state: "active", probe: "active" },
    { section: "top", label: "Launch icon-only button", selector: "#btnStartPlat", state: "hover", probe: "hover" },
    { section: "top", label: "Launch icon-only button", selector: "#btnStartPlat", state: "active", probe: "active" },
    { section: "top", label: "Account actionbar input", selector: ".preview_accounts_actionbar #pvCurrentStatus", state: "hover", probe: "hover" },
    { section: "top", label: "Account actionbar input", selector: ".preview_accounts_actionbar #pvCurrentStatus", state: "active", probe: "active" },
    { section: "bottom", label: "Live toast button", selector: ".modalTestPanel .modalTestButtons .btnicontext:first-child", state: "hover", probe: "hover" },
    { section: "bottom", label: "Live toast button", selector: ".modalTestPanel .modalTestButtons .btnicontext:first-child", state: "active", probe: "active" },
    { section: "bottom", label: "Disabled preview button", selector: "#pvDisabledButton", state: "disabled", probe: "disabledControl" },
    { section: "bottom", label: "Prompt test button", selector: ".modalTestButtons button:nth-of-type(5)", state: "hover", probe: "hover" },
    { section: "bottom", label: "Prompt test button", selector: ".modalTestButtons button:nth-of-type(5)", state: "active", probe: "active" },
    { section: "bottom", label: "Live modal text input", selector: ".modalFG .modal-input", state: "hover", probe: "hover", prepare: "promptModal", cleanup: "promptModal" },
    { section: "bottom", label: "Live modal text input", selector: ".modalFG .modal-input", state: "active", probe: "active", prepare: "promptModal", cleanup: "promptModal" },
    { section: "bottom", label: "Settings dropdown toggle", selector: ".settingsCol .dropdown-toggle", state: "hover", probe: "hover" },
    { section: "bottom", label: "Settings dropdown toggle", selector: ".settingsCol .dropdown-toggle", state: "active", probe: "active" },
    { section: "bottom", label: "Settings number input", selector: "#pvImageExpiry", state: "hover", probe: "hover" },
    { section: "bottom", label: "Settings number input", selector: "#pvImageExpiry", state: "active", probe: "active" },
    { section: "bottom", label: "Settings text input", selector: "#pvText", state: "hover", probe: "hover" },
    { section: "bottom", label: "Settings text input", selector: "#pvText", state: "active", probe: "active" },
    { section: "bottom", label: "Settings text input", selector: "#pvText", state: "invalid", probe: "invalidControl" },
    { section: "bottom", label: "Settings disabled text input", selector: "#pvDisabledText", state: "disabled", probe: "disabledControl" },
    { section: "top", label: "Selected account row container", selector: ".preview_accounts_wrap input.acc:checked + label.acc", state: "selected", probe: "selectedAccount" },
    { section: "top", label: "Current account row container", selector: ".preview_accounts_wrap label.acc.currentAcc", state: "current", probe: "currentAccount" },
    { section: "top", label: "Normal account row container", selector: ".preview_accounts_wrap label.acc:not(.currentAcc)", state: "hover", probe: "hover" },
    { section: "top", label: "Disabled account edge row", selector: ".preview-account-edge--disabled", state: "disabled", probe: "classAttributeControlState", className: "preview-account-edge--disabled", attr: "aria-disabled", value: "true" },
    { section: "top", label: "Broken account edge row", selector: ".preview-account-edge--broken", state: "broken-data", probe: "classAttributeState", className: "acc--broken" },
    { section: "top", label: "Hover account edge row", selector: ".preview-account-edge--hover", state: "hover", probe: "classAttributeState", className: "preview-account-edge--hover" },
    { section: "top", label: "Account edge drop target row", selector: ".preview-account-edge--drop-target", state: "drop-target", probe: "classAttributeState", className: "acc--drop-target" },
    { section: "top", label: "Active accent dropdown item", selector: ".accent-dropdown-menu .accent-dropdown-item.active", state: "selected", probe: "activeClass", prepare: "accentDropdown", cleanup: "accentDropdown" },
    { section: "top", label: "Theme dropdown item", selector: ".theme-control-group > .rowDropdown:first-child .dropdown-item:first-child", state: "hover", probe: "hover", prepare: "themeDropdown", cleanup: "themeDropdown" },
    { section: "top", label: "Accent dropdown item", selector: ".accent-dropdown-menu .accent-dropdown-item:first-child", state: "hover", probe: "hover", prepare: "accentDropdown", cleanup: "accentDropdown" },
    { section: "bottom", label: "Settings dropdown item", selector: ".settingsCol .dropdown-item:first-child", state: "hover", probe: "hover" },
    { section: "bottom", label: "Settings checkbox control", selector: "#pvTray + .form-check-label", state: "disabled", probe: "disabledLabel" },
    { section: "bottom", label: "Disabled settings checkbox input", selector: "#pvTray", state: "disabled", probe: "disabledControl" },
    { section: "top", label: "Theme dropdown toggle", selector: ".theme-control-group > .rowDropdown:first-child .dropdown-toggle", state: "focus-visible", probe: "focusVisible" },
    { section: "top", label: "Platform manage button", selector: ".preview_fake_actionbar .btnicontext", state: "focus-visible", probe: "focusVisible" },
    { section: "top", label: "Add account button", selector: "#btnAddNew", state: "focus-visible", probe: "focusVisible" },
    { section: "top", label: "Launch icon-only button", selector: "#btnStartPlat", state: "focus-visible", probe: "focusVisible" },
    { section: "top", label: "Account actionbar input", selector: ".preview_accounts_actionbar #pvCurrentStatus", state: "focus-visible", probe: "focusVisible" },
    { section: "bottom", label: "Live modal text input", selector: ".modalFG .modal-input", state: "focus-visible", probe: "focusVisible", prepare: "promptModal", cleanup: "promptModal" },
    { section: "bottom", label: "Settings number input", selector: "#pvImageExpiry", state: "focus-visible", probe: "focusVisible" },
    { section: "bottom", label: "Settings text input", selector: "#pvText", state: "focus-visible", probe: "focusVisible" },
    { section: "bottom", label: "Settings dropdown toggle", selector: ".settingsCol .dropdown-toggle", state: "focus-visible", probe: "focusVisible" },
  ];

  function wait(ms = WAIT_MS) {
    return new Promise((resolve) => window.setTimeout(resolve, ms));
  }

  function compactText(value) {
    return (value || "").replace(/\s+/g, " ").trim();
  }

  function parseColor(value) {
    if (!value || value === "transparent") {
      return { r: 0, g: 0, b: 0, a: 0 };
    }

    const color = String(value).trim();
    const hex = color.match(/^#([0-9a-f]{3}|[0-9a-f]{6})$/i);
    if (hex) {
      const raw = hex[1].length === 3
        ? hex[1].split("").map((char) => char + char).join("")
        : hex[1];
      return {
        r: parseInt(raw.slice(0, 2), 16),
        g: parseInt(raw.slice(2, 4), 16),
        b: parseInt(raw.slice(4, 6), 16),
        a: 1,
      };
    }

    const rgb = color.match(/^rgba?\((.*)\)$/i);
    if (!rgb) {
      return null;
    }

    const parts = rgb[1]
      .replace(/\//g, " ")
      .split(/[,\s]+/)
      .map((part) => part.trim())
      .filter(Boolean);

    if (parts.length < 3) {
      return null;
    }

    const channel = (part) => {
      if (part.endsWith("%")) {
        return Math.round((parseFloat(part) / 100) * 255);
      }
      return Math.round(parseFloat(part));
    };

    const alpha = parts[3] === undefined
      ? 1
      : parts[3].endsWith("%")
        ? parseFloat(parts[3]) / 100
        : parseFloat(parts[3]);

    return {
      r: clamp(channel(parts[0]), 0, 255),
      g: clamp(channel(parts[1]), 0, 255),
      b: clamp(channel(parts[2]), 0, 255),
      a: clamp(Number.isFinite(alpha) ? alpha : 1, 0, 1),
    };
  }

  function clamp(value, min, max) {
    return Math.min(max, Math.max(min, value));
  }

  function composite(foreground, background) {
    const fg = foreground || { r: 0, g: 0, b: 0, a: 0 };
    const bg = background || { r: 255, g: 255, b: 255, a: 1 };
    const alpha = fg.a + bg.a * (1 - fg.a);

    if (alpha === 0) {
      return { r: 0, g: 0, b: 0, a: 0 };
    }

    return {
      r: Math.round((fg.r * fg.a + bg.r * bg.a * (1 - fg.a)) / alpha),
      g: Math.round((fg.g * fg.a + bg.g * bg.a * (1 - fg.a)) / alpha),
      b: Math.round((fg.b * fg.a + bg.b * bg.a * (1 - fg.a)) / alpha),
      a: alpha,
    };
  }

  function colorToString(color) {
    if (!color) {
      return null;
    }

    const alpha = Math.round(color.a * 1000) / 1000;
    if (alpha >= 1) {
      return `rgb(${color.r}, ${color.g}, ${color.b})`;
    }

    return `rgba(${color.r}, ${color.g}, ${color.b}, ${alpha})`;
  }

  function luminance(color) {
    const channel = (value) => {
      const srgb = value / 255;
      return srgb <= 0.03928
        ? srgb / 12.92
        : Math.pow((srgb + 0.055) / 1.055, 2.4);
    };

    return 0.2126 * channel(color.r)
      + 0.7152 * channel(color.g)
      + 0.0722 * channel(color.b);
  }

  function contrast(first, second) {
    if (!first || !second || first.a === 0 || second.a === 0) {
      return null;
    }

    const l1 = luminance(first);
    const l2 = luminance(second);
    const high = Math.max(l1, l2);
    const low = Math.min(l1, l2);
    return round2((high + 0.05) / (low + 0.05));
  }

  function round2(value) {
    if (value === null || value === undefined || Number.isNaN(value)) {
      return null;
    }
    return Math.round(value * 100) / 100;
  }

  function resolveBackground(element) {
    const chain = [];
    let current = element;
    while (current && current.nodeType === Node.ELEMENT_NODE) {
      chain.push(current);
      current = current.parentElement;
    }

    let background = { r: 255, g: 255, b: 255, a: 1 };
    let backgroundImage = false;
    for (const item of chain.reverse()) {
      const style = window.getComputedStyle(item);
      const parsed = parseColor(style.backgroundColor);
      if (parsed && parsed.a > 0) {
        background = composite(parsed, background);
      }
      if (style.backgroundImage && style.backgroundImage !== "none") {
        backgroundImage = true;
      }
    }

    return { color: background, hasImage: backgroundImage };
  }

  function findBorderColor(style) {
    const sides = ["Top", "Right", "Bottom", "Left"];
    for (const side of sides) {
      const width = parseFloat(style[`border${side}Width`]);
      const parsed = parseColor(style[`border${side}Color`]);
      if (width > 0 && parsed && parsed.a > 0) {
        return parsed;
      }
    }

    return null;
  }

  function borderWidthSummary(style) {
    return ["Top", "Right", "Bottom", "Left"]
      .map((side) => style[`border${side}Width`])
      .join(" ");
  }

  function hasBoxShadow(style) {
    return Boolean(style.boxShadow && style.boxShadow !== "none");
  }

  function backgroundImageNote(style, resolvedBackground) {
    if (style.backgroundImage && style.backgroundImage !== "none") {
      return "self";
    }
    return resolvedBackground.hasImage ? "ancestor" : "";
  }

  function hasOwnBackgroundImage(style) {
    return Boolean(style.backgroundImage && style.backgroundImage !== "none");
  }

  function bestContrastColor(colors, background) {
    let best = null;
    let bestContrast = -1;

    for (const color of colors) {
      if (!color || color.a === 0) {
        continue;
      }

      const ratio = contrast(color, background);
      if (ratio !== null && ratio > bestContrast) {
        best = color;
        bestContrast = ratio;
      }
    }

    return best;
  }

  function findFocusColor(style, background) {
    const outlineWidth = parseFloat(style.outlineWidth);
    const outlineColor = parseColor(style.outlineColor);
    if (outlineWidth > 0 && style.outlineStyle !== "none" && outlineColor && outlineColor.a > 0) {
      return outlineColor;
    }

    const shadow = style.boxShadow || "";
    const colors = Array.from(shadow.matchAll(/rgba?\([^)]+\)/gi))
      .map((match) => parseColor(match[0]));
    return bestContrastColor(colors, background);
  }

  function measureFocus(element, resolvedBackground) {
    const previous = document.activeElement;
    const previousScrollX = window.scrollX;
    const previousScrollY = window.scrollY;
    const hadTabindex = element.hasAttribute("tabindex");
    const previousTabindex = element.getAttribute("tabindex");
    const temporaryTabindex = previousTabindex === "-1";
    const notes = [];

    let focusColor = null;
    let focusContrast = null;
    let focusPass = null;
    let focused = false;

    if (temporaryTabindex) {
      element.setAttribute("tabindex", "0");
      notes.push("temporarily set tabindex=0 for focus probe");
    }

    try {
      element.focus({ preventScroll: true });
      focused = document.hasFocus() && document.activeElement === element && element.matches(":focus");
      if (focused) {
        const focusStyle = window.getComputedStyle(element);
        focusColor = findFocusColor(focusStyle, resolvedBackground.color);
        focusContrast = focusColor
          ? contrast(focusColor, resolvedBackground.color)
          : null;
        focusPass = focusColor ? pass(focusContrast, UI_THRESHOLD) : false;
      }
    } finally {
      if (temporaryTabindex) {
        if (hadTabindex) {
          element.setAttribute("tabindex", previousTabindex);
        } else {
          element.removeAttribute("tabindex");
        }
      }

      if (previous && previous !== element && typeof previous.focus === "function") {
        previous.focus({ preventScroll: true });
      } else if (typeof element.blur === "function") {
        element.blur();
      }

      window.scrollTo(previousScrollX, previousScrollY);
    }

    return {
      focusColor,
      focusContrast,
      focusPass,
      focused,
      notes,
    };
  }

  function isLargeText(style) {
    const fontSize = parseFloat(style.fontSize);
    const weightRaw = style.fontWeight;
    const weight = weightRaw === "bold" ? 700 : parseInt(weightRaw, 10) || 400;
    return fontSize >= 24 || (fontSize >= 18.66 && weight >= 700);
  }

  function pass(value, threshold) {
    if (value === null || value === undefined) {
      return null;
    }
    return value >= threshold;
  }

  function getThemeName() {
    const previewLabel = document.documentElement.getAttribute("data-preview-theme-label");
    if (previewLabel) {
      return previewLabel;
    }
    const toggle = document.querySelector(".theme-control-group > .rowDropdown:first-child .dropdown-toggle");
    const label = compactText(toggle && toggle.textContent);
    return label || document.documentElement.getAttribute("data-theme") || "unknown";
  }

  function getAccentName() {
    const label = document.querySelector(".accent-toggle-label");
    return compactText(label && label.textContent) || null;
  }

  function measureElement(sample, element) {
    const style = window.getComputedStyle(element);
    const placeholderStyle = sample.placeholder ? window.getComputedStyle(element, "::placeholder") : null;
    const resolvedBackground = resolveBackground(element);
    const foreground = parseColor(placeholderStyle ? placeholderStyle.color : style.color);
    const border = findBorderColor(style);
    const text = sample.placeholder
      ? compactText(element.getAttribute("placeholder"))
      : compactText(element.innerText || element.textContent);
    const hasVisibleText = !sample.uiOnly && text.length > 0 && foreground && foreground.a > 0;
    const ownBackgroundImage = hasOwnBackgroundImage(style);
    const textUnmeasured = hasVisibleText && ownBackgroundImage;
    const textThreshold = isLargeText(style) ? LARGE_TEXT_THRESHOLD : NORMAL_TEXT_THRESHOLD;
    const textContrast = hasVisibleText && !textUnmeasured
      ? contrast(foreground, resolvedBackground.color)
      : null;
    const borderContrast = border
      ? contrast(border, resolvedBackground.color)
      : null;

    let focusColor = null;
    let focusContrast = null;
    let focusPass = null;
    let focused = false;
    const focusNotes = [];

    if (sample.focus && typeof element.focus === "function") {
      const focusResult = measureFocus(element, resolvedBackground);
      focusColor = focusResult.focusColor;
      focusContrast = focusResult.focusContrast;
      focusPass = focusResult.focusPass;
      focused = focusResult.focused;
      focusNotes.push(...focusResult.notes);
    }

    const notes = [];
    notes.push(...focusNotes);
    const backgroundImageScope = backgroundImageNote(style, resolvedBackground);
    if (backgroundImageScope === "self") {
      notes.push("element background image ignored");
    } else if (backgroundImageScope === "ancestor") {
      notes.push("ancestor background image ignored");
    }
    if (textUnmeasured) {
      notes.push("text contrast unmeasured: element background image or gradient");
    }
    if (sample.focus && !focused) {
      notes.push("focus() did not move focus");
    }
    if (sample.focus && !focusColor) {
      notes.push("no outline or box-shadow color after focus()");
    }

    const textPass = pass(textContrast, textThreshold);
    const borderPass = pass(borderContrast, UI_THRESHOLD);
    const checks = [textPass, borderPass, focusPass].filter((value) => value !== null);
    const unresolved = [];
    if (textUnmeasured) {
      unresolved.push("textContrastBackgroundImage");
    }

    return {
      theme: getThemeName(),
      accent: getAccentName(),
      section: sample.section,
      label: sample.label,
      selector: sample.selector,
      state: sample.state || "",
      tag: element.tagName.toLowerCase(),
      role: element.getAttribute("role") || "",
      text: text.slice(0, 80),
      foreground: colorToString(foreground),
      background: colorToString(resolvedBackground.color),
      border: colorToString(border),
      focus: colorToString(focusColor),
      opacity: style.opacity,
      cursor: style.cursor,
      outlineStyle: style.outlineStyle,
      outlineWidth: style.outlineWidth,
      borderWidth: borderWidthSummary(style),
      boxShadow: hasBoxShadow(style),
      backgroundImage: backgroundImageScope,
      textContrastStatus: textUnmeasured ? "unmeasured-background-image" : (hasVisibleText ? "measured" : "not-applicable"),
      textContrast,
      borderContrast,
      focusContrast,
      textThreshold: hasVisibleText ? textThreshold : null,
      borderThreshold: border ? UI_THRESHOLD : null,
      focusThreshold: sample.focus ? UI_THRESHOLD : null,
      textPass,
      borderPass,
      focusPass,
      pass: unresolved.length ? false : (checks.length ? checks.every(Boolean) : null),
      unresolved: unresolved.join(", "),
      found: true,
      notes: notes.join("; "),
    };
  }

  async function preparePreviewStates() {
    if (window.location.hash !== ROUTE) {
      window.location.hash = ROUTE;
      await wait(250);
    }

    await ensureThemeDropdown();
    await clickToOpen("#shortcutDropdownBtn", "#shortcutDropdown #btnOpenShortcutFolder");
    await clickToOpen(".settingsCol .dropdown-toggle", ".settingsCol .dropdown-item");
    await ensureAccentDropdown();
  }

  async function prepareSample(sample) {
    if (!sample.prepare) {
      return;
    }

    const fn = SAMPLE_PREPARE[sample.prepare];
    if (fn) {
      await fn();
    }
  }

  async function cleanupSample(sample) {
    if (!sample.cleanup) {
      return;
    }

    const fn = SAMPLE_CLEANUP[sample.cleanup];
    if (fn) {
      await fn();
    }
  }

  async function clickToOpen(buttonSelector, visibleSelector) {
    if (document.querySelector(visibleSelector)) {
      return;
    }

    const button = document.querySelector(buttonSelector);
    if (!button) {
      return;
    }

    button.click();
    await wait();
  }

  async function clickToClose(buttonSelector, visibleSelector) {
    if (!document.querySelector(visibleSelector)) {
      return;
    }

    const button = document.querySelector(buttonSelector);
    if (!button) {
      return;
    }

    button.click();
    await wait();
  }

  function removeAuditDropdownFixture(scope = document) {
    scope.querySelectorAll("[data-preview-audit-dropdown-fixture]").forEach((node) => node.remove());
  }

  async function ensureThemeDropdown() {
    await clickToOpen(
      ".theme-control-group > .rowDropdown:first-child .dropdown-toggle",
      ".theme-control-group > .rowDropdown:first-child .dropdown-item"
    );
    if (document.querySelector(".theme-control-group > .rowDropdown:first-child .dropdown-item")) {
      return;
    }

    const dropdown = document.querySelector(".theme-control-group > .rowDropdown:first-child .dropdown");
    if (!dropdown) {
      return;
    }
    removeAuditDropdownFixture(dropdown);

    const menu = document.createElement("ul");
    menu.className = "custom-dropdown-menu dropdown-menu";
    menu.setAttribute("data-preview-audit-dropdown-fixture", "");
    const item = document.createElement("li");
    item.setAttribute("role", "none");
    const button = document.createElement("button");
    button.type = "button";
    button.className = "dropdown-item";
    button.textContent = getThemeName();
    item.appendChild(button);
    menu.appendChild(item);
    dropdown.appendChild(menu);
  }

  async function ensureAccentDropdown() {
    await clickToOpen(".accent-dropdown .accent-toggle", ".accent-dropdown-menu .accent-dropdown-item");
    if (document.querySelector(".accent-dropdown-menu .accent-dropdown-item")) {
      return;
    }

    const dropdown = document.querySelector(".accent-dropdown");
    if (!dropdown) {
      return;
    }
    removeAuditDropdownFixture(dropdown);

    const menu = document.createElement("ul");
    menu.className = "custom-dropdown-menu dropdown-menu accent-dropdown-menu";
    menu.setAttribute("data-preview-audit-dropdown-fixture", "");
    const item = document.createElement("li");
    item.setAttribute("role", "none");
    const button = document.createElement("button");
    button.type = "button";
    button.className = "dropdown-item accent-dropdown-item active";
    const chip = document.createElement("span");
    chip.className = "theme-accent-chip";
    chip.style.setProperty(
      "--accent-chip-color",
      getComputedStyle(document.documentElement).getPropertyValue("--accent").trim() || "#80ffea"
    );
    const label = document.createElement("span");
    label.textContent = getAccentName() || "Accent";
    button.append(chip, label);
    item.appendChild(button);
    menu.appendChild(item);
    dropdown.appendChild(menu);
  }

  function dispatchEscape(target = document) {
    target.dispatchEvent(new KeyboardEvent("keydown", {
      key: "Escape",
      bubbles: true,
      cancelable: true,
    }));
  }

  async function closeContextMenuSurface() {
    if (!document.querySelector(".ctx-menu-root.contextmenu")) {
      return;
    }

    dispatchEscape(document.querySelector(".ctx-menu-root.contextmenu") || document);
    await wait();
  }

  async function openContextMenuSurface() {
    if (document.querySelector(".ctx-menu-root.contextmenu")) {
      return;
    }

    const target = document.querySelector(".preview_program_main .platform_tile_ctx");
    if (!target) {
      return;
    }

    const rect = target.getBoundingClientRect();
    target.dispatchEvent(new MouseEvent("contextmenu", {
      bubbles: true,
      cancelable: true,
      clientX: rect.left + Math.min(24, rect.width / 2),
      clientY: rect.top + Math.min(24, rect.height / 2),
    }));
    await wait(160);
  }

  async function openPromptModal() {
    if (document.querySelector(".modalFG")) {
      return;
    }

    const modalStore = await getModalStore();
    if (modalStore && typeof modalStore.openPrompt === "function") {
      void modalStore.openPrompt({
        title: "Preview Modals",
        body: "Change username",
        inputType: "text",
        initialValue: "demo",
      });
      await wait(250);
      if (document.querySelector(".modalFG .modal-input")) {
        return;
      }
    }

    const fixture = document.createElement("div");
    fixture.className = "modalBG";
    fixture.setAttribute("data-preview-audit-modal-fixture", "");
    fixture.innerHTML = `
      <div class="modalFG modalFG--ready" role="dialog" aria-label="Preview Modals">
        <div class="modal-scroll">
          <div class="modal-block">
            <div class="modal-input-row">
              <input type="text" class="modal-input" autocomplete="off" aria-label="Change username" value="demo">
            </div>
          </div>
        </div>
      </div>
    `;
    document.body.appendChild(fixture);
    await wait(160);
    if (document.querySelector(".modalFG .modal-input")) {
      return;
    }

    const modalPanel = document.querySelector(".modalTestOutput")?.closest(".modalTestPanel");
    const button = modalPanel?.querySelector(".modalTestButtons button:nth-of-type(5)")
      || Array.from(document.querySelectorAll(".modalTestButtons button"))
        .find((candidate) => compactText(candidate.textContent).toLowerCase().includes("prompt"));
    if (!button) {
      return;
    }

    button.click();
    await wait(250);
  }

  async function closePromptModal() {
    const fixture = document.querySelector("[data-preview-audit-modal-fixture]");
    if (fixture) {
      fixture.remove();
      await wait(80);
      return;
    }

    const modalStore = await getModalStore();
    if (modalStore && typeof modalStore.dismissModal === "function" && document.querySelector(".modalFG")) {
      modalStore.dismissModal(null);
      await wait(160);
      return;
    }

    const closeButton = document.querySelector(".modalFG .win-btn-close");
    if (closeButton) {
      closeButton.click();
      await wait(160);
      return;
    }

    if (document.querySelector(".modalFG")) {
      dispatchEscape(document.querySelector(".modalFG"));
      await wait(160);
    }
  }

  const SAMPLE_PREPARE = {
    accentDropdown: ensureAccentDropdown,
    themeDropdown: ensureThemeDropdown,
    contextMenu: openContextMenuSurface,
    promptModal: openPromptModal,
  };

  const SAMPLE_CLEANUP = {
    accentDropdown: async () => {
      removeAuditDropdownFixture();
      await clickToClose(".accent-dropdown .accent-toggle", ".accent-dropdown-menu .accent-dropdown-item");
    },
    themeDropdown: async () => {
      removeAuditDropdownFixture();
      await clickToClose(
        ".theme-control-group > .rowDropdown:first-child .dropdown-toggle",
        ".theme-control-group > .rowDropdown:first-child .dropdown-item"
      );
    },
    contextMenu: closeContextMenuSurface,
    promptModal: closePromptModal,
  };

  function missingResult(sample) {
    return {
      theme: getThemeName(),
      accent: getAccentName(),
      section: sample.section,
      label: sample.label,
      selector: sample.selector,
      state: sample.state || "",
      tag: "",
      role: "",
      text: "",
      foreground: null,
      background: null,
      border: null,
      focus: null,
      opacity: null,
      cursor: null,
      outlineStyle: null,
      outlineWidth: null,
      borderWidth: null,
      boxShadow: null,
      backgroundImage: null,
      textContrastStatus: "not-applicable",
      textContrast: null,
      borderContrast: null,
      focusContrast: null,
      textThreshold: null,
      borderThreshold: null,
      focusThreshold: sample.focus ? UI_THRESHOLD : null,
      textPass: null,
      borderPass: null,
      focusPass: null,
      pass: false,
      unresolved: "",
      found: false,
      notes: "selector not found",
    };
  }

  function sideSummary(style, prefix, suffix) {
    return ["Top", "Right", "Bottom", "Left"]
      .map((side) => style[`${prefix}${side}${suffix}`])
      .join(" ");
  }

  function sideColorSummary(style) {
    return ["Top", "Right", "Bottom", "Left"]
      .map((side) => style[`border${side}Color`])
      .join(" | ");
  }

  function iconCount(element) {
    return element.querySelectorAll("svg, i, [class*='icon'], [class*='Icon']").length;
  }

  function pickElementStateSnapshot(element) {
    const style = window.getComputedStyle(element);
    const resolvedBackground = resolveBackground(element);
    const foreground = parseColor(style.color);
    const border = findBorderColor(style);
    const text = compactText(element.innerText || element.textContent).slice(0, 80);
    const ownBackgroundImage = hasOwnBackgroundImage(style);
    const textContrast = text && !ownBackgroundImage && foreground && foreground.a > 0
      ? contrast(foreground, resolvedBackground.color)
      : null;
    const borderContrast = border ? contrast(border, resolvedBackground.color) : null;

    return {
      tag: element.tagName.toLowerCase(),
      role: element.getAttribute("role") || "",
      text,
      iconCount: iconCount(element),
      classList: Array.from(element.classList).join(" "),
      ariaPressed: element.getAttribute("aria-pressed") || "",
      ariaCurrent: element.getAttribute("aria-current") || "",
      ariaSelected: element.getAttribute("aria-selected") || "",
      ariaExpanded: element.getAttribute("aria-expanded") || "",
      ariaDisabled: element.getAttribute("aria-disabled") || "",
      disabled: "disabled" in element ? Boolean(element.disabled) : false,
      checked: "checked" in element ? Boolean(element.checked) : false,
      foreground: colorToString(foreground),
      background: colorToString(resolvedBackground.color),
      borderColor: colorToString(border),
      borderColorSides: sideColorSummary(style),
      borderWidth: borderWidthSummary(style),
      borderStyle: sideSummary(style, "border", "Style"),
      outlineColor: style.outlineColor,
      outlineWidth: style.outlineWidth,
      outlineStyle: style.outlineStyle,
      boxShadow: style.boxShadow,
      hasBoxShadow: hasBoxShadow(style),
      backgroundImage: style.backgroundImage,
      opacity: style.opacity,
      cursor: style.cursor,
      transform: style.transform,
      filter: style.filter,
      textContrast,
      borderContrast,
    };
  }

  function valueChanged(before, after) {
    return String(before ?? "") !== String(after ?? "");
  }

  function recordCue(list, name, before, after) {
    if (valueChanged(before, after)) {
      list.push(`${name}: ${valueOrBlank(before)} -> ${valueOrBlank(after)}`);
    }
  }

  function diffStateSnapshots(baseline, state) {
    const nonColorCues = [];
    const colorChanges = [];

    recordCue(nonColorCues, "borderWidth", baseline.borderWidth, state.borderWidth);
    recordCue(nonColorCues, "borderStyle", baseline.borderStyle, state.borderStyle);
    recordCue(nonColorCues, "outlineWidth", baseline.outlineWidth, state.outlineWidth);
    recordCue(nonColorCues, "outlineStyle", baseline.outlineStyle, state.outlineStyle);
    recordCue(nonColorCues, "boxShadow", baseline.boxShadow, state.boxShadow);
    recordCue(nonColorCues, "backgroundImage", baseline.backgroundImage, state.backgroundImage);
    recordCue(nonColorCues, "opacity", baseline.opacity, state.opacity);
    recordCue(nonColorCues, "cursor", baseline.cursor, state.cursor);
    recordCue(nonColorCues, "transform", baseline.transform, state.transform);
    recordCue(nonColorCues, "filter", baseline.filter, state.filter);
    recordCue(nonColorCues, "text", baseline.text, state.text);
    recordCue(nonColorCues, "iconCount", baseline.iconCount, state.iconCount);
    recordCue(nonColorCues, "classList", baseline.classList, state.classList);
    recordCue(nonColorCues, "ariaPressed", baseline.ariaPressed, state.ariaPressed);
    recordCue(nonColorCues, "ariaCurrent", baseline.ariaCurrent, state.ariaCurrent);
    recordCue(nonColorCues, "ariaSelected", baseline.ariaSelected, state.ariaSelected);
    recordCue(nonColorCues, "ariaExpanded", baseline.ariaExpanded, state.ariaExpanded);
    recordCue(nonColorCues, "ariaDisabled", baseline.ariaDisabled, state.ariaDisabled);
    recordCue(nonColorCues, "disabled", baseline.disabled, state.disabled);
    recordCue(nonColorCues, "checked", baseline.checked, state.checked);

    recordCue(colorChanges, "foreground", baseline.foreground, state.foreground);
    recordCue(colorChanges, "background", baseline.background, state.background);
    recordCue(colorChanges, "borderColor", baseline.borderColor, state.borderColor);
    recordCue(colorChanges, "borderColorSides", baseline.borderColorSides, state.borderColorSides);
    recordCue(colorChanges, "outlineColor", baseline.outlineColor, state.outlineColor);
    recordCue(colorChanges, "textContrast", baseline.textContrast, state.textContrast);
    recordCue(colorChanges, "borderContrast", baseline.borderContrast, state.borderContrast);

    return {
      nonColorCues,
      colorChanges,
      changed: nonColorCues.length > 0 || colorChanges.length > 0,
      hueOnly: nonColorCues.length === 0 && colorChanges.length > 0,
    };
  }

  function ensureForcedPseudoStyles(pseudo) {
    const id = `${FORCED_STATE_CLASS_PREFIX}${pseudo}-style`;
    let styleElement = document.getElementById(id);
    if (styleElement) {
      return {
        className: `${FORCED_STATE_CLASS_PREFIX}${pseudo}`,
        ruleCount: Number(styleElement.getAttribute("data-rule-count") || 0),
      };
    }

    const className = `${FORCED_STATE_CLASS_PREFIX}${pseudo}`;
    const replacePattern = new RegExp(`:${pseudo}(?![a-z-])`, "g");
    const wrapRules = (rules) => {
      const chunks = [];
      for (const rule of Array.from(rules || [])) {
        try {
          if (rule.type === CSSRule.STYLE_RULE) {
            replacePattern.lastIndex = 0;
            if (!rule.selectorText || !replacePattern.test(rule.selectorText)) {
              continue;
            }
            replacePattern.lastIndex = 0;
            chunks.push(`${rule.selectorText.replace(replacePattern, `.${className}`)} { ${rule.style.cssText} }`);
            continue;
          }
          if (rule.type === CSSRule.MEDIA_RULE || rule.type === CSSRule.SUPPORTS_RULE) {
            const nested = wrapRules(rule.cssRules);
            if (nested.length) {
              const atRule = rule.type === CSSRule.MEDIA_RULE ? "@media" : "@supports";
              chunks.push(`${atRule} ${rule.conditionText} { ${nested.join(" ")} }`);
            }
          }
        } catch (_error) {
          continue;
        }
      }
      return chunks;
    };

    const cssText = [];
    for (const sheet of Array.from(document.styleSheets)) {
      try {
        cssText.push(...wrapRules(sheet.cssRules));
      } catch (_error) {
        continue;
      }
    }

    styleElement = document.createElement("style");
    styleElement.id = id;
    styleElement.setAttribute("data-rule-count", String(cssText.length));
    styleElement.textContent = cssText.join("\n");
    document.head.appendChild(styleElement);
    return { className, ruleCount: cssText.length };
  }

  function dispatchMouseState(element, type) {
    const rect = element.getBoundingClientRect();
    const point = {
      clientX: rect.left + Math.min(12, rect.width / 2 || 0),
      clientY: rect.top + Math.min(12, rect.height / 2 || 0),
    };
    const events = type === "active"
      ? ["pointerdown", "mousedown"]
      : ["pointerover", "mouseover", "mouseenter", "pointerenter"];

    for (const name of events) {
      element.dispatchEvent(new MouseEvent(name, {
        bubbles: true,
        cancelable: true,
        buttons: type === "active" ? 1 : 0,
        ...point,
      }));
    }
  }

  function applyForcedPseudoState(element, pseudo) {
    const forced = ensureForcedPseudoStyles(pseudo);
    dispatchMouseState(element, pseudo);
    element.classList.add(forced.className);
    return {
      cleanup: () => {
        element.classList.remove(forced.className);
        if (pseudo === "active") {
          element.dispatchEvent(new MouseEvent("mouseup", { bubbles: true, cancelable: true, buttons: 0 }));
        }
      },
      notes: forced.ruleCount ? [] : [`no :${pseudo} rules found to mirror`],
      unresolved: forced.ruleCount === 0,
    };
  }

  function applyFocusVisibleState(element) {
    const forced = ensureForcedPseudoStyles("focus-visible");
    const previous = document.activeElement;
    element.classList.add(forced.className);
    if (typeof element.focus === "function") {
      element.focus({ preventScroll: true });
    }
    return {
      cleanup: () => {
        element.classList.remove(forced.className);
        if (previous && previous !== element && typeof previous.focus === "function") {
          previous.focus({ preventScroll: true });
        } else if (typeof element.blur === "function") {
          element.blur();
        }
      },
      notes: forced.ruleCount ? [] : ["no :focus-visible rules found to mirror"],
      unresolved: forced.ruleCount === 0,
    };
  }

  function inputForElement(element) {
    if (element instanceof HTMLInputElement) {
      return element;
    }
    if (element instanceof HTMLLabelElement && element.control instanceof HTMLInputElement) {
      return element.control;
    }
    const sibling = element.previousElementSibling;
    return sibling instanceof HTMLInputElement ? sibling : null;
  }

  function setCheckedState(element, checked) {
    const input = inputForElement(element);
    if (!input) {
      return { cleanup: () => {}, notes: ["checkbox input not found"], unresolved: true };
    }

    const previous = input.checked;
    input.checked = checked;
    return {
      cleanup: () => {
        input.checked = previous;
      },
      notes: [],
      unresolved: false,
    };
  }

  function setDisabledState(element, disabled) {
    const control = inputForElement(element)
      || (element instanceof HTMLButtonElement ? element : null)
      || (element instanceof HTMLInputElement ? element : null)
      || (element instanceof HTMLSelectElement ? element : null)
      || (element instanceof HTMLTextAreaElement ? element : null);
    if (!control) {
      return { cleanup: () => {}, notes: ["disabled control not found"], unresolved: true };
    }

    const previous = control.disabled;
    control.disabled = disabled;
    return {
      cleanup: () => {
        control.disabled = previous;
      },
      notes: [],
      unresolved: false,
    };
  }

  function setInvalidState(element, invalid) {
    const control = inputForElement(element)
      || (element instanceof HTMLInputElement ? element : null)
      || (element instanceof HTMLSelectElement ? element : null)
      || (element instanceof HTMLTextAreaElement ? element : null);
    if (!control) {
      return { cleanup: () => {}, notes: ["invalid control not found"], unresolved: true };
    }

    const hadAriaInvalid = control.hasAttribute("aria-invalid");
    const previousAriaInvalid = control.getAttribute("aria-invalid");
    control.setAttribute("aria-invalid", invalid ? "true" : "false");
    return {
      cleanup: () => {
        if (hadAriaInvalid) {
          control.setAttribute("aria-invalid", previousAriaInvalid);
        } else {
          control.removeAttribute("aria-invalid");
        }
      },
      notes: [],
      unresolved: false,
    };
  }

  function setClassState(element, className, enabled) {
    const previous = element.classList.contains(className);
    element.classList.toggle(className, enabled);
    return {
      cleanup: () => {
        element.classList.toggle(className, previous);
      },
      notes: [],
      unresolved: false,
    };
  }

  function setAttributeState(element, attr, enabled, value = "true", offValue = null) {
    if (!attr) {
      return { cleanup: () => {}, notes: [], unresolved: false };
    }

    const hadAttribute = element.hasAttribute(attr);
    const previousValue = element.getAttribute(attr);
    if (enabled) {
      element.setAttribute(attr, value);
    } else if (offValue === null || offValue === undefined) {
      element.removeAttribute(attr);
    } else {
      element.setAttribute(attr, offValue);
    }

    return {
      cleanup: () => {
        if (hadAttribute) {
          element.setAttribute(attr, previousValue);
        } else {
          element.removeAttribute(attr);
        }
      },
      notes: [],
      unresolved: false,
    };
  }

  function setClassAttributeState(element, mode, sample) {
    const enabled = mode === "state";
    const classState = sample.className
      ? setClassState(element, sample.className, enabled)
      : { cleanup: () => {}, notes: [], unresolved: false };
    const attrState = setAttributeState(element, sample.attr, enabled, sample.value, sample.offValue);
    return {
      cleanup: () => {
        attrState.cleanup();
        classState.cleanup();
      },
      notes: [...classState.notes, ...attrState.notes],
      unresolved: classState.unresolved || attrState.unresolved,
    };
  }

  function setClassAttributeControlState(element, mode, sample) {
    const classAttrState = setClassAttributeState(element, mode, sample);
    const disabledState = setDisabledState(element, mode === "state");
    return {
      cleanup: () => {
        disabledState.cleanup();
        classAttrState.cleanup();
      },
      notes: [...classAttrState.notes, ...disabledState.notes],
      unresolved: classAttrState.unresolved || disabledState.unresolved,
    };
  }

  const STATE_PROBES = {
    hover: (element, mode) => mode === "state"
      ? applyForcedPseudoState(element, "hover")
      : { cleanup: () => {}, notes: [], unresolved: false },
    active: (element, mode) => mode === "state"
      ? applyForcedPseudoState(element, "active")
      : { cleanup: () => {}, notes: [], unresolved: false },
    focusVisible: (element, mode) => mode === "state"
      ? applyFocusVisibleState(element)
      : { cleanup: () => {}, notes: [], unresolved: false },
    selectedAccount: (element, mode) => setCheckedState(element, mode === "state"),
    currentAccount: (element, mode) => setClassState(element, "currentAcc", mode === "state"),
    activeClass: (element, mode) => setClassState(element, "active", mode === "state"),
    classAttributeState: setClassAttributeState,
    classAttributeControlState: setClassAttributeControlState,
    disabledControl: (element, mode) => setDisabledState(element, mode === "state"),
    disabledLabel: (element, mode) => setDisabledState(element, mode === "state"),
    invalidControl: (element, mode) => setInvalidState(element, mode === "state"),
  };

  function missingStateResult(sample, notes) {
    return {
      theme: getThemeName(),
      accent: getAccentName(),
      section: sample.section,
      label: sample.label,
      selector: sample.selector,
      state: sample.state,
      probe: sample.probe,
      found: false,
      pass: false,
      unresolved: "selector not found",
      nonColorCues: [],
      colorChanges: [],
      baseline: null,
      stateSnapshot: null,
      notes: notes.join("; ") || "selector not found",
    };
  }

  async function measureStateSample(sample) {
    await prepareSample(sample);
    const element = document.querySelector(sample.selector);
    if (!element) {
      await cleanupSample(sample);
      return missingStateResult(sample, []);
    }

    const probe = STATE_PROBES[sample.probe];
    if (!probe) {
      await cleanupSample(sample);
      return missingStateResult(sample, [`unknown probe: ${sample.probe}`]);
    }

    const baselineState = probe(element, "baseline", sample);
    const stateNotes = [];
    let baseline;
    let stateSnapshot;
    let delta;
    try {
      await wait();
      baseline = pickElementStateSnapshot(element);
      const activeState = probe(element, "state", sample);
      try {
        await wait();
        stateSnapshot = pickElementStateSnapshot(element);
        delta = diffStateSnapshots(baseline, stateSnapshot);
        stateNotes.push(...baselineState.notes, ...activeState.notes);
        if (!delta.changed) {
          stateNotes.push("no measurable state delta");
        }
        if (delta.hueOnly) {
          stateNotes.push("state changed by color only");
        }
        return {
          theme: getThemeName(),
          accent: getAccentName(),
          section: sample.section,
          label: sample.label,
          selector: sample.selector,
          state: sample.state,
          probe: sample.probe,
          found: true,
          pass: delta.nonColorCues.length > 0 && !baselineState.unresolved && !activeState.unresolved,
          unresolved: [
            baselineState.unresolved ? "baseline-unprobeable" : "",
            activeState.unresolved ? "state-unprobeable" : "",
          ].filter(Boolean).join(", "),
          nonColorCues: delta.nonColorCues,
          colorChanges: delta.colorChanges,
          baseline,
          stateSnapshot,
          notes: stateNotes.join("; "),
        };
      } finally {
        activeState.cleanup();
      }
    } finally {
      baselineState.cleanup();
      await cleanupSample(sample);
    }
  }

  function stateTableRow(result) {
    return {
      theme: result.theme,
      section: result.section,
      label: result.label,
      selector: result.selector,
      state: result.state,
      probe: result.probe,
      pass: result.pass,
      nonColorCues: result.nonColorCues.join("; "),
      colorChanges: result.colorChanges.join("; "),
      unresolved: result.unresolved,
      baselineClasses: result.baseline ? result.baseline.classList : "",
      stateClasses: result.stateSnapshot ? result.stateSnapshot.classList : "",
      baselineText: result.baseline ? result.baseline.text : "",
      stateText: result.stateSnapshot ? result.stateSnapshot.text : "",
      notes: result.notes,
    };
  }

  function stateMarkdown(results) {
    const header = [
      "theme",
      "section",
      "label",
      "selector",
      "state",
      "probe",
      "pass",
      "nonColorCues",
      "colorChanges",
      "unresolved",
      "baselineClasses",
      "stateClasses",
      "baselineText",
      "stateText",
      "notes",
    ];

    const rows = results.map((result) => [
      result.theme,
      result.section,
      result.label,
      result.selector,
      result.state,
      result.probe,
      String(result.pass),
      result.nonColorCues.join("; "),
      result.colorChanges.join("; "),
      result.unresolved || "",
      result.baseline ? result.baseline.classList : "",
      result.stateSnapshot ? result.stateSnapshot.classList : "",
      result.baseline ? result.baseline.text : "",
      result.stateSnapshot ? result.stateSnapshot.text : "",
      result.notes || "",
    ]);

    return [
      `| ${header.join(" | ")} |`,
      `| ${header.map(() => "---").join(" | ")} |`,
      ...rows.map((row) => `| ${row.map(escapeMarkdownCell).join(" | ")} |`),
    ].join("\n");
  }

  function printStateResults(results, options) {
    const table = results.map(stateTableRow);
    const report = stateMarkdown(results);
    window.PreviewCssContrastSampler.stateLast = {
      generatedAt: new Date().toISOString(),
      route: window.location.href,
      zoom: options.zoom || "",
      results,
      failures: results.filter((result) => result.pass === false),
      markdown: report,
    };

    if (options.log !== false) {
      console.table(table);
      console.log("PreviewCss state matrix JSON:");
      console.log(JSON.stringify(window.PreviewCssContrastSampler.stateLast, null, 2));
      console.log("PreviewCss state matrix Markdown:");
      console.log(report);
    }

    return window.PreviewCssContrastSampler.stateLast;
  }

  function summarizeForTable(result) {
    return {
      theme: result.theme,
      section: result.section,
      label: result.label,
      selector: result.selector,
      state: result.state,
      tag: result.tag,
      role: result.role,
      textContrast: result.textContrast,
      borderContrast: result.borderContrast,
      focusContrast: result.focusContrast,
      threshold: [
        result.textThreshold ? `text>=${result.textThreshold}` : null,
        result.borderThreshold ? `border>=${result.borderThreshold}` : null,
        result.focusThreshold ? `focus>=${result.focusThreshold}` : null,
      ].filter(Boolean).join(", "),
      pass: result.pass,
      foreground: result.foreground,
      background: result.background,
      border: result.border,
      focus: result.focus,
      opacity: result.opacity,
      cursor: result.cursor,
      outline: [result.outlineStyle, result.outlineWidth].filter(Boolean).join(" "),
      borderWidth: result.borderWidth,
      boxShadow: result.boxShadow,
      backgroundImage: result.backgroundImage,
      textContrastStatus: result.textContrastStatus,
      unresolved: result.unresolved,
      notes: result.notes,
    };
  }

  function markdown(results) {
    const header = [
      "theme",
      "section",
      "label",
      "selector",
      "state",
      "tag",
      "role",
      "textContrast",
      "borderContrast",
      "focusContrast",
      "threshold",
      "pass",
      "foreground",
      "background",
      "border",
      "focus",
      "opacity",
      "cursor",
      "outline",
      "borderWidth",
      "boxShadow",
      "backgroundImage",
      "textContrastStatus",
      "unresolved",
      "notes",
    ];

    const rows = results.map((result) => [
      result.theme,
      result.section,
      result.label,
      result.selector,
      result.state,
      result.tag || "",
      result.role || "",
      valueOrBlank(result.textContrast),
      valueOrBlank(result.borderContrast),
      valueOrBlank(result.focusContrast),
      [
        result.textThreshold ? `text>=${result.textThreshold}` : null,
        result.borderThreshold ? `border>=${result.borderThreshold}` : null,
        result.focusThreshold ? `focus>=${result.focusThreshold}` : null,
      ].filter(Boolean).join(", "),
      String(result.pass),
      result.foreground || "",
      result.background || "",
      result.border || "",
      result.focus || "",
      result.opacity || "",
      result.cursor || "",
      [result.outlineStyle, result.outlineWidth].filter(Boolean).join(" "),
      result.borderWidth || "",
      valueOrBlank(result.boxShadow),
      result.backgroundImage || "",
      result.textContrastStatus || "",
      result.unresolved || "",
      result.notes || "",
    ]);

    return [
      `| ${header.join(" | ")} |`,
      `| ${header.map(() => "---").join(" | ")} |`,
      ...rows.map((row) => `| ${row.map(escapeMarkdownCell).join(" | ")} |`),
    ].join("\n");
  }

  function escapeMarkdownCell(value) {
    return String(value).replace(/\|/g, "\\|").replace(/\n/g, " ");
  }

  function valueOrBlank(value) {
    return value === null || value === undefined ? "" : value;
  }

  function printResults(results, options) {
    const table = results.map(summarizeForTable);
    const report = markdown(results);

    window.PreviewCssContrastSampler.last = {
      generatedAt: new Date().toISOString(),
      route: window.location.href,
      results,
      failures: results.filter((result) => result.pass === false),
      markdown: report,
    };

    if (options.log !== false) {
      console.table(table);
      console.log("PreviewCss contrast sampler JSON:");
      console.log(JSON.stringify(window.PreviewCssContrastSampler.last, null, 2));
      console.log("PreviewCss contrast sampler Markdown:");
      console.log(report);
    }

    return window.PreviewCssContrastSampler.last;
  }

  async function run(options = {}) {
    const runOptions = { prepare: true, log: true, ...options };
    if (runOptions.prepare) {
      await preparePreviewStates();
    }

    const results = [];
    for (const sample of SAMPLES) {
      await prepareSample(sample);
      const element = document.querySelector(sample.selector);
      results.push(element ? measureElement(sample, element) : missingResult(sample));
      await cleanupSample(sample);
    }

    return printResults(results, runOptions);
  }

  let themeCatalogPromise = null;
  let themeDomPromise = null;
  let themeStoresPromise = null;
  let modalStorePromise = null;

  async function getThemeCatalog() {
    if (!themeCatalogPromise) {
      themeCatalogPromise = import("/src/lib/theme/catalog.ts").catch(() => null);
    }
    return themeCatalogPromise;
  }

  async function getThemeDom() {
    if (!themeDomPromise) {
      themeDomPromise = import("/src/lib/theme/dom.ts").catch(() => null);
    }
    return themeDomPromise;
  }

  async function getThemeStores() {
    if (!themeStoresPromise) {
      themeStoresPromise = import("/src/lib/theme/stores.ts").catch(() => null);
    }
    return themeStoresPromise;
  }

  async function getModalStore() {
    if (!modalStorePromise) {
      modalStorePromise = import("/src/stores/modal.ts").catch(() => null);
    }
    return modalStorePromise;
  }

  async function applyThemeFromCatalog(label) {
    const catalog = await getThemeCatalog();
    if (!catalog || typeof catalog.listThemes !== "function") {
      return false;
    }

    const theme = catalog.listThemes().find((candidate) => candidate.label === label);
    if (!theme) {
      throw new Error(`Theme option not found: ${label}`);
    }

    const dom = await getThemeDom();
    const stores = await getThemeStores();
    dom?.removeThemeOverlay?.();
    dom?.removeAccentOverlay?.();
    dom?.removeThemeGoogleFontLinks?.();
    document.documentElement.setAttribute("data-preview-theme-label", theme.label);

    if (theme.id !== "default") {
      const key = catalog.styleLoaderPathForId?.(theme.id);
      const load = key ? catalog.themeStyles?.[key] : null;
      if (typeof load !== "function") {
        throw new Error(`Theme style loader not found: ${theme.id}`);
      }
      const css = await load();
      const style = document.createElement("style");
      style.id = "tcno-theme-overlay";
      style.setAttribute("data-tcno-theme-overlay", "");
      style.textContent = css;
      document.head.appendChild(style);
    }

    stores?.currentThemeId?.set?.(theme.id);
    stores?.currentThemeBgUrl?.set?.(theme.backgroundUrl ?? "");
    stores?.currentThemeAccentKey?.set?.("");
    stores?.currentThemeCustomAccentColor?.set?.("");
    await wait(180);
    return true;
  }

  async function waitForThemeLabel(label) {
    const startedAt = performance.now();
    while (performance.now() - startedAt < 1500) {
      if (getThemeName() === label) {
        return;
      }
      await wait(80);
    }
  }

  async function readThemeLabels() {
    const catalog = await getThemeCatalog();
    if (catalog && typeof catalog.listThemes === "function") {
      return catalog.listThemes().map((theme) => theme.label).filter(Boolean);
    }

    await clickToOpen(
      ".theme-control-group > .rowDropdown:first-child .dropdown-toggle",
      ".theme-control-group > .rowDropdown:first-child .dropdown-item"
    );

    return Array.from(document.querySelectorAll(
      ".theme-control-group > .rowDropdown:first-child .dropdown-item"
    ))
      .map((button) => compactText(button.textContent))
      .filter(Boolean);
  }

  async function selectTheme(label) {
    if (await applyThemeFromCatalog(label)) {
      await waitForThemeLabel(label);
      await wait(120);
      return;
    }

    await clickToOpen(
      ".theme-control-group > .rowDropdown:first-child .dropdown-toggle",
      ".theme-control-group > .rowDropdown:first-child .dropdown-item"
    );

    const buttons = Array.from(document.querySelectorAll(
      ".theme-control-group > .rowDropdown:first-child .dropdown-item"
    ));
    const match = buttons.find((button) => compactText(button.textContent) === label);
    if (!match) {
      throw new Error(`Theme option not found: ${label}`);
    }

    match.click();
    await waitForThemeLabel(label);
    await wait(120);
  }

  async function runAllThemes(options = {}) {
    const runOptions = { log: true, ...options };
    await preparePreviewStates();
    const labels = await readThemeLabels();
    const allResults = [];

    for (const label of labels) {
      await selectTheme(label);
      await preparePreviewStates();
      const result = await run({ prepare: false, log: false });
      allResults.push(...result.results);
    }

    return printResults(allResults, runOptions);
  }

  async function runStateMatrix(options = {}) {
    const runOptions = { prepare: true, log: true, ...options };
    if (runOptions.prepare) {
      await preparePreviewStates();
    }

    const results = [];
    for (const sample of STATE_SAMPLES) {
      results.push(await measureStateSample(sample));
    }

    return printStateResults(results, runOptions);
  }

  async function runStateMatrixAllThemes(options = {}) {
    const runOptions = { log: true, ...options };
    await preparePreviewStates();
    const labels = await readThemeLabels();
    const allResults = [];

    for (const label of labels) {
      await selectTheme(label);
      await preparePreviewStates();
      const result = await runStateMatrix({ prepare: false, log: false });
      allResults.push(...result.results);
    }

    return printStateResults(allResults, runOptions);
  }

  async function runWithPageZoom(zoom, callback) {
    const normalizedZoom = Number(zoom);
    if (!Number.isFinite(normalizedZoom) || normalizedZoom <= 0) {
      throw new Error(`Invalid zoom value: ${zoom}`);
    }

    const root = document.documentElement;
    const body = document.body;
    const previousRootZoom = root.style.zoom;
    const previousBodyZoom = body.style.zoom;
    root.style.zoom = String(normalizedZoom);
    body.style.zoom = String(normalizedZoom);
    await wait(160);

    try {
      return await callback(normalizedZoom);
    } finally {
      root.style.zoom = previousRootZoom;
      body.style.zoom = previousBodyZoom;
      await wait(160);
    }
  }

  async function runStateMatrixAllThemesAtZoom(zoom = 2, options = {}) {
    const normalizedZoom = Number(zoom);
    if (!Number.isFinite(normalizedZoom) || normalizedZoom <= 0) {
      throw new Error(`Invalid zoom value: ${zoom}`);
    }

    const runOptions = { log: true, ...options, zoom: normalizedZoom };
    await preparePreviewStates();
    const labels = await readThemeLabels();
    const allResults = [];

    for (const label of labels) {
      await selectTheme(label);
      await preparePreviewStates();
      const result = await runWithPageZoom(normalizedZoom, () => runStateMatrix({ prepare: true, log: false }));
      allResults.push(...result.results);
    }

    return printStateResults(allResults, runOptions);
  }

  async function copyMarkdown() {
    const last = window.PreviewCssContrastSampler.last;
    if (!last) {
      throw new Error("Run the sampler before copying Markdown.");
    }

    await navigator.clipboard.writeText(last.markdown);
    return last.markdown;
  }

  async function copyJson() {
    const last = window.PreviewCssContrastSampler.last;
    if (!last) {
      throw new Error("Run the sampler before copying JSON.");
    }

    const json = JSON.stringify(last, null, 2);
    await navigator.clipboard.writeText(json);
    return json;
  }

  async function copyStateMarkdown() {
    const last = window.PreviewCssContrastSampler.stateLast;
    if (!last) {
      throw new Error("Run the state matrix before copying Markdown.");
    }

    await navigator.clipboard.writeText(last.markdown);
    return last.markdown;
  }

  async function copyStateJson() {
    const last = window.PreviewCssContrastSampler.stateLast;
    if (!last) {
      throw new Error("Run the state matrix before copying JSON.");
    }

    const json = JSON.stringify(last, null, 2);
    await navigator.clipboard.writeText(json);
    return json;
  }

  window.PreviewCssContrastSampler = {
    run,
    runAllThemes,
    runStateMatrix,
    runStateMatrixAllThemes,
    runStateMatrixAllThemesAtZoom,
    selectTheme,
    readThemeLabels,
    copyMarkdown,
    copyJson,
    copyStateMarkdown,
    copyStateJson,
    samples: SAMPLES.slice(),
    stateSamples: STATE_SAMPLES.slice(),
    last: null,
    stateLast: null,
  };

  console.log("PreviewCssContrastSampler ready. Run await PreviewCssContrastSampler.run()");
})();

import { Events } from "@wailsio/runtime";
import { get } from "svelte/store";
import { offlineMode } from "../../stores/offlineMode";
import { scheduleUpdaterThemeSync } from "../updaterTheme";
import modalPrimaryCss from "../../styles/modal-primary.scss?inline";
import { DEFAULT_THEME_ID, WINDOWS_THEME_ACCENT_KEY, CUSTOM_THEME_ACCENT_KEY } from "./types";
import type { ThemeOption } from "./types";
import { getThemeOptionById } from "./catalog";
import { normalizeHexColor, buildAccentOverlayCss } from "./color";
import {
  currentWindowsThemeAccentColor,
  currentThemeAccentKey,
  currentThemeId,
  currentThemeCustomAccentColor,
} from "./stores";

const OVERLAY_STYLE_ID = "tcno-theme-overlay";
const OVERLAY_STYLE_ATTR = "data-tcno-theme-overlay";
const ACCENT_OVERLAY_STYLE_ID = "tcno-theme-accent-overlay";
const ACCENT_OVERLAY_STYLE_ATTR = "data-tcno-theme-accent-overlay";
const MODAL_PRIMARY_STYLE_ID = "tcno-modal-primary-overlay";
const MODAL_PRIMARY_STYLE_ATTR = "data-tcno-modal-primary-overlay";
const THEME_FONT_LINK_ATTR = "data-tcno-google-fonts-theme";

const WINDOWS_ACCENT_CHANGED_EVENT = "windows-accent-changed";

let windowsAccentSubscribed = false;

export function supportsWindowsThemeAccent(): boolean {
  if (typeof navigator === "undefined") {
    return false;
  }
  return /windows/i.test(navigator.userAgent) || /win32/i.test(navigator.userAgent);
}

export function removeThemeGoogleFontLinks(): void {
  if (typeof document === "undefined") {
    return;
  }
  document.querySelectorAll(`link[${THEME_FONT_LINK_ATTR}]`).forEach((node) => node.remove());
}

export function syncThemeGoogleFonts(themeId: string): void {
  if (typeof document === "undefined") {
    return;
  }
  removeThemeGoogleFontLinks();
  if (themeId === DEFAULT_THEME_ID || get(offlineMode)) {
    return;
  }
  const theme = getThemeOptionById(themeId);
  if (!theme?.googleFontsCss) {
    return;
  }
  const link = document.createElement("link");
  link.rel = "stylesheet";
  link.href = theme.googleFontsCss;
  link.setAttribute(THEME_FONT_LINK_ATTR, themeId);
  document.head.appendChild(link);
}

export function removeThemeOverlay(): void {
  if (typeof document === "undefined") {
    return;
  }
  document.querySelectorAll(`style[${OVERLAY_STYLE_ATTR}]`).forEach((node) => node.remove());
}

export function removeAccentOverlay(): void {
  if (typeof document === "undefined") {
    return;
  }
  document.querySelectorAll(`style[${ACCENT_OVERLAY_STYLE_ATTR}]`).forEach((node) => node.remove());
}

export function applyAccentOverlay(color: string | null): void {
  removeAccentOverlay();
  if (typeof document === "undefined" || !color) {
    syncModalPrimaryStyles();
    scheduleUpdaterThemeSync();
    return;
  }
  const style = document.createElement("style");
  style.id = ACCENT_OVERLAY_STYLE_ID;
  style.setAttribute(ACCENT_OVERLAY_STYLE_ATTR, "");
  style.textContent = buildAccentOverlayCss(color);
  document.head.appendChild(style);
  syncModalPrimaryStyles();
  scheduleUpdaterThemeSync();
}

function syncModalPrimaryStyles(): void {
  if (typeof document === "undefined") {
    return;
  }
  let style = document.getElementById(MODAL_PRIMARY_STYLE_ID) as HTMLStyleElement | null;
  if (!style) {
    style = document.createElement("style");
    style.id = MODAL_PRIMARY_STYLE_ID;
    style.setAttribute(MODAL_PRIMARY_STYLE_ATTR, "");
    style.textContent = modalPrimaryCss;
  }
  document.head.appendChild(style);
}

export function ensureWindowsAccentSubscription(): void {
  if (!supportsWindowsThemeAccent() || windowsAccentSubscribed) {
    return;
  }
  windowsAccentSubscribed = true;
  Events.On(WINDOWS_ACCENT_CHANGED_EVENT, (event) => {
    const color = normalizeHexColor(event.data) ?? "";
    currentWindowsThemeAccentColor.set(color);
    if (get(currentThemeAccentKey) === WINDOWS_THEME_ACCENT_KEY) {
      applyResolvedAccent(get(currentThemeId), WINDOWS_THEME_ACCENT_KEY, get(currentThemeCustomAccentColor));
    }
  });
}

export function validateAccentKey(theme: ThemeOption, key: string): string {
  const trimmed = key.trim();
  if (!trimmed) {
    return "";
  }
  if (trimmed === CUSTOM_THEME_ACCENT_KEY) {
    return CUSTOM_THEME_ACCENT_KEY;
  }
  if (trimmed === WINDOWS_THEME_ACCENT_KEY && supportsWindowsThemeAccent()) {
    return WINDOWS_THEME_ACCENT_KEY;
  }
  return theme.accents.some((option) => option.id === trimmed) ? trimmed : "";
}

export function applyResolvedAccent(themeId: string, accentKey: string, customColor: string): void {
  const theme = getThemeOptionById(themeId);
  const validCustomColor = normalizeHexColor(customColor) ?? "";
  const validAccentKey = validateAccentKey(theme, accentKey);

  currentThemeCustomAccentColor.set(validCustomColor);

  if (validAccentKey === CUSTOM_THEME_ACCENT_KEY && validCustomColor) {
    currentThemeAccentKey.set(CUSTOM_THEME_ACCENT_KEY);
    applyAccentOverlay(validCustomColor);
    return;
  }

  if (validAccentKey === WINDOWS_THEME_ACCENT_KEY) {
    currentThemeAccentKey.set(WINDOWS_THEME_ACCENT_KEY);
    applyAccentOverlay(get(currentWindowsThemeAccentColor) || theme.defaultAccentColor);
    return;
  }

  if (validAccentKey && validAccentKey !== theme.defaultAccentKey) {
    const preset = theme.accents.find((option) => option.id === validAccentKey);
    if (preset) {
      currentThemeAccentKey.set(validAccentKey);
      applyAccentOverlay(preset.color);
      return;
    }
  }

  currentThemeAccentKey.set("");
  applyAccentOverlay(null);
}

export function clearThemeAccentState(): void {
  currentThemeAccentKey.set("");
  currentThemeCustomAccentColor.set("");
  applyAccentOverlay(null);
}

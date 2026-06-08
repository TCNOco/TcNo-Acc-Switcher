import { Events } from "@wailsio/runtime";
import { parse as parseYaml } from "yaml";
import { get, writable } from "svelte/store";
import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
import { offlineMode } from "../stores/offlineMode";
import { setUserOverride } from "../stores/backgroundImage";
import { scheduleUpdaterThemeSync } from "./updaterTheme";

const DEFAULT_THEME_ID = "default";
const CUSTOM_THEME_ACCENT_KEY = "custom";
export const WINDOWS_THEME_ACCENT_KEY = "windows";

const OVERLAY_STYLE_ID = "tcno-theme-overlay";
const OVERLAY_STYLE_ATTR = "data-tcno-theme-overlay";
const ACCENT_OVERLAY_STYLE_ID = "tcno-theme-accent-overlay";
const ACCENT_OVERLAY_STYLE_ATTR = "data-tcno-theme-accent-overlay";
/** Marks runtime-injected Google Fonts stylesheets (see syncThemeGoogleFonts). */
const THEME_FONT_LINK_ATTR = "data-tcno-google-fonts-theme";
const THEME_STORAGE_KEY = "tcno:theme";
const THEME_ACCENT_STORAGE_KEY = "tcno:theme-accent";
const THEME_ACCENT_CUSTOM_STORAGE_KEY = "tcno:theme-accent-custom";

const DEFAULT_LABEL = "Dracula Cyan (Default)";
const WINDOWS_ACCENT_CHANGED_EVENT = "windows-accent-changed";

const themeInfos = import.meta.glob("../styles/themes/*/info.yaml", {
  query: "?raw",
  import: "default",
  eager: true,
}) as Record<string, string>;

const themeStyles = import.meta.glob("../styles/themes/*/style.scss", {
  query: "?inline",
  import: "default",
}) as Record<string, () => Promise<string>>;

const themeBackgrounds = import.meta.glob(
  "../styles/themes/*/*.{jpg,jpeg,png,webp,gif}",
  { eager: true, import: "default" }
) as Record<string, string>;

type ThemeInfoYaml = {
  name?: unknown;
  accent?: unknown;
  accents?: unknown;
  googleFontsCss?: unknown;
  background?: unknown;
};

export type ThemeAccentOption = {
  id: string;
  label: string;
  color: string;
};

export type ResolvedThemeAccent = ThemeAccentOption & {
  isCustom: boolean;
};

export type ThemeOption = {
  id: string;
  label: string;
  googleFontsCss: string | null;
  backgroundUrl: string | null;
  defaultAccentColor: string;
  defaultAccentKey: string;
  accents: ThemeAccentOption[];
};

const DEFAULT_THEME_OPTION: ThemeOption = {
  id: DEFAULT_THEME_ID,
  label: DEFAULT_LABEL,
  googleFontsCss: null,
  backgroundUrl: null,
  defaultAccentColor: "#80ffea",
  defaultAccentKey: "cyan",
  accents: [
    { id: "cyan", label: "Cyan", color: "#80ffea" },
    { id: "green", label: "Green", color: "#8aff80" },
    { id: "orange", label: "Orange", color: "#ffca80" },
    { id: "pink", label: "Pink", color: "#ff80bf" },
    { id: "purple", label: "Purple", color: "#9580ff" },
    { id: "red", label: "Red", color: "#ff9580" },
    { id: "yellow", label: "Yellow", color: "#ffff80" },
  ],
};

function normPath(p: string): string {
  return p.replace(/\\/g, "/");
}

function folderFromInfoPath(path: string): string {
  const m = normPath(path).match(/themes\/([^/]+)\/info\.yaml$/i);
  return m?.[1] ?? "";
}

function styleLoaderPathForId(themeId: string): string | null {
  const suffix = `themes/${themeId}/style.scss`;
  for (const k of Object.keys(themeStyles)) {
    if (normPath(k).endsWith(suffix)) {
      return k;
    }
  }
  return null;
}

function normalizeHexColor(value: unknown): string | null {
  if (typeof value !== "string") {
    return null;
  }
  const trimmed = value.trim();
  const match = trimmed.match(/^#([0-9a-f]{3}|[0-9a-f]{6})$/i);
  if (!match) {
    return null;
  }
  const hex = match[1];
  if (hex.length === 3) {
    return `#${hex
      .split("")
      .map((ch) => `${ch}${ch}`)
      .join("")
      .toLowerCase()}`;
  }
  return `#${hex.toLowerCase()}`;
}

function sanitizeGoogleFontsCssUrl(value: unknown): string | null {
  if (typeof value !== "string") {
    return null;
  }
  const trimmed = value.trim();
  if (!trimmed) {
    return null;
  }
  try {
    const url = new URL(trimmed);
    if (url.protocol !== "https:") {
      return null;
    }
    if (!/^fonts\.googleapis\.com$/i.test(url.hostname)) {
      return null;
    }
    return url.href;
  } catch {
    return null;
  }
}

function accentKeyFromLabel(label: string): string {
  return (
    label
      .trim()
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, "-")
      .replace(/^-+|-+$/g, "") || "accent"
  );
}

function dedupKey(baseKey: string, seen: Set<string>): string {
  let key = baseKey;
  let suffix = 2;
  while (seen.has(key)) {
    key = `${baseKey}-${suffix++}`;
  }
  return key;
}

function parseAccentOptions(value: unknown, fallbackColor: string): ThemeAccentOption[] {
  const fallback = () => [{ id: "accent", label: "Accent", color: fallbackColor }];
  if (!Array.isArray(value)) return fallback();

  const options: ThemeAccentOption[] = [];
  const seen = new Set<string>();
  for (const entry of value) {
    if (!entry || typeof entry !== "object") continue;
    const record = entry as Record<string, unknown>;
    const label = typeof record.name === "string" ? record.name.trim() : "";
    const color = normalizeHexColor(record.color);
    if (!label || !color) continue;

    const key = dedupKey(accentKeyFromLabel(label), seen);
    seen.add(key);
    options.push({ id: key, label, color });
  }
  return options.length ? options : fallback();
}


function parseThemeInfo(raw: string, id: string): ThemeOption | null {
  let parsed: ThemeInfoYaml;
  try {
    const next = parseYaml(raw);
    if (!next || typeof next !== "object") {
      return null;
    }
    parsed = next as ThemeInfoYaml;
  } catch {
    return null;
  }

  const label = typeof parsed.name === "string" && parsed.name.trim() ? parsed.name.trim() : id;
  const accent = normalizeHexColor(parsed.accent) ?? DEFAULT_THEME_OPTION.defaultAccentColor;
  let accents = parseAccentOptions(parsed.accents, accent);
  let defaultAccentKey = accents.find((option) => option.color === accent)?.id ?? "";
  if (!defaultAccentKey) {
    accents = [{ id: "accent", label: "Accent", color: accent }, ...accents];
    defaultAccentKey = "accent";
  }

  return {
    id,
    label,
    googleFontsCss: sanitizeGoogleFontsCssUrl(parsed.googleFontsCss),
    backgroundUrl: (() => {
      const fn = sanitizeThemeBackgroundFilename(parsed.background);
      return fn ? findThemeBackgroundUrl(id, fn) : null;
    })(),
    defaultAccentColor: accent,
    defaultAccentKey,
    accents,
  };
}

function sanitizeThemeBackgroundFilename(value: unknown): string | null {
  if (typeof value !== "string") return null;
  const trimmed = value.trim();
  if (!/^[a-zA-Z0-9_-]+\.(jpe?g|png|webp|gif)$/i.test(trimmed)) return null;
  return trimmed;
}

function findThemeBackgroundUrl(themeId: string, filename: string): string | null {
  const suffix = normPath(`themes/${themeId}/${filename}`);
  for (const [path, url] of Object.entries(themeBackgrounds)) {
    if (normPath(path).endsWith(suffix)) {
      return url;
    }
  }
  return null;
}

function buildThemeCatalog(): ThemeOption[] {
  const rest: ThemeOption[] = [];
  for (const [path, raw] of Object.entries(themeInfos)) {
    const id = folderFromInfoPath(path);
    if (!id || !styleLoaderPathForId(id)) {
      continue;
    }
    const theme = parseThemeInfo(raw, id);
    if (theme) {
      rest.push(theme);
    }
  }
  rest.sort((a, b) => a.label.localeCompare(b.label, undefined, { sensitivity: "base" }));
  return [DEFAULT_THEME_OPTION, ...rest];
}

const THEMES = buildThemeCatalog();
const THEMES_BY_ID = new Map(THEMES.map((theme) => [theme.id, theme] as const));

function removeThemeGoogleFontLinks(): void {
  if (typeof document === "undefined") {
    return;
  }
  document.querySelectorAll(`link[${THEME_FONT_LINK_ATTR}]`).forEach((node) => node.remove());
}

/** Injects googleFontsCss from the theme's info.yaml when online; removes links when offline or on default theme. */
function syncThemeGoogleFonts(themeId: string): void {
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

/** Theme folder ids that have a compiled `style.scss` (inline) bundle. */
export function listThemes(): ThemeOption[] {
  return THEMES;
}

function getThemeOptionById(id: string): ThemeOption {
  return THEMES_BY_ID.get(id) ?? DEFAULT_THEME_OPTION;
}

export const currentThemeId = writable<string>(DEFAULT_THEME_ID);
export const currentThemeBgUrl = writable<string>("");
export const currentThemeAccentKey = writable<string>("");
export const currentThemeCustomAccentColor = writable<string>("");
export const currentWindowsThemeAccentColor = writable<string>("");

let activeThemeRequestId = 0;
let windowsAccentSubscribed = false;

export function supportsWindowsThemeAccent(): boolean {
  if (typeof navigator === "undefined") {
    return false;
  }
  return /windows/i.test(navigator.userAgent) || /win32/i.test(navigator.userAgent);
}

function removeThemeOverlay(): void {
  if (typeof document === "undefined") {
    return;
  }
  document.querySelectorAll(`style[${OVERLAY_STYLE_ATTR}]`).forEach((node) => node.remove());
}

function removeAccentOverlay(): void {
  if (typeof document === "undefined") {
    return;
  }
  document.querySelectorAll(`style[${ACCENT_OVERLAY_STYLE_ATTR}]`).forEach((node) => node.remove());
}

function hexToRgb(hex: string): { r: number; g: number; b: number } {
  const normalized = normalizeHexColor(hex) ?? DEFAULT_THEME_OPTION.defaultAccentColor;
  return {
    r: Number.parseInt(normalized.slice(1, 3), 16),
    g: Number.parseInt(normalized.slice(3, 5), 16),
    b: Number.parseInt(normalized.slice(5, 7), 16),
  };
}

function rgbToHsl({ r, g, b }: { r: number; g: number; b: number }): { h: number; s: number; l: number } {
  const nr = r / 255;
  const ng = g / 255;
  const nb = b / 255;
  const max = Math.max(nr, ng, nb);
  const min = Math.min(nr, ng, nb);
  const delta = max - min;

  let h = 0;
  if (delta !== 0) {
    switch (max) {
      case nr:
        h = ((ng - nb) / delta) % 6;
        break;
      case ng:
        h = (nb - nr) / delta + 2;
        break;
      default:
        h = (nr - ng) / delta + 4;
        break;
    }
  }
  h = Math.round(((h * 60) + 360) % 360);

  const l = (max + min) / 2;
  const s = delta === 0 ? 0 : delta / (1 - Math.abs((2 * l) - 1));

  return {
    h,
    s: Math.round(s * 100),
    l: Math.round(l * 100),
  };
}

function buildAccentOverlayCss(color: string): string {
  const normalized = normalizeHexColor(color) ?? DEFAULT_THEME_OPTION.defaultAccentColor;
  const rgb = hexToRgb(normalized);
  const hsl = rgbToHsl(rgb);
  return `
:root {
  --accent: ${normalized};
  --accentHS: ${hsl.h}, ${hsl.s}%;
  --accentL: ${hsl.l}%;
  --accentInt: ${rgb.r}, ${rgb.g}, ${rgb.b};
  --notification-info-border: var(--accent);
  --accent-border-deep: hsl(var(--accentHS), 100%, 9%);
  --accent-text-bright: hsl(var(--accentHS), calc(var(--accentL) + 32%));
  --accent-border-strong: hsl(var(--accentHS), 65%, 40%);
  --accent-type-dim: hsl(var(--accentHS), calc(var(--accentL) - 12%));
  --accent-overlay-border: rgba(var(--accentInt), 0.35);
  --accent-overlay-border-strong: rgba(var(--accentInt), 0.55);
  --accent-overlay-hover: rgba(var(--accentInt), 0.85);
  --accent-fill-very-soft: rgba(var(--accentInt), 0.12);
  --accent-fill-soft: rgba(var(--accentInt), 0.32);
  --accent-fill-medium: rgba(var(--accentInt), 0.42);
  --accent-fill-panel: rgba(var(--accentInt), 0.45);
  --accent-highlight-row: rgba(var(--accentInt), 0.9);
  --accent-text-on-selection: rgba(var(--accentInt), 0.95);
  --accent-text-heading: rgba(var(--accentInt), 0.98);
  --home-ico-gradient-end: rgba(var(--accentInt), 1);
  --miniprofile-radial-glow: rgba(var(--accentInt), 0.45);
  --miniprofile-radial-fade: rgba(var(--accentInt), 0.08);
  --miniprofile-linear-top: rgba(var(--accentInt), 0.55);
  --miniprofile-linear-mid: rgba(var(--accentInt), 0.14);
  --miniprofile-linear-low: rgba(var(--accentInt), 0.1);
}`;
}

function applyAccentOverlay(color: string | null): void {
  removeAccentOverlay();
  if (typeof document === "undefined" || !color) {
    scheduleUpdaterThemeSync();
    return;
  }
  const style = document.createElement("style");
  style.id = ACCENT_OVERLAY_STYLE_ID;
  style.setAttribute(ACCENT_OVERLAY_STYLE_ATTR, "");
  style.textContent = buildAccentOverlayCss(color);
  document.head.appendChild(style);
  scheduleUpdaterThemeSync();
}

function validateAccentKey(theme: ThemeOption, key: string): string {
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

export function resolveThemeAccent(
  themeId: string,
  accentKey = get(currentThemeAccentKey),
  customColor = get(currentThemeCustomAccentColor),
): ResolvedThemeAccent {
  const theme = getThemeOptionById(themeId);
  if (accentKey === CUSTOM_THEME_ACCENT_KEY) {
    return {
      id: CUSTOM_THEME_ACCENT_KEY,
      label: "Custom",
      color: normalizeHexColor(customColor) ?? theme.defaultAccentColor,
      isCustom: true,
    };
  }
  if (accentKey === WINDOWS_THEME_ACCENT_KEY && supportsWindowsThemeAccent()) {
    return {
      id: WINDOWS_THEME_ACCENT_KEY,
      label: "Windows Accent",
      color: get(currentWindowsThemeAccentColor) || theme.defaultAccentColor,
      isCustom: false,
    };
  }
  const preset =
    theme.accents.find((option) => option.id === accentKey) ??
    theme.accents.find((option) => option.id === theme.defaultAccentKey) ?? {
      id: theme.defaultAccentKey,
      label: "Accent",
      color: theme.defaultAccentColor,
    };
  return { ...preset, isCustom: false };
}

function applyResolvedAccent(themeId: string, accentKey: string, customColor: string): void {
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

function clearThemeAccentState(): void {
  currentThemeAccentKey.set("");
  currentThemeCustomAccentColor.set("");
  applyAccentOverlay(null);
}

async function refreshWindowsThemeAccentColor(): Promise<string> {
  if (!supportsWindowsThemeAccent()) {
    currentWindowsThemeAccentColor.set("");
    return "";
  }
  return get(currentWindowsThemeAccentColor);
}

function ensureWindowsAccentSubscription(): void {
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

async function loadStoredThemeId(): Promise<string> {
  try {
    const persisted = String((await PlatformService.GetTheme()) ?? "").trim();
    return persisted || DEFAULT_THEME_ID;
  } catch {
    return localStorage.getItem(THEME_STORAGE_KEY)?.trim() || DEFAULT_THEME_ID;
  }
}

async function loadStoredAccentState(): Promise<{ accentKey: string; customColor: string }> {
  try {
    const [accentKey, customColor] = await Promise.all([
      PlatformService.GetThemeAccentPreset(),
      PlatformService.GetThemeAccentCustom(),
    ]);
    return {
      accentKey: String(accentKey ?? "").trim(),
      customColor: String(customColor ?? "").trim(),
    };
  } catch {
    return {
      accentKey: localStorage.getItem(THEME_ACCENT_STORAGE_KEY)?.trim() || "",
      customColor: localStorage.getItem(THEME_ACCENT_CUSTOM_STORAGE_KEY)?.trim() || "",
    };
  }
}

async function persistAccentState(accentKey: string, customColor: string): Promise<void> {
  try {
    await Promise.all([
      PlatformService.SetThemeAccentPreset(accentKey),
      PlatformService.SetThemeAccentCustom(customColor),
    ]);
  } catch {
    /* offline / early boot */
  }

  if (accentKey) {
    localStorage.setItem(THEME_ACCENT_STORAGE_KEY, accentKey);
  } else {
    localStorage.removeItem(THEME_ACCENT_STORAGE_KEY);
  }

  if (customColor) {
    localStorage.setItem(THEME_ACCENT_CUSTOM_STORAGE_KEY, customColor);
  } else {
    localStorage.removeItem(THEME_ACCENT_CUSTOM_STORAGE_KEY);
  }
}

async function applyTheme(id: string): Promise<void> {
  const requestId = ++activeThemeRequestId;
  removeThemeOverlay();
  removeAccentOverlay();
  removeThemeGoogleFontLinks();

  if (id === DEFAULT_THEME_ID) {
    currentThemeId.set(DEFAULT_THEME_ID);
    currentThemeBgUrl.set("");
    syncThemeGoogleFonts(DEFAULT_THEME_ID);
    scheduleUpdaterThemeSync();
    return;
  }

  const key = styleLoaderPathForId(id);
  if (!key) {
    console.warn("[themes] Unknown or missing style for theme:", id);
    currentThemeId.set(DEFAULT_THEME_ID);
    currentThemeBgUrl.set("");
    syncThemeGoogleFonts(DEFAULT_THEME_ID);
    scheduleUpdaterThemeSync();
    return;
  }

  const load = themeStyles[key];
  const css = await load();
  if (requestId !== activeThemeRequestId) {
    return;
  }

  removeThemeOverlay();
  const style = document.createElement("style");
  style.id = OVERLAY_STYLE_ID;
  style.setAttribute(OVERLAY_STYLE_ATTR, "");
  style.textContent = css;
  document.head.appendChild(style);
  currentThemeId.set(id);
  currentThemeBgUrl.set(getThemeOptionById(id).backgroundUrl ?? "");
  syncThemeGoogleFonts(id);
  scheduleUpdaterThemeSync();
}

function isKnownThemeId(id: string): boolean {
  return THEMES_BY_ID.has(id);
}

export async function initTheme(): Promise<void> {
  let [id, storedAccent] = await Promise.all([loadStoredThemeId(), loadStoredAccentState()]);
  if (!isKnownThemeId(id)) {
    id = DEFAULT_THEME_ID;
  }
  ensureWindowsAccentSubscription();
  await refreshWindowsThemeAccentColor();
  await applyTheme(id);
  applyResolvedAccent(id, storedAccent.accentKey, storedAccent.customColor);
}

/** Persist theme to backend + localStorage and apply. */
export async function setUserTheme(id: string): Promise<void> {
  const next = isKnownThemeId(id) ? id : DEFAULT_THEME_ID;
  const previous = get(currentThemeId);
  const persist = next === DEFAULT_THEME_ID ? "" : next;

  try {
    await PlatformService.SetTheme(persist);
  } catch {
    /* offline / early boot */
  }

  localStorage.setItem(THEME_STORAGE_KEY, next);

  if (next === previous) {
    return;
  }

  await persistAccentState("", "");
  clearThemeAccentState();
  await applyTheme(next);
  // New theme chosen — reset any user background override so the theme bg shows.
  await setUserOverride(false);
}

export async function setUserThemeAccentPreset(accentKey: string): Promise<void> {
  const theme = getThemeOptionById(get(currentThemeId));
  const validAccentKey = validateAccentKey(theme, accentKey);
  const customColor = normalizeHexColor(get(currentThemeCustomAccentColor)) ?? "";

  if (validAccentKey === WINDOWS_THEME_ACCENT_KEY) {
    await refreshWindowsThemeAccentColor();
  }

  if (!validAccentKey || validAccentKey === theme.defaultAccentKey) {
    await persistAccentState("", customColor);
    applyResolvedAccent(theme.id, "", customColor);
    return;
  }

  await persistAccentState(validAccentKey, customColor);
  applyResolvedAccent(theme.id, validAccentKey, customColor);
}

export async function setUserThemeAccentCustom(color: string): Promise<void> {
  const theme = getThemeOptionById(get(currentThemeId));
  const normalized =
    normalizeHexColor(color) ?? resolveThemeAccent(theme.id).color ?? theme.defaultAccentColor;
  await persistAccentState(CUSTOM_THEME_ACCENT_KEY, normalized);
  applyResolvedAccent(theme.id, CUSTOM_THEME_ACCENT_KEY, normalized);
}

offlineMode.subscribe(() => {
  syncThemeGoogleFonts(get(currentThemeId));
});

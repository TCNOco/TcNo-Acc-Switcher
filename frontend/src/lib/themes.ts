import { get, writable } from "svelte/store";
import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
import { offlineMode } from "../stores/offlineMode";

export const DEFAULT_THEME_ID = "default";

const OVERLAY_STYLE_ID = "tcno-theme-overlay";
/** Marks runtime-injected Google Fonts stylesheets (see syncThemeGoogleFonts). */
const THEME_FONT_LINK_ATTR = "data-tcno-google-fonts-theme";
export const THEME_STORAGE_KEY = "tcno:theme";

const DEFAULT_LABEL = "Dracula Cyan (Default)";

const themeInfos = import.meta.glob("../styles/themes/*/info.yaml", {
  query: "?raw",
  import: "default",
  eager: true,
}) as Record<string, string>;

const themeStyles = import.meta.glob("../styles/themes/*/style.scss", {
  query: "?inline",
  import: "default",
}) as Record<string, () => Promise<string>>;

export type ThemeOption = { id: string; label: string };

function normPath(p: string): string {
  return p.replace(/\\/g, "/");
}

function folderFromInfoPath(path: string): string {
  const m = normPath(path).match(/themes\/([^/]+)\/info\.yaml$/i);
  return m?.[1] ?? "";
}

function parseNameFromInfoYaml(raw: string): string {
  const m = raw.match(/^name:\s*(.+)$/m);
  const v = m?.[1]?.trim();
  if (!v) return "";
  return v.replace(/^["']|["']$/g, "");
}

/** Optional per-theme Google Fonts CSS URL from info.yaml (online-only injection). */
function parseGoogleFontsCssUrl(raw: string): string | null {
  const m = raw.match(/^googleFontsCss:\s*(.+)$/m);
  if (!m?.[1]) {
    return null;
  }
  let v = m[1].trim();
  if (
    (v.startsWith('"') && v.endsWith('"')) ||
    (v.startsWith("'") && v.endsWith("'"))
  ) {
    v = v.slice(1, -1).trim();
  }
  if (!v) {
    return null;
  }
  try {
    const u = new URL(v);
    if (u.protocol !== "https:") {
      return null;
    }
    if (!/^fonts\.googleapis\.com$/i.test(u.hostname)) {
      return null;
    }
    return u.href;
  } catch {
    return null;
  }
}

function infoYamlRawForThemeId(themeId: string): string | null {
  for (const [path, raw] of Object.entries(themeInfos)) {
    if (folderFromInfoPath(path) === themeId) {
      return raw;
    }
  }
  return null;
}

function removeThemeGoogleFontLinks(): void {
  if (typeof document === "undefined") {
    return;
  }
  document.querySelectorAll(`link[${THEME_FONT_LINK_ATTR}]`).forEach((n) => n.remove());
}

/** Injects googleFontsCss from the theme's info.yaml when online; removes links when offline or on default theme. */
export function syncThemeGoogleFonts(themeId: string): void {
  if (typeof document === "undefined") {
    return;
  }
  removeThemeGoogleFontLinks();
  if (themeId === DEFAULT_THEME_ID) {
    return;
  }
  if (get(offlineMode)) {
    return;
  }
  const raw = infoYamlRawForThemeId(themeId);
  if (!raw) {
    return;
  }
  const href = parseGoogleFontsCssUrl(raw);
  if (!href) {
    return;
  }
  const link = document.createElement("link");
  link.rel = "stylesheet";
  link.href = href;
  link.setAttribute(THEME_FONT_LINK_ATTR, themeId);
  document.head.appendChild(link);
}

/** Theme folder ids that have a compiled `style.scss` (inline) bundle. */
export function listThemes(): ThemeOption[] {
  const rest: ThemeOption[] = [];
  for (const [path, raw] of Object.entries(themeInfos)) {
    const id = folderFromInfoPath(path);
    if (!id) continue;
    if (!styleLoaderPathForId(id)) continue;
    const label = parseNameFromInfoYaml(raw) || id;
    rest.push({ id, label });
  }
  rest.sort((a, b) => a.label.localeCompare(b.label, undefined, { sensitivity: "base" }));
  return [{ id: DEFAULT_THEME_ID, label: DEFAULT_LABEL }, ...rest];
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

export const currentThemeId = writable<string>(DEFAULT_THEME_ID);

function removeOverlay(): void {
  document.getElementById(OVERLAY_STYLE_ID)?.remove();
}

export async function applyTheme(id: string): Promise<void> {
  removeOverlay();
  removeThemeGoogleFontLinks();
  if (id === DEFAULT_THEME_ID) {
    currentThemeId.set(DEFAULT_THEME_ID);
    return;
  }
  const key = styleLoaderPathForId(id);
  if (!key) {
    console.warn("[themes] Unknown or missing style for theme:", id);
    currentThemeId.set(DEFAULT_THEME_ID);
    return;
  }
  const load = themeStyles[key];
  const css = await load();
  const style = document.createElement("style");
  style.id = OVERLAY_STYLE_ID;
  style.textContent = css;
  document.head.appendChild(style);
  currentThemeId.set(id);
  syncThemeGoogleFonts(id);
}

function isKnownThemeId(id: string): boolean {
  return listThemes().some((t) => t.id === id);
}

export async function initTheme(): Promise<void> {
  let id = DEFAULT_THEME_ID;
  try {
    const t = await PlatformService.GetTheme();
    const s = String(t ?? "").trim();
    if (s) id = s;
  } catch {
    const ls = localStorage.getItem(THEME_STORAGE_KEY)?.trim();
    if (ls) id = ls;
  }
  if (!isKnownThemeId(id)) {
    id = DEFAULT_THEME_ID;
  }
  await applyTheme(id);
}

/** Persist theme to backend + localStorage and apply. */
export async function setUserTheme(id: string): Promise<void> {
  const themes = listThemes();
  const next = themes.some((t) => t.id === id) ? id : DEFAULT_THEME_ID;
  const persist = next === DEFAULT_THEME_ID ? "" : next;
  try {
    await PlatformService.SetTheme(persist);
  } catch {
    /* offline / early boot */
  }
  localStorage.setItem(THEME_STORAGE_KEY, next);
  await applyTheme(next);
}

offlineMode.subscribe(() => {
  syncThemeGoogleFonts(get(currentThemeId));
});

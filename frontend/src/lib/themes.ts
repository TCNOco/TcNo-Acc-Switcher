import { writable } from "svelte/store";
import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";

export const DEFAULT_THEME_ID = "default";

const OVERLAY_STYLE_ID = "tcno-theme-overlay";
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

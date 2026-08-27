import { derived, writable } from "svelte/store";
import localeKeys from "virtual:locale-keys";
import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
import { zipMessages } from "../lib/localeMessages";

const STORAGE_KEY = "language";

// Value arrays, not objects: vite-plugin-locale-values strips the keys, which
// are shared across all 38 locales via `localeKeys`.
const modules = import.meta.glob("../Resources/*.json") as Record<
  string,
  () => Promise<{ default: (string | null)[] }>
>;

export const locale = writable("en-US");
const messages = writable<Record<string, string>>({});

export const availableLocales = Object.keys(modules)
  .map((p) => (p.match(/([^/]+)\.json$/)?.[1] ?? "").replace(/\.json$/, ""))
  .filter(Boolean)
  .sort();

function interpolate(template: string, vars?: Record<string, string | number>) {
  if (!vars) return template;
  return template.replace(/\{(\w+)\}/g, (_, k) => String(vars[k] ?? `{${k}}`));
}

export const t = derived(
  messages,
  ($m) => (key: string, vars?: Record<string, string | number>) =>
    interpolate($m[key] ?? key, vars),
);

function pathKey(p: string) {
  return p.replace(/\\/g, "/");
}

function syncDocumentLanguageAndDirection(code: string) {
  if (typeof document === "undefined") {
    return;
  }
  const tag = code.replace(/_/g, "-");
  const dir = localeIsRtl(code) ? "rtl" : "ltr";
  document.documentElement.setAttribute("dir", dir);
  document.documentElement.setAttribute("lang", tag);
}

/** Whether the locale uses right-to-left UI (Arabic, Hebrew, Persian, etc.). */
function localeIsRtl(code: string): boolean {
  const normalized = code.trim().replace(/_/g, "-");
  try {
    const Locale = Intl.Locale as typeof Intl.Locale & {
      prototype: { textInfo?: { direction?: string } };
    };
    const loc = new Locale(normalized);
    const d = (loc as { textInfo?: { direction?: string } }).textInfo?.direction;
    if (d === "rtl") {
      return true;
    }
    if (d === "ltr") {
      return false;
    }
  } catch {
    /* ignore */
  }
  const prim = normalized.split("-")[0]?.toLowerCase() ?? "";
  return ["ar", "fa", "he", "ur", "yi"].includes(prim);
}

function resolveLocale(wanted: string | null) {
  if (wanted && availableLocales.includes(wanted)) return wanted;
  return availableLocales.includes("en-US")
    ? "en-US"
    : (availableLocales[0] ?? "en-US");
}

let enUSMessagesCache: Record<string, string> | null = null;

async function loadEnUSMessages(): Promise<Record<string, string>> {
  if (enUSMessagesCache) {
    return enUSMessagesCache;
  }
  const enEntry = Object.entries(modules).find(([path]) =>
    pathKey(path).endsWith("/en-US.json"),
  );
  if (!enEntry) {
    return {};
  }
  const enMod = await enEntry[1]();
  enUSMessagesCache = zipMessages(localeKeys, enMod.default);
  return enUSMessagesCache;
}

async function loadLocale(code: string) {
  const entry = Object.entries(modules).find(([path]) =>
    pathKey(path).endsWith(`/${code}.json`),
  );
  if (!entry) throw new Error(`Missing locale file: ${code}.json`);
  const mod = await entry[1]();
  let merged = zipMessages(localeKeys, mod.default);
  if (code !== "en-US") {
    const en = await loadEnUSMessages();
    merged = { ...en, ...merged };
  }
  messages.set(merged);
  locale.set(code);
  syncDocumentLanguageAndDirection(code);
}

export async function initI18n() {
  let code: string | null = null;
  try {
    code = await PlatformService.GetLanguage();
  } catch {
    code = null;
  }
  const saved = code ?? localStorage.getItem(STORAGE_KEY);
  await loadLocale(resolveLocale(saved));
}

export async function setUserLanguage(code: string) {
  const next = resolveLocale(code);
  try {
    await PlatformService.SetLanguage(next);
  } catch {
    /* offline / early boot */
  }
  localStorage.setItem(STORAGE_KEY, next);
  await loadLocale(next);
}

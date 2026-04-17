import { derived, writable, get } from "svelte/store";

const STORAGE_KEY = "language";

const modules = import.meta.glob("../Resources/*.json") as Record<
  string,
  () => Promise<{ default: Record<string, string> }>
>;

export const locale = writable("en-US");
export const messages = writable<Record<string, string>>({});

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

function resolveLocale(wanted: string | null) {
  if (wanted && availableLocales.includes(wanted)) return wanted;
  return availableLocales.includes("en-US")
    ? "en-US"
    : (availableLocales[0] ?? "en-US");
}

export async function loadLocale(code: string) {
  const entry = Object.entries(modules).find(([path]) =>
    pathKey(path).endsWith(`/${code}.json`),
  );
  if (!entry) throw new Error(`Missing locale file: ${code}.json`);
  const mod = await entry[1]();
  messages.set(mod.default);
  locale.set(code);
}

export async function initI18n() {
  const saved = localStorage.getItem(STORAGE_KEY);
  await loadLocale(resolveLocale(saved));
}

export async function setUserLanguage(code: string) {
  const next = resolveLocale(code);
  localStorage.setItem(STORAGE_KEY, next);
  await loadLocale(next);
}

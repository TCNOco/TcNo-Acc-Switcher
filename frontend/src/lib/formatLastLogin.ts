const DATE_FORMAT: Intl.DateTimeFormatOptions = {
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
  hour: "2-digit",
  minute: "2-digit",
  hour12: false,
};

// Building an Intl.DateTimeFormat is the expensive part of formatting a date,
// and this runs once per account row per render pass. There is one formatter
// per language, and the app has one active language at a time.
const formatters = new Map<string, Intl.DateTimeFormat>();

function formatterFor(locale: string): Intl.DateTimeFormat {
  const key = locale || "en-US";
  const existing = formatters.get(key);
  if (existing) return existing;
  const created = new Intl.DateTimeFormat(key, DATE_FORMAT);
  formatters.set(key, created);
  return created;
}

/**
 * Formats a backend last-login instant for the UI using the app language (BCP-47).
 * Expects RFC3339 from Go (`time.RFC3339`); falls back to `Date.parse` for older payloads.
 */
export function formatLastLoginForLocale(raw: string, locale: string): string {
  const s = raw.trim();
  if (!s) return "";
  const ms = Date.parse(s);
  if (Number.isNaN(ms)) return s;
  try {
    return formatterFor(locale).format(ms);
  } catch {
    return s;
  }
}

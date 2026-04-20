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
    return new Intl.DateTimeFormat(locale || "en-US", {
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      hour12: false,
    }).format(ms);
  } catch {
    return s;
  }
}

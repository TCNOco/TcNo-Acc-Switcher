const I18N_PREFIX = "i18n:";

/**
 * Resolves a GameStats.json `Tooltip` value for display.
 *
 * Plain strings pass through. `i18n:<Key>` is looked up in the active language;
 * an unknown key falls back to the key text itself, matching how `$t` behaves
 * everywhere else, so a typo in a custom definition is visible rather than
 * silently dropping the tooltip.
 */
export function resolveGameStatTooltip(
  raw: string | undefined | null,
  translate: (key: string) => string,
): string {
  const text = (raw ?? "").trim();
  if (!text.startsWith(I18N_PREFIX)) return text;
  const key = text.slice(I18N_PREFIX.length).trim();
  if (!key) return "";
  return translate(key);
}

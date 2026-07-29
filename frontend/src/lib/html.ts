/**
 * Escapes text for interpolation into a modal `body` string, which AppModal
 * renders as HTML.
 *
 * The ampersand pass has to come first: replacing it after the others would
 * turn the `&` in an already-substituted `&lt;` into `&amp;lt;`.
 */
export function escapeHtml(value: string): string {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

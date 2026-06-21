/** Windows: <>:"/\|?* ; Unix: / ; strip all for consistent asset filenames. */
const ILLEGAL_FILENAME_CHARS = /[<>:"/\\|?*\u0000-\u001f]/g;

function iconFileBase(platformName: string): string {
  return platformName.trim().replace(ILLEGAL_FILENAME_CHARS, "").trim();
}

/** Href for `<use href="...">` — same pattern as legacy Blazor (`img/platform/Name.svg#FG`). */
export function platformIconFgHref(platformName: string): string {
  const base = iconFileBase(platformName);
  return `/img/platform/${encodeURIComponent(base)}.svg#FG`;
}

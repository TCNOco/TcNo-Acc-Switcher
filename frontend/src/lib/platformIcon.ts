import { platformArtworkName } from "./platformName";

/** Windows: <>:"/\|?* ; Unix: / ; strip all for consistent asset filenames. */
const ILLEGAL_FILENAME_CHARS = /[<>:"/\\|?*\u0000-\u001f]/g;

function iconFileBase(platformName: string): string {
  return platformArtworkName(platformName).replace(ILLEGAL_FILENAME_CHARS, "").trim();
}

/** Href for `<use href="...">` — same pattern as legacy Blazor (`img/platform/Name.svg#FG`). */
export function platformIconFgHref(platformName: string): string {
  const base = iconFileBase(platformName);
  return `/img/platform/${encodeURIComponent(base)}.svg#FG`;
}

/** `src` for the whole bundled platform artwork, for use as an `<img>`. */
export function platformIconSrc(platformName: string): string {
  const base = iconFileBase(platformName);
  if (!base) return "";
  return `/img/platform/${encodeURIComponent(base)}.svg`;
}

/**
 * The Steam games list filters in place through its own search bar rather than the
 * app-wide search overlay. While that list is mounted it registers here, and every
 * caller that would otherwise open the overlay hands the search to it instead.
 */

type FocusHandler = (append: string) => void;

let focusHandler: FocusHandler | null = null;

export function setSteamGamesSearchFocusHandler(handler: FocusHandler | null): void {
  focusHandler = handler;
}

/**
 * Focuses the games search bar, appending `append` — the keystroke that triggered
 * type-anywhere, which the caller has already swallowed so the browser will not
 * insert it. Returns false when the list is not mounted, so callers fall back to
 * the overlay.
 */
export function focusSteamGamesSearch(append = ""): boolean {
  if (!focusHandler) return false;
  focusHandler(append);
  return true;
}

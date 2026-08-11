/**
 * The destinations a Steam Guard session window can be opened on.
 *
 * Go holds the same closed set and derives the URL from it, so only the name
 * crosses the boundary. Adding one means adding it there too; a name this side
 * alone is refused rather than opened.
 */
export type SteamBrowserSite =
  | "store"
  | "community"
  | "chat"
  | "gamedata-730"
  | "gamedata-440"
  | "gamedata-570";

/**
 * Steam publishes a Personal Game Data page for a few of its own titles only,
 * so this is a lookup rather than a rule: a game that is not here has no page,
 * and inventing one from its app id would land on a Steam error.
 */
const GAME_DATA_SITES: Readonly<Record<string, SteamBrowserSite>> = {
  "730": "gamedata-730", // Counter-Strike 2
  "440": "gamedata-440", // Team Fortress 2
  "570": "gamedata-570", // Dota 2
};

/** The site for a game's Personal Game Data page, or undefined if it has none. */
export function gameDataSite(appId: string): SteamBrowserSite | undefined {
  return GAME_DATA_SITES[appId.trim()];
}

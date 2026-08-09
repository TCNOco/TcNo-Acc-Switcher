import type { PlatformSortKind } from "../../stores/platformListSort";
import { fuzzyWordsMatch } from "../searchFuzzy";

export type OwnedGameRow = {
  appId: string;
  name: string;
  iconUrl: string;
  /** SteamID64s. Empty means the game is installed locally and ownership is unknown. */
  owners: string[];
};

/**
 * The games list carries no timestamps, so only the shared sort menu's alphabetical
 * entries mean anything here and every other kind leaves the order untouched. Steam's
 * by-username variants fall back to the game name for the same reason.
 */
export function sortOwnedGames(games: OwnedGameRow[], kind: PlatformSortKind): OwnedGameRow[] {
  const cmp = (a: OwnedGameRow, b: OwnedGameRow) =>
    a.name.trim().toLowerCase().localeCompare(b.name.trim().toLowerCase());
  switch (kind) {
    case "alpha_asc":
    case "steam_user_asc":
      return [...games].sort(cmp);
    case "alpha_desc":
    case "steam_user_desc":
      return [...games].sort((a, b) => -cmp(a, b));
    default:
      return games;
  }
}

export function filterOwnedGames(games: OwnedGameRow[], query: string): OwnedGameRow[] {
  const trimmed = query.trim();
  if (!trimmed) return games;
  return games.filter((game) => fuzzyWordsMatch(trimmed, game.name));
}

export type OwnerSplit = {
  shown: string[];
  /** How many owners the row hides behind the "+N" badge. */
  overflow: number;
};

export function splitGameOwners(owners: string[], max: number): OwnerSplit {
  if (max <= 0) return { shown: [], overflow: owners.length };
  if (owners.length <= max) return { shown: owners, overflow: 0 };
  return { shown: owners.slice(0, max), overflow: owners.length - max };
}

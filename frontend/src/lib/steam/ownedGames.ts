import type { PlatformSortKind } from "../../stores/platformListSort";
import { fuzzyWordsMatch } from "../searchFuzzy";

export type OwnedGameRow = {
  appId: string;
  name: string;
  iconUrl: string;
  /** SteamID64s. Empty means the game is installed locally and ownership is unknown. */
  owners: string[];
};

const byName = (a: OwnedGameRow, b: OwnedGameRow) =>
  a.name.trim().toLowerCase().localeCompare(b.name.trim().toLowerCase());

/**
 * An empty `owners` list means ownership was never resolved, not that nobody owns the
 * game, so ranking those rows as zero would mix guesses in with real counts. They keep
 * to one block at the end of both directions instead. The name tie-break stays
 * ascending whichever way the counts run, so an equal-count run reads the same in both.
 */
function byOwnerCount(direction: 1 | -1) {
  return (a: OwnedGameRow, b: OwnedGameRow) => {
    const aUnknown = a.owners.length === 0;
    const bUnknown = b.owners.length === 0;
    if (aUnknown !== bUnknown) return aUnknown ? 1 : -1;
    if (aUnknown) return byName(a, b);
    return direction * (a.owners.length - b.owners.length) || byName(a, b);
  };
}

/**
 * The games list carries no timestamps, so beyond the alphabetical and owner-count
 * entries every kind leaves the order untouched. Steam's by-username variants fall
 * back to the game name for the same reason.
 */
export function sortOwnedGames(games: OwnedGameRow[], kind: PlatformSortKind): OwnedGameRow[] {
  switch (kind) {
    case "alpha_asc":
    case "steam_user_asc":
      return [...games].sort(byName);
    case "alpha_desc":
    case "steam_user_desc":
      return [...games].sort((a, b) => -byName(a, b));
    case "owned_count_asc":
      return [...games].sort(byOwnerCount(1));
    case "owned_count_desc":
      return [...games].sort(byOwnerCount(-1));
    default:
      return games;
  }
}

export function filterOwnedGames(games: OwnedGameRow[], query: string): OwnedGameRow[] {
  const trimmed = query.trim();
  if (!trimmed) return games;
  return games.filter((game) => fuzzyWordsMatch(trimmed, game.name));
}

/**
 * Rows grouped into fixed-size blocks so the list can hand the renderer a handful of
 * `content-visibility: auto` boxes instead of one per game. A 2000-game library is
 * ~20k DOM nodes, and every style recalc or layout the page is forced into — opening
 * a context menu is enough — walks all of them. Per-row containment still leaves 2000
 * boxes to size and intersection-test; per-block containment leaves a few dozen.
 */
export type OwnedGameChunk = {
  /** Stable across re-sorts of the same library length, so blocks update in place. */
  index: number;
  rows: OwnedGameRow[];
};

export function chunkOwnedGames(games: OwnedGameRow[], size: number): OwnedGameChunk[] {
  if (size <= 0) return games.length > 0 ? [{ index: 0, rows: games }] : [];
  const out: OwnedGameChunk[] = [];
  for (let i = 0; i < games.length; i += size) {
    out.push({ index: out.length, rows: games.slice(i, i + size) });
  }
  return out;
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

/**
 * The accounts that can play the selected game, in the order the row lists them.
 * No game, or a game whose ownership was never resolved, yields nothing rather than
 * every account — the caller offers these as one-click switches. Owner ids with no
 * account behind them are dropped: a tile with no name or avatar is not clickable.
 */
export function gameOwnerAccounts<T>(
  game: Pick<OwnedGameRow, "owners"> | null | undefined,
  accountsById: Map<string, T>,
): T[] {
  if (!game) return [];
  const out: T[] = [];
  for (const steamId64 of game.owners) {
    const account = accountsById.get(steamId64);
    if (account) out.push(account);
  }
  return out;
}

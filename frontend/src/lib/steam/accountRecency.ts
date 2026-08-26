export type AccountRecency = {
  steamId64: string;
  /** RFC3339 from the backend, empty when Steam has no login date for the account. */
  lastLogin: string;
};

/**
 * No date means Steam has never recorded a login for this account, which is not
 * the same as one at the epoch: it ranks behind every real date rather than
 * being treated as an ancient login.
 */
const NEVER = Number.NEGATIVE_INFINITY;

function lastLoginMs(raw: string): number {
  const ms = Date.parse(raw.trim());
  return Number.isNaN(ms) ? NEVER : ms;
}

/**
 * The SteamID64s in the order the games strip should offer them: most recently
 * logged into first, so the accounts actually being played on hold its few
 * pinned tiles and the rest fall into the dropdown.
 *
 * `accounts` arrives in the order the account list shows. When that order is one
 * the user arranged - `hasSavedOrder` - it is handed back untouched: sorting or
 * dragging the list is a choice they made, and the next login must not undo it.
 */
export function rankAccountsByRecency(accounts: AccountRecency[], hasSavedOrder: boolean): string[] {
  const ids = accounts.map((account) => account.steamId64);
  if (hasSavedOrder) return ids;

  const listIndex = new Map(ids.map((id, index) => [id, index]));
  const lastLogin = new Map(accounts.map((account) => [account.steamId64, lastLoginMs(account.lastLogin)]));
  return ids.sort((a, b) => {
    const ma = lastLogin.get(a) ?? NEVER;
    const mb = lastLogin.get(b) ?? NEVER;
    // Compared rather than subtracted: two accounts that have never been logged
    // into are both NEVER, and Infinity - Infinity is NaN, which a comparator
    // must never return.
    if (ma !== mb) return ma < mb ? 1 : -1;
    // Equal dates - and the whole never-logged-in block - keep the order the
    // account list is already in, so the strip never shuffles for no reason.
    return (listIndex.get(a) ?? 0) - (listIndex.get(b) ?? 0);
  });
}

/**
 * A game's owner tiles in strip order, from the ranking above. Kept beside it so
 * the two cannot drift: the strip pins only the first few and drops the rest into
 * its dropdown, so this is what decides which accounts are one click away.
 */
export function orderByAccountRank<T extends { steamId64: string }>(
  tiles: T[],
  rank: Map<string, number>,
): T[] {
  // An owner the ranking has not caught up with trails the ones it knows rather
  // than jumping the queue.
  const at = (id: string) => rank.get(id) ?? Number.MAX_SAFE_INTEGER;
  return [...tiles].sort((a, b) => at(a.steamId64) - at(b.steamId64));
}

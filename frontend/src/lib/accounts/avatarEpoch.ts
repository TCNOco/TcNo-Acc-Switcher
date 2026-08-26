import { get, writable } from "svelte/store";

/**
 * Avatar cache-bust counters, per platform, keyed by the platform's account id.
 *
 * The counter goes into the avatar URL as `_tcv=`, so every view drawing the
 * same account has to agree on it - the account list is not the only one, the
 * Steam Guard vault draws the same faces at the same time. Counters last for the
 * session, not the page: one that restarts at zero asks for a URL the webview
 * still has cached from before the refresh.
 */
export const avatarEpochs = writable<Record<string, Record<string, number>>>({});

/** Snapshot of one platform's counters, for a caller that is not in a template. */
export function currentPlatformAvatarEpochs(platformKey: string): Record<string, number> {
  return get(avatarEpochs)[platformKey.trim()] ?? {};
}

export function setPlatformAvatarEpochs(platformKey: string, epochs: Record<string, number>): void {
  const key = platformKey.trim();
  if (!key) return;
  avatarEpochs.update((all) => {
    // The account list re-publishes on every load, changed or not, and each write
    // re-renders every avatar subscribed to the store.
    if (sameEpochs(all[key], epochs)) return all;
    return { ...all, [key]: epochs };
  });
}

function sameEpochs(a: Record<string, number> | undefined, b: Record<string, number>): boolean {
  if (!a) return false;
  const keys = Object.keys(a);
  if (keys.length !== Object.keys(b).length) return false;
  return keys.every((k) => a[k] === b[k]);
}

/**
 * Takes the whole map rather than reading the store itself so a template can pass
 * `$avatarEpochs` and get a reactive dependency on it.
 */
export function avatarEpochOf(
  all: Record<string, Record<string, number>>,
  platformKey: string,
  accountId: string,
): number {
  return all[platformKey.trim()]?.[accountId.trim()] ?? 0;
}

export function resetAvatarEpochs(): void {
  avatarEpochs.set({});
}

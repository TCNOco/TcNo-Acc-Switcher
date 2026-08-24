import { get, writable } from "svelte/store";

/**
 * Avatar cache-bust counters, per platform, keyed by the platform's account id.
 *
 * Refetching an avatar means asking for its URL with a different `_tcv=`, so the
 * counter is part of the URL, and every view drawing the same account has to
 * agree on it. That is why this is a store rather than account-list state: the
 * list is not the only view. The Steam Guard vault draws the same faces at the
 * same time, and while it kept its own counters it drew them at zero - the URL
 * from before the refresh, which the webview still had cached.
 *
 * Session-lifetime rather than page-lifetime for the same reason: counters that
 * restarted at zero when the platform page was rebuilt asked for `_tcv=0` again
 * and got back the avatar from before the refresh.
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

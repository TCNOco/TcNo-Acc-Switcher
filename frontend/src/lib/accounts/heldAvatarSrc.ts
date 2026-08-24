import { writable } from "svelte/store";
import { isProfileVideoUrl } from "../profileImageDrop";

/**
 * Keeps the avatar an account is already showing on screen until its replacement
 * has finished loading.
 *
 * A refresh changes an avatar's URL twice over: the backend marks the row pending
 * while it fetches, and the page bumps the row's avatar epoch to defeat the
 * webview's cache. Either one used to blank the tile - the first by swapping in
 * the platform placeholder, the second by rebuilding the row around a URL the
 * browser had never seen - so a refresh emptied the whole list and filled it back
 * in one face at a time.
 *
 * The state lives here rather than in the avatar component because that component
 * is destroyed and rebuilt on every epoch bump, which is precisely the moment the
 * previous src needs remembering.
 */
const shown = new Map<string, string>();
const loading = new Map<string, string>();

/**
 * Bumped whenever a swap lands. Components read it only to take a reactive
 * dependency; the maps above are ordinary and would not trigger a re-render.
 */
export const avatarSwapped = writable(0);

/**
 * Key for one account's avatar *in one view*.
 *
 * The hold has to outlive the component, so it cannot be keyed by the element -
 * but it must not be keyed by the account alone either. A slot holds one src, and
 * two views wanting different srcs for the same account take it from each other
 * on every swap, each swap re-rendering the other: the pair then alternates a
 * frame at a time for as long as both views are open.
 *
 * An account with no id gets no hold at all; a scope on its own is every account.
 */
export function heldAvatarKey(scope: string, accountId: string): string {
  const id = accountId.trim();
  return id ? `${scope}:${id}` : "";
}

export type AvatarPreloader = (src: string) => Promise<void>;

const browserPreloader: AvatarPreloader = (src) =>
  new Promise((resolve, reject) => {
    const image = new Image();
    image.onload = () => resolve();
    image.onerror = () => reject(new Error("avatar preload failed"));
    image.src = src;
  });

let preload: AvatarPreloader = browserPreloader;

/** Test seam. jsdom never fires load events for real URLs. */
export function setAvatarPreloader(next: AvatarPreloader | null): void {
  preload = next ?? browserPreloader;
}

export function resetHeldAvatars(): void {
  shown.clear();
  loading.clear();
  avatarSwapped.set(0);
}

/**
 * The src to paint for `key` right now: `desired` once it has loaded, and
 * whatever was on screen before until then.
 *
 * A failed load keeps the old image rather than falling through to the
 * placeholder. That is the case this exists for - a 404 on a file that is still
 * downloading, or a rate-limited refresh that could not fetch anything - and in
 * all of them the face already on screen is the best answer available.
 *
 * `_swapTick` is unused. Callers pass the `avatarSwapped` store value so that a
 * Svelte template takes a reactive dependency on it, since the maps behind this
 * are ordinary and nothing would otherwise re-render when a swap lands.
 */
export function heldAvatarSrc(key: string, desired: string, _swapTick = 0): string {
  const id = key.trim();
  const next = desired.trim();
  if (!id || !next) return desired;

  const current = shown.get(id);
  if (current === next) return next;

  // Nothing to preserve yet, so there is nothing to wait for either.
  if (!current) {
    shown.set(id, next);
    return next;
  }

  // A video cannot be validated by loading it into an Image, and an animated
  // avatar arriving is not the case that blanks a tile. Swap it straight in.
  if (isProfileVideoUrl(next)) {
    shown.set(id, next);
    return next;
  }

  if (loading.get(id) !== next) {
    loading.set(id, next);
    void preload(next).then(
      () => settle(id, next, true),
      () => settle(id, next, false),
    );
  }
  return current;
}

function settle(id: string, src: string, ok: boolean): void {
  // A third src may have been asked for while this one was loading; that request
  // owns the slot now and this result is stale.
  if (loading.get(id) !== src) return;
  loading.delete(id);
  if (!ok) return;
  shown.set(id, src);
  avatarSwapped.update((n) => n + 1);
}

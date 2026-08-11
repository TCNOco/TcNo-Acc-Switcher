import { offlineSafeImageSrc, withAssetCacheBust } from "../stores/offlineMode";
import { accountAvatar } from "./generatedAvatar";

export interface AccountAvatarInput {
  /** Streamer mode: never show the real face, whatever else is true. */
  streamer: boolean;
  /** Machine-local salt from the backend; see generatedAvatar. */
  salt: string;
  platformKey: string;
  /** Login name or platform ID — the seed the generated avatar is built from. */
  accountKey: string;
  imageUrl?: string | null;
  /** An avatar that is being fetched right now, not one that does not exist. */
  pending: boolean;
  epoch: number;
  offline: boolean;
  /** The platform's static placeholder, still used while an avatar is in flight. */
  fallback: string;
}

/**
 * The single rule for what an account tile shows.
 *
 * A generated avatar stands in for a real one in streamer mode, and for a missing
 * one anywhere — but never for one that is merely still loading. Swapping a
 * generated face in mid-refresh and back out again would make every avatar refresh
 * look like a bug.
 */
export function accountAvatarSrc(input: AccountAvatarInput): string {
  if (input.streamer) {
    return accountAvatar(input.salt, input.platformKey, input.accountKey) || input.fallback;
  }
  if (input.pending) {
    return input.fallback;
  }
  const real = offlineSafeImageSrc(
    input.offline,
    withAssetCacheBust(input.imageUrl, input.epoch),
    "",
  );
  if (real) {
    return real;
  }
  return accountAvatar(input.salt, input.platformKey, input.accountKey) || input.fallback;
}

<script lang="ts">
  import { get } from "svelte/store";
  import { offlineMode } from "../stores/offlineMode";
  import { avatarSalt, streamerMode } from "../stores/streamerMode";
  import { animationsSuspended } from "../stores/screenCovered";
  import { accountAvatarSrc } from "../lib/accountAvatarSrc";
  import { avatarEpochOf, avatarEpochs } from "../lib/accounts/avatarEpoch";
  import { avatarSwapped, heldAvatarKey, heldAvatarSrc } from "../lib/accounts/heldAvatarSrc";
  import { isProfileVideoUrl } from "../lib/profileImageDrop";
  import { miniProfileHover } from "../lib/actions/miniProfileHover";
  import { t } from "../stores/i18n";
  import { steamGuardBadge } from "../lib/steam/steamGuardBadge";
  import type { SteamAccountRow } from "../lib/steam/types";

  export let account: SteamAccountRow;
  /**
   * Cache-bust counter. Null rather than 0, which is itself a real epoch: unset
   * means fall back to the account's shared counter, which is what every other
   * view of the same account is drawing.
   */
  export let epoch: number | null = null;
  export let fallback = "";
  export let boundary: HTMLElement | null = null;
  /** Which view this avatar belongs to; see heldAvatarKey. */
  export let scope = "list";

  function steamListAvatarUrl(): string | undefined {
    const acc = account;
    const primary = acc.imageUrl?.trim() || undefined;
    const fb = acc.staticImageUrl?.trim() || undefined;
    // Behind a fullscreen game the same reasoning as offline applies: prefer the
    // still. It also covers animated GIF avatars, which an <img> plays forever
    // and no CSS can stop.
    if ($offlineMode || $animationsSuspended) {
      if (fb) return fb;
      if (primary && !isProfileVideoUrl(primary)) return primary;
      return undefined;
    }
    return primary ?? fb;
  }

  $: avatarEpoch = epoch ?? avatarEpochOf($avatarEpochs, "Steam", account.steamId64 ?? "");
  $: avatarSrc = accountAvatarSrc({
    streamer: $streamerMode,
    salt: $avatarSalt,
    platformKey: "Steam",
    accountKey: account.accountName || account.steamId64 || "",
    imageUrl: steamListAvatarUrl(),
    pending: account.avatarPending === true,
    epoch: avatarEpoch,
    offline: $offlineMode,
    fallback,
  });
  // $avatarSwapped is passed for its dependency rather than its value: the src
  // held for each account lives in a plain map, which Svelte cannot observe.
  $: displaySrc = heldAvatarSrc(heldAvatarKey(scope, account.steamId64 ?? ""), avatarSrc, $avatarSwapped);
  // Computed from what is actually painted, not from what was asked for. While a
  // held image is still on screen the two differ, and reading the wrong one puts
  // an image URL inside a <video>.
  $: avatarIsVideo = !$offlineMode && !$animationsSuspended && !$streamerMode && isProfileVideoUrl(displaySrc);
  // The hover card is a slab of the account's Steam profile — persona name, level,
  // games. Exactly what streamer mode exists to keep off the screen.
  $: miniProfileEnabled =
    !$streamerMode && !!(account.showMiniProfile && (account.miniProfileHtml ?? "").trim() !== "");
  $: guardBadge = steamGuardBadge(account);
  $: guardBadgeLabel = guardBadge ? $t(guardBadge.labelKey) : "";
  // The frame is always rendered when the account has one; suspending only stops
  // it moving, so it never disappears from a list the user can still see.
  $: showAvatarFrame =
    account.showAvatarFrame === true &&
    (account.avatarFrameUrl ?? "").trim() !== "" &&
    !$offlineMode &&
    !$streamerMode;
  /**
   * Suspending swaps the APNG for a still cut from it by the backend.
   *
   * The page cannot produce that still itself. There is no way to pause an
   * animated <img>, and drawImage only ever hands back the default image — five
   * captures spread across three seconds of a running animation came back
   * byte-identical. In these frames the default image is frame 0, which on a
   * lightning border is the peak of the flash and several times thicker than
   * whatever was on screen a moment earlier, so freezing onto a canvas made the
   * border visibly swell. profileimage.StillFromAPNG picks the median frame
   * instead, which is what the border looks like for most of its cycle.
   *
   * A frame with no still is one that does not animate, so leaving it running
   * costs nothing.
   */
  $: frameSrc =
    $animationsSuspended && (account.avatarFrameStillUrl ?? "").trim() !== ""
      ? (account.avatarFrameStillUrl ?? "")
      : (account.avatarFrameUrl ?? "");
</script>

<span class="steam-acc-avatar-wrap">
  {#if avatarIsVideo}
    <video
      class="steam-acc-avatar"
      src={displaySrc}
      autoplay loop muted playsinline
      aria-hidden="true" draggable="false"
      use:miniProfileHover={{
        html: account.miniProfileHtml ?? "",
        boundary,
        offline: $offlineMode,
        enabled: miniProfileEnabled,
      }}
    ></video>
  {:else}
    <img
      class="steam-acc-avatar"
      src={displaySrc}
      alt="" draggable="false"
      use:miniProfileHover={{
        html: account.miniProfileHtml ?? "",
        boundary,
        offline: $offlineMode,
        enabled: miniProfileEnabled,
      }}
    />
  {/if}
  {#if showAvatarFrame}
    <img class="steam-acc-avatar-frame" src={frameSrc} alt="" draggable="false" />
  {/if}
  <!-- Sits above the avatar frame so a framed avatar does not hide it. A pending
       setup and a login-only record get their own look: neither is protection,
       and showing the same badge would claim an authenticator the account
       cannot actually use. -->
  {#if guardBadge}
    <span
      class="steam-acc-avatar-guard"
      class:steam-acc-avatar-guard--pending={guardBadge.variant === "pending"}
      class:steam-acc-avatar-guard--login-only={guardBadge.variant === "login-only"}
      role="img"
      aria-label={guardBadgeLabel}
      title={guardBadgeLabel}
    >
      <svg viewBox="0 0 448 512" aria-hidden="true"><path d="M400 224h-24v-72C376 68.2 307.8 0 224 0S72 68.2 72 152v72H48c-26.5 0-48 21.5-48 48v192c0 26.5 21.5 48 48 48h352c26.5 0 48-21.5 48-48V272c0-26.5-21.5-48-48-48zm-104 0H152v-72c0-39.7 32.3-72 72-72s72 32.3 72 72v72z" /></svg>
    </span>
  {/if}
</span>

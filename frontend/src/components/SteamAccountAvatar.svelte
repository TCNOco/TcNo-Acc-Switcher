<script lang="ts">
  import { get } from "svelte/store";
  import { offlineMode } from "../stores/offlineMode";
  import { avatarSalt, streamerMode } from "../stores/streamerMode";
  import { animationsSuspended } from "../stores/screenCovered";
  import { accountAvatarSrc } from "../lib/accountAvatarSrc";
  import { avatarSwapped, heldAvatarSrc } from "../lib/accounts/heldAvatarSrc";
  import { isProfileVideoUrl } from "../lib/profileImageDrop";
  import { miniProfileHover } from "../lib/actions/miniProfileHover";
  import { t } from "../stores/i18n";
  import { steamGuardBadge } from "../lib/steam/steamGuardBadge";
  import type { SteamAccountRow } from "../lib/steam/types";

  export let account: SteamAccountRow;
  export let epoch = 0;
  export let fallback = "";
  export let boundary: HTMLElement | null = null;

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

  $: avatarSrc = accountAvatarSrc({
    streamer: $streamerMode,
    salt: $avatarSalt,
    platformKey: "Steam",
    accountKey: account.accountName || account.steamId64 || "",
    imageUrl: steamListAvatarUrl(),
    pending: account.avatarPending === true,
    epoch,
    offline: $offlineMode,
    fallback,
  });
  // $avatarSwapped is passed for its dependency rather than its value: the src
  // held for each account lives in a plain map, which Svelte cannot observe.
  $: displaySrc = heldAvatarSrc(account.steamId64 || "", avatarSrc, $avatarSwapped);
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
  /**
   * Steam's avatar frames are APNGs, and suspending one means taking it out of
   * the document — there is no way to pause an animated <img>, and removing it is
   * what stops Chromium decoding it.
   *
   * It used to be replaced by a canvas holding a still of itself, so the
   * decoration stayed put and merely stopped moving. That still was always wrong.
   * drawImage hands back an animated PNG's default image and nothing else: five
   * captures spread across three seconds of a running animation came back
   * byte-identical, and in these files the default image is frame 0 — which for a
   * lightning border is the peak of the flash, several times thicker than the
   * frame that was on screen a moment earlier. No browser API exposes the current
   * frame, so an accurate still cannot be produced from the page at all.
   *
   * Showing nothing is at least honest, and it costs little: a frame is only
   * suspended when the user is looking somewhere else.
   */
  $: showAvatarFrame =
    account.showAvatarFrame === true &&
    (account.avatarFrameUrl ?? "").trim() !== "" &&
    !$offlineMode &&
    !$streamerMode &&
    !$animationsSuspended;
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
    <img
      class="steam-acc-avatar-frame"
      src={account.avatarFrameUrl ?? ""}
      alt=""
      draggable="false"
    />
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

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
  // The frame is always rendered when the account has one; suspending only
  // freezes it, so it never disappears from a list the user can still see.
  $: showAvatarFrame =
    account.showAvatarFrame === true &&
    (account.avatarFrameUrl ?? "").trim() !== "" &&
    !$offlineMode &&
    !$streamerMode;
  $: framePaused = showAvatarFrame && $animationsSuspended;

  let frameImgEl: HTMLImageElement | undefined;
  let frameCanvasEl: HTMLCanvasElement | undefined;

  /**
   * Copy whatever the frame is showing right now onto the canvas that replaces
   * it. Runs while the <img> is still painted — Svelte applies reactive
   * statements before it patches the DOM, so the element has not been hidden yet.
   */
  function captureFrame(): void {
    const img = frameImgEl;
    const canvas = frameCanvasEl;
    if (!img || !canvas || !img.complete || img.naturalWidth === 0) {
      return;
    }
    if (canvas.width !== img.naturalWidth || canvas.height !== img.naturalHeight) {
      canvas.width = img.naturalWidth;
      canvas.height = img.naturalHeight;
    }
    try {
      canvas.getContext("2d")?.drawImage(img, 0, 0);
    } catch {
      /* A frame that taints the canvas is not worth failing the render over. */
    }
  }

  /** A frame that finishes loading while already suspended still needs a still. */
  function onFrameLoad(): void {
    if (framePaused) {
      captureFrame();
    }
  }

  $: if (framePaused) {
    captureFrame();
  }
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
  <!-- Frozen rather than removed while a game is running. There is no way to
       pause an APNG or animated GIF in an <img>, but Chromium stops decoding one
       that is display:none — so the last painted frame is copied to a canvas and
       shown in its place. The decoration stays exactly where it was, it simply
       stops moving. Removing it outright was wrong: with the game on a second
       monitor the list is still on screen. -->
  {#if showAvatarFrame}
    <img
      bind:this={frameImgEl}
      class="steam-acc-avatar-frame"
      class:steam-acc-avatar-frame--paused={framePaused}
      src={account.avatarFrameUrl ?? ""}
      alt=""
      draggable="false"
      on:load={onFrameLoad}
    />
    <canvas
      bind:this={frameCanvasEl}
      class="steam-acc-avatar-frame"
      class:steam-acc-avatar-frame--paused={!framePaused}
      aria-hidden="true"
    ></canvas>
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

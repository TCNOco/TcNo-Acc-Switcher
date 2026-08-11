<script lang="ts">
  import { get } from "svelte/store";
  import { offlineMode } from "../stores/offlineMode";
  import { avatarSalt, streamerMode } from "../stores/streamerMode";
  import { accountAvatarSrc } from "../lib/accountAvatarSrc";
  import { isProfileVideoUrl } from "../lib/profileImageDrop";
  import { miniProfileHover } from "../lib/actions/miniProfileHover";
  import { t } from "../stores/i18n";
  import type { SteamAccountRow } from "../lib/steam/types";

  export let account: SteamAccountRow;
  export let epoch = 0;
  export let fallback = "";
  export let boundary: HTMLElement | null = null;

  function steamListAvatarUrl(): string | undefined {
    const acc = account;
    const primary = acc.imageUrl?.trim() || undefined;
    const fb = acc.staticImageUrl?.trim() || undefined;
    if ($offlineMode) {
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
  $: avatarIsVideo = !$offlineMode && !$streamerMode && isProfileVideoUrl(avatarSrc);
  // The hover card is a slab of the account's Steam profile — persona name, level,
  // games. Exactly what streamer mode exists to keep off the screen.
  $: miniProfileEnabled =
    !$streamerMode && !!(account.showMiniProfile && (account.miniProfileHtml ?? "").trim() !== "");
  // Defaults to shown: the flag arrives with account enrichment, so it is
  // briefly undefined on first paint and the lock should not flicker off.
  $: guardBadgeLabel = account.showSteamGuardLock === false
    ? ""
    : account.hasSteamGuard
      ? $t("SteamGuard_Badge_Stored")
      : account.steamGuardPending
        ? $t("SteamGuard_Badge_Unfinished")
        : "";
</script>

<span class="steam-acc-avatar-wrap">
  {#if avatarIsVideo}
    <video
      class="steam-acc-avatar"
      src={avatarSrc}
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
      src={avatarSrc}
      alt="" draggable="false"
      use:miniProfileHover={{
        html: account.miniProfileHtml ?? "",
        boundary,
        offline: $offlineMode,
        enabled: miniProfileEnabled,
      }}
    />
  {/if}
  {#if account.showAvatarFrame && (account.avatarFrameUrl ?? "").trim() !== "" && !$offlineMode && !$streamerMode}
    <img class="steam-acc-avatar-frame" src={account.avatarFrameUrl ?? ""} alt="" draggable="false" />
  {/if}
  <!-- Sits above the avatar frame so a framed avatar does not hide it. A pending
       setup gets its own look: it is not protection yet, and showing the same
       badge would claim an authenticator the account cannot actually use. -->
  {#if guardBadgeLabel}
    <span
      class="steam-acc-avatar-guard"
      class:steam-acc-avatar-guard--pending={account.steamGuardPending && !account.hasSteamGuard}
      role="img"
      aria-label={guardBadgeLabel}
      title={guardBadgeLabel}
    >
      <svg viewBox="0 0 448 512" aria-hidden="true"><path d="M400 224h-24v-72C376 68.2 307.8 0 224 0S72 68.2 72 152v72H48c-26.5 0-48 21.5-48 48v192c0 26.5 21.5 48 48 48h352c26.5 0 48-21.5 48-48V272c0-26.5-21.5-48-48-48zm-104 0H152v-72c0-39.7 32.3-72 72-72s72 32.3 72 72v72z" /></svg>
    </span>
  {/if}
</span>

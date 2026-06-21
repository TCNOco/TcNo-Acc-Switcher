<script lang="ts">
  import { get } from "svelte/store";
  import { offlineMode, offlineSafeImageSrc, withAssetCacheBust } from "../stores/offlineMode";
  import { isProfileVideoUrl } from "../lib/profileImageDrop";
  import { miniProfileHover } from "../lib/actions/miniProfileHover";
  import type { SteamAccountRow } from "../lib/steam/types";

  export let account: SteamAccountRow;
  export let epoch = 0;
  export let fallback = "";
  export let boundary: HTMLElement | null = null;

  function steamListAvatarUrl(): string | undefined {
    const acc = account;
    if (acc.avatarPending) return undefined;
    const primary = acc.imageUrl?.trim() || undefined;
    const fb = acc.staticImageUrl?.trim() || undefined;
    if ($offlineMode) {
      if (fb) return fb;
      if (primary && !isProfileVideoUrl(primary)) return primary;
      return undefined;
    }
    return primary ?? fb;
  }

  $: avatarSrc = offlineSafeImageSrc(
    $offlineMode,
    withAssetCacheBust(steamListAvatarUrl(), epoch),
    fallback,
  );
  $: avatarIsVideo = !$offlineMode && isProfileVideoUrl(avatarSrc);
</script>

<span class="steam-acc-avatar-wrap">
  {#if avatarIsVideo}
    <video
      class="steam-acc-avatar"
      class:status_vac={account.showVac && account.vac}
      class:status_limited={account.showLimited && account.ltd}
      src={avatarSrc}
      autoplay loop muted playsinline
      aria-hidden="true" draggable="false"
      use:miniProfileHover={{
        html: account.miniProfileHtml ?? "",
        boundary,
        offline: $offlineMode,
        enabled: !!(account.showMiniProfile && (account.miniProfileHtml ?? "").trim() !== ""),
      }}
    ></video>
  {:else}
    <img
      class="steam-acc-avatar"
      class:status_vac={account.showVac && account.vac}
      class:status_limited={account.showLimited && account.ltd}
      src={avatarSrc}
      alt="" draggable="false"
      use:miniProfileHover={{
        html: account.miniProfileHtml ?? "",
        boundary,
        offline: $offlineMode,
        enabled: !!(account.showMiniProfile && (account.miniProfileHtml ?? "").trim() !== ""),
      }}
    />
  {/if}
  {#if account.showAvatarFrame && (account.avatarFrameUrl ?? "").trim() !== "" && !$offlineMode}
    <img class="steam-acc-avatar-frame" src={offlineSafeImageSrc($offlineMode, account.avatarFrameUrl ?? "", fallback)} alt="" draggable="false" />
  {/if}
</span>

<script lang="ts">
  import { onDestroy } from "svelte";
  import {
    accountTagBubbleLabel,
    formatTagExpiryCountdown,
    tagHasExpiry,
    textColorForTagBackground,
    type TagDefRow,
  } from "../lib/accountTagsContext";
  import { tooltip } from "../lib/actions/tooltip";
  import { t } from "../stores/i18n";

  export let tags: TagDefRow[] = [];

  const fallbackTagBg = "#555555";

  let nowMs = Date.now();
  let expiryTimer: number | null = null;

  function clearExpiryTimer(): void {
    if (expiryTimer != null) {
      window.clearInterval(expiryTimer);
      expiryTimer = null;
    }
  }

  function syncExpiryTimer(enabled: boolean): void {
    if (typeof window === "undefined") {
      return;
    }
    if (enabled && expiryTimer == null) {
      expiryTimer = window.setInterval(() => {
        nowMs = Date.now();
      }, 1000);
    } else if (!enabled) {
      clearExpiryTimer();
    }
  }

  function expiryTooltip(tg: TagDefRow, now: number): string {
    const countdown = formatTagExpiryCountdown(tg.expiresAt, now);
    if (!countdown) {
      return "";
    }
    return $t("Tags_ExpiryCountdown", { countdown });
  }

  $: syncExpiryTimer(tags.some(tagHasExpiry));

  onDestroy(clearExpiryTimer);
</script>

{#if tags.length > 0}
  <div class="acc_tag_bubbles">
    {#each tags as tg (tg.id)}
      <span
        class="acc_tag_bubble"
        class:acc_tag_bubble--default={!tg.color?.trim()}
        style:background-color={tg.color?.trim() ? tg.color : undefined}
        style:color={textColorForTagBackground(tg.color?.trim() ? tg.color : fallbackTagBg)}
        use:tooltip={expiryTooltip(tg, nowMs) ? { text: expiryTooltip(tg, nowMs), placement: "top" } : undefined}
        aria-label={expiryTooltip(tg, nowMs) ? `${accountTagBubbleLabel(tg.name)}. ${expiryTooltip(tg, nowMs)}` : undefined}
      >
        {#if tagHasExpiry(tg)}
          <svg class="acc_tag_bubble_icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
            <circle cx="12" cy="12" r="9" />
            <path d="M12 7v5l3 2" />
          </svg>
        {/if}
        <span class="acc_tag_bubble_text">{accountTagBubbleLabel(tg.name)}</span>
      </span>
    {/each}
  </div>
{/if}

<style lang="scss">
  .acc_tag_bubbles {
    display: flex;
    flex-wrap: wrap;
    gap: 0.25rem;
    margin: 0.2rem 0 0;
    max-width: 100%;
    flex-direction: row;
    justify-content: center;
  }

  .acc_tag_bubble--default {
    background-color: var(--tag-default-bg);
  }

  .acc_tag_bubble {
    display: inline-flex;
    align-items: center;
    gap: 0.22rem;
    padding: 0.1rem 0.4rem;
    border-radius: 999px;
    font-size: 0.65rem;
    font-weight: 600;
    line-height: 1.3;
    max-width: 100%;
    overflow: hidden;
    white-space: nowrap;
  }

  .acc_tag_bubble_icon {
    width: 0.72rem;
    height: 0.72rem;
    flex: 0 0 auto;
    fill: none;
    stroke: currentColor;
    stroke-width: 2;
    stroke-linecap: round;
    stroke-linejoin: round;
    opacity: 0.85;
  }

  .acc_tag_bubble_text {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>

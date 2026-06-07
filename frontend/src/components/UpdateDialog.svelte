<script lang="ts">
  import { createEventDispatcher } from "svelte";
  import { fly, fade } from "svelte/transition";
  import { cubicOut } from "svelte/easing";
  import { motionEnabled } from "../lib/animation";
  import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
  import { t } from "../stores/i18n";

  export let message = "";
  export let downloadUrl = "";

  const dispatch = createEventDispatcher();

  let loading = false;

  function dismiss() {
    dispatch("dismiss");
  }

  async function updateNow() {
    loading = true;
    try {
      await PlatformService.CheckForUpdatesAndInstall();
    } catch {
      /* ignore */
    }
    loading = false;
    dismiss();
  }

  function openDownloadPage() {
    window.open(downloadUrl, "_blank");
  }

  function onBackdropClick(e: MouseEvent) {
    if ((e.target as HTMLElement).classList.contains("updateDialog__backdrop")) {
      dismiss();
    }
  }

  function onKeydown(e: KeyboardEvent) {
    if (e.key === "Escape") {
      dismiss();
    }
  }
</script>

<svelte:window on:keydown={onKeydown} />

<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
<div
  class="updateDialog__backdrop"
  transition:fade={{ duration: motionEnabled() ? 160 : 0 }}
  on:click={onBackdropClick}
  role="dialog"
  aria-modal="true"
  aria-label={$t("Update")}
>
  <div
    class="updateDialog"
    transition:fly={{ y: 24, duration: motionEnabled() ? 200 : 0, easing: cubicOut }}
  >
    <h2 class="updateDialog__title">{$t("Heading_UpdateAvailable")}</h2>

    {#if message}
      <div class="updateDialog__message">{message}</div>
    {/if}

    <div class="updateDialog__actions">
      <button
        type="button"
        class="updateDialog__btn updateDialog__btn--primary"
        disabled={loading}
        on:click={updateNow}
      >
        {loading ? $t("Button_Loading") : $t("Button_UpdateNow")}
      </button>
      <button
        type="button"
        class="updateDialog__btn updateDialog__btn--secondary"
        on:click={openDownloadPage}
      >
        {$t("Button_OpenDownloadPage")}
      </button>
    </div>

    <button
      type="button"
      class="updateDialog__close"
      aria-label={$t("Button_Close")}
      on:click={dismiss}
    >
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" aria-hidden="true">
        <path
          d="M256 8C119 8 8 119 8 256s111 248 248 248 248-111 248-248S393 8 256 8zm121.6 313.1c4.7 4.7 4.7 12.3 0 17L338 377.6c-4.7 4.7-12.3 4.7-17 0L256 312l-65.1 65.6c-4.7 4.7-12.3 4.7-17 0L134.4 338c-4.7-4.7-4.7-12.3 0-17l65.6-65-65.6-65.1c-4.7-4.7-4.7-12.3 0-17l39.6-39.6c4.7-4.7 12.3-4.7 17 0l65 65.7 65.1-65.6c4.7-4.7 12.3-4.7 17 0l39.6 39.6c4.7 4.7 4.7 12.3 0 17L312 256l65.6 65.1z"
        />
      </svg>
    </button>
  </div>
</div>

<style>
  .updateDialog__backdrop {
    position: fixed;
    inset: 0;
    z-index: 9999;
    display: flex;
    align-items: center;
    justify-content: center;
    background: rgb(0 0 0 / 55%);
  }

  .updateDialog {
    position: relative;
    background: var(--surface-bg, #1b2636);
    border: 1px solid var(--border, #334155);
    border-radius: 12px;
    padding: 28px 32px;
    max-width: 420px;
    width: 90vw;
    box-shadow: 0 8px 32px rgb(0 0 0 / 40%);
  }

  .updateDialog__title {
    margin: 0 0 14px;
    font-size: 1.15rem;
    font-weight: 600;
    color: var(--text, #e2e8f0);
  }

  .updateDialog__message {
    color: var(--text-secondary, #94a3b8);
    font-size: 0.9rem;
    line-height: 1.5;
    margin-bottom: 20px;
    white-space: pre-wrap;
  }

  .updateDialog__actions {
    display: flex;
    gap: 10px;
  }

  .updateDialog__btn {
    flex: 1;
    padding: 10px 16px;
    border: none;
    border-radius: 8px;
    font-size: 0.9rem;
    font-weight: 500;
    cursor: pointer;
    transition: opacity 0.15s;
  }

  .updateDialog__btn:disabled {
    opacity: 0.5;
    cursor: wait;
  }

  .updateDialog__btn--primary {
    background: var(--accent, #f90);
    color: var(--text-on-bright-bg, #0f172a);
  }

  .updateDialog__btn--secondary {
    background: var(--btn-bg, #334155);
    color: var(--text, #e2e8f0);
  }

  .updateDialog__close {
    position: absolute;
    top: 12px;
    right: 14px;
    background: none;
    border: none;
    cursor: pointer;
    padding: 4px;
    color: var(--text-secondary, #94a3b8);
  }

  .updateDialog__close svg {
    width: 18px;
    height: 18px;
    fill: currentColor;
  }
</style>

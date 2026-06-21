<script lang="ts">
  import { Browser } from "@wailsio/runtime";
  import { get } from "svelte/store";
  import * as PlatformService from "../../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
  import { t } from "../../stores/i18n";
  import { dismissModal } from "../../stores/modal";
  import { offlineMode } from "../../stores/offlineMode";
  import { pushToast } from "../../stores/toast";

  export let message = "";
  export let downloadUrl = "";

  let loading = false;

  async function updateNow(): Promise<void> {
    loading = true;
    try {
      await PlatformService.CheckForUpdatesAndInstall();
    } catch {
      /* ignore */
    }
    loading = false;
    dismissModal();
  }

  function openDownloadPage(): void {
    if (get(offlineMode)) {
      pushToast({
        type: "info",
        message: get(t)("Toast_OfflineModeNoLinks"),
        duration: 5000,
      });
      return;
    }
    void Browser.OpenURL(downloadUrl);
  }
</script>

{#if message}
  <div class="modal-text update-message">{message}</div>
{/if}

<div class="update-actions">
  <button type="button" class="update-btn update-btn--primary" disabled={loading} on:click={updateNow}>
    {loading ? $t("Button_Loading") : $t("Button_UpdateNow")}
  </button>
  <button type="button" class="update-btn update-btn--secondary" on:click={openDownloadPage}>
    {$t("Button_OpenDownloadPage")}
  </button>
</div>

<style lang="scss">
  .update-message {
    margin: 0;
    white-space: pre-wrap;
  }

  .update-actions {
    display: flex;
    gap: 0.65rem;
    width: 100%;
    margin-top: 0.15rem;
  }

  .update-btn {
    flex: 1;
    min-width: 0;
    padding: 0.65rem 1rem;
    border: none;
    font: inherit;
    font-size: 0.9rem;
    font-weight: 500;
    cursor: pointer;
    transition:
      opacity 0.15s ease,
      transform 0.08s ease-out;

    &:active:not(:disabled) {
      transform: scale(0.97);
    }

    &:disabled {
      opacity: 0.5;
      cursor: wait;
    }
  }

  .update-btn--primary {
    background: var(--accent);
    color: var(--text-on-bright-bg);
  }

  .update-btn--secondary {
    background: var(--button-bg);
    color: var(--whiteSecondary);
  }
</style>

<script lang="ts">
  import * as PlatformService from "../../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
  import { formatToastWithError } from "../../lib/formatWailsError";
  import { openExternalUrl } from "../../lib/openExternalUrl";
  import { t } from "../../stores/i18n";
  import { dismissModal } from "../../stores/modal";
  import { pushToast } from "../../stores/toast";

  export let message = "";
  export let downloadUrl = "";

  let loading = false;

  async function updateNow(): Promise<void> {
    loading = true;
    try {
      await PlatformService.CheckForUpdatesAndInstall();
      dismissModal();
    } catch (e) {
      pushToast({
        type: "error",
        message: formatToastWithError($t("Toast_SaveFailed"), e),
        duration: 8000,
      });
    } finally {
      loading = false;
    }
  }

  function openDownloadPage(): void {
    void openExternalUrl(downloadUrl);
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

<script lang="ts">
  import * as PlatformService from "../../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
  import { formatToastWithError } from "../../lib/formatWailsError";
  import { openExternalUrl } from "../../lib/openExternalUrl";
  import { renderReleaseNotes } from "../../lib/releaseNotes";
  import { sanitizeHtml } from "../../lib/sanitizeHtml";
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

  // The navigation guard neutralizes every anchor in injected HTML, so
  // release-notes links have to be routed through openExternalUrl here.
  function onNotesClick(e: MouseEvent): void {
    const anchor = e.target instanceof Element ? e.target.closest("a[href]") : null;
    if (!anchor) {
      return;
    }
    e.preventDefault();
    void openExternalUrl((anchor as HTMLAnchorElement).href);
  }
</script>

{#if message}
  <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
  <div class="modal-text update-message" on:click={onNotesClick}>
    {@html sanitizeHtml(renderReleaseNotes(message), "inline")}
  </div>
{/if}

<div class="update-actions">
  <button type="button" class="update-btn update-btn--primary" disabled={loading} on:click={updateNow}>
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" aria-hidden="true"><path d="M216 0h80c13.3 0 24 10.7 24 24v168h87.7c17.8 0 26.7 21.5 14.1 34.1L269.7 378.3c-7.5 7.5-19.8 7.5-27.3 0L90.1 226.1c-12.6-12.6-3.7-34.1 14.1-34.1H192V24c0-13.3 10.7-24 24-24zm296 376v112c0 13.3-10.7 24-24 24H24c-13.3 0-24-10.7-24-24V376c0-13.3 10.7-24 24-24h146.7l49 49c20.1 20.1 52.5 20.1 72.6 0l49-49H488c13.3 0 24 10.7 24 24z" /></svg>
    {loading ? $t("Button_Loading") : $t("Button_UpdateNow")}
  </button>
  <button type="button" class="update-btn update-btn--secondary" on:click={openDownloadPage}>
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M1.5 8.5h7.5v3H4.5v8h8v-4h3v7h-14V8.5zM22.5 1.5v8.5h-3V6.6l-7.2 7.2-2.1-2.1 7.2-7.2H14v-3h8.5z"/></svg>
    {$t("Button_OpenDownloadPage")}
  </button>
</div>

<style lang="scss">
  .update-message {
    margin: 0;
    max-height: 45vh;
    overflow-y: auto;

    /* Rendered release-notes markup comes from @html, so scoping needs :global. */
    :global(p) {
      margin: 0 0 0.5em;
    }

    :global(ul),
    :global(ol) {
      margin: 0 0 0.5em;
      padding-left: 1.2em;
    }

    :global(li) {
      margin-bottom: 0.2em;
    }

    :global(> :last-child) {
      margin-bottom: 0;
    }

    :global(code) {
      font-size: 0.85em;
      background: rgb(255 255 255 / 8%);
      border-radius: 4px;
      padding: 0.1em 0.35em;
    }

    :global(a) {
      color: var(--accent);
    }
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

    svg {
      height: 1rem;
      margin-right: 0.35rem;
      fill: currentColor;
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

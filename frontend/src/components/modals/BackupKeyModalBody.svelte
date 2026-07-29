<script lang="ts">
  import { t } from "../../stores/i18n";
  import { pushToast } from "../../stores/toast";
  import { formatToastWithError } from "../../lib/formatWailsError";

  export let code = "";
  export let intro = "";
  /** Injected so this component stays free of service imports and testable. */
  export let saveToFile: ((code: string) => Promise<string>) | null = null;

  let copied = false;
  let copyResetTimer: ReturnType<typeof setTimeout> | undefined;
  let saving = false;
  let inputEl: HTMLInputElement | undefined;

  async function copy(): Promise<void> {
    try {
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(code);
      } else {
        // Fallback for a webview without the async clipboard: select the field
        // the user can already see and copy that.
        inputEl?.select();
        if (!document.execCommand?.("copy")) throw new Error("copy command failed");
      }
      copied = true;
      if (copyResetTimer) clearTimeout(copyResetTimer);
      copyResetTimer = setTimeout(() => { copied = false; }, 2_500);
    } catch (error) {
      pushToast({
        type: "error",
        message: formatToastWithError($t("SteamGuard_BackupKey_CopyFailed"), error),
        duration: 6000,
      });
    }
  }

  async function download(): Promise<void> {
    if (!saveToFile || saving) return;
    saving = true;
    try {
      await saveToFile(code);
      pushToast({ type: "success", message: $t("SteamGuard_BackupKey_Saved"), duration: 5000 });
    } catch (error) {
      // Cancelling the save dialog is not a failure worth a toast.
      const message = error instanceof Error ? error.message : String(error);
      if (!message.toLowerCase().includes("cancel")) {
        pushToast({
          type: "error",
          message: formatToastWithError($t("SteamGuard_BackupKey_SaveFailed"), error),
          duration: 8000,
        });
      }
    } finally {
      saving = false;
    }
  }
</script>

<div class="backup-key">
  {#if intro}<p class="backup-key__intro">{intro}</p>{/if}

  <div class="backup-key__row">
    <!-- readonly rather than a code block: the point is that the user can select
         and copy it. Not disabled, which would take it out of the tab order and
         block selection in some engines. -->
    <input
      bind:this={inputEl}
      class="modal-input backup-key__value"
      value={code}
      readonly
      spellcheck="false"
      autocomplete="off"
      aria-label={$t("SteamGuard_Factor_BackupKey")}
      on:focus={(event) => event.currentTarget.select()}
    />

    <button
      type="button"
      class="btnicontext backup-key__copy"
      class:backup-key__copy--done={copied}
      on:click={() => void copy()}
    >
      {#if copied}
        <svg class="backup-key__icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M20 6 9 17l-5-5" /></svg>
        {$t("SteamGuard_BackupKey_Copied")}
      {:else}
        <svg class="backup-key__icon" viewBox="0 0 448 512" aria-hidden="true"><path d="M320 448v40c0 13.255-10.745 24-24 24H24c-13.255 0-24-10.745-24-24V120c0-13.255 10.745-24 24-24h72v296c0 30.879 25.121 56 56 56h168zm0-344V0H152c-13.255 0-24 10.745-24 24v368c0 13.255 10.745 24 24 24h272c13.255 0 24-10.745 24-24V128H344c-13.2 0-24-10.8-24-24zm120.971-31.029L375.029 7.029A24 24 0 0 0 358.059 0H352v96h96v-6.059a24 24 0 0 0-7.029-16.97z" /></svg>
        {$t("SteamGuard_BackupKey_Copy")}
      {/if}
    </button>

    <button
      type="button"
      class="btnicontext backup-key__download"
      disabled={!saveToFile || saving}
      title={$t("SteamGuard_BackupKey_Download")}
      aria-label={$t("SteamGuard_BackupKey_Download")}
      on:click={() => void download()}
    >
      <svg class="backup-key__icon" viewBox="0 0 512 512" aria-hidden="true"><path d="M216 0h80c13.3 0 24 10.7 24 24v168h87.7c17.8 0 26.7 21.5 14.1 34.1L269.7 378.3c-7.5 7.5-19.8 7.5-27.3 0L90.1 226.1c-12.6-12.6-3.7-34.1 14.1-34.1H192V24c0-13.3 10.7-24 24-24zm296 376v112c0 13.3-10.7 24-24 24H24c-13.3 0-24-10.7-24-24V376c0-13.3 10.7-24 24-24h146.7l49 49c20.1 20.1 52.5 20.1 72.6 0l49-49H488c13.3 0 24 10.7 24 24z" /></svg>
    </button>
  </div>
</div>

<style lang="scss">
  .backup-key__intro {
    margin: 0 0 0.75rem;
  }

  .backup-key__row {
    display: flex;
    gap: 0.4rem;
    align-items: stretch;
  }

  .backup-key__value {
    flex: 1 1 auto;
    min-width: 0;
    font-family: ui-monospace, "Cascadia Mono", Consolas, monospace;
    letter-spacing: 0.04em;
  }

  .backup-key__copy,
  .backup-key__download {
    display: inline-flex;
    gap: 0.4em;
    align-items: center;
    justify-content: center;
    white-space: nowrap;
  }

  /* Square, icon only. Both sides are stated: aspect-ratio does not work here
     because a flex item's base size resolves from the lone glyph, and the row
     stretches its items, which was making the height follow the input next to
     it while the width stayed pinned. */
  .backup-key__download {
    flex: 0 0 auto;
    align-self: center;
    width: 2.4rem;
    min-width: 2.4rem;
    height: 2.4rem;
    min-height: 2.4rem;
    padding: 0;
  }

  .backup-key__icon {
    width: 1em;
    height: 1em;
    fill: currentColor;
    /* The shared icon rule spaces an icon from its label. This row uses gap
       instead, and the save button has no label for it to sit against. */
    margin-right: 0;
  }

  .backup-key__copy--done {
    background: var(--role-success, #4caf50) !important;
    border-color: var(--role-success, #4caf50) !important;
    color: #fff !important;

    .backup-key__icon {
      fill: none;
      stroke: currentColor;
      stroke-width: 2.5;
      stroke-linecap: round;
      stroke-linejoin: round;
    }
  }
</style>

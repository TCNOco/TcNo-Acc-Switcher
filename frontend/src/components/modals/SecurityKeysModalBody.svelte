<script lang="ts">
  import { t } from "../../stores/i18n";
  import { dismissModal } from "../../stores/modal";

  type SecurityKeyRow = {
    id: string;
    label: string;
    requiresPassword: boolean;
    removable: boolean;
    blocks: string;
  };

  export let keys: SecurityKeyRow[] = [];
  export let available = true;
  export let unavailableReason = "";
  export let busy = false;
  /** Why "Add" is unavailable, empty when it is not. */
  export let addBlockedReason = "";
  export let onAdd: () => void = () => {};
  export let onRemove: (id: string, label: string) => void = () => {};
  export let onRename: (id: string, label: string) => void = () => {};
  export let onClose: () => void = () => {};
  /** Settles the caller's promise; the chosen action is recorded before it. */
  export let onDone: (value: unknown) => void = () => {};

  // The choice is recorded first, then the caller is settled, then the modal
  // closes - so the next dialog opens against a torn-down modal rather than
  // replacing a live one.
  function finish(chosen: () => void): void {
    chosen();
    onDone(null);
    dismissModal();
  }

  function blockReason(row: SecurityKeyRow): string {
    switch (row.blocks) {
      case "last": return $t("SteamGuard_Factors_BlockLast");
      case "lastInteractive": return $t("SteamGuard_Factors_BlockLastInteractive");
      case "backupNeeded": return $t("SteamGuard_Factors_BlockBackupNeeded");
      default: return "";
    }
  }
</script>

<div class="security-keys">
  <p class="security-keys__intro">{$t("SteamGuard_Factors_SecurityKeysIntro")}</p>

  {#if keys.length === 0}
    <p class="security-keys__empty">{$t("SteamGuard_Factors_NoSecurityKeys")}</p>
  {:else}
    <ul class="security-keys__list">
      {#each keys as row (row.id)}
        <li class="security-keys__row">
          <span class="security-keys__name">
            {row.label}
            {#if row.requiresPassword}
              <span class="security-keys__note">{$t("SteamGuard_Factors_AlsoNeedsPassword")}</span>
            {/if}
          </span>
          <button type="button" class="security-keys__link" disabled={busy}
            on:click={() => finish(() => onRename(row.id, row.label))}>
            {$t("SteamGuard_Factors_Rename")}
          </button>
          <button
            type="button"
            class="security-keys__link security-keys__link--remove"
            disabled={busy || !row.removable}
            title={row.removable ? "" : blockReason(row)}
            on:click={() => finish(() => onRemove(row.id, row.label))}
          >
            {$t("SteamGuard_Factors_Remove")}
          </button>
        </li>
      {/each}
    </ul>
  {/if}

  {#if !available}
    <p class="security-keys__hint">{unavailableReason || $t("SteamGuard_Factors_SecurityKeyUnavailable")}</p>
  {/if}

  <div class="security-keys__actions">
    <button
      type="button"
      class="btnicontext"
      disabled={busy || !available || addBlockedReason !== ""}
      title={addBlockedReason}
      on:click={() => finish(onAdd)}
    >
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path fill-rule="evenodd" d="M12 2A10 10 0 1 0 12 22A10 10 0 1 0 12 2ZM10 6L14 6L14 10L18 10L18 14L14 14L14 18L10 18L10 14L6 14L6 10L10 10Z"/></svg>
      {$t("SteamGuard_Factors_AddSecurityKey")}
    </button>
    <button type="button" class="btnicontext modal-positive" on:click={() => finish(onClose)}>
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M3.2 12.0 5.6 9.6l3.8 3.8L18.4 4.4l2.4 2.4L9.4 18.2z"/></svg>
      {$t("SteamGuard_Close")}
    </button>
  </div>
</div>

<style lang="scss">
  .security-keys {
    display: grid;
    gap: 0.6rem;
  }

  .security-keys__intro,
  .security-keys__empty,
  .security-keys__hint {
    margin: 0;
  }

  .security-keys__empty,
  .security-keys__hint {
    font-size: 0.9em;
    opacity: 0.8;
  }

  .security-keys__list {
    display: grid;
    gap: 0.3rem;
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .security-keys__row {
    display: flex;
    gap: 0.5rem;
    align-items: baseline;
    padding: 0.35rem 0.5rem;
    border-radius: 0.35rem;
    background: var(--modalListRowBg, rgb(255 255 255 / 5%));
  }

  .security-keys__name {
    flex: 1 1 auto;
    min-width: 0;
    overflow-wrap: anywhere;
  }

  .security-keys__note {
    margin-left: 0.4em;
    font-size: 0.8em;
    opacity: 0.75;
  }

  /* Same resets as every other link-styled button here: the global button rule
     sets display, width and a horizontal margin that this must not inherit. */
  .security-keys__link {
    display: inline;
    flex: 0 0 auto;
    width: auto;
    margin: 0;
    padding: 0;
    border: 0;
    background: none;
    color: inherit;
    cursor: pointer;
    font: inherit;
    font-size: 0.85em;
    text-decoration: underline;

    &:disabled {
      cursor: not-allowed;
      opacity: 0.45;
      text-decoration: none;
    }
  }

  .security-keys__link--remove:not(:disabled) {
    color: var(--role-danger, #e57373);
  }

  .security-keys__actions {
    display: flex;
    gap: 0.4rem;
    justify-content: flex-end;
    margin-top: 0.25rem;
  }
</style>

<script lang="ts">
  import { dismissModal } from "../../stores/modal";
  import { t } from "../../stores/i18n";

  type RestoreAccountRow = {
    steamId64: string;
    accountName: string;
    exists: boolean;
    backupTokenExpiry?: number;
    currentTokenExpiry?: number;
  };

  export let accounts: RestoreAccountRow[] = [];
  /** Receives the chosen SteamID64s, or null when the dialog is dismissed. */
  export let onDone: (selected: string[] | null) => void = () => {};

  // New accounts default in; replacing an existing one is the decision this
  // dialog exists to ask, so conflicts start unticked.
  let selected: Record<string, boolean> = Object.fromEntries(
    accounts.map((account) => [account.steamId64, !account.exists]),
  );

  $: count = accounts.filter((account) => selected[account.steamId64]).length;

  /**
   * Which copy carries the fresher Steam session, read from token expiries the
   * backend extracted offline. Silent when either side has no readable token.
   */
  function freshness(account: RestoreAccountRow): string {
    if (!account.exists || !account.backupTokenExpiry || !account.currentTokenExpiry) return "";
    if (account.backupTokenExpiry > account.currentTokenExpiry) return $t("SteamGuard_RestoreMerge_BackupNewer");
    if (account.backupTokenExpiry < account.currentTokenExpiry) return $t("SteamGuard_RestoreMerge_CurrentNewer");
    return "";
  }

  function toggle(steamId64: string, event: Event): void {
    selected[steamId64] = (event.currentTarget as HTMLInputElement).checked;
  }

  function finish(result: string[] | null): void {
    onDone(result);
    dismissModal();
  }
</script>

<div class="sg-restore">
  <p>{$t("SteamGuard_RestoreMerge_ChooseBody")}</p>
  <ul class="sg-restore__list">
    {#each accounts as account (account.steamId64)}
      <li class="sg-restore__row">
        <input
          type="checkbox"
          class="form-check-input"
          id={"sg-restore-" + account.steamId64}
          checked={selected[account.steamId64]}
          on:change={(event) => toggle(account.steamId64, event)}
        />
        <label class="form-check-label" for={"sg-restore-" + account.steamId64}></label>
        <label class="sg-restore__name" for={"sg-restore-" + account.steamId64}>
          <strong>{account.accountName || account.steamId64}</strong>
          <small>{account.steamId64}</small>
        </label>
        <span class="sg-restore__status" class:sg-restore__status--replace={account.exists}>
          {account.exists ? $t("SteamGuard_RestoreMerge_Replaces") : $t("SteamGuard_RestoreMerge_New")}
          {#if freshness(account)}<small>{freshness(account)}</small>{/if}
        </span>
      </li>
    {/each}
  </ul>
  <div class="sg-restore__actions">
    <button type="button" class="btnicontext" on:click={() => finish(null)}><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M4.6 2.9 12 10.3l7.4-7.4 1.7 1.7L13.7 12l7.4 7.4-1.7 1.7L12 13.7l-7.4 7.4-1.7-1.7L10.3 12 2.9 4.6z"/></svg>{$t("SteamGuard_Cancel")}</button>
    <button type="button" class="btnicontext modal-primary" disabled={count === 0} on:click={() => finish(accounts.filter((account) => selected[account.steamId64]).map((account) => account.steamId64))}>
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M3.2 12.0 5.6 9.6l3.8 3.8L18.4 4.4l2.4 2.4L9.4 18.2z"/></svg>
      {$t("SteamGuard_RestoreMerge_Restore", { count })}
    </button>
  </div>
</div>

<style lang="scss">
  .sg-restore {
    display: grid;
    gap: 0.75rem;
    text-align: left;
  }

  .sg-restore p {
    margin: 0;
  }

  .sg-restore__list {
    display: grid;
    gap: 0.5rem;
    max-height: min(40vh, 22rem);
    margin: 0;
    padding: 0.15rem;
    overflow-y: auto;
    list-style: none;
  }

  .sg-restore__row {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    border: 1px solid var(--modal-border, var(--border-bar-bg));
    border-radius: 0.25rem;
    padding: 0.5rem 0.65rem;
  }

  .sg-restore__name {
    display: grid;
    gap: 0.1rem;
    flex: 1 1 auto;
    min-width: 0;
    padding: 0;
    cursor: pointer;
  }

  .sg-restore__name strong,
  .sg-restore__name small {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .sg-restore__name small,
  .sg-restore__status small {
    color: var(--whiteSecondary, #d7d7d7);
    font-size: 0.8rem;
  }

  .sg-restore__status {
    display: grid;
    gap: 0.1rem;
    flex: none;
    text-align: right;
    color: var(--role-success, #4caf50);
    font-size: 0.85rem;
    font-weight: 600;
  }

  /* Replacing is the destructive half of the choice, so it reads as a caution. */
  .sg-restore__status--replace {
    color: var(--role-warning, #ffd166);
  }

  .sg-restore__actions {
    display: flex;
    justify-content: flex-end;
    gap: 0.5rem;
  }
</style>

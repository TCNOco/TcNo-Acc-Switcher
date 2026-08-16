<script lang="ts">
  /**
   * The "other ways in" block: backup key and keyfile.
   *
   * Shared by every screen that unlocks the vault. It used to exist only on the
   * account unlock screen, so the screens that ADD an account offered a password
   * box and nothing else - a vault protected by a keyfile or a backup key simply
   * could not be opened from them. Keeping one copy is what stops the two
   * drifting apart again on a path where the cost is being locked out.
   */
  import { t } from "../../stores/i18n";

  export let idPrefix: string;
  export let backupKey = "";
  export let keyfilePath = "";
  export let busy = false;
  /** Absent when the host cannot open a file picker; the section still shows the backup key. */
  export let pickKeyfile: (() => Promise<string>) | undefined = undefined;
  /** Enter inside a field submits the same action the primary button runs. */
  export let onSubmit: () => void = () => {};

  async function chooseKeyfile(): Promise<void> {
    if (!pickKeyfile || busy) return;
    try {
      const chosen = await pickKeyfile();
      if (chosen) keyfilePath = chosen;
    } catch (error) {
      console.error("Steam Guard: keyfile could not be chosen", error);
    }
  }

  function runOnEnter(event: KeyboardEvent): void {
    if (event.key !== "Enter" || event.isComposing) return;
    event.preventDefault();
    onSubmit();
  }
</script>

<!-- Collapsed by default: most vaults are password-only, and the extra ways in
     are what the user reaches for after a failure, not before. -->
<details class="steam-guard__other-factors">
  <summary>{$t("SteamGuard_Unlock_OtherWays")}</summary>
  <label class="steam-guard__field" for="{idPrefix}-backup-key">
    <span>{$t("SteamGuard_Factor_BackupKey")}</span>
    <input
      id="{idPrefix}-backup-key"
      class="modal-input"
      bind:value={backupKey}
      autocomplete="off"
      spellcheck="false"
      placeholder={$t("SteamGuard_Unlock_BackupKeyHint")}
      disabled={busy}
      on:keydown={runOnEnter}
    />
  </label>
  {#if pickKeyfile}
    <div class="steam-guard__keyfile-row">
      <button type="button" class="btnicontext" disabled={busy} on:click={() => void chooseKeyfile()}>
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path fill-rule="evenodd" d="M6.5 2h7.1L20 8.4v11.1A2.5 2.5 0 0 1 17.5 22h-11A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2Zm1 9.5h9v2h-9Zm0 3.6h9v2h-9Zm0 3.6h6v2h-6Z"/></svg>
        {$t("SteamGuard_Unlock_ChooseKeyfile")}
      </button>
      <span class="steam-guard__keyfile-name">
        {keyfilePath ? keyfilePath.split(/[\\/]/).pop() : $t("SteamGuard_Unlock_NoKeyfile")}
      </span>
      {#if keyfilePath}
        <button type="button" class="fancyLink steam-guard__link" disabled={busy} on:click={() => { keyfilePath = ""; }}>
          {$t("SteamGuard_Unlock_ClearKeyfile")}
        </button>
      {/if}
    </div>
  {/if}
</details>

<style lang="scss">
  /* Moved here with the markup: Svelte scopes styles to the component that
     declares them, so leaving these behind rendered the block unstyled. The
     field and link rules are restated for the same reason. */
  .steam-guard__other-factors summary {
    cursor: pointer;
    opacity: 0.85;
  }

  .steam-guard__field {
    display: grid;
    gap: 0.35rem;
    width: 100%;
    padding: 0;
    text-align: left;
  }

  /* Beats the global `input` rules so the input spans the form, not its content. */
  .steam-guard__field :global(.modal-input) {
    width: 100%;
    min-width: 0;
    margin: 0;
  }

  .steam-guard__keyfile-row {
    display: flex;
    gap: 0.5rem;
    align-items: center;
    margin-top: 0.5rem;
  }

  .steam-guard__keyfile-name {
    flex: 1 1 auto;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    opacity: 0.8;
  }

  /* A link with a touch target: fancyLink strips the button chrome the themes
     paint on, the size and ink are this row's own. */
  .steam-guard__link {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
    min-height: 2.75rem;
    padding: 0.5rem;
    border-radius: 0.25rem;
    color: var(--whiteSecondary, #d7d7d7);
    text-decoration: underline;
  }
</style>

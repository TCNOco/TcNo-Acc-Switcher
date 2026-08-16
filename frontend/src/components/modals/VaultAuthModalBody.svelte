<script lang="ts">
  import { t } from "../../stores/i18n";
  import { dismissModal } from "../../stores/modal";
  import { pushToast } from "../../stores/toast";
  import { formatToastWithError } from "../../lib/formatWailsError";

  type VaultAuth = { password: string; keyfilePath: string; backupKey: string };

  /** What the vault will accept, so unusable fields are not offered. */
  export let usesPassword = true;
  export let usesKeyfile = false;
  export let usesSecurityKey = false;
  export let intro = "";
  export let confirmLabel = "";
  /** Shown above the fields after a rejected attempt, so the retry is in place. */
  export let error = "";
  /** Set when the action cannot proceed without the keyfile, not merely open the
   *  vault without it - changing the password has to re-key the keyfile too. */
  export let requireKeyfile = false;
  export let requireKeyfileReason = "";
  /** Injected so this component stays free of service imports and testable. */
  export let pickKeyfile: (() => Promise<string>) | null = null;
  /** Receives the collected factors, or null when the dialog is dismissed. */
  export let onDone: (result: VaultAuth | null) => void = () => {};

  let password = "";
  let keyfilePath = "";
  let backupKey = "";
  let showOtherWays = false;
  let picking = false;

  $: canSubmit = requireKeyfile
    ? keyfilePath !== "" && (!usesPassword || password.length > 0)
    : password.length > 0 || keyfilePath !== "" || backupKey.trim() !== "";

  // Changing the password has to re-key the keyfile that shares it, so a
  // security key cannot stand in for that one: it would authenticate and then
  // fail at the point of no return.
  $: securityKeyBlocked = !usesSecurityKey || requireKeyfile;

  // A required keyfile is the point of the dialog, not an afterthought behind a
  // disclosure, so the other ways open by themselves.
  $: if (requireKeyfile) showOtherWays = true;

  function keyfileName(path: string): string {
    const parts = path.split(/[\\/]/);
    return parts[parts.length - 1] || path;
  }

  async function chooseKeyfile(): Promise<void> {
    if (!pickKeyfile || picking) return;
    picking = true;
    try {
      const picked = await pickKeyfile();
      if (picked) keyfilePath = picked;
    } catch (pickError) {
      pushToast({
        type: "error",
        message: formatToastWithError($t("SteamGuard_Factors_KeyfileTitle"), pickError),
        duration: 6000,
      });
    } finally {
      picking = false;
    }
  }

  function onEnter(event: KeyboardEvent): void {
    // isComposing: committing an IME composition sends Enter, and submitting
    // half-composed text here spends an authentication attempt on it.
    if (event.key !== "Enter" || event.isComposing) return;
    event.preventDefault();
    submit();
  }

  function finish(result: VaultAuth | null): void {
    onDone(result);
    dismissModal();
  }

  function submit(): void {
    if (!canSubmit) return;
    finish({ password, keyfilePath, backupKey: backupKey.trim() });
  }

  /** Continue by security key, which needs nothing typed in. */
  function submitWithSecurityKey(): void {
    if (securityKeyBlocked) return;
    finish({ password, keyfilePath, backupKey: backupKey.trim() });
  }
</script>

<!-- Not a <form>: the app-wide navigation guard cancels form submission, so the
     button is a plain button and Enter is handled on each field. -->
<div class="vault-auth">
  {#if error}<p class="modal-warning vault-auth__error">{error}</p>{/if}
  {#if intro}<p class="vault-auth__intro">{intro}</p>{/if}
  {#if requireKeyfile && requireKeyfileReason}
    <p class="vault-auth__hint">{requireKeyfileReason}</p>
  {/if}

  {#if usesPassword}
    <label class="vault-auth__field">
      <span>{$t("SteamGuard_Factor_Password")}</span>
      <!-- svelte-ignore a11y-autofocus -->
      <input
        class="modal-input"
        type="password"
        autocomplete="off"
        autofocus
        bind:value={password}
        on:keydown={onEnter}
      />
    </label>
  {/if}

  {#if usesKeyfile || usesSecurityKey || !usesPassword}
    <button
      type="button"
      class="vault-auth__disclosure"
      aria-expanded={showOtherWays}
      on:click={() => { showOtherWays = !showOtherWays; }}
    >
      {showOtherWays ? "▾" : "▸"}
      {$t("SteamGuard_Unlock_OtherWays")}
    </button>
  {/if}

  {#if showOtherWays || !usesPassword}
    <div class="vault-auth__other">
      <div class="vault-auth__field">
        <span>{$t("SteamGuard_Factor_Keyfile")}</span>
        <div class="vault-auth__row">
          <span class="vault-auth__path" title={keyfilePath}>
            {keyfilePath ? keyfileName(keyfilePath) : $t("SteamGuard_Unlock_NoKeyfile")}
          </span>
          <button type="button" class="btnicontext" disabled={picking} on:click={() => void chooseKeyfile()}>
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path fill-rule="evenodd" d="M6.5 2h7.1L20 8.4v11.1A2.5 2.5 0 0 1 17.5 22h-11A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2Zm1 9.5h9v2h-9Zm0 3.6h9v2h-9Zm0 3.6h6v2h-6Z"/></svg>
            {$t("SteamGuard_Unlock_ChooseKeyfile")}
          </button>
          {#if keyfilePath}
            <button type="button" class="btnicontext" on:click={() => { keyfilePath = ""; }}>
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M4.6 2.9 12 10.3l7.4-7.4 1.7 1.7L13.7 12l7.4 7.4-1.7 1.7L12 13.7l-7.4 7.4-1.7-1.7L10.3 12 2.9 4.6z"/></svg>
              {$t("Button_Cancel")}
            </button>
          {/if}
        </div>
      </div>

      <label class="vault-auth__field">
        <span>{$t("SteamGuard_Factor_BackupKey")}</span>
        <input
          class="modal-input vault-auth__code"
          type="text"
          spellcheck="false"
          autocomplete="off"
          placeholder={$t("SteamGuard_Unlock_BackupKeyHint")}
          bind:value={backupKey}
          on:keydown={onEnter}
        />
      </label>

    </div>
  {/if}

  <div class="vault-auth__actions">
    <button type="button" class="btnicontext" on:click={() => finish(null)}>
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M4.6 2.9 12 10.3l7.4-7.4 1.7 1.7L13.7 12l7.4 7.4-1.7 1.7L12 13.7l-7.4 7.4-1.7-1.7L10.3 12 2.9 4.6z"/></svg>
      {$t("Button_Cancel")}
    </button>
    <!-- Always shown, so the option is not conditional on the user noticing
         some text. Disabled once the vault has said it has no key to ask for. -->
    <button
      type="button"
      class="btnicontext"
      disabled={securityKeyBlocked}
      title={securityKeyBlocked && !usesSecurityKey ? $t("SteamGuard_Unlock_NoSecurityKey") : ""}
      on:click={submitWithSecurityKey}
    >
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M12 1.6 21.5 5.2V11.4C21.5 16.6 17.6 20.6 12 22.4 6.4 20.6 2.5 16.6 2.5 11.4V5.2Z"/></svg>
      {$t("SteamGuard_Unlock_SecurityKey")}
    </button>
    <button type="button" class="btnicontext modal-positive" disabled={!canSubmit} on:click={submit}>
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M3.2 12.0 5.6 9.6l3.8 3.8L18.4 4.4l2.4 2.4L9.4 18.2z"/></svg>
      {confirmLabel || $t("Ok")}
    </button>
  </div>
</div>

<style lang="scss">
  .vault-auth {
    display: grid;
    gap: 0.6rem;
  }

  .vault-auth__intro,
  .vault-auth__error {
    margin: 0;
  }

  .vault-auth__field {
    display: grid;
    gap: 0.25rem;

    span {
      font-size: 0.85em;
      opacity: 0.85;
    }
  }

  .vault-auth__row {
    display: flex;
    gap: 0.4rem;
    align-items: center;
  }

  .vault-auth__path {
    flex: 1 1 auto;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    opacity: 0.85;
  }

  .vault-auth__code {
    font-family: ui-monospace, "Cascadia Mono", Consolas, monospace;
  }

  .vault-auth__disclosure {
    justify-self: start;
    padding: 0;
    border: 0;
    background: none;
    color: inherit;
    cursor: pointer;
    font: inherit;
    opacity: 0.85;
  }

  .vault-auth__other {
    display: grid;
    gap: 0.6rem;
    padding-left: 0.6rem;
    border-left: 2px solid var(--borderColor, rgb(255 255 255 / 15%));
  }

  .vault-auth__hint {
    margin: 0;
    font-size: 0.85em;
    opacity: 0.8;
  }

  .vault-auth__actions {
    display: flex;
    gap: 0.4rem;
    justify-content: flex-end;
    margin-top: 0.25rem;
  }
</style>

<script lang="ts">
  import { t } from "../../stores/i18n";
  import { dismissModal } from "../../stores/modal";
  import { passwordPolicyMessage, validateNewPassword } from "../../lib/passwordPolicy";

  type NewFactorOptions = { name: string; password: string };

  /** Shown only for factors the user can have several of, so they can be told apart. */
  export let askName = false;
  export let nameLabel = "";
  export let namePlaceholder = "";
  export let intro = "";
  export let passwordLabel = "";
  export let passwordHint = "";
  export let confirmLabel = "";
  /** Receives the chosen options, or null when the dialog is dismissed. */
  export let onDone: (result: NewFactorOptions | null) => void = () => {};

  let name = "";
  let password = "";
  let confirmation = "";
  let error = "";

  // Blank is the normal answer: it means this factor opens the vault on its own.
  $: wantsPassword = password.length > 0 || confirmation.length > 0;

  function onEnter(event: KeyboardEvent): void {
    // isComposing: committing an IME composition sends Enter, which must not
    // submit a half-composed name or password.
    if (event.key !== "Enter" || event.isComposing) return;
    event.preventDefault();
    submit();
  }

  function submit(): void {
    error = "";
    if (wantsPassword) {
      const policyError = validateNewPassword(password);
      if (policyError) {
        error = passwordPolicyMessage(policyError, $t);
        return;
      }
      if (password !== confirmation) {
        error = $t("SteamGuard_Settings_PasswordMismatch");
        return;
      }
    }
    finish({ name: name.trim(), password });
  }

  function finish(result: NewFactorOptions | null): void {
    onDone(result);
    dismissModal();
  }
</script>

<!-- Not a <form>: the app-wide navigation guard cancels form submission. -->
<div class="new-factor">
  {#if intro}<p class="new-factor__intro">{intro}</p>{/if}

  {#if askName}
    <label class="new-factor__field">
      <span>{nameLabel || $t("SteamGuard_Factors_NameLabel")}</span>
      <!-- svelte-ignore a11y-autofocus -->
      <input class="modal-input" type="text" autocomplete="off" autofocus
        placeholder={namePlaceholder} bind:value={name} on:keydown={onEnter} />
    </label>
  {/if}

  <label class="new-factor__field">
    <span>{passwordLabel || $t("SteamGuard_Factors_OptionalPassword")}</span>
    <input class="modal-input" type="password" autocomplete="off" bind:value={password} on:keydown={onEnter} />
  </label>

  {#if wantsPassword}
    <label class="new-factor__field">
      <span>{$t("SteamGuard_Settings_ConfirmPasswordTitle")}</span>
      <input class="modal-input" type="password" autocomplete="off" bind:value={confirmation} on:keydown={onEnter} />
    </label>
  {/if}

  <p class="new-factor__hint">
    {passwordHint || $t("SteamGuard_Factors_OptionalPasswordHint")}
  </p>

  {#if error}<p class="modal-warning new-factor__error">{error}</p>{/if}

  <div class="new-factor__actions">
    <button type="button" class="btnicontext" on:click={() => finish(null)}>
      {$t("Button_Cancel")}
    </button>
    <button type="button" class="btnicontext modal-positive" on:click={submit}>
      {confirmLabel || $t("Ok")}
    </button>
  </div>
</div>

<style lang="scss">
  .new-factor {
    display: grid;
    gap: 0.6rem;
  }

  .new-factor__intro {
    margin: 0;
  }

  .new-factor__field {
    display: grid;
    gap: 0.25rem;

    span {
      font-size: 0.85em;
      opacity: 0.85;
    }
  }

  .new-factor__hint {
    margin: 0;
    font-size: 0.85em;
    opacity: 0.8;
  }

  .new-factor__error {
    margin: 0;
  }

  .new-factor__actions {
    display: flex;
    gap: 0.4rem;
    justify-content: flex-end;
    margin-top: 0.25rem;
  }
</style>

<script lang="ts">
  import { tick, onMount } from "svelte";
  import { createEventDispatcher } from "svelte";
  import { t } from "../../stores/i18n";
  import type { PasswordSetupResult } from "../../stores/modal";
  import {
    MIN_PASSWORD_LENGTH,
    passwordPolicyMessage,
    validateNewPassword,
  } from "../../lib/passwordPolicy";
  import { escapeHtml } from "../../lib/html";
  import { openExternalUrl } from "../../lib/openExternalUrl";
  import ModalBodyShell from "./ModalBodyShell.svelte";

  // A real handler, not an anchor inside setupBodyHtml: sanitizeHtml stamps
  // target="_blank" on anchors and the navigation guard then refuses them, so
  // an injected link would be silently dead.
  const PASSWORD_HELP_URL =
    "https://github.com/TCNOco/TcNo-Acc-Switcher/blob/master/docs/steam-guard-passwords-and-factors.md";

  function openPasswordHelp(): void {
    void openExternalUrl(PASSWORD_HELP_URL);
  }

  export let positiveLabel = "";
  export let negativeLabel = "";

  const dispatch = createEventDispatcher<{ resolve: PasswordSetupResult | null }>();

  let password = "";
  let confirm = "";
  let error = "";
  let passwordEl: HTMLInputElement | undefined;
  let confirmEl: HTMLInputElement | undefined;
  let setupBodyHtml = "";
  const errorId = "password-setup-error";

  $: setupBodyHtml = `<p>${escapeHtml($t("Security_SetPasswordIntro"))}</p><p>${escapeHtml($t("Security_SetPasswordEncryptionHint"))}</p>`;

  function cancel(): void {
    dispatch("resolve", null);
  }

  function submit(): void {
    error = "";
    const policyError = validateNewPassword(password);
    if (policyError) {
      error = passwordPolicyMessage(policyError, $t);
      requestAnimationFrame(() => {
        passwordEl?.focus();
      });
      return;
    }
    if (password !== confirm) {
      error = $t("Security_SetPasswordMismatch");
      requestAnimationFrame(() => {
        confirmEl?.focus();
      });
      return;
    }
    dispatch("resolve", { password });
  }

  onMount(() => {
    void tick().then(() =>
      requestAnimationFrame(() => {
        passwordEl?.focus();
      }),
    );
  });
</script>

<div class="modal-block">
  <ModalBodyShell html={setupBodyHtml} />

  <label class="modal-field">
    <span>{$t("Security_Password")}</span>
    <input
      bind:this={passwordEl}
      bind:value={password}
      type="password"
      class="modal-input"
      autocomplete="new-password"
      aria-invalid={error ? "true" : undefined}
      aria-describedby={error ? errorId : undefined}
      on:keydown={(e) => e.key === "Enter" && submit()}
    />
  </label>

  <label class="modal-field">
    <span>{$t("Security_ConfirmPassword")}</span>
    <input
      bind:this={confirmEl}
      bind:value={confirm}
      type="password"
      class="modal-input"
      autocomplete="new-password"
      aria-invalid={error ? "true" : undefined}
      aria-describedby={error ? errorId : undefined}
      on:keydown={(e) => e.key === "Enter" && submit()}
    />
  </label>

  {#if error}
    <p id={errorId} class="modal-error" role="alert">{error}</p>
  {/if}

  <p class="modal-help">
    {$t("Security_PasswordHint", { count: MIN_PASSWORD_LENGTH })}
    <button type="button" class="fancyLink modal-link" on:click={openPasswordHelp}>
      {$t("Security_PasswordLearnMore")}
    </button>
  </p>

  <div class="modal-inline-actions settingsCol inputAndButton">
    <span class="modal-actions-spacer"></span>
    <button type="button" class="btnicontext modal-primary" on:click={submit}>
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M3.2 12.0 5.6 9.6l3.8 3.8L18.4 4.4l2.4 2.4L9.4 18.2z"/></svg>
      {positiveLabel}
    </button>
    <button type="button" class="btnicontext" on:click={cancel}>
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M4.6 2.9 12 10.3l7.4-7.4 1.7 1.7L13.7 12l7.4 7.4-1.7 1.7L12 13.7l-7.4 7.4-1.7-1.7L10.3 12 2.9 4.6z"/></svg>
      {negativeLabel}
    </button>
  </div>
</div>

<style lang="scss">
  .modal-field {
    display: grid;
    gap: 0.35rem;
    margin-top: 0.75rem;
  }

  .modal-error {
    color: var(--danger, #ff6b6b);
    margin: 0.75rem 0 0;
  }

  .modal-help {
    margin: 0.75rem 0 0;
    opacity: 0.8;
    font-size: 0.9em;
  }

  .modal-link {
    text-decoration: underline;
  }
</style>

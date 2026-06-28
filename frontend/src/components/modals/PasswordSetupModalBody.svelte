<script lang="ts">
  import { tick, onMount } from "svelte";
  import { createEventDispatcher } from "svelte";
  import { t } from "../../stores/i18n";
  import type { PasswordSetupResult } from "../../stores/modal";
  import ModalBodyShell from "./ModalBodyShell.svelte";

  export let positiveLabel = "";
  export let negativeLabel = "";

  const dispatch = createEventDispatcher<{ resolve: PasswordSetupResult | null }>();

  let password = "";
  let confirm = "";
  let error = "";
  let passwordEl: HTMLInputElement | undefined;
  let setupBodyHtml = "";

  function escapeHtml(value: string): string {
    return value
      .replaceAll("&", "&amp;")
      .replaceAll("<", "&lt;")
      .replaceAll(">", "&gt;")
      .replaceAll("\"", "&quot;")
      .replaceAll("'", "&#39;");
  }

  $: setupBodyHtml = `<p>${escapeHtml($t("Security_SetPasswordIntro"))}</p><p>${escapeHtml($t("Security_SetPasswordEncryptionHint"))}</p>`;

  function cancel(): void {
    dispatch("resolve", null);
  }

  function submit(): void {
    error = "";
    if (password !== confirm) {
      error = $t("Security_SetPasswordMismatch");
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
      on:keydown={(e) => e.key === "Enter" && submit()}
    />
  </label>

  <label class="modal-field">
    <span>{$t("Security_ConfirmPassword")}</span>
    <input
      bind:value={confirm}
      type="password"
      class="modal-input"
      autocomplete="new-password"
      on:keydown={(e) => e.key === "Enter" && submit()}
    />
  </label>

  {#if error}
    <p class="modal-error">{error}</p>
  {/if}

  <div class="modal-inline-actions settingsCol inputAndButton">
    <span class="modal-actions-spacer"></span>
    <button type="button" class="btnicontext modal-primary" on:click={submit}>
      {positiveLabel}
    </button>
    <button type="button" class="btnicontext" on:click={cancel}>
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
</style>

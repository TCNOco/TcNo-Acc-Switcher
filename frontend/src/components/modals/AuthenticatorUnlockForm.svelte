<script lang="ts">
  import { t } from "../../stores/i18n";
  import { focusOnShow } from "../../lib/actions/focusOnShow";
  import SteamGuardVaultFactors from "./SteamGuardVaultFactors.svelte";

  export let idPrefix: string;
  export let password = "";
  export let keyfilePath = "";
  export let backupKey = "";
  export let rememberForSession = false;
  export let busy = false;
  export let available = true;
  export let error = "";
  export let hasSecurityKey: boolean | undefined = undefined;
  export let creating = false;
  export let pickKeyfile: (() => Promise<string>) | undefined = undefined;
  export let onUnlock: () => void;
  export let onSecurityKey: () => void = () => {};

  $: disabled = busy || !available;
  $: canUnlock = !disabled && (password.length > 0 || (!creating && (keyfilePath !== "" || backupKey.trim() !== "")));

  function submit(): void {
    if (canUnlock) onUnlock();
  }

  function runOnEnter(event: KeyboardEvent): void {
    if (event.key !== "Enter" || event.isComposing) return;
    event.preventDefault();
    submit();
  }
</script>

<form class="steam-guard__stack steam-guard__unlock-form" on:submit|preventDefault={submit}>
  <slot name="identity" />
  <p>{$t(creating ? "Authenticator_Create" : "Authenticator_Unlock_Body")}</p>
  <label class="steam-guard__field" for="{idPrefix}-password">
    <span>{$t("Authenticator_Password")}</span>
    <input
      id="{idPrefix}-password"
      class="modal-input"
      bind:value={password}
      type="password"
      autocomplete={creating ? "new-password" : "current-password"}
      disabled={disabled}
      data-steamguard-focus
      use:focusOnShow={!disabled}
      aria-invalid={error ? "true" : undefined}
      aria-describedby={error ? `${idPrefix}-unlock-error` : undefined}
      on:keydown={runOnEnter}
    />
  </label>
  <slot />
  {#if !creating}
    <SteamGuardVaultFactors
      {idPrefix}
      bind:backupKey
      bind:keyfilePath
      busy={disabled}
      {pickKeyfile}
      onSubmit={submit}
    />
  {/if}
  {#if error}
    <p id="{idPrefix}-unlock-error" class="steam-guard__error" role="alert">{error}</p>
  {/if}
  <div class="steam-guard__actions steam-guard__actions--split">
    {#if !creating}
      <div class="steam-guard__check">
        <span class="form-check">
          <input id="{idPrefix}-remember-session" class="form-check-input" bind:checked={rememberForSession} type="checkbox" disabled={disabled} />
          <label class="form-check-label" for="{idPrefix}-remember-session" aria-hidden="true"></label>
        </span>
        <label for="{idPrefix}-remember-session">{$t("SteamGuard_RememberMe")}</label>
      </div>
    {/if}
    <div class="steam-guard__actions-group">
      {#if !creating}
        <button
          class="btnicontext"
          type="button"
          disabled={disabled || hasSecurityKey === false}
          title={hasSecurityKey === false ? $t("SteamGuard_Unlock_NoSecurityKey") : ""}
          on:click={onSecurityKey}
        >
          {$t("SteamGuard_Unlock_SecurityKey")}
        </button>
      {/if}
      <button class="btnicontext modal-primary" type="button" disabled={!canUnlock} on:click={submit}>
        <svg class="steam-guard__icon" viewBox="0 0 512 512" aria-hidden="true"><path d="M512 176.001C512 273.203 433.202 352 336 352c-11.22 0-22.19-1.062-32.827-3.069l-24.012 27.014A23.999 23.999 0 0 1 261.223 384H224v40c0 13.255-10.745 24-24 24h-40v40c0 13.255-10.745 24-24 24H24c-13.255 0-24-10.745-24-24v-78.059c0-6.365 2.529-12.47 7.029-16.971l161.802-161.802C163.108 213.814 160 195.271 160 176 160 78.798 238.797.001 335.999 0 433.488-.001 512 78.511 512 176.001zM336 128c0 26.51 21.49 48 48 48s48-21.49 48-48-21.49-48-48-48-48 21.49-48 48z" /></svg>
        {$t(busy ? "SteamGuard_Unlocking" : creating ? "Authenticator_Create" : "SteamGuard_Unlock")}
      </button>
    </div>
  </div>
</form>

<style>
  .steam-guard__stack { display: grid; gap: 0.5rem; min-width: 0; }
  .steam-guard__unlock-form { justify-items: stretch; text-align: left; }
  p { margin: 0; line-height: 1.45; }
  .steam-guard__field { display: grid; gap: 0.35rem; width: 100%; padding: 0; text-align: left; }
  .steam-guard__field :global(.modal-input) { width: 100%; min-width: 0; margin: 0; }
  .steam-guard__check { display: flex; align-items: center; gap: 0.55rem; width: fit-content; justify-self: start; text-align: left; }
  .steam-guard__check > :global(.form-check) { display: inline-flex; align-items: center; }
  .steam-guard__check > :global(label) { padding: 0; }
  .steam-guard__actions { display: flex; align-items: center; flex-wrap: wrap; gap: 0.5rem; }
  .steam-guard__actions :global(button) { display: inline-flex; align-items: center; justify-content: center; gap: 0.5rem; width: auto; min-height: 2.75rem; margin: 0; }
  .steam-guard__actions--split { flex-wrap: nowrap; justify-content: space-between; gap: 1rem; }
  .steam-guard__actions-group { display: flex; align-items: center; gap: 0.5rem; }
  .steam-guard__icon { flex: none; width: 0.95em; height: 0.95em; fill: currentColor; }
  .steam-guard__error { margin: 0; color: var(--role-error, #ff6b6b); }
  @media (max-width: 640px) { .steam-guard__actions--split { flex-wrap: wrap; } }
</style>

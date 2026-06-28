<script lang="ts">
  import { t } from "../stores/i18n";
  import {
    securityStatus,
    securityStatusLoaded,
    unlockApp,
  } from "../stores/security";

  let password = "";
  let loading = false;
  let error = "";

  async function submit(): Promise<void> {
    if (loading) return;
    error = "";
    loading = true;
    try {
      await unlockApp(password);
      password = "";
    } catch (e) {
      error = $t("Security_UnlockFailed");
    } finally {
      loading = false;
    }
  }

</script>

{#if $securityStatusLoaded && $securityStatus.appLocked}
  <div class="app-lock-overlay">
    <form class="app-lock-panel" on:submit|preventDefault={submit}>
      <h2>{$t("Security_Locked_Title")}</h2>
      <p>{$t("Security_Locked_Body")}</p>
      <input
        bind:value={password}
        type="password"
        autocomplete="current-password"
        class="modal-input"
        placeholder={$t("Security_Password")}
        disabled={loading}
      />
      {#if error}
        <p class="app-lock-error">{error}</p>
      {/if}
      <div class="app-lock-actions">
        <button type="submit" class="btnicontext modal-primary" disabled={loading}>
          {$t("Security_Unlock")}
        </button>
      </div>
    </form>
  </div>
{/if}

<style lang="scss">
  .app-lock-overlay {
    position: absolute;
    inset: 0;
    z-index: 40;
    display: grid;
    place-items: center;
    padding: 1rem;
    background: rgba(0, 0, 0, 0.68);
  }

  .app-lock-panel {
    width: min(420px, calc(100vw - 2rem));
    display: grid;
    gap: 0.85rem;
    padding: 1.25rem;
    border: 1px solid var(--input-number-border);
    background: var(--program-bg);
    box-shadow: 0 18px 50px rgba(0, 0, 0, 0.35);
  }

  .app-lock-panel h2,
  .app-lock-panel p {
    margin: 0;
  }

  .app-lock-panel h2 {
    font-size: 1.15rem;
  }

  .app-lock-panel p {
    color: var(--whiteSecondary);
  }

  .app-lock-error {
    color: var(--danger, #ff6b6b) !important;
  }

  .app-lock-actions {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 0.75rem;
  }
</style>

<script lang="ts">
  import { modalFocus } from "../lib/modalFocus";
  import { t } from "../stores/i18n";
  import {
    securityStatus,
    securityStatusLoaded,
    unlockApp,
  } from "../stores/security";

  let password = "";
  let loading = false;
  let error = "";
  let passwordEl: HTMLInputElement | undefined;
  const titleId = "app-lock-title";
  const passwordId = "app-lock-password";
  const errorId = "app-lock-error";

  async function submit(): Promise<void> {
    if (loading) return;
    error = "";
    loading = true;
    try {
      await unlockApp(password);
      password = "";
    } catch (e) {
      error = $t("Security_UnlockFailed");
      requestAnimationFrame(() => {
        passwordEl?.focus();
      });
    } finally {
      loading = false;
    }
  }

</script>

{#if $securityStatusLoaded && $securityStatus.appLocked}
  <div class="app-lock-overlay">
    <form
      class="app-lock-panel"
      use:modalFocus={{ initialFocus: () => passwordEl }}
      role="dialog"
      aria-modal="true"
      aria-labelledby={titleId}
      on:submit|preventDefault={submit}
    >
      <div class="app-lock-panel-inner">
        <header class="app-lock-headerbar">
          <span class="app-lock-title-left">
            <svg
              class="header_icon"
              xmlns="http://www.w3.org/2000/svg"
              viewBox="0 0 768 264"
              fill-rule="evenodd"
              stroke-linejoin="round"
              stroke-miterlimit="2"
              aria-hidden="true"
            >
              <use href="img/TcNo_Logo_Flat.svg#logo"></use>
            </svg>
          </span>
          <span id={titleId} class="app-lock-title-drag">
            {$t("Security_Locked_Title")}
          </span>
          <span class="app-lock-title-right" aria-hidden="true"></span>
        </header>

        <div class="app-lock-scroll">
          <div class="modal-block">
            <p class="app-lock-body">{$t("Security_Locked_Body")}</p>
            <label class="app-lock-field" for={passwordId}>
              <span>{$t("Security_Password")}</span>
              <input
                id={passwordId}
                bind:this={passwordEl}
                bind:value={password}
                type="password"
                autocomplete="current-password"
                class="modal-input"
                disabled={loading}
                aria-invalid={error ? "true" : undefined}
                aria-describedby={error ? errorId : undefined}
              />
            </label>
            {#if error}
              <p id={errorId} class="app-lock-error" role="alert">{error}</p>
            {/if}
            <div class="app-lock-actions modal-inline-actions settingsCol inputAndButton">
              <span class="modal-actions-spacer"></span>
              <button type="submit" class="btnicontext modal-primary" disabled={loading}>
                {$t("Security_Unlock")}
              </button>
            </div>
          </div>
        </div>
      </div>
    </form>
  </div>
{/if}

<style lang="scss">
  .app-lock-overlay {
    position: absolute;
    inset: 0;
    z-index: 50;
    display: grid;
    place-items: center;
    padding: 1rem;
    box-sizing: border-box;
    background: var(--modal-scrim, var(--backdrop-scrim-55));
  }

  .app-lock-panel {
    width: min(420px, calc(100vw - 2rem));
    max-height: min(75vh, calc(100vh - 2rem));
    display: flex;
    flex-direction: column;
    padding: 0;
    border: var(--border-bar-size, 1px) solid var(--border-bar-bg);
    background: var(--modal-bg, var(--mainContentBackground, var(--program-bg)));
    box-shadow: 0 12px 40px var(--shadow-color-45);
    overflow: hidden;
  }

  .app-lock-panel-inner {
    display: flex;
    flex-direction: column;
    min-height: 0;
  }

  .app-lock-headerbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    height: 32px;
    min-height: 32px;
    background: var(--modal-header-bg, var(--border-bar-bg));
    color: var(--modal-header-fg, var(--whiteSecondary));
    user-select: none;
  }

  .app-lock-title-left,
  .app-lock-title-right {
    display: flex;
    align-items: center;
    height: 100%;
    width: 46px;
  }

  .app-lock-headerbar .header_icon {
    height: 10px;
    margin: 0 12px;
    display: block;
    fill: var(--modal-header-fg, var(--whiteSecondary));
  }

  .app-lock-title-drag {
    flex: 1;
    font-family: "Segoe UI", sans-serif;
    font-size: 12px;
    font-weight: 500;
    text-align: center;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    padding: 0 8px;
  }

  .app-lock-scroll {
    min-height: 0;
    overflow: auto;
    padding: 1.25rem 1.5rem;
  }

  .app-lock-body {
    margin: 0 0 0.85rem;
    color: var(--white, #fff);
    line-height: 1.45;
  }

  .app-lock-field {
    display: grid;
    gap: 0.35rem;
    margin-bottom: 0.75rem;
  }

  .app-lock-error {
    color: var(--danger, #ff6b6b) !important;
    margin: 0;
  }

  .app-lock-actions {
    margin-top: 0.35rem;
  }
</style>

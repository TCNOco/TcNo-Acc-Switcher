<script lang="ts">
  import { onMount } from "svelte";
  import { t } from "../../stores/i18n";
  import { pushToast } from "../../stores/toast";
  import { formatToastWithError } from "../../lib/formatWailsError";
  import { passwordPolicyMessage, validateNewPassword } from "../../lib/passwordPolicy";
  import { openAlert, openPrompt } from "../../stores/modal";
  import { steamGuardSettings } from "../../stores/steamGuardSettings";

  const warningId = "steam-guard-backup-warning";
  const pathId = "steam-guard-folder-path";

  $: ready = $steamGuardSettings.availability === "ready";
  $: busy = $steamGuardSettings.operation !== null || $steamGuardSettings.availability === "loading";
  $: configured = ready && $steamGuardSettings.status.vaultConfigured;
  $: path = $steamGuardSettings.status.folderPath;

  function actionError(error: unknown): void {
    pushToast({
      type: "error",
      message: formatToastWithError($t("SteamGuard_Settings_ActionFailed"), error),
      duration: 8000,
    });
  }

  async function setRememberForSession(enabled: boolean): Promise<void> {
    try {
      await steamGuardSettings.setRememberPasswordForSession(enabled);
    } catch (error) {
      actionError(error);
    }
  }

  async function changePassword(): Promise<void> {
    const currentPassword = await openPrompt({
      title: $t("SteamGuard_Settings_ChangePassword"),
      body: $t("SteamGuard_Settings_CurrentPasswordBody"),
      inputType: "password",
      positiveLabel: $t("Ok"),
      negativeLabel: $t("Button_Cancel"),
    });
    if (currentPassword === null) return;

    const nextPassword = await openPrompt({
      title: $t("SteamGuard_Settings_NewPasswordTitle"),
      body: $t("SteamGuard_Settings_NewPasswordBody"),
      inputType: "password",
      positiveLabel: $t("Ok"),
      negativeLabel: $t("Button_Cancel"),
    });
    if (nextPassword === null) return;
    const policyError = validateNewPassword(nextPassword);
    if (policyError) {
      pushToast({ type: "error", message: passwordPolicyMessage(policyError), duration: 5000 });
      return;
    }

    const confirmation = await openPrompt({
      title: $t("SteamGuard_Settings_ConfirmPasswordTitle"),
      body: $t("SteamGuard_Settings_ConfirmPasswordBody"),
      inputType: "password",
      positiveLabel: $t("SteamGuard_Settings_ChangePassword"),
      negativeLabel: $t("Button_Cancel"),
    });
    if (confirmation === null) return;
    if (!nextPassword || nextPassword !== confirmation) {
      pushToast({ type: "error", message: $t("SteamGuard_Settings_PasswordMismatch"), duration: 5000 });
      return;
    }

    try {
      await steamGuardSettings.changePassword(currentPassword, nextPassword);
      pushToast({ type: "success", message: $t("SteamGuard_Settings_PasswordChanged"), duration: 4000 });
      const folder = $steamGuardSettings.status.folderPath;
      await openAlert({
        title: "Replace your Steam Guard backup",
        body: "Your old backup uses the previous password. Create a new verified backup of the full Steam Guard folder:<br><code>" +
          escapeHtml(folder || "Path unavailable") + "</code><br><br>Forgotten passwords cannot be recovered.",
      });
    } catch (error) {
      actionError(error);
    }
  }

  function escapeHtml(value: string): string {
    return value.replace(/[&<>"']/g, (character) => ({
      "&": "&amp;",
      "<": "&lt;",
      ">": "&gt;",
      '"': "&quot;",
      "'": "&#39;",
    })[character] ?? character);
  }

  async function lockNow(): Promise<void> {
    try {
      await steamGuardSettings.lockNow();
      pushToast({ type: "success", message: $t("SteamGuard_Settings_Locked"), duration: 4000 });
    } catch (error) {
      actionError(error);
    }
  }

  async function copyPath(): Promise<void> {
    if (!path || !navigator.clipboard?.writeText) return;
    try {
      await navigator.clipboard.writeText(path);
      pushToast({ type: "success", message: $t("SteamGuard_Settings_PathCopied"), duration: 3000 });
    } catch (error) {
      actionError(error);
    }
  }

  async function openFolder(): Promise<void> {
    try {
      await steamGuardSettings.openFolder();
    } catch (error) {
      actionError(error);
    }
  }

  async function createBackup(): Promise<void> {
    try {
      await steamGuardSettings.createVerifiedBackup();
      pushToast({ type: "success", message: $t("SteamGuard_Settings_BackupCreated"), duration: 5000 });
    } catch (error) {
      actionError(error);
    }
  }

  async function restoreFromBackup(): Promise<void> {
    try {
      await steamGuardSettings.restoreFromBackup();
    } catch (error) {
      actionError(error);
    }
  }

  function formatVerifiedAt(value: string): string {
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
  }

  onMount(() => {
    void steamGuardSettings.refresh().catch(() => undefined);
  });
</script>

<section class="steam-guard-settings" aria-labelledby="steam-guard-settings-title">
  <div class="steam-guard-heading-row">
    <!-- h2 to match the sibling sections of the Steam settings page it lives on. -->
    <h2 id="steam-guard-settings-title" class="SettingsHeader">{$t("SteamGuard_Settings_Title")}</h2>
    {#if $steamGuardSettings.availability === "error"}
      <span class="steam-guard-service-state" role="status">{$t("SteamGuard_Settings_StatusError")}</span>
    {:else if !ready}
      <span class="steam-guard-service-state" role="status">{$t("SteamGuard_Settings_StatusUnavailable")}</span>
    {/if}
  </div>

  <p id={warningId} class="steam-guard-warning">
    {$t("SteamGuard_Settings_BackupWarning")}
  </p>

  <div class="steam-guard-control-row">
    <span class="form-check">
      <input
        id="steam-guard-remember-session"
        type="checkbox"
        checked={$steamGuardSettings.status.rememberPasswordForSession}
        disabled={!configured || busy}
        on:change={(event) => void setRememberForSession(event.currentTarget.checked)}
      />
      <label class="form-check-label" for="steam-guard-remember-session"></label>
    </span>
    <label for="steam-guard-remember-session">{$t("SteamGuard_Settings_RememberSession")}</label>
    <span class="steam-guard-actions">
      <button type="button" class="btnicontext" disabled={!configured || busy} on:click={() => void changePassword()}>
        {$t("SteamGuard_Settings_ChangePassword")}
      </button>
      <!-- Locking is only meaningful once the vault is unlocked; showing it otherwise
           offers an action that would do nothing. -->
      {#if configured && $steamGuardSettings.status.unlocked}
        <button type="button" class="btnicontext" disabled={busy} on:click={() => void lockNow()}>
          {$t("SteamGuard_Settings_LockNow")}
        </button>
      {/if}
    </span>
  </div>

  <div class="steam-guard-path-block">
    <span class="steam-guard-label">{$t("SteamGuard_Settings_Folder")}</span>
    <code id={pathId} dir="ltr" title={path || $t("SteamGuard_Settings_PathUnavailable")}>
      {path || $t("SteamGuard_Settings_PathUnavailable")}
    </code>
    <span class="steam-guard-actions">
      <button type="button" class="btnicontext" disabled={!ready || !path || busy} aria-describedby={pathId} on:click={() => void copyPath()}>
        {$t("SteamGuard_Settings_CopyPath")}
      </button>
      <button type="button" class="btnicontext" disabled={!ready || !path || busy} aria-describedby={pathId} on:click={() => void openFolder()}>
        {$t("SteamGuard_Settings_OpenFolder")}
      </button>
    </span>
  </div>

  <p class="steam-guard-copy-warning">{$t("SteamGuard_Settings_ManualCopyWarning")}</p>

  <div class="steam-guard-backup-row">
    <!-- With no vault there is nothing to back up, so the action is hidden rather
         than offered as a disabled button. Restoring stays available: it is how a
         fresh install gets its accounts back. -->
    {#if configured}
      <button
        type="button"
        class="btnicontext"
        disabled={busy}
        aria-describedby={warningId}
        on:click={() => void createBackup()}
      >
        {$t("SteamGuard_Settings_CreateBackup")}
      </button>
    {/if}
    <button
      type="button"
      class="btnicontext"
      disabled={!ready || busy}
      aria-describedby={warningId}
      on:click={() => void restoreFromBackup()}
    >
      {$t("SteamGuard_Settings_Restore")}
    </button>
    <span class="steam-guard-backup-status" role="status" aria-live="polite">
      {#if $steamGuardSettings.status.lastVerifiedBackup}
        {$t("SteamGuard_Settings_LastBackup", {
          time: formatVerifiedAt($steamGuardSettings.status.lastVerifiedBackup.verifiedAt),
          path: $steamGuardSettings.status.lastVerifiedBackup.path,
        })}
      {:else}
        {$t("SteamGuard_Settings_NoVerifiedBackup")}
      {/if}
    </span>
  </div>
</section>

<style lang="scss">
  .steam-guard-settings {
    display: grid;
    gap: 0.75rem;
    margin: 0.65rem 0 0.85rem;
    padding: 0.85rem;
    border: 1px solid var(--border-bar-bg);
    background: color-mix(in srgb, var(--mainContentBackground, var(--program-bg)) 92%, var(--accent) 8%);
  }

  .steam-guard-heading-row,
  .steam-guard-control-row,
  .steam-guard-backup-row,
  .steam-guard-actions {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    flex-wrap: wrap;
  }

  .steam-guard-heading-row {
    justify-content: space-between;
  }

  p {
    margin: 0;
  }

  .steam-guard-service-state,
  .steam-guard-copy-warning,
  .steam-guard-backup-status {
    color: var(--whiteSecondary);
    font-size: 0.85rem;
  }

  .steam-guard-warning {
    color: var(--role-warning, var(--white));
    font-weight: 600;
  }

  .steam-guard-control-row > .steam-guard-actions {
    margin-inline-start: auto;
  }

  .steam-guard-path-block {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 0.45rem 0.75rem;
    align-items: center;
  }

  .steam-guard-label {
    grid-column: 1 / -1;
    font-weight: 600;
  }

  code {
    min-width: 0;
    padding: 0.5rem 0.6rem;
    overflow-wrap: anywhere;
    user-select: text;
    background: var(--role-field-bg, rgb(0 0 0 / 20%));
    border: 1px solid var(--border-bar-bg);
  }

  .steam-guard-backup-status {
    min-width: 0;
    overflow-wrap: anywhere;
  }

  button {
    min-height: 38px;
  }

  @media (max-width: 620px) {
    .steam-guard-control-row > .steam-guard-actions {
      width: 100%;
      margin-inline-start: 0;
    }

    .steam-guard-path-block {
      grid-template-columns: minmax(0, 1fr);
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .steam-guard-settings,
    .steam-guard-settings * {
      scroll-behavior: auto;
      transition-duration: 0.01ms !important;
      animation-duration: 0.01ms !important;
    }
  }
</style>

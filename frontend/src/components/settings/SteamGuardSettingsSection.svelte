<script lang="ts">
  import { onMount } from "svelte";
  import { t } from "../../stores/i18n";
  import { copyText } from "../../lib/clipboard";
  import { pushToast } from "../../stores/toast";
  import { formatToastWithError, formatUnknownError } from "../../lib/formatWailsError";
  import { passwordPolicyMessage, validateNewPassword } from "../../lib/passwordPolicy";
  import { escapeHtml } from "../../lib/html";
  import { passwordIsUsed } from "../../lib/steamGuardFactors";
  import { modalResult } from "../../lib/modalResult";
  import { openAlert, openAlertNoButton, openConfirm, openPrompt } from "../../stores/modal";
  import BackupKeyModalBody from "../modals/BackupKeyModalBody.svelte";
  import VaultAuthModalBody from "../modals/VaultAuthModalBody.svelte";
  import NewFactorModalBody from "../modals/NewFactorModalBody.svelte";
  import SecurityKeysModalBody from "../modals/SecurityKeysModalBody.svelte";
  import {
    steamGuardSettings,
    type SteamGuardFactorStatus,
    type SteamGuardVaultFactor,
  } from "../../stores/steamGuardSettings";

  let factorStatus: SteamGuardFactorStatus | null = null;
  let securityKeySupport: { available: boolean; reason: string } | null = null;

  const warningId = "steam-guard-backup-warning";
  const pathId = "steam-guard-folder-path";

  $: ready = $steamGuardSettings.availability === "ready";
  $: busy = $steamGuardSettings.operation !== null || $steamGuardSettings.availability === "loading";
  $: configured = ready && $steamGuardSettings.status.vaultConfigured;
  $: path = $steamGuardSettings.status.folderPath;

  // Security keys live in their own screen, so the list here is everything else.
  $: listedFactors = (factorStatus?.factors ?? []).filter((factor) => factor.kind !== "securitykey");
  $: securityKeyRows = (factorStatus?.factors ?? [])
    .filter((factor) => factor.kind === "securitykey")
    .map((factor) => ({
      id: factor.id,
      label: factor.label,
      requiresPassword: factor.requiresPassword,
      removable: factor.removable,
      blocks: factor.blocks,
    }));

  type WayInEntry = {
    key: string;
    name: string;
    note: string;
    action: { label: string; disabled: boolean; title: string; run: () => void } | null;
  };

  // Every way in as one list, so the markup is a single loop rendering a
  // sentence rather than a stack of rows.
  $: waysIn = [
    ...listedFactors.map((factor): WayInEntry => ({
      key: factor.id,
      name: factorRowName(factor),
      // A migrated way in can need the password as well. It is named after the
      // keyfile, so this is the only thing saying so.
      note: factor.requiresPassword ? $t("SteamGuard_Factors_AlsoNeedsPassword") : "",
      // The backup key is rotated rather than removed: "Replace backup key"
      // issues a new one and retires the old, which is the only thing anyone
      // sensibly wants to do to it.
      action: factor.kind === "recovery" ? null : {
        label: $t("SteamGuard_Factors_Remove"),
        disabled: !factor.removable,
        title: factor.removable ? "" : blockReason(factor),
        run: () => void removeFactor(factor),
      },
    })),
    // However many keys are enrolled, they are named and managed on their own
    // screen. Absent entirely when there are none: the button below enrols the
    // first one.
    ...((factorStatus?.securityKeyCount ?? 0) > 0
      ? [{
          key: "security-keys",
          name: $t("SteamGuard_Factor_SecurityKey"),
          note: $t("SteamGuard_Factors_SecurityKeyCount", { count: factorStatus?.securityKeyCount ?? 0 }),
          action: {
            label: $t("SteamGuard_Factors_Manage"),
            disabled: false,
            title: "",
            run: () => void openSecurityKeys(),
          },
        } satisfies WayInEntry]
      : []),
  ];

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
    // Rebuilding the password part of a way in needs every other factor that way
    // in lists, so this collects the whole set rather than just the password.
    const auth = await authenticateForManagement($t("SteamGuard_Settings_ChangePassword"), {
      intro: $t("SteamGuard_Settings_CurrentPasswordBody"),
      requireKeyfile: changeNeedsKeyfile,
      requireKeyfileReason: $t("SteamGuard_Factors_ChangeNeedsKeyfile"),
    });
    if (auth === null) return;

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
      pushToast({ type: "error", message: passwordPolicyMessage(policyError, $t), duration: 5000 });
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
      await steamGuardSettings.changePassword(
        auth.password, nextPassword, auth.keyfilePath, auth.backupKey,
      );
      pushToast({ type: "success", message: $t("SteamGuard_Settings_PasswordChanged"), duration: 4000 });
      await loadFactors();
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

  async function lockNow(): Promise<void> {
    try {
      await steamGuardSettings.lockNow();
      pushToast({ type: "success", message: $t("SteamGuard_Settings_Locked"), duration: 4000 });
    } catch (error) {
      actionError(error);
    }
  }

  async function copyPath(): Promise<void> {
    if (!path) return;
    try {
      await copyText(path);
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
    // Restoring swaps in another vault's slots, so the buttons above - which are
    // now "add" or "remove" depending on what exists - would otherwise be stale.
    await loadFactors();
  }

  type VaultAuth = { password: string; keyfilePath: string; backupKey: string };

  // Authenticates a management action and leaves the vault open for it.
  //
  // One dialog offering every way in, the same as the Steam page's unlock
  // screen. Which factors are needed depends on the vault, and asking for a
  // password and then springing a file dialog on the user made a coherent set of
  // options look like a sequence of unrelated demands. A security key is not a
  // field here: the backend asks the device once nothing else fits.
  type AuthOptions = {
    intro?: string;
    /** The action cannot proceed without the keyfile, not merely open the vault. */
    requireKeyfile?: boolean;
    requireKeyfileReason?: string;
  };

  async function authenticateForManagement(
    title: string,
    options: AuthOptions = {},
  ): Promise<VaultAuth | null> {
    // Retried in place. A rejected password used to close the dialog and toast,
    // which on the password-change path meant choosing a whole new password
    // before being told the old one was wrong.
    let error = "";
    for (;;) {
      const auth = await modalResult<VaultAuth>(title, VaultAuthModalBody, {
        usesPassword: passwordIsUsed(factorStatus),
        usesKeyfile: (factorStatus?.keyfileCount ?? 0) > 0,
        usesSecurityKey: (factorStatus?.securityKeyCount ?? 0) > 0,
        intro: options.intro ?? $t("SteamGuard_Factors_PasswordBody"),
        confirmLabel: $t("SteamGuard_Continue"),
        pickKeyfile: () => steamGuardSettings.pickKeyfile(),
        requireKeyfile: options.requireKeyfile ?? false,
        requireKeyfileReason: options.requireKeyfileReason ?? "",
        error,
      });
      if (!auth) return null;
      try {
        await steamGuardSettings.unlockForManagement(auth.password, auth.keyfilePath, auth.backupKey);
        return auth;
      } catch (failure) {
        error = withFailureReason($t("SteamGuard_Factors_AuthRejected"), failure);
      }
    }
  }

  /**
   * The translated line plus the reason the vault gave. A Go error is never
   * empty, so showing its message alone left the retry banner untranslated -
   * and, for a Wails JSON-object message, showing the raw blob.
   */
  function withFailureReason(message: string, error: unknown): string {
    const reason = formatUnknownError(error).split("\n")[0]?.trim() ?? "";
    return reason ? `${message} ${reason}` : message;
  }

  // Changing the password re-keys every way in that uses it, so a keyfile that
  // is paired with the password has to be present. Without this the change is
  // refused at the very end, after a new password has already been chosen.
  $: changeNeedsKeyfile = (factorStatus?.factors ?? []).some(
    (factor) => factor.requiresPassword && factor.requires.includes("keyfile"),
  );

  // The second step of adding a factor: what to call it, and whether it should
  // need a password of its own. Blank means it opens the vault by itself.
  async function askNewFactorOptions(
    title: string,
    props: Record<string, unknown>,
  ): Promise<{ name: string; password: string } | null> {
    return modalResult<{ name: string; password: string }>(title, NewFactorModalBody, {
      ...props,
      confirmLabel: $t("SteamGuard_Continue"),
    });
  }

  // A losable factor is only safe to enrol once a backup key exists, so the
  // flow offers to make one rather than refusing with an error the user then
  // has to act on themselves.
  async function ensureBackupKey(auth: VaultAuth): Promise<boolean> {
    if (factorStatus?.hasBackupKey) return true;
    const confirmed = await openConfirm({
      title: $t("SteamGuard_Factors_BackupKeyTitle"),
      style: "okcancel",
      positiveLabel: $t("SteamGuard_Factors_CreateBackupKey"),
      negativeLabel: $t("Button_Cancel"),
      body: `<p>${escapeHtml($t("SteamGuard_Factors_NeedsBackupKey"))}</p>`,
    });
    if (!confirmed) return false;
    await issueBackupKey(auth);
    return factorStatus?.hasBackupKey ?? false;
  }

  async function loadFactors(): Promise<void> {
    if (!configured) return;
    try {
      factorStatus = await steamGuardSettings.listVaultFactors();
    } catch {
      // A locked or absent vault simply has nothing to list.
      factorStatus = null;
    }
    try {
      securityKeySupport = await steamGuardSettings.securityKeyAvailable();
    } catch {
      securityKeySupport = null;
    }
  }

  function factorLabel(kind: string): string {
    switch (kind) {
      case "password": return $t("SteamGuard_Factor_Password");
      case "keyfile": return $t("SteamGuard_Factor_Keyfile");
      case "recovery": return $t("SteamGuard_Factor_BackupKey");
      case "securitykey": return $t("SteamGuard_Factor_SecurityKey");
      default: return kind;
    }
  }

  // A way in is named after the thing the user holds. A user-chosen name wins,
  // since a key called "Desk drawer" is the whole point of naming it.
  function factorRowName(factor: SteamGuardVaultFactor): string {
    if (factor.kind === "securitykey" && factor.label) return factor.label;
    return factorLabel(factor.kind);
  }

  function blockReason(factor: SteamGuardVaultFactor): string {
    switch (factor.blocks) {
      case "last": return $t("SteamGuard_Factors_BlockLast");
      case "lastInteractive": return $t("SteamGuard_Factors_BlockLastInteractive");
      case "backupNeeded": return $t("SteamGuard_Factors_BlockBackupNeeded");
      default: return "";
    }
  }

  async function createBackupKey(): Promise<void> {
    const auth = await authenticateForManagement($t("SteamGuard_Factors_BackupKeyTitle"));
    if (auth === null) return;
    await issueBackupKey(auth);
  }

  async function issueBackupKey(auth: VaultAuth): Promise<void> {
    let code: string;
    try {
      code = await steamGuardSettings.createBackupKey(auth.password);
    } catch (error) {
      actionError(error);
      return;
    }
    // Shown once and never retrievable. A component rather than injected HTML
    // so the key sits in a field the user can select, copy and save - sanitized
    // body HTML cannot carry those controls.
    await openAlert({
      title: $t("SteamGuard_Factors_BackupKeyTitle"),
      bodyComponent: BackupKeyModalBody,
      bodyProps: {
        code,
        intro: $t("SteamGuard_Factors_BackupKeyBody"),
        saveToFile: (value: string) => steamGuardSettings.saveBackupKey(value),
      },
    });
    pushToast({ type: "success", message: $t("SteamGuard_Factors_BackupKeyIssued"), duration: 6000 });
    await loadFactors();
  }

  async function enrollKeyfile(): Promise<void> {
    const auth = await authenticateForManagement($t("SteamGuard_Factors_KeyfileTitle"));
    if (auth === null) return;
    if (!(await ensureBackupKey(auth))) return;
    const options = await askNewFactorOptions($t("SteamGuard_Factors_KeyfileTitle"), {
      intro: $t("SteamGuard_Factors_KeyfileWarnAlone"),
      passwordLabel: $t("SteamGuard_Factors_KeyfilePasswordLabel"),
      passwordHint: $t("SteamGuard_Factors_OptionalPasswordHint"),
    });
    if (options === null) return;
    try {
      const path = await steamGuardSettings.enrollKeyfile(auth.password, options.password);
      await openAlert({
        title: $t("SteamGuard_Factors_KeyfileTitle"),
        body: `<p>${escapeHtml($t("SteamGuard_Factors_KeyfileSaved"))}</p><code>${escapeHtml(path)}</code>`,
      });
      pushToast({ type: "success", message: $t("SteamGuard_Factors_KeyfileEnrolled"), duration: 6000 });
    } catch (error) {
      actionError(error);
    }
    await loadFactors();
  }

  async function enrollSecurityKey(): Promise<void> {
    const auth = await authenticateForManagement($t("SteamGuard_Factors_SecurityKeyTitle"));
    if (auth === null) return;
    if (!(await ensureBackupKey(auth))) return;
    const options = await askNewFactorOptions($t("SteamGuard_Factors_SecurityKeyTitle"), {
      askName: true,
      nameLabel: $t("SteamGuard_Factors_NameLabel"),
      namePlaceholder: $t("SteamGuard_Factors_NamePlaceholder"),
      intro: $t("SteamGuard_Factors_SecurityKeyWarnAlone"),
      passwordLabel: $t("SteamGuard_Factors_SecurityKeyPasswordLabel"),
      passwordHint: $t("SteamGuard_Factors_OptionalPasswordHint"),
    });
    if (options === null) return;
    // The key prompt is Windows' own and appears over the app, so say what is
    // about to happen before it does.
    pushToast({ type: "info", message: $t("SteamGuard_Factors_SecurityKeyTouch"), duration: 8000 });
    try {
      await steamGuardSettings.enrollSecurityKey(auth.password, options.name, options.password);
      pushToast({ type: "success", message: $t("SteamGuard_Factors_SecurityKeyEnrolled"), duration: 6000 });
    } catch (error) {
      actionError(error);
    }
    await loadFactors();
  }

  // Adds a password back to a vault that has none, which happens when the user
  // removed it in favour of a key, or an older build folded it into a keyfile.
  async function enrollPassword(): Promise<void> {
    const auth = await authenticateForManagement($t("SteamGuard_Factors_AddPassword"));
    if (auth === null) return;
    const options = await askNewFactorOptions($t("SteamGuard_Factors_AddPassword"), {
      intro: $t("SteamGuard_Factors_AddPasswordBody"),
      passwordLabel: $t("SteamGuard_Settings_NewPasswordTitle"),
      passwordHint: $t("SteamGuard_Factors_AddPasswordHint"),
    });
    if (options === null) return;
    if (!options.password) {
      pushToast({ type: "error", message: $t("SteamGuard_Factors_AddPasswordHint"), duration: 5000 });
      return;
    }
    try {
      await steamGuardSettings.enrollPassword(auth.password, options.password);
      pushToast({ type: "success", message: $t("SteamGuard_Factors_PasswordAdded"), duration: 5000 });
    } catch (error) {
      actionError(error);
    }
    await loadFactors();
  }

  async function removeFactor(factor: SteamGuardVaultFactor): Promise<void> {
    const label = factorRowName(factor);
    // A row named after a keyfile does not look like it is carrying the
    // password too, so removing the last way in that uses one is spelled out.
    const warning = factor.lastPasswordWayIn && factor.kind !== "password"
      ? `<p class="modal-warning">${escapeHtml($t("SteamGuard_Factors_RemoveTakesPassword"))}</p>`
      : "";
    const confirmed = await openConfirm({
      title: $t("SteamGuard_Factors_RemoveTitle", { factor: label }),
      style: "okcancel",
      positiveLabel: $t("SteamGuard_Factors_Remove"),
      negativeLabel: $t("Button_Cancel"),
      body: `<p>${escapeHtml($t("SteamGuard_Factors_RemoveBody", { factor: label }))}</p>${warning}`,
    });
    if (!confirmed) return;

    const auth = await authenticateForManagement($t("SteamGuard_Factors_RemoveTitle", { factor: label }));
    if (auth === null) return;
    try {
      await steamGuardSettings.removeVaultFactor(auth.password, factor.id);
      pushToast({ type: "success", message: $t("SteamGuard_Factors_Removed"), duration: 4000 });
    } catch (error) {
      actionError(error);
    }
    await loadFactors();
  }

  async function renameFactor(factorId: string, current: string): Promise<void> {
    const name = await openPrompt({
      title: $t("SteamGuard_Factors_Rename"),
      body: $t("SteamGuard_Factors_NameLabel"),
      inputType: "text",
      initialValue: current,
      positiveLabel: $t("Ok"),
      negativeLabel: $t("Button_Cancel"),
    });
    if (name === null || !name.trim()) return;
    const auth = await authenticateForManagement($t("SteamGuard_Factors_Rename"));
    if (auth === null) return;
    try {
      await steamGuardSettings.renameVaultFactor(auth.password, factorId, name.trim());
    } catch (error) {
      actionError(error);
    }
    await loadFactors();
  }

  // Security keys get their own screen: there can be several, each needs a name
  // to be told apart, and a row per key on the settings page would bury
  // everything else.
  async function openSecurityKeys(): Promise<void> {
    // The chosen action runs after this modal is gone, not from inside it: the
    // next step opens a modal of its own and two cannot be on screen at once.
    type PendingAction = () => Promise<void>;
    // Held on an object: assigning a plain local from inside these callbacks
    // leaves TypeScript narrowing it to never at the call below.
    const pending: { run: PendingAction | null } = { run: null };
    const choose = (next: PendingAction) => { pending.run = next; };
    await modalResult<void>($t("SteamGuard_Factors_SecurityKeysTitle"), SecurityKeysModalBody, {
      keys: securityKeyRows,
      available: securityKeySupport?.available ?? false,
      unavailableReason: securityKeySupport?.reason ?? "",
      onAdd: () => choose(enrollSecurityKey),
      onRemove: (id: string) => {
        const row = factorStatus?.factors.find((factor) => factor.id === id);
        if (row) choose(() => removeFactor(row));
      },
      onRename: (id: string, label: string) => choose(() => renameFactor(id, label)),
      onClose: () => {},
    });
    if (pending.run) await pending.run();
  }

  function formatVerifiedAt(value: string): string {
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
  }

  onMount(() => {
    void steamGuardSettings.refresh().then(loadFactors).catch(() => undefined);
  });
</script>

<!-- `settings-group` so the page's search treats this as one findable section. -->
<section class="steam-guard-settings settings-group" aria-labelledby="steam-guard-settings-title">
  <div class="steam-guard-heading-row">
    <!-- h2 to match the sibling sections of the Steam settings page it lives on. -->
    <h2 id="steam-guard-settings-title" class="SettingsHeader">{$t("SteamGuard_Settings_Title")}</h2>
    {#if $steamGuardSettings.availability === "error"}
      <span class="steam-guard-service-state" role="status">{$t("SteamGuard_Settings_StatusError")}</span>
    {:else if !ready}
      <span class="steam-guard-service-state" role="status">{$t("SteamGuard_Settings_StatusUnavailable")}</span>
    {/if}
  </div>

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
      <!-- Changing the password lives with the other factor actions below, since
           the password is one of the ways in that block lists. -->
      <!-- Locking is only meaningful once the vault is unlocked; showing it otherwise
           offers an action that would do nothing. -->
      {#if configured && $steamGuardSettings.status.unlocked}
        <button type="button" class="btnicontext" disabled={busy} on:click={() => void lockNow()}>
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 448 512" aria-hidden="true"><path d="M400 224h-24v-72C376 68.2 307.8 0 224 0S72 68.2 72 152v72H48c-26.5 0-48 21.5-48 48v192c0 26.5 21.5 48 48 48h352c26.5 0 48-21.5 48-48V272c0-26.5-21.5-48-48-48zm-104 0H152v-72c0-39.7 32.3-72 72-72s72 32.3 72 72v72z" /></svg>
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
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 448 512" aria-hidden="true"><path d="M320 448v40c0 13.255-10.745 24-24 24H24c-13.255 0-24-10.745-24-24V120c0-13.255 10.745-24 24-24h72v296c0 30.879 25.121 56 56 56h168zm0-344V0H152c-13.255 0-24 10.745-24 24v368c0 13.255 10.745 24 24 24h272c13.255 0 24-10.745 24-24V128H344c-13.2 0-24-10.8-24-24zm120.971-31.029L375.029 7.029A24 24 0 0 0 358.059 0H352v96h96v-6.059a24 24 0 0 0-7.029-16.97z" /></svg>
        {$t("SteamGuard_Settings_CopyPath")}
      </button>
      <button type="button" class="btnicontext" disabled={!ready || !path || busy} aria-describedby={pathId} on:click={() => void openFolder()}>
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M2 5.5A1.5 1.5 0 0 1 3.5 4h5.2a1.5 1.5 0 0 1 1.06.44L12 6.5h5.5A1.5 1.5 0 0 1 19 8v2.6H6.4L4.3 19.9A1.5 1.5 0 0 1 2 18.6ZM8 12.2h14l-3.1 9.6H5.9Z"/></svg>
        {$t("SteamGuard_Settings_OpenFolder")}
      </button>
    </span>
  </div>

  <!-- Sits with the two buttons it is about, rather than at the top of the
       section where it read as a caption for the folder path. -->
  <p id={warningId} class="steam-guard-warning">{$t("SteamGuard_Settings_KeepVerifiedBackup")}</p>

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
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path fill-rule="evenodd" d="M2 3.5A1.5 1.5 0 0 1 3.5 2h17A1.5 1.5 0 0 1 22 3.5v3A1.5 1.5 0 0 1 20.5 8h-17A1.5 1.5 0 0 1 2 6.5Zm1.5 6.1h17v10A2.4 2.4 0 0 1 18.1 22H5.9a2.4 2.4 0 0 1-2.4-2.4Zm5.7 2.8h5.6v2.4H9.2Z"/></svg>
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
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M8.4 15.2A6.6 6.6 0 1 1 21.6 15.2 6.6 6.6 0 1 1 8.4 15.2zM13.9 16.3h5.8v-2.2h-3.6V10.4h-2.2zM16.69 3.69A10 10 0 0 0 3.61 12.85L5.7 12.92A7.9 7.9 0 0 1 16.04 5.69zM2.16 12.8L4.49 17.89 7.16 12.98z"/></svg>
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

  <!-- The action row is gated on the vault existing, not on the factor list
       loading: changing the password must not disappear because listing the
       slots failed. -->
  {#if configured}
    <!-- One line: a heading, then the ways in as a sentence. They are
         alternatives, and a comma-separated list says so as plainly as a bulleted
         one did while costing a quarter of the page. -->
    <p class="steam-guard-ways">
      <span class="steam-guard-subheading steam-guard-ways-label">{$t("SteamGuard_Factors_Title")}:</span>
      {#each waysIn as way, index (way.key)}{#if index > 0}<span class="steam-guard-ways-sep">, </span>{/if}<span
          class="steam-guard-way"
        >{way.name}{#if way.note}<span class="steam-guard-factor-note">{way.note}</span>{/if}{#if way.action}<button
            type="button"
            class="fancyLink steam-guard-factor-remove"
            disabled={busy || way.action.disabled}
            title={way.action.title}
            on:click={way.action.run}
          >({way.action.label})</button>{/if}</span>{/each}
    </p>

    <div class="steam-guard-control-row">
      <span class="steam-guard-actions">
        {#if factorStatus && !factorStatus.passwordOpens}
          <button type="button" class="btnicontext" disabled={busy} on:click={() => void enrollPassword()}>
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M7 6A6 6 0 1 1 7 18 6 6 0 1 1 7 6ZM7 9.4A2.6 2.6 0 1 0 7 14.6 2.6 2.6 0 1 0 7 9.4ZM11 10.2H22.5V17.5H19.8V13.8H17.3V18.5H14.8V13.8H11Z"/></svg>
            {$t("SteamGuard_Factors_AddPassword")}
          </button>
        {:else}
          <button type="button" class="btnicontext" disabled={!configured || busy} on:click={() => void changePassword()}>
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M17.73 2.73 21.27 6.27 11.27 16.27 5.61 18.39 7.73 12.73Z"/></svg>
            {$t("SteamGuard_Settings_ChangePassword")}
          </button>
        {/if}
        <button type="button" class="btnicontext" disabled={busy} on:click={() => void createBackupKey()}>
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M7 6A6 6 0 1 1 7 18 6 6 0 1 1 7 6ZM7 9.4A2.6 2.6 0 1 0 7 14.6 2.6 2.6 0 1 0 7 9.4ZM11 10.2H22.5V17.5H19.8V13.8H17.3V18.5H14.8V13.8H11Z"/></svg>
          {factorStatus?.hasBackupKey
            ? $t("SteamGuard_Factors_ReplaceBackupKey")
            : $t("SteamGuard_Factors_CreateBackupKey")}
        </button>
        <!-- One keyfile, so this is a toggle rather than a pair of buttons. Two
             would mean "Remove keyfile" left a second file still opening the
             vault. Removal happens from its row above. -->
        {#if !factorStatus?.hasKeyfile}
          <button type="button" class="btnicontext" disabled={busy} on:click={() => void enrollKeyfile()}>
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path fill-rule="evenodd" d="M6.5 2h7.1L20 8.4v11.1A2.5 2.5 0 0 1 17.5 22h-11A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2Zm1 9.5h9v2h-9Zm0 3.6h9v2h-9Zm0 3.6h6v2h-6Z"/></svg>
            {$t("SteamGuard_Factors_AddKeyfile")}
          </button>
        {/if}
        {#if securityKeySupport?.available}
          <button type="button" class="btnicontext" disabled={busy} on:click={() => void openSecurityKeys()}>
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M12 1.6 21.5 5.2V11.4C21.5 16.6 17.6 20.6 12 22.4 6.4 20.6 2.5 16.6 2.5 11.4V5.2Z"/></svg>
            {$t("SteamGuard_Factors_SecurityKeysTitle")}
          </button>
        {/if}
      </span>
    </div>
    {#if securityKeySupport && !securityKeySupport.available}
      <p class="steam-guard-factors-intro">
        {securityKeySupport.reason || $t("SteamGuard_Factors_SecurityKeyUnavailable")}
      </p>
    {/if}
    {#if factorStatus && !factorStatus.hasBackupKey}
      <p class="steam-guard-warning">{$t("SteamGuard_Factors_NeedsBackupKey")}</p>
    {/if}
  {/if}
</section>

<style lang="scss">
  /* Reads as one more settings section, so no box of its own — spacing comes
     from `.settings-group + .settings-group` like every other section. */
  .steam-guard-settings {
    display: grid;
    gap: 0.75rem;
  }

  /* Content lines up with the toggles and fields in neighbouring sections. The
     heading is excluded: it keeps the flush alignment its accent bar needs. */
  .steam-guard-settings > :not(.steam-guard-heading-row) {
    padding-inline: var(--settings-toggle-pad-x, 0.4rem);
  }

  .steam-guard-heading-row,
  .steam-guard-subheading {
    font-size: 1em;
    color: var(--whiteSecondary);
  }

  .steam-guard-factors-intro {
    margin: 0 0 0.35rem;
    opacity: 0.8;
  }

  /* The heading and the ways in share a line and wrap together. */
  .steam-guard-ways {
    margin: 0.5rem 0;
  }

  .steam-guard-ways-label {
    margin: 0 0.35em 0 0;
  }

  /* The action is an atomic inline, which offers the line a place to break on
     either side of it however the markup is written. Without this, "(Remove)"
     lands on the next line with nothing to say what it removes. Every label
     here is a fixed string, so keeping an entry whole cannot run away. */
  .steam-guard-way {
    font-weight: 600;
    white-space: nowrap;
  }

  /* The comma belongs to the sentence, not to the entry before it. */
  .steam-guard-ways-sep {
    font-weight: 400;
  }

  .steam-guard-factor-note {
    margin-left: 0.4em;
    font-size: 0.85em;
    opacity: 0.75;
  }

  /* Reads as a link but stays a button: it performs an action, and the app-wide
     navigation guard cancels anchor clicks, so an <a> here would do nothing.
     fancyLink carries the link look and the reset that keeps a theme from
     skinning it as one; only the spacing, weight and underline are local. */
  .steam-guard-factor-remove {
    margin: 0 0 0 0.35em;
    font-weight: 400;
    text-decoration: underline;
  }

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

  /* The settings-control height, which a link-styled button is not: it would
     stand a line of text in a 38px box. */
  button:not(.fancyLink) {
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

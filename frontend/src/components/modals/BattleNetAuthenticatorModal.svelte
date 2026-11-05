<script lang="ts">
  import { onMount } from "svelte";
  import { t } from "../../stores/i18n";
  import { pushToast } from "../../stores/toast";
  import { setModalBusy } from "../../stores/modal";
  import { clearModalBackAction, setModalBackAction } from "../../stores/modalBack";
  import { requestModalAutoFit } from "../../lib/modalFrame";
  import { formatToastWithError, formatUnknownError } from "../../lib/formatWailsError";
  import { copyText } from "../../lib/clipboard";
  import { emptyBattleNetForm, type BattleNetAccount, type BattleNetActions, type BattleNetRecovery, type BattleNetScreen } from "../../lib/battleNetAuthenticator";
  import AuthenticatorUnlockForm from "./AuthenticatorUnlockForm.svelte";
  import BattleNetAuthenticatorPanel from "./BattleNetAuthenticatorPanel.svelte";
  import { battleNetCall as call, requestBattleNetView, publishBattleNetAuthenticators } from "../../lib/battleNetAuthenticatorBridge";

  export let accountLink = "", suggestedLabel = "", initialSerial = "";
  export let initialScreen: BattleNetScreen = "home";
  let token = "", lease = "", password = "", keyfile = "", backupKey = "";
  let accounts: BattleNetAccount[] = [], preview: BattleNetAccount[] = [];
  let serial = initialSerial, code = "", expires = 0, remaining = 0, error = "";
  let busy = false, ready = false, configured = true, mounted = false, closed = false, polling = false;
  let copyingCode = false;
  let accountNameRefresh: Promise<void> | null = null;
  let nextAccountNameRefresh = 0;
  let enrollment = "", enrollmentState = "";
  let appPassword = "", appPasswordRequired = false;
  let rememberForSession = false, hasSecurityKey: boolean | undefined;
  let recovery: BattleNetRecovery | null = null;
  let screen: BattleNetScreen = "home";
  let form = emptyBattleNetForm();

  $: if (mounted) setModalBackAction(ready && screen !== "home" ? {
    label: $t("Button_Back"),
    run: () => { void back(); },
  } : null);
  $: if (mounted) setModalBusy(busy || (!!enrollment && enrollmentState === "saving"));

async function grant() {
    if (token) {
        await call("EndSensitiveView", token, lease).catch(() => { });
        token = "";
    }
    const data = await requestBattleNetView();
    if (closed) {
      await call("EndSensitiveView", data.capability, data.lease);
      throw new Error("closed");
    }
    token = data.capability;
    lease = data.lease;
}

  function clearSecrets(): void {
    code = "";
    recovery = null;
    password = "";
    keyfile = "";
    backupKey = "";
    appPassword = "";
    form = emptyBattleNetForm();
  }

  async function run(action: () => Promise<void>): Promise<void> {
    if (busy || closed) return;
    busy = true;
    error = "";
    try {
      if (accountNameRefresh) await accountNameRefresh;
      if (closed) return;
      await action();
    } catch (cause) {
      code = "";
      error = formatUnknownError(cause);
    } finally {
      busy = false;
      if (closed) clearSecrets();
    }
  }

  function showScreen(next: BattleNetScreen): void {
    screen = next;
    error = "";
    recovery = null;
    code = "";
    expires = 0;
    remaining = 0;
    preview = [];
    form = emptyBattleNetForm();
    if (next === "rename") form.accountLabel = accounts.find(a => a.serial === serial)?.label ?? "";
    requestModalAutoFit();
  }

  function navigate(next: BattleNetScreen): void {
    if (!busy && !enrollment) showScreen(next);
  }

  async function back(): Promise<void> {
    if (busy || enrollmentState === "saving") return;
    if (enrollment) {
      await cancelEnrollment();
      return;
    }
    navigate(["enroll", "import", "restore"].includes(screen) ? "add" : "home");
  }

  async function refresh(): Promise<void> {
    const result: BattleNetAccount[] = await call("ListBattleNetAccounts", token);
    if (closed) return;
    const initial = !ready;
    accounts = result;
    publishBattleNetAuthenticators(accounts);
    if (!accounts.some(a => a.serial === serial)) {
      serial = accounts.find(a => accountLink && a.accountLink.trim().toLowerCase() === accountLink.trim().toLowerCase())?.serial
        ?? (!accountLink && !initialSerial ? accounts[0]?.serial : "") ?? "";
    }
    ready = true;
    if (initial && serial) showScreen(initialScreen);
    else if (initial && accounts.length && (accountLink || initialSerial)) showScreen("add");
  }

  async function unlock(): Promise<void> {
    await run(async () => {
      if (!configured) {
        await call("Initialize", password, appPassword);
        configured = true;
      }
      appPassword = "";
      await grant();
      await call("UnlockBattleNet", password, keyfile, backupKey.trim(), rememberForSession, token);
      password = "";
      keyfile = "";
      backupKey = "";
      await refresh();
    });
  }

  async function pickImport(): Promise<void> {
    await run(async () => {
      const chosen = await call("PickBattleNetImport");
      if (!chosen) return;
      form.source = chosen;
      preview = [];
      form.selected = [];
    });
  }

  async function previewImport(): Promise<void> {
    await run(async () => {
      preview = await call("PreviewBattleNetImport", form.source.trim(), form.sourcePassword, token);
      form.selected = preview.filter(a => form.replaceExisting || !accounts.some(existing => existing.serial === a.serial)).map(a => a.serial);
      if (!preview.length) pushToast({ type: "info", message: $t("Authenticator_NoImportAccounts") });
    });
  }

  async function importSelected(): Promise<void> {
    await run(async () => {
      const imported = form.selected[0];
      await call("ImportBattleNet", form.source.trim(), form.sourcePassword, form.selected, form.replaceExisting, token);
      form.sourcePassword = "";
      await grant();
      await refresh();
      serial = imported;
      showScreen("home");
      pushToast({ type: "success", message: $t("Authenticator_Imported") });
    });
  }

  async function exportSelected(): Promise<void> {
    await run(async () => {
      const result = await call("ExportBattleNet", [serial], form.password, form.keyfile, form.backupKey.trim(), form.exportPassword, token);
      if (!result) return;
      showScreen("home");
      pushToast({ type: "success", message: $t("Authenticator_Exported") });
    });
  }

  async function enroll(): Promise<void> {
    await run(async () => {
      const result = await call("BeginBattleNetEnrollment", token);
      enrollment = result.handle;
      enrollmentState = result.state;
    });
  }

  async function cancelEnrollment(): Promise<void> {
    await run(async () => {
      await call("CancelBattleNetEnrollment", enrollment, token);
      enrollment = "";
      enrollmentState = "";
      showScreen("add");
    });
  }

  async function restore(): Promise<void> {
    await run(async () => {
      const restoredSerial = form.restoreSerial.replace(/\s/g, "").toUpperCase();
      await call("RestoreBattleNetAuthenticator", form.accountIdentifier.trim(), restoredSerial, form.restoreCode.trim(), token);
      form.restoreCode = "";
      await grant();
      await refresh();
      serial = accounts.find(a => a.serial.replaceAll("-", "") === restoredSerial.replaceAll("-", ""))?.serial ?? serial;
      showScreen("home");
      pushToast({ type: "success", message: $t("Authenticator_Restored") });
    });
  }

  async function recover(): Promise<void> {
    await run(async () => {
      const saved = new Set(accounts.map(a => a.serial));
      await call("RecoverBattleNetEnrollments", token);
      await grant();
      await refresh();
      const recovered = accounts.filter(a => !saved.has(a.serial));
      enrollment = "";
      enrollmentState = "";
      if (!recovered.length) {
        pushToast({ type: "info", message: $t("Authenticator_NoRecovery") });
        return;
      }
      serial = recovered[0].serial;
      showScreen("home");
      pushToast({ type: "success", message: $t("Authenticator_Recovered", { count: recovered.length }) });
    });
  }

  async function acknowledgeBackup(): Promise<void> {
    await run(async () => {
      await call("AcknowledgeBattleNetBackup", serial, token);
      recovery = null;
      await grant();
      await refresh();
      showScreen("home");
    });
  }

  async function rename(): Promise<void> {
    await run(async () => {
      await call("SetBattleNetLabel", serial, form.accountLabel.trim(), token);
      await grant();
      await refresh();
      showScreen("home");
    });
  }

  async function link(): Promise<void> {
    await run(async () => {
      await call("LinkBattleNetAccount", serial, accountLink, suggestedLabel, token);
      await grant();
      await refresh();
      pushToast({ type: "success", message: $t("Authenticator_Linked") });
    });
  }

  async function copyCode(): Promise<void> {
    if (busy || closed || copyingCode || screen !== "home" || !code) return;
    copyingCode = true;
    const requestedSerial = serial;
    try {
      if (accountNameRefresh) await accountNameRefresh;
      if (closed || busy || requestedSerial !== serial || screen !== "home" || !code) return;
      await call("CopyBattleNetCode", serial, token);
      if (!closed) pushToast({ type: "success", message: $t("Authenticator_Copied") });
    } catch (cause) {
      if (!closed) pushToast({ type: "error", message: formatToastWithError($t("Toast_CopyFailed"), cause) });
    } finally {
      copyingCode = false;
    }
  }

  async function refreshAccountName(requestedSerial: string): Promise<void> {
    try {
      const changed = await call("RefreshBattleNetAccountName", requestedSerial, token);
      if (changed && !closed) {
        await grant();
        await refresh();
      }
    } catch { }
  }

  async function copyRecovery(field: "serial" | "restoreCode" | "uri"): Promise<void> {
    if (busy || closed || !recovery?.[field]) return;
    try {
      await copyText(recovery[field]);
      pushToast({ type: "success", message: $t("Toast_Copied") });
    } catch (cause) {
      pushToast({ type: "error", message: formatToastWithError($t("Toast_CopyFailed"), cause) });
    }
  }

  const actions: BattleNetActions = {
    navigate, pickImport, previewImport, importSelected, enroll, cancelEnrollment, restore, recover,
    exportSelected, acknowledgeBackup, rename, link, copyCode, copyRecovery,
    selectAccount: () => { code = ""; expires = 0; remaining = 0; recovery = null; error = ""; },
    showRecovery: () => run(async () => { recovery = await call("RevealBattleNetRecovery", serial, token); }),
    browse: () => run(async () => { await call("OpenBattleNetAccount", serial, token); }),
    pickKeyfile: () => call("PickVaultKeyfile"),
    openEnrollmentAccount: () => run(async () => { await call("OpenBattleNetEnrollmentAccount", enrollment, token); }),
  };

  onMount(() => {
    mounted = true;
    void run(async () => {
      const status = await call("GetSettingsStatus");
      configured = status.vaultConfigured;
      appPasswordRequired = status.savedAccountDataEncrypted;
      rememberForSession = status.rememberPasswordForSession;
      hasSecurityKey = status.hasSecurityKey;
      await grant();
      if (status.unlocked) await refresh();
    });
    const timer = setInterval(async () => {
      remaining = Math.max(0, Math.ceil((expires - Date.now()) / 1000));
      if (!remaining) code = "";
      if (busy || polling || closed) return;
      if (enrollment && !["verification-required", "recovery-required"].includes(enrollmentState)) {
        polling = true;
        try {
          const result = await call("GetBattleNetEnrollmentStatus", enrollment, token);
          if (closed) return;
          enrollmentState = result.state;
          if (result.state === "pending-backup") {
            enrollment = "";
            await run(async () => {
              await grant();
              await refresh();
              serial = result.serial;
              showScreen("recovery");
            });
          } else if (result.state === "verification-required") {
            await actions.openEnrollmentAccount();
          } else if (["failed", "expired", "canceled"].includes(result.state)) {
            enrollment = "";
            error = result.error || $t(result.state === "expired" ? "Authenticator_EnrollmentExpired" : "Authenticator_EnrollmentFailed");
          }
        } catch (cause) {
          if (!closed) error = formatUnknownError(cause);
          enrollment = "";
        } finally {
          polling = false;
        }
        return;
      }
      if (!ready || !serial || !["home", "recovery"].includes(screen)) return;
      polling = true;
      const requestedSerial = serial;
      try {
        if (screen === "home") {
          const result = await call("GetBattleNetCode", requestedSerial, token);
          if (!closed && screen === "home" && requestedSerial === serial) {
            code = result.code;
            expires = result.expiresAt;
            remaining = Math.max(0, Math.ceil((expires - Date.now()) / 1000));
          }
        }
        if (!closed && !busy && !copyingCode && serial === requestedSerial && Date.now() >= nextAccountNameRefresh) {
          nextAccountNameRefresh = Date.now() + 5000;
          accountNameRefresh = refreshAccountName(requestedSerial);
          await accountNameRefresh;
          accountNameRefresh = null;
        }
      } catch {
        if (!closed && screen === "home" && requestedSerial === serial) {
          clearSecrets();
          ready = false;
        }
      } finally {
        polling = false;
      }
    }, 1000);
    return () => {
      closed = true;
      mounted = false;
      clearInterval(timer);
      clearModalBackAction();
      setModalBusy(false);
      clearSecrets();
      if (enrollment) void call("CancelBattleNetEnrollment", enrollment, token);
      if (token) void call("EndSensitiveView", token, lease);
    };
  });
</script>

<div class="authenticator">
  {#if !ready}
    <AuthenticatorUnlockForm
      idPrefix="battlenet-authenticator"
      bind:password
      bind:keyfilePath={keyfile}
      bind:backupKey
      bind:rememberForSession
      {busy}
      {error}
      {hasSecurityKey}
      pickKeyfile={actions.pickKeyfile}
      available={!!token}
      creating={!configured}
      onUnlock={unlock}
      onSecurityKey={unlock}
    >
      {#if !configured && appPasswordRequired}
        <label class="app-password"><span>{$t("Authenticator_AppPassword")}</span><input class="modal-input" type="password" bind:value={appPassword} autocomplete="current-password" disabled={busy} /></label>
      {/if}
    </AuthenticatorUnlockForm>
  {:else}
    <BattleNetAuthenticatorPanel
      {screen} {accounts} bind:serial {code} {remaining} {accountLink} {busy} {error}
      {enrollment} {enrollmentState} bind:preview {recovery} bind:form {actions}
    />
  {/if}
</div>

<style>
  .authenticator { display: flex; flex: 0 1 auto; flex-direction: column; width: 100%; min-width: 0; max-width: 40rem; margin: auto; color: var(--white, #fff); }
  :global(.modalFG:not(.modalFG--ready)) .authenticator { width: 34rem; }
  .app-password { display: grid; gap: 0.35rem; padding: 0; text-align: left; }
  .app-password .modal-input { width: 100%; min-width: 0; margin: 0; }
  @media (max-width: 640px) { :global(.modalFG:not(.modalFG--ready)) .authenticator { width: 100%; } }
</style>

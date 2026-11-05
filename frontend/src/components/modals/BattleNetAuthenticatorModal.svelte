<script lang="ts">
import { onMount } from "svelte";
import { Call, Events } from "@wailsio/runtime";
import { t } from "../../stores/i18n";
import AuthenticatorUnlockForm from "./AuthenticatorUnlockForm.svelte";
import SteamGuardVaultFactors from "./SteamGuardVaultFactors.svelte";
type Summary = {
    serial: string;
    label: string;
    pending: boolean;
    hasRestoreCode: boolean;
    accountLink: string;
};
export let accountLink = "", suggestedLabel = "";
const call = (method: string, ...args: unknown[]) => Call.ByName(`TcNo-Acc-Switcher/internal/steamguard.Service.${method}`, ...args);
let token = "", lease = "", password = "", keyfile = "", backupKey = "", source = "", sourcePassword = "", exportPassword = "";
let accounts: Summary[] = [], preview: Summary[] = [], selected: string[] = [], serial = "", code = "", expires = 0, remaining = 0, error = "", busy = false, ready = false, configured = true;
let closed = false, polling = false;
let enrollment = "", enrollmentState = "", accountIdentifier = "", restoreSerial = "", restoreCode = "";
let replaceExisting = false, accountLabel = "";
let appPassword = "", appPasswordRequired = false;
let rememberForSession = false, hasSecurityKey: boolean | undefined;
let recovery: {
    serial: string;
    restoreCode: string;
    uri: string;
    qrImage: string;
} | null = null;
async function grant() {
    if (token) {
        await call("EndSensitiveView", token, lease).catch(() => { });
        token = "";
    }
    const requestId = crypto.randomUUID();
    await new Promise<void>((resolve, reject) => {
        const off = Events.On("steamguard:sensitive-view-grant", event => {
            const data = event.data as {
                requestId: string;
                accountId: string;
                capability: string;
                lease: string;
            };
            if (data?.requestId !== requestId || data.accountId !== "battlenet-authenticators")
                return;
            clearTimeout(timer);
            off();
            if (closed) {
                void call("EndSensitiveView", data.capability, data.lease);
                reject(new Error("closed"));
                return;
            }
            token = data.capability;
            lease = data.lease;
            resolve();
        });
        const timer = setTimeout(() => { off(); reject(new Error("Authenticator view unavailable")); }, 5000);
        void call("RequestBattleNetView", requestId).catch(e => { clearTimeout(timer); off(); reject(e); });
    });
}
async function run(action: () => Promise<void>) {
    if (busy)
        return;
    busy = true;
    error = "";
    try {
        await action();
    }
    catch (e) {
        code = "";
        error = String(e);
    }
    finally {
        busy = false;
        if (closed) {
            code = "";
            recovery = null;
            password = "";
            appPassword = "";
            sourcePassword = "";
            exportPassword = "";
            restoreCode = "";
            backupKey = "";
        }
    }
}
async function refresh() { accounts = await call("ListBattleNetAccounts", token); if (!accounts.some(a => a.serial === serial))
    serial = accounts.find(a => accountLink && a.accountLink === accountLink)?.serial ?? accounts[0]?.serial ?? ""; ready = true; }
async function unlock() {
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
const pickKeyfile = (): Promise<string> => call("PickVaultKeyfile");
async function pick() { await run(async () => { source = await call("PickBattleNetImport"); preview = []; }); }
async function inspect() { await run(async () => { preview = await call("PreviewBattleNetImport", source, sourcePassword, token); selected = preview.filter(a => !accounts.some(existing => existing.serial === a.serial)).map(a => a.serial); }); }
async function importSelected() { await run(async () => { await call("ImportBattleNet", source, sourcePassword, selected, replaceExisting, token); sourcePassword = ""; preview = []; replaceExisting = false; await grant(); await refresh(); }); }
async function exportSelected() { await run(async () => { const result = await call("ExportBattleNet", [serial], password, keyfile, backupKey, exportPassword, token); if (result) {
    exportPassword = "";
    password = "";
    backupKey = "";
} }); }
async function enroll() { await run(async () => { const result = await call("BeginBattleNetEnrollment", token); enrollment = result.handle; enrollmentState = result.state; }); }
async function restore() { await run(async () => { await call("RestoreBattleNetAuthenticator", accountIdentifier, restoreSerial, restoreCode, token); restoreCode = ""; await grant(); await refresh(); serial = restoreSerial.trim().toUpperCase(); }); }
async function recover() { await run(async () => { await call("RecoverBattleNetEnrollments", token); await grant(); await refresh(); }); }
onMount(() => {
    void run(async () => { const status = await call("GetSettingsStatus"); configured = status.vaultConfigured; appPasswordRequired = status.savedAccountDataEncrypted; rememberForSession = status.rememberPasswordForSession; hasSecurityKey = status.hasSecurityKey; await grant(); if (status.unlocked)
        await refresh(); });
    const timer = setInterval(async () => {
        remaining = Math.max(0, Math.ceil((expires - Date.now()) / 1000));
        if (enrollment && !polling && !busy && !closed) {
            polling = true;
            try {
                const result = await call("GetBattleNetEnrollmentStatus", enrollment, token);
                enrollmentState = result.state;
                if (result.state === "pending-backup") {
                    enrollment = "";
                    await run(async () => { await grant(); await refresh(); serial = result.serial; });
                }
                else if (["failed", "expired", "canceled"].includes(result.state)) {
                    enrollment = "";
                    error = result.error || result.state;
                }
            }
            catch (e) {
                error = String(e);
                enrollment = "";
            }
            finally {
                polling = false;
            }
        }
        if (!ready || !serial || busy || polling || closed)
            return;
        polling = true;
        try {
            const result = await call("GetBattleNetCode", serial, token);
            if (!closed) {
                code = result.code;
                expires = result.expiresAt;
                remaining = Math.max(0, Math.ceil((expires - Date.now()) / 1000));
            }
        }
        catch {
            code = "";
            recovery = null;
            ready = false;
        }
        finally {
            polling = false;
        }
    }, 1000);
    return () => { closed = true; clearInterval(timer); code = ""; recovery = null; password = ""; appPassword = ""; sourcePassword = ""; exportPassword = ""; backupKey = ""; restoreCode = ""; if (enrollment)
        void call("CancelBattleNetEnrollment", enrollment, token); if (token)
        void call("EndSensitiveView", token, lease); };
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
      {pickKeyfile}
      available={!!token}
      creating={!configured}
      onUnlock={unlock}
      onSecurityKey={unlock}
    >
      {#if !configured && appPasswordRequired}
        <label class="app-password">
          <span>{$t("Authenticator_AppPassword")}</span>
          <input class="modal-input" type="password" bind:value={appPassword} autocomplete="current-password" disabled={busy} />
        </label>
      {/if}
    </AuthenticatorUnlockForm>
  {:else}
    {#if error}<p role="alert">{error}</p>{/if}
    <p>{$t("Authenticator_OnlineNotice")}</p>
    <button disabled={busy||!!enrollment} on:click={enroll}>{$t("Authenticator_Enroll")}</button>
    {#if enrollment}<p role="status">{$t("Authenticator_Waiting")}</p><button on:click={()=>run(async()=>{await call("CancelBattleNetEnrollment",enrollment,token);enrollment="";})}>{$t("Authenticator_Cancel")}</button>{/if}
    <button disabled={busy} on:click={recover}>{$t("Authenticator_Recover")}</button>
    {#if accounts.length}
      <label>{$t("Authenticator_Account")}<select bind:value={serial} on:change={()=>{code="";recovery=null;}}>{#each accounts as account}<option value={account.serial}>{account.label||account.serial}</option>{/each}</select></label>
      <output class="auth-code" aria-label={$t("Authenticator_Code")}>{code||"••••••••"}</output>
      <label>{$t("Authenticator_Label")}<input bind:value={accountLabel} maxlength="200" /></label>
      <button disabled={busy||!accountLabel.trim()} on:click={()=>run(async()=>{await call("SetBattleNetLabel",serial,accountLabel,token);accountLabel="";await grant();await refresh();})}>{$t("Authenticator_SaveLabel")}</button>
      {#if accountLink}<button disabled={busy} on:click={()=>run(async()=>{await call("LinkBattleNetAccount",serial,accountLink,suggestedLabel,token);await grant();await refresh();})}>{$t("Authenticator_LinkAccount")}</button>{/if}
      <button disabled={busy||!code} on:click={()=>run(async()=>{await call("CopyBattleNetCode",serial,token);})}>{$t("Authenticator_Copy")}</button>
      <progress max="30" value={remaining}></progress>
      <p>{$t("Authenticator_Seconds",{seconds:remaining})}</p>
      <button disabled={busy} on:click={()=>run(async()=>{await call("OpenBattleNetAccount",serial,token);})}>{$t("Authenticator_Browse")}</button>
      <button disabled={busy} on:click={()=>run(async()=>{recovery=recovery?null:await call("RevealBattleNetRecovery",serial,token);})}>{$t("Authenticator_Recovery")}</button>
      {#if recovery}<p>{$t("Authenticator_RecoveryWarning")}</p><p>{recovery.serial}</p><p>{recovery.restoreCode}</p><img src={recovery.qrImage} alt={$t("Authenticator_QR")} width="240" height="240" /><textarea readonly value={recovery.uri}></textarea>{/if}
      {#if recovery&&accounts.find(a=>a.serial===serial)?.pending}<button disabled={busy} on:click={()=>run(async()=>{await call("AcknowledgeBattleNetBackup",serial,token);recovery=null;await grant();await refresh();})}>{$t("Authenticator_BackupAcknowledged")}</button>{/if}
    {/if}
    <fieldset disabled={busy||!!enrollment}>
      <legend>{$t("Authenticator_Import")}</legend>
      <label>{$t("Authenticator_Source")}<input bind:value={source} /></label>
      <button on:click={pick}>{$t("Authenticator_ChooseFile")}</button>
      <label>{$t("Authenticator_SourcePassword")}<input type="password" bind:value={sourcePassword} autocomplete="off" /></label>
      <button disabled={!source} on:click={inspect}>{$t("Authenticator_Preview")}</button>
      <label><input type="checkbox" bind:checked={replaceExisting} on:change={()=>{selected=[];}} />{$t("Authenticator_Replace")}</label>
      {#each preview as account}
        <label><input type="checkbox" bind:group={selected} value={account.serial} disabled={!replaceExisting&&accounts.some(a=>a.serial===account.serial)} />{account.label||account.serial}</label>
      {/each}
      {#if preview.length}<button disabled={!selected.length} on:click={importSelected}>{$t("Authenticator_Import")}</button>{/if}
    </fieldset>
    <fieldset disabled={busy||!!enrollment}>
      <legend>{$t("Authenticator_Restore")}</legend>
      <label>{$t("Authenticator_AccountIdentifier")}<input bind:value={accountIdentifier} autocomplete="off" /></label>
      <label>{$t("Authenticator_Serial")}<input bind:value={restoreSerial} /></label>
      <label>{$t("Authenticator_RestoreCode")}<input type="password" bind:value={restoreCode} autocomplete="off" /></label>
      <button disabled={!accountIdentifier||!restoreSerial||!restoreCode} on:click={restore}>{$t("Authenticator_Restore")}</button>
    </fieldset>
    {#if serial}
      <fieldset disabled={busy}>
        <legend>{$t("Authenticator_Export")}</legend>
        <label>{$t("Authenticator_Password")}<input type="password" bind:value={password} autocomplete="off" /></label>
        <label>{$t("Authenticator_ExportPassword")}<input type="password" bind:value={exportPassword} autocomplete="new-password" /></label>
        <SteamGuardVaultFactors idPrefix="battlenet-export" bind:backupKey bind:keyfilePath={keyfile} {busy} {pickKeyfile} onSubmit={() => { if (exportPassword) void exportSelected(); }} />
        <button disabled={!exportPassword} on:click={exportSelected}>{$t("Authenticator_Export")}</button>
      </fieldset>
    {/if}
  {/if}
</div>

<style>
  .authenticator { display: flex; flex: 0 1 auto; flex-direction: column; gap: 12px; width: 100%; min-width: 0; max-width: 40rem; margin: auto; color: var(--white, #fff); }
  :global(.modalFG:not(.modalFG--ready)) .authenticator { width: 34rem; }
  .authenticator :global(button) { margin: 0; }
  .app-password { gap: 0.35rem; padding: 0; text-align: left; }
  .app-password .modal-input { min-width: 0; margin: 0; }
  @media (max-width: 640px) { :global(.modalFG:not(.modalFG--ready)) .authenticator { width: 100%; } }
  label{display:grid;gap:4px}input,select{width:100%}input[type=checkbox]{width:auto}fieldset{display:grid;gap:10px} .auth-code{font-family:monospace;font-size:2.5rem;text-align:center;letter-spacing:.15em}progress{width:100%}
</style>

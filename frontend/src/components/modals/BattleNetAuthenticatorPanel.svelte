<script lang="ts">
  import { t } from "../../stores/i18n";
  import { focusOnShow } from "../../lib/actions/focusOnShow";
  import { battleNetAccountLabel, type BattleNetAccount, type BattleNetActions, type BattleNetForm, type BattleNetRecovery, type BattleNetScreen } from "../../lib/battleNetAuthenticator";
  import SteamGuardVaultFactors from "./SteamGuardVaultFactors.svelte";

  export let screen: BattleNetScreen;
  export let accounts: BattleNetAccount[] = [];
  export let serial = "";
  export let code = "";
  export let remaining = 0;
  export let accountLink = "";
  export let busy = false;
  export let error = "";
  export let enrollment = "";
  export let enrollmentState = "";
  export let preview: BattleNetAccount[] = [];
  export let recovery: BattleNetRecovery | null = null;
  export let form: BattleNetForm;
  export let actions: BattleNetActions;

  $: canPreview = !busy && !!form.source.trim();
  $: canRestore = !busy && !!form.accountIdentifier.trim() && !!form.restoreSerial.trim() && !!form.restoreCode.trim();
  $: canExport = !busy && !!form.exportPassword;
  $: canRename = !busy && !!form.accountLabel.trim();

  function previewImport(): void { if (canPreview) actions.previewImport(); }
  function restore(): void { if (canRestore) actions.restore(); }
  function exportSelected(): void { if (canExport) actions.exportSelected(); }
  function rename(): void { if (canRename) actions.rename(); }

  function runOnEnter(event: KeyboardEvent, action: () => void): void {
    if (event.key !== "Enter" || event.isComposing) return;
    event.preventDefault();
    action();
  }

  $: account = accounts.find(item => item.serial === serial);
  $: titleKey = {
    home: "Authenticator_Title", add: "Authenticator_Add", enroll: enrollmentState === "recovery-required" ? "Authenticator_RecoverResponse" : enrollmentState === "verification-required" ? "Authenticator_AccountSetupTitle" : "Authenticator_Enroll",
    import: "Authenticator_Import", restore: "Authenticator_Restore", export: "Authenticator_Export",
    recovery: "Authenticator_Recovery", rename: "Authenticator_SaveLabel",
  }[screen];
</script>

<section class="panel" aria-busy={busy}>
  {#if screen !== "home"}
    {#key screen}<h2 data-steamguard-focus tabindex="-1" use:focusOnShow>{$t(titleKey)}</h2>{/key}
  {/if}
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  {#if account?.requiresAccountSetup && (screen === "home" || screen === "recovery")}
    <p class="hint" role="status">{$t("Authenticator_AccountSetupNotice")}</p>
  {/if}
  {#if account && (!account.label || account.label === account.serial) && screen === "home"}
    <p class="hint">{$t("Authenticator_IdentifyAccount")}</p>
  {/if}

  {#if screen === "home"}
    {#if accounts.length}
      <div class="account-row">
        <label class="field account-select">
          <span>{$t("Authenticator_Account")}</span>
          <select class="modal-input" bind:value={serial} disabled={busy} on:change={actions.selectAccount}>
            {#each accounts as item}<option value={item.serial}>{battleNetAccountLabel(item)}</option>{/each}
          </select>
        </label>
        <button type="button" class="btnicontext" disabled={busy} on:click={() => actions.navigate("add")}>{$t("Authenticator_Add")}</button>
      </div>
      <button class="code" type="button" disabled={busy || !code} aria-label={$t("Authenticator_Copy")} title={$t("Authenticator_Copy")} aria-describedby="battlenet-code-expiry" on:click={actions.copyCode}>
        <span>{code || "••••••••"}</span>
        <span class="countdown" style:width={`${Math.min(30, Math.max(0, remaining)) / 30 * 100}%`} aria-hidden="true"></span>
      </button>
      <p id="battlenet-code-expiry" class="expiry">{code ? $t("Authenticator_Seconds", { seconds: remaining }) : $t("Authenticator_LoadingCode")}</p>
      {#if account?.pending}
        <div class="backup-prompt">
          <p>{$t("Authenticator_BackupNeeded")}</p>
          <button type="button" class="btnicontext modal-primary" disabled={busy} on:click={() => actions.navigate("recovery")}>{$t("Authenticator_Recovery")}</button>
        </div>
      {/if}
      <div class="actions primary-actions">
        <button type="button" class="btnicontext" disabled={busy} on:click={actions.browse}>{$t("Authenticator_Browse")}</button>
        {#if !account?.pending}<button type="button" class="btnicontext" disabled={busy} on:click={() => actions.navigate("recovery")}>{$t("Authenticator_Recovery")}</button>{/if}
        <button type="button" class="btnicontext" disabled={busy} on:click={() => actions.navigate("export")}>{$t("Authenticator_Export")}</button>
      </div>
      <div class="actions secondary-actions">
        <button type="button" class="fancyLink" disabled={busy} on:click={() => actions.navigate("rename")}>{$t("Authenticator_Rename")}</button>
        {#if accountLink && account?.accountLink !== accountLink}<button type="button" class="fancyLink" disabled={busy} on:click={actions.link}>{$t("Authenticator_LinkAccount")}</button>{/if}
      </div>
    {:else}
      <div class="empty">
        <h2>{$t("Authenticator_Empty")}</h2>
        <p class="hint">{$t("Authenticator_EmptyHint")}</p>
        <button type="button" class="btnicontext modal-primary" disabled={busy} on:click={() => actions.navigate("add")}>{$t("Authenticator_Add")}</button>
      </div>
    {/if}
  {:else if screen === "add"}
    <div class="choices">
      <button type="button" class="btnicontext choice modal-primary" disabled={busy} on:click={() => actions.navigate("enroll")}>
        <svg class="choice-icon" viewBox="0 0 24 24" aria-hidden="true"><rect x="3" y="4" width="18" height="16" rx="2" /><path d="M3 9h18M7 6.5h.01M10 6.5h.01" /></svg>
        <span class="choice-copy"><strong>{$t("Authenticator_Enroll")}</strong><span class="choice-hint">{$t("Authenticator_EnrollHint")}</span></span>
      </button>
      <button type="button" class="btnicontext choice" disabled={busy} on:click={() => actions.navigate("import")}>
        <svg class="choice-icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M14 3H6a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9l-6-6Zm0 0v6h6M12 12v6m-3-3 3 3 3-3" /></svg>
        <span class="choice-copy"><strong>{$t("Authenticator_Import")}</strong><span class="choice-hint">{$t("Authenticator_ImportHint")}</span></span>
      </button>
      <button type="button" class="btnicontext choice" disabled={busy} on:click={() => actions.navigate("restore")}>
        <svg class="choice-icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M3 4v5h5M3 9a9 9 0 1 1 0 6m9-8v5l3 2" /></svg>
        <span class="choice-copy"><strong>{$t("Authenticator_Restore")}</strong><span class="choice-hint">{$t("Authenticator_RestoreHint")}</span></span>
      </button>
    </div>
    <details class="recover-setup">
      <summary>{$t("Authenticator_Recover")}</summary>
      <p class="hint">{$t("Authenticator_RecoverHint")}</p>
      <button type="button" class="btnicontext" disabled={busy} on:click={actions.recover}>{$t("Authenticator_CheckRecovery")}</button>
    </details>
  {:else if screen === "enroll"}
    {#if !["verification-required", "recovery-required"].includes(enrollmentState)}<p class="hint">{$t("Authenticator_OnlineNotice")}</p>{/if}
    {#if enrollmentState === "recovery-required"}
      <p role="status">{$t("Authenticator_RecoveryRequired")}</p>
      <p class="hint">{$t("Authenticator_RecoveryRequiredHint")}</p>
      <div class="actions">
        <button type="button" class="btnicontext modal-primary" disabled={busy} on:click={actions.recover}>{$t("Authenticator_RecoverResponse")}</button>
        <button type="button" class="btnicontext" disabled={busy} on:click={actions.openEnrollmentAccount}>{$t("Authenticator_Browse")}</button>
      </div>
    {:else if enrollmentState === "verification-required"}
      <p role="status">{$t("Authenticator_AccountSetupRequired")}</p>
      <ol class="setup-steps">
        <li>{$t("Authenticator_AccountSetupStepQuestion")}</li>
        <li>{$t("Authenticator_AccountSetupStepVerification")}</li>
        <li>{$t("Authenticator_AccountSetupStepAuthenticator")}</li>
        <li>{$t("Authenticator_AccountSetupStepRetry")}</li>
      </ol>
      <p class="hint">{$t("Authenticator_AccountSetupHint")}</p>
      <div class="actions">
        <button type="button" class="btnicontext modal-primary" disabled={busy} on:click={actions.openEnrollmentAccount}>{$t("Authenticator_Browse")}</button>
        <button type="button" class="btnicontext" disabled={busy} on:click={actions.enroll}>{$t("Authenticator_RetrySetup")}</button>
      </div>
    {:else if enrollment}
      <p role="status">{$t(enrollmentState === "saving" ? "Authenticator_Saving" : "Authenticator_Waiting")}</p>
      <div class="actions"><button type="button" class="btnicontext" disabled={busy || enrollmentState === "saving"} on:click={actions.cancelEnrollment}>{$t("Authenticator_Cancel")}</button></div>
    {:else}
      <p class="hint">{$t("Authenticator_EnrollmentEffect")}</p>
      <ol class="setup-steps">
        <li>{$t("Authenticator_EnrollStepSignIn")}</li>
        <li>{$t("Authenticator_EnrollStepSetup")}</li>
        <li>{$t("Authenticator_EnrollStepBackup")}</li>
      </ol>
      <div class="actions"><button type="button" class="btnicontext modal-primary" disabled={busy} on:click={actions.enroll}>{$t("Authenticator_OpenBrowser")}</button></div>
    {/if}
  {:else if screen === "import"}
    <form class="stack" on:submit|preventDefault={previewImport}>
      <p class="hint">{$t("Authenticator_ImportHint")}</p>
      <label class="field"><span>{$t("Authenticator_Source")}</span><input class="modal-input" bind:value={form.source} disabled={busy} on:input={() => { preview = []; form.selected = []; }} on:keydown={(event) => runOnEnter(event, previewImport)} /></label>
      <div class="actions"><button type="button" class="btnicontext" disabled={busy} on:click={actions.pickImport}>{$t("Authenticator_ChooseFile")}</button></div>
      <label class="field"><span>{$t("Authenticator_SourcePassword")}</span><input class="modal-input" type="password" bind:value={form.sourcePassword} autocomplete="off" disabled={busy} on:input={() => { preview = []; form.selected = []; }} on:keydown={(event) => runOnEnter(event, previewImport)} /></label>
      <div class="actions"><button type="button" class="btnicontext" disabled={!canPreview} on:click={previewImport}>{$t("Authenticator_Preview")}</button></div>
      {#if preview.length}
        <fieldset disabled={busy} class="stack import-selection">
          <legend>{$t("Authenticator_SelectImport")}</legend>
          {#if preview.some(item => accounts.some(existing => existing.serial === item.serial))}
            <label class="check"><input type="checkbox" bind:checked={form.replaceExisting} on:change={() => { form.selected = []; }} /><span>{$t("Authenticator_Replace")}</span></label>
          {/if}
          {#each preview as item}
            <label class="check"><input type="checkbox" bind:group={form.selected} value={item.serial} disabled={!form.replaceExisting && accounts.some(existing => existing.serial === item.serial)} /><span>{item.label || item.serial}{#if accounts.some(existing => existing.serial === item.serial)} <small>{$t("Authenticator_AlreadySaved")}</small>{/if}</span></label>
          {/each}
          <div class="actions"><button type="button" class="btnicontext modal-primary" disabled={!form.selected.length} on:click={actions.importSelected}>{$t("Authenticator_ImportSelected")}</button></div>
        </fieldset>
      {/if}
    </form>
  {:else if screen === "restore"}
    <form class="stack" on:submit|preventDefault={restore}>
      <p class="hint">{$t("Authenticator_RestoreHint")} {$t("Authenticator_OnlineNotice")}</p>
      <label class="field"><span>{$t("Authenticator_AccountIdentifier")}</span><input class="modal-input" bind:value={form.accountIdentifier} autocomplete="off" disabled={busy} on:keydown={(event) => runOnEnter(event, restore)} /></label>
      <label class="field"><span>{$t("Authenticator_Serial")}</span><input class="modal-input" bind:value={form.restoreSerial} disabled={busy} on:keydown={(event) => runOnEnter(event, restore)} /></label>
      <label class="field"><span>{$t("Authenticator_RestoreCode")}</span><input class="modal-input" type="password" bind:value={form.restoreCode} autocomplete="off" disabled={busy} on:keydown={(event) => runOnEnter(event, restore)} /></label>
      <div class="actions"><button type="button" class="btnicontext modal-primary" aria-busy={busy} disabled={!canRestore} on:click={restore}>{$t(busy ? "Authenticator_Restoring" : "Authenticator_Restore")}</button></div>
    </form>
  {:else if screen === "export"}
    <form class="stack" on:submit|preventDefault={exportSelected}>
      <p class="hint">{$t("Authenticator_ExportHint", { account: account ? battleNetAccountLabel(account) : serial })}</p>
      <label class="field"><span>{$t("Authenticator_Password")}</span><input class="modal-input" type="password" bind:value={form.password} autocomplete="current-password" disabled={busy} on:keydown={(event) => runOnEnter(event, exportSelected)} /></label>
      <SteamGuardVaultFactors idPrefix="battlenet-export" bind:backupKey={form.backupKey} bind:keyfilePath={form.keyfile} {busy} pickKeyfile={actions.pickKeyfile} onSubmit={exportSelected} />
      <label class="field"><span>{$t("Authenticator_ExportPassword")}</span><input class="modal-input" type="password" bind:value={form.exportPassword} autocomplete="new-password" disabled={busy} on:keydown={(event) => runOnEnter(event, exportSelected)} /></label>
      <div class="actions"><button type="button" class="btnicontext modal-primary" disabled={!canExport} on:click={exportSelected}>{$t("Authenticator_Export")}</button></div>
    </form>
  {:else if screen === "recovery"}
    {#if recovery}
      <p class="hint">{$t("Authenticator_RecoveryWarning")}</p>
      <dl>
        <dt>{$t("Authenticator_Serial")}</dt><dd><input class="modal-input recovery-value" aria-label={$t("Authenticator_Serial")} title={$t("Authenticator_CopyValue")} readonly spellcheck="false" autocomplete="off" value={recovery.serial} disabled={busy} on:click={() => actions.copyRecovery("serial")} on:keydown={(event) => runOnEnter(event, () => actions.copyRecovery("serial"))} /></dd>
        {#if recovery.restoreCode}<dt>{$t("Authenticator_RestoreCode")}</dt><dd><input class="modal-input recovery-value" aria-label={$t("Authenticator_RestoreCode")} title={$t("Authenticator_CopyValue")} readonly spellcheck="false" autocomplete="off" value={recovery.restoreCode} disabled={busy} on:click={() => actions.copyRecovery("restoreCode")} on:keydown={(event) => runOnEnter(event, () => actions.copyRecovery("restoreCode"))} /></dd>{/if}
      </dl>
      <details><summary>{$t("Authenticator_ShowQR")}</summary><div class="qr"><img src={recovery.qrImage} alt={$t("Authenticator_QR")} width="240" height="240" /><textarea class="modal-input recovery-value" aria-label={$t("Authenticator_SecretLink")} title={$t("Authenticator_CopyValue")} readonly value={recovery.uri} disabled={busy} on:click={() => actions.copyRecovery("uri")} on:keydown={(event) => runOnEnter(event, () => actions.copyRecovery("uri"))}></textarea></div></details>
      {#if account?.pending}<div class="actions"><button type="button" class="btnicontext modal-primary" disabled={busy} on:click={actions.acknowledgeBackup}>{$t("Authenticator_BackupAcknowledged")}</button></div>{/if}
    {:else}
      <p>{$t("Authenticator_RecoveryHint")}</p>
      <div class="actions"><button type="button" class="btnicontext modal-primary" disabled={busy} on:click={actions.showRecovery}>{$t("Authenticator_ShowRecovery")}</button></div>
    {/if}
  {:else if screen === "rename"}
    <form class="stack" on:submit|preventDefault={rename}>
      <label class="field"><span>{$t("Authenticator_Label")}</span><input class="modal-input" bind:value={form.accountLabel} maxlength="200" disabled={busy} on:keydown={(event) => runOnEnter(event, rename)} /></label>
      <div class="actions"><button type="button" class="btnicontext modal-primary" disabled={!canRename} on:click={rename}>{$t("Authenticator_SaveLabel")}</button></div>
    </form>
  {/if}
</section>

<style>
  .panel, .stack { display: grid; gap: 0.75rem; min-width: 0; }
  h2, p { margin: 0; line-height: 1.45; }
  h2 { font-size: 1rem; font-weight: 650; }
  h2:focus { outline: none; }
  .hint, .expiry, small { color: var(--whiteSecondary, #d7d7d7); font-size: 0.85rem; }
  .setup-steps { display: grid; gap: 0.6rem; margin: 0; padding-inline-start: 1.5rem; line-height: 1.45; }
  .setup-steps li { padding-inline-start: 0.15rem; }
  .error { color: var(--role-error, #ff6b6b); overflow-wrap: anywhere; white-space: pre-line; }
  .field { display: grid; gap: 0.35rem; min-width: 0; width: 100%; padding: 0; text-align: left; }
  .field .modal-input, textarea.modal-input { width: 100%; min-width: 0; margin: 0; }
  .account-row { display: flex; align-items: flex-end; gap: 0.75rem; }
  .account-select { flex: 1; }
  .actions { display: flex; align-items: center; flex-wrap: wrap; gap: 0.5rem; }
  .panel :global(button) { margin: 0; }
  .actions button, .account-row button { display: inline-flex; align-items: center; justify-content: center; width: auto; min-height: 2.75rem; }
  .primary-actions { justify-content: center; }
  .secondary-actions { justify-content: center; gap: 1rem; }
  .secondary-actions button { padding: 0.35rem; }
  .code { position: relative; justify-self: center; width: min(18rem, 100%); margin-top: 0.5rem; padding: 1rem 1.25rem; border: 1px solid var(--modal-border, var(--border-bar-bg)); border-radius: 0.25rem; background: var(--role-tile-bg, rgb(255 255 255 / 8%)); color: inherit; font: 700 clamp(1.8rem, 7vw, 2.7rem) / 1 "Consolas", "Cascadia Mono", monospace; letter-spacing: 0.16em; text-align: center; cursor: pointer; }
  .code:hover:not(:disabled) { background: var(--role-dropdown-hover-bg, rgb(255 255 255 / 12%)); }
  .code:focus-visible { outline: 2px solid var(--role-focus-ring, var(--white, #fff)); outline-offset: 2px; }
  .countdown { position: absolute; left: 0; bottom: 0; height: 0.22rem; background: var(--accent, #66c0f4); transition: width 250ms linear; }
  .expiry { text-align: center; }
  .empty { display: grid; justify-items: center; gap: 0.75rem; padding: 1rem 0; text-align: center; }
  .choices { display: grid; gap: 0.5rem; }
  .choices .choice { display: flex; align-items: center; justify-content: flex-start; gap: 0.9rem; width: 100%; padding: 0.75rem 1rem; text-align: left; white-space: normal; }
  .choices .choice .choice-icon { flex: none; width: 1.8rem; height: 1.8rem; fill: none !important; stroke: currentColor; stroke-width: 1.75; stroke-linecap: round; stroke-linejoin: round; }
  .choice-copy { display: grid; gap: 0.25rem; min-width: 0; }
  .choice strong { font-weight: 600; }
  .choice-hint { font-size: 0.85rem; font-weight: 400; }
  summary { cursor: pointer; }
  .recover-setup { margin-top: 0.25rem; }
  .recover-setup p { margin: 0.5rem 0; }
  .backup-prompt { display: grid; justify-items: center; gap: 0.5rem; text-align: center; }
  .check { display: flex; align-items: center; gap: 0.5rem; padding: 0; }
  .check input { flex: none; width: auto; margin: 0; }
  .import-selection { min-width: 0; margin: 0; padding: 0.75rem; border: 1px solid var(--modal-border, var(--border-bar-bg)); }
  .import-selection legend { padding: 0 0.25rem; }
  dl { display: grid; grid-template-columns: auto minmax(0, 1fr); align-items: center; gap: 0.5rem 1rem; margin: 0; }
  dd { margin: 0; min-width: 0; }
  .recovery-value { width: 100%; min-width: 0; margin: 0; font-family: "Consolas", "Cascadia Mono", monospace; user-select: text; -webkit-user-select: text; cursor: copy; }
  .qr { display: grid; justify-items: center; gap: 0.75rem; margin-top: 0.75rem; }
  .qr img { max-width: 100%; height: auto; }
  @media (prefers-reduced-motion: reduce) { .countdown { transition: none; } }
</style>

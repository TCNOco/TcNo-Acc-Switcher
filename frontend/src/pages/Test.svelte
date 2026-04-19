<script lang="ts">
  import { get } from "svelte/store";
  import { route, previousPage, appBarTitle } from "../stores/nav";
  import "../styles/Settings.scss";
  import {
    openAlert,
    openAlertNoButton,
    openConfirm,
    openPrompt,
    openFolderPicker,
  } from "../stores/modal";
  import { t } from "../stores/i18n";
  import { pushToast } from "../stores/toast";

  /** Max delay safe for `setTimeout` in major browsers (~24.85 days). */
  const toastPermanentDurationMs = 2_147_483_647;

  let toastPermanent = false;

  function toastDuration(normalMs: number): number {
    return toastPermanent ? toastPermanentDurationMs : normalMs;
  }

  $: appBarTitle.set("TcNo Account Switcher - Test");
  previousPage.set({ page: "settings" });
  route.set({ page: "test" });

  let modalLog: string[] = [];

  function logModal(kind: string, detail: string) {
    const line = `${new Date().toLocaleTimeString()} — ${kind}: ${detail}`;
    modalLog = [line, ...modalLog].slice(0, 40);
  }

  async function testAlert() {
    const tt = get(t);
    await openAlert({
      title: tt("Preview_Modals"),
      body: tt("Modal_ConfirmAction"),
    });
    logModal("openAlert", "resolved");
  }

  async function testAlertNoButton() {
    const tt = get(t);
    await openAlertNoButton({
      title: tt("Preview_Modals"),
      body: tt("Modal_ConfirmAction"),
    });
    logModal("openAlertNoButton", "resolved");
  }

  async function testConfirmYesNo() {
    const tt = get(t);
    const r = await openConfirm({
      title: tt("Preview_Modals"),
      body: tt("Modal_ConfirmAction"),
      style: "yesno",
    });
    logModal("openConfirm (yes/no)", JSON.stringify(r));
  }

  async function testConfirmOkCancel() {
    const tt = get(t);
    const r = await openConfirm({
      title: tt("Preview_Modals"),
      body: tt("Modal_ConfirmAction"),
      style: "okcancel",
    });
    logModal("openConfirm (OK/cancel)", JSON.stringify(r));
  }

  async function testPromptText() {
    const tt = get(t);
    const r = await openPrompt({
      title: tt("Preview_Modals"),
      body: `${tt("Modal_ChangeUsername")}`,
      inputType: "text",
      initialValue: "demo",
    });
    logModal("openPrompt (text)", r === null ? "null (cancel)" : JSON.stringify(r));
  }

  async function testPromptPassword() {
    const tt = get(t);
    const r = await openPrompt({
      title: tt("Preview_Modals"),
      body: tt("Modal_SetPassword"),
      inputType: "password",
    });
    logModal("openPrompt (password)", r === null ? "null (cancel)" : `(length ${r.length})`);
  }

  async function testFolderPicker() {
    const tt = get(t);
    const r = await openFolderPicker({
      title: tt("Preview_Modals"),
      body: tt("Modal_SetUserdata"),
      initialPath: "",
    });
    logModal("openFolderPicker", r === null ? "null (cancel)" : JSON.stringify(r));
  }

  async function testFolderPickerWithFiles() {
    const tt = get(t);
    const r = await openFolderPicker({
      title: tt("Preview_Modals"),
      body: tt("Modal_SetBackground"),
      initialPath: "",
      dirsOnly: false,
      soughtFilename: "package.json",
      positiveLabel: tt("Modal_SetBackground_ChooseImage"),
    });
    logModal("openFolderPicker (dirsOnly: false)", r === null ? "null (cancel)" : JSON.stringify(r));
  }

  function toastSuccess() {
    pushToast({
      type: "success",
      title: "Saved",
      message: "Settings were applied (test toast).",
      duration: toastDuration(6000),
    });
  }

  function toastWarning() {
    pushToast({
      type: "warning",
      title: "Heads up",
      message: "Something may need your attention.",
      duration: toastDuration(8000),
    });
  }

  function toastError() {
    pushToast({
      type: "error",
      title: "",
      message: "A critical component could not be loaded. Please restart the application! (test)",
      duration: toastDuration(10000),
    });
  }

  function toastInfo() {
    pushToast({
      type: "info",
      title: "FYI",
      message: "This is an informational toast.",
      duration: toastDuration(5000),
    });
  }
  function toastViaWindowNotification() {
    window.notification?.new({
      type: "success",
      title: "window.notification",
      message: "Dispatched via window.notification.new (JS bridge style).",
      duration: toastDuration(5000),
    });
  }
</script>

<div class="main-content">
  <h1 class="SettingsHeader">{$t("Preview_Modals")}</h1>

  <div class="modalTestPanel">
    <pre class="modalTestOutput" aria-live="polite">{#if modalLog.length === 0}<span class="modalTestPlaceholder">{$t("Preview_Modals")} — run a test below.</span>{:else}{modalLog.join("\n")}{/if}</pre>
    <div class="modalTestButtons">
      <button type="button" class="btnicontext" on:click={() => void testAlert()}>Alert</button>
      <button type="button" class="btnicontext" on:click={() => void testAlertNoButton()}>Alert (no button)</button>
      <button type="button" class="btnicontext" on:click={() => void testConfirmYesNo()}>Confirm Yes/No</button>
      <button type="button" class="btnicontext" on:click={() => void testConfirmOkCancel()}>Confirm OK/Cancel</button>
      <button type="button" class="btnicontext" on:click={() => void testPromptText()}>Prompt (text)</button>
      <button type="button" class="btnicontext" on:click={() => void testPromptPassword()}>Prompt (password)</button>
      <button type="button" class="btnicontext" on:click={() => void testFolderPicker()}>Folder picker</button>
      <button type="button" class="btnicontext" on:click={() => void testFolderPickerWithFiles()}>Folder + files</button>
    </div>
  </div>

  <h2 class="SettingsHeader toastTestHeading">Toasts</h2>
  <div class="modalTestPanel">
    <label class="toastPermanentRow">
      <input type="checkbox" class="toastPermanentCheckbox" bind:checked={toastPermanent} />
      <span>Permanent</span>
      <span class="toastPermanentNote">(× to close)</span>
    </label>
    <div class="modalTestButtons">
      <button type="button" class="btnicontext" on:click={toastSuccess}>Toast success</button>
      <button type="button" class="btnicontext" on:click={toastWarning}>Toast warning</button>
      <button type="button" class="btnicontext" on:click={toastError}>Toast error</button>
      <button type="button" class="btnicontext" on:click={toastInfo}>Toast info</button>
      <button type="button" class="btnicontext" on:click={toastViaWindowNotification}>window.notification</button>
    </div>
  </div>
</div>

<style lang="scss">
  .toastTestHeading {
    margin-top: 0.5rem;
    margin-bottom: 0.5rem;
    font-size: 1.25rem;
  }
  .toastTestHint {
    margin: 0 0 0.65rem;
    font-size: 0.85rem;
    color: var(--blackTernary, #a7abbe);
    code {
      font-size: 0.8em;
      color: var(--cyan, #80ffea);
    }
  }
  .toastPermanentRow {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.4rem 0.6rem;
    margin: 0 0 0.75rem;
    cursor: pointer;
    user-select: none;
    font-size: 0.9rem;
    color: var(--whiteSecondary, #fff);
  }
  .toastPermanentCheckbox {
    opacity: 1;
    z-index: auto;
    width: 1.1rem;
    height: 1.1rem;
    margin: 0;
    cursor: pointer;
    accent-color: var(--accent, #80ffea);
  }
  .toastPermanentNote {
    flex: 1 1 100%;
    margin-left: 1.5rem;
    font-size: 0.78rem;
    color: var(--blackTernary, #a7abbe);
    font-weight: normal;
    cursor: pointer;
  }
  @media (min-width: 520px) {
    .toastPermanentNote {
      flex: 0 1 auto;
      margin-left: 0;
    }
  }
  .modalTestPanel {
    margin-bottom: 1.25rem;
  }
  .modalTestOutput {
    margin: 0 0 0.75rem;
    padding: 0.65rem 0.75rem;
    max-height: 11rem;
    overflow: auto;
    background: #070a0d;
    border: 1px solid var(--button-bg, #2c3e50);
    color: #e8f4ff;
    font-size: 11px;
    line-height: 1.45;
    white-space: pre-wrap;
    word-break: break-word;
  }
  .modalTestPlaceholder {
    opacity: 0.65;
  }
  .modalTestButtons {
    display: flex;
    flex-wrap: wrap;
    gap: 0.35rem;
    align-items: center;
  }
</style>

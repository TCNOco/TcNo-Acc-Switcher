<script lang="ts">
  import { get } from "svelte/store";
  import { route, previousPage, appBarTitle } from "../stores/nav";
  import "../styles/Settings.scss";
  import { openAlert, openConfirm, openPrompt, openFolderPicker } from "../stores/modal";
  import { t } from "../stores/i18n";

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
</script>

<div class="main-content">
  <h1 class="SettingsHeader">{$t("Preview_Modals")}</h1>

  <div class="modalTestPanel">
    <pre class="modalTestOutput" aria-live="polite">{#if modalLog.length === 0}<span class="modalTestPlaceholder">{$t("Preview_Modals")} — run a test below.</span>{:else}{modalLog.join("\n")}{/if}</pre>
    <div class="modalTestButtons">
      <button type="button" class="btnicontext" on:click={() => void testAlert()}>Alert</button>
      <button type="button" class="btnicontext" on:click={() => void testConfirmYesNo()}>Confirm Yes/No</button>
      <button type="button" class="btnicontext" on:click={() => void testConfirmOkCancel()}>Confirm OK/Cancel</button>
      <button type="button" class="btnicontext" on:click={() => void testPromptText()}>Prompt (text)</button>
      <button type="button" class="btnicontext" on:click={() => void testPromptPassword()}>Prompt (password)</button>
      <button type="button" class="btnicontext" on:click={() => void testFolderPicker()}>Folder picker</button>
      <button type="button" class="btnicontext" on:click={() => void testFolderPickerWithFiles()}>Folder + files</button>
    </div>
  </div>
</div>

<style lang="scss">
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

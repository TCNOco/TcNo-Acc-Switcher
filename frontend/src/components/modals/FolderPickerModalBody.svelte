<script lang="ts">
  import { tick, onMount } from "svelte";
  import { createEventDispatcher } from "svelte";
  import type { Component } from "svelte";
  import ModalBodyShell from "./ModalBodyShell.svelte";
  import PathPickerTree from "./PathPickerTree.svelte";
  import { t } from "../../stores/i18n";
  import { FOLDER_PICKER_APPDATA, FOLDER_PICKER_PORTABLE } from "../../stores/modal";
  import * as FilesystemService from "../../../bindings/TcNo-Acc-Switcher/filesystemservice";
  import { normalizeDisplayPath, stripSurroundingQuotes } from "../../lib/fsPaths";
  import { tooltip } from "../../lib/actions/tooltip";

  export let html: string | undefined = undefined;
  export let component: Component<any> | undefined = undefined;
  export let componentProps: Record<string, unknown> | undefined = undefined;
  export let initialPath = "";
  export let dirsOnly = true;
  export let soughtFilename = "";
  export let suggestedFolder = "";
  export let positiveLabel = "";
  export let showPortableButton = false;

  const dispatch = createEventDispatcher<{ resolve: string | null }>();

  let path = normalizeDisplayPath(initialPath);
  let statSeq = 0;
  let pathStat: { exists: boolean; isDir: boolean } | null = null;

  $: enteredPath = stripSurroundingQuotes(path);

  $: if (!enteredPath) {
    statSeq++;
    pathStat = null;
  }

  $: if (enteredPath) {
    const p = enteredPath;
    const seq = ++statSeq;
    void FilesystemService.StatPath(p).then((st: { exists: boolean; isDir: boolean }) => {
      if (seq === statSeq) {
        pathStat = { exists: st.exists, isDir: st.isDir };
      }
    });
  }

  $: primaryDisabled = (() => {
    if (!enteredPath) return true;
    if (!pathStat) return true;
    if (dirsOnly) {
      return !pathStat.exists || !pathStat.isDir;
    }
    return !pathStat.exists || pathStat.isDir;
  })();

  // The indicator turns green only when Select is usable: matching the name
  // alone lets a path that no longer exists, or a folder, read as "found".
  $: soughtMismatch =
    !!soughtFilename.trim() &&
    (primaryDisabled ||
      !enteredPath.toLowerCase().includes(soughtFilename.trim().toLowerCase()));

  let inputEl: HTMLInputElement | undefined;
  $: pathInputLabel = dirsOnly ? $t("Modal_SetUserdata_ChooseFolder") : positiveLabel;
  $: soughtFileId = "folder-picker-sought-file";

  function ok(): void {
    if (primaryDisabled) return;
    const v = normalizeDisplayPath(enteredPath);
    if (!v) return;
    dispatch("resolve", v);
  }

  function portable(): void {
    dispatch("resolve", FOLDER_PICKER_PORTABLE);
  }

  function appData(): void {
    dispatch("resolve", FOLDER_PICKER_APPDATA);
  }

  function pickTreePath(p: string, _entryIsDir: boolean): void {
    path = normalizeDisplayPath(p);
  }

  onMount(() => {
    void tick().then(() =>
      requestAnimationFrame(() => {
        inputEl?.focus();
        inputEl?.select?.();
      }),
    );
  });
</script>

<div class="modal-block">
  <ModalBodyShell
    {html}
    {component}
    {componentProps}
  />
  <div class="modal-input-row">
    <input
      bind:this={inputEl}
      bind:value={path}
      type="text"
      class="modal-input"
      spellcheck="false"
      autocomplete="off"
      aria-label={pathInputLabel}
      aria-describedby={soughtFilename.trim() ? soughtFileId : undefined}
      aria-invalid={soughtMismatch ? "true" : undefined}
      on:keydown={(e) => e.key === "Enter" && !primaryDisabled && ok()}
    />
  </div>
  {#if soughtFilename.trim()}
    <div
      id={soughtFileId}
      class="folder_indicator_stack"
      class:indicator-warn={soughtMismatch}
      role="status"
      aria-live="polite"
      aria-label={soughtMismatch ? `${soughtFilename.trim()} ${$t("NotFound")}` : soughtFilename.trim()}
    >
      <div
        class="folder_indicator"
        class:notfound={soughtMismatch}
        aria-hidden="true"
      >
        <div class="folder_indicator_text"></div>
      </div>
      <div class="folder_indicator_bg" class:notfound={soughtMismatch}>
        <span>{soughtFilename.trim()}</span>
      </div>
    </div>
  {/if}
  <div class="modal-inline-actions settingsCol inputAndButton">
    <span class="modal-actions-spacer"></span>
    {#if showPortableButton}
      <button
        type="button"
        class="btnicontext"
        on:click={appData}
      >
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M2 5.5A1.5 1.5 0 0 1 3.5 4h5.2a1.5 1.5 0 0 1 1.06.44L12 6.5h8.5A1.5 1.5 0 0 1 22 8v10.5a1.5 1.5 0 0 1-1.5 1.5h-17A1.5 1.5 0 0 1 2 18.5Z"/></svg>
        AppData
      </button>
      <button
        type="button"
        class="btnicontext"
        use:tooltip={$t("Tooltip_PortableMode")}
        on:click={portable}
      >
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path fill-rule="evenodd" d="M2 3.5A1.5 1.5 0 0 1 3.5 2h17A1.5 1.5 0 0 1 22 3.5v3A1.5 1.5 0 0 1 20.5 8h-17A1.5 1.5 0 0 1 2 6.5Zm1.5 6.1h17v10A2.4 2.4 0 0 1 18.1 22H5.9a2.4 2.4 0 0 1-2.4-2.4Zm5.7 2.8h5.6v2.4H9.2Z"/></svg>
        {$t("Button_PortableMode")}
      </button>
    {/if}
    <button
      type="button"
      class="btnicontext modal-primary"
      disabled={primaryDisabled}
      on:click={ok}
    >
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M3.2 12.0 5.6 9.6l3.8 3.8L18.4 4.4l2.4 2.4L9.4 18.2z"/></svg>
      {positiveLabel}
    </button>
  </div>
  <PathPickerTree
    selectedPath={enteredPath}
    {dirsOnly}
    soughtFilename={soughtFilename.trim()}
    suggestedFolder={suggestedFolder.trim()}
    onPick={pickTreePath}
  />
</div>

<style lang="scss">
  .folder_indicator_stack {
    display: flex;
    flex-direction: row;
    gap: 0;
    width: 100%;
    margin: 0.15rem 0 0;
  }

  .folder_indicator {
    min-width: 0.5rem;
    margin-right: 0.25rem;
    border-radius: 2px 2px 0 0;
    background: var(--success-solid);
    transition: background 0.15s ease;
  }

  .folder_indicator.notfound {
    background: var(--error-solid);
  }

  .folder_indicator_text {
    min-height: 0;
  }

  .folder_indicator_bg {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 0.35rem 1rem 0.35rem 0.75rem;
    background: var(--success-soft-bg);
    color: var(--white);
    font-size: 12px;
    border-radius: 0 0 2px 2px;
    transition:
      color 0.15s ease,
      background 0.15s ease;
  }

  .folder_indicator_bg.notfound span {
    color: var(--white);
  }

  .indicator-warn .folder_indicator_bg {
    background: var(--error-soft-bg);
  }

  :global(.modal-pathPicker.pathPicker) {
    min-width: 0;
    width: 100%;
    max-width: none;
    margin: 0.35rem 0 0;
    min-height: 280px;
    max-height: calc(75vh - 11rem);
  }
</style>

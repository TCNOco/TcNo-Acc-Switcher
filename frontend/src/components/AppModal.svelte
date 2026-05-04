<script lang="ts">
  import { tick, onMount } from "svelte";
  import { get } from "svelte/store";
  import { activeModal, dismissModal, cancelActiveModal } from "../stores/modal";
  import { t } from "../stores/i18n";
  import PathPickerTree from "./modals/PathPickerTree.svelte";
  import ModalBodyShell from "./modals/ModalBodyShell.svelte";
  import { normalizeDisplayPath } from "../lib/fsPaths";
  import * as FilesystemService from "../../bindings/TcNo-Acc-Switcher/filesystemservice";

  $: m = $activeModal;

  let folderPath = "";
  let promptValue = "";
  let modalReady = false;

  let lastModalSyncId = -1;
  let folderStatSeq = 0;
  let folderPathStat: { exists: boolean; isDir: boolean } | null = null;

  $: if (m?.id !== undefined && m.id !== lastModalSyncId) {
    lastModalSyncId = m.id;
    modalReady = false;
    folderPathStat = null;
    if (m.kind === "prompt") {
      promptValue = m.initialValue ?? "";
      void tick().then(() => {
        promptEl?.focus();
        promptEl?.select?.();
      });
    }
    if (m.kind === "folder") {
      folderPath = normalizeDisplayPath(m.initialPath ?? "");
    }
    void tick().then(() =>
      requestAnimationFrame(() => {
        if (get(activeModal)?.id === m.id) {
          modalReady = true;
        }
      }),
    );
  }

  $: if (!m) {
    lastModalSyncId = -1;
    modalReady = false;
    folderStatSeq++;
    folderPathStat = null;
  }

  $: if (!m || m.kind !== "folder" || !folderPath.trim()) {
    if (m?.kind === "folder" && !folderPath.trim()) {
      folderStatSeq++;
    }
    folderPathStat = null;
  }

  $: if (m?.kind === "folder" && folderPath.trim()) {
    const p = folderPath.trim();
    const seq = ++folderStatSeq;
    void FilesystemService.StatPath(p).then((st: { exists: boolean; isDir: boolean }) => {
      if (seq === folderStatSeq) {
        folderPathStat = { exists: st.exists, isDir: st.isDir };
      }
    });
  }

  $: soughtMismatch =
    m?.kind === "folder" &&
    !!m.soughtFilename?.trim() &&
    !folderPath.toLowerCase().includes(m.soughtFilename.trim().toLowerCase());

  $: primaryFolderDisabled = (() => {
    if (!m || m.kind !== "folder") return true;
    const v = folderPath.trim();
    if (!v) return true;
    if (!folderPathStat) return true;
    const dirsOnly = m.dirsOnly ?? true;
    if (dirsOnly) {
      return !folderPathStat.exists || !folderPathStat.isDir;
    }
    return !folderPathStat.exists || folderPathStat.isDir;
  })();

  let backdropEl: HTMLDivElement | undefined;
  let promptEl: HTMLInputElement | HTMLTextAreaElement | undefined;

  function onBackdropDown(e: MouseEvent): void {
    if (e.target === backdropEl) cancelActiveModal();
  }

  function onKeydown(e: KeyboardEvent): void {
    if (e.key === "Escape" && get(activeModal)) {
      e.preventDefault();
      cancelActiveModal();
    }
  }

  onMount(() => {
    window.addEventListener("keydown", onKeydown);
    return () => window.removeEventListener("keydown", onKeydown);
  });

  function confirmPositive(): void {
    if (!m || m.kind !== "confirm") return;
    dismissModal(true);
  }

  function confirmNegative(): void {
    if (!m || m.kind !== "confirm") return;
    dismissModal(false);
  }

  function promptOk(): void {
    if (!m || m.kind !== "prompt") return;
    dismissModal(promptValue);
  }

  function folderOk(): void {
    if (!m || m.kind !== "folder") return;
    if (primaryFolderDisabled) return;
    const v = normalizeDisplayPath(folderPath.trim());
    if (!v) return;
    dismissModal(v);
  }

  function pickTreePath(p: string, _entryIsDir: boolean): void {
    folderPath = normalizeDisplayPath(p);
  }
</script>

{#if m}
  <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
  <div
    class="modalBG"
    bind:this={backdropEl}
    on:mousedown={onBackdropDown}
    role="presentation"
  >
    <div
      class="modalFG"
      class:modalFG--ready={modalReady}
      role="dialog"
      aria-modal="true"
      aria-labelledby="modal-title-label"
      on:mousedown|stopPropagation
    >
      <header class="modal-headerbar">
        <span class="modal-title-left">
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
        <span id="modal-title-label" class="modal-title-drag">{m.title}</span>
        <span class="modal-window-controls" role="toolbar">
          <button
            type="button"
            class="win-btn win-btn-close"
            aria-label={$t("Button_Close")}
            on:click={() => cancelActiveModal()}
          >
            <img
              class="icon"
              srcset="img/icons/close-w-10.webp 1x, img/icons/close-w-12.webp 1.25x, img/icons/close-w-15.webp 1.5x, img/icons/close-w-15.webp 1.75x, img/icons/close-w-20.webp 2x, img/icons/close-w-20.webp 2.25x, img/icons/close-w-24.webp 2.5x, img/icons/close-w-30.webp 3x, img/icons/close-w-30.webp 3.5x"
              draggable="false"
              alt=""
            />
          </button>
        </span>
      </header>

      <div class="modal-scroll">
        {#key m.id}
          {#if m.kind === "alert" || m.kind === "alertNoButton"}
            <div class="modal-block">
              <ModalBodyShell
                html={m.body}
                component={m.bodyComponent}
                componentProps={m.bodyProps}
              />
              {#if m.kind === "alert"}
                <div class="modal-inline-actions settingsCol inputAndButton">
                  <span class="modal-actions-spacer"></span>
                  <button type="button" class="btnicontext" on:click={() => dismissModal()}>
                    {m.dismissLabel ?? $t("Ok")}
                  </button>
                </div>
              {/if}
            </div>
          {:else if m.kind === "confirm"}
            <div class="modal-block">
              <ModalBodyShell
                html={m.body}
                component={m.bodyComponent}
                componentProps={m.bodyProps}
              />
              <div class="modal-inline-actions settingsCol inputAndButton">
                <span class="modal-actions-spacer"></span>
                <button type="button" class="btnicontext" on:click={confirmPositive}>
                  {#if m.style === "yesno"}
                    {m.positiveLabel ?? $t("Yes")}
                  {:else}
                    {m.positiveLabel ?? $t("Ok")}
                  {/if}
                </button>
                {#if m.style === "yesno"}
                  <button type="button" class="btnicontext" on:click={confirmNegative}>
                    {m.negativeLabel ?? $t("No")}
                  </button>
                {/if}
              </div>
            </div>
          {:else if m.kind === "prompt"}
            <div class="modal-block">
              <ModalBodyShell
                html={m.body}
                component={m.bodyComponent}
                componentProps={m.bodyProps}
              />
              <div class="modal-input-row">
                {#if m.inputType === "password"}
                  <input
                    bind:this={promptEl}
                    bind:value={promptValue}
                    type="password"
                    class="modal-input"
                    autocomplete="off"
                    on:keydown={(e) => e.key === "Enter" && promptOk()}
                  />
                {:else if m.multiline}
                  <textarea
                    bind:this={promptEl}
                    bind:value={promptValue}
                    class="modal-input modal-input--multiline"
                    rows="6"
                    spellcheck="true"
                    autocomplete="off"
                  ></textarea>
                {:else}
                  <input
                    bind:this={promptEl}
                    bind:value={promptValue}
                    type="text"
                    class="modal-input"
                    autocomplete="off"
                    on:keydown={(e) => e.key === "Enter" && promptOk()}
                  />
                {/if}
              </div>
              <div class="modal-inline-actions settingsCol inputAndButton">
                <span class="modal-actions-spacer"></span>
                <button type="button" class="btnicontext" on:click={promptOk}>
                  {m.positiveLabel ?? $t("Ok")}
                </button>
              </div>
            </div>
          {:else if m.kind === "folder"}
            <div class="modal-block">
              <ModalBodyShell
                html={m.body}
                component={m.bodyComponent}
                componentProps={m.bodyProps}
              />
              <div class="modal-input-row">
                <input
                  bind:value={folderPath}
                  type="text"
                  class="modal-input"
                  spellcheck="false"
                  autocomplete="off"
                  on:keydown={(e) => e.key === "Enter" && !primaryFolderDisabled && folderOk()}
                />
              </div>
              {#if m.soughtFilename?.trim()}
                <div class="folder_indicator_stack" class:indicator-warn={soughtMismatch}>
                  <div
                    class="folder_indicator"
                    class:notfound={soughtMismatch}
                    aria-hidden="true"
                  >
                    <div class="folder_indicator_text"></div>
                  </div>
                  <div class="folder_indicator_bg" class:notfound={soughtMismatch}>
                    <span>{m.soughtFilename.trim()}</span>
                  </div>
                </div>
              {/if}
              <div class="modal-inline-actions settingsCol inputAndButton">
                <span class="modal-actions-spacer"></span>
                <button
                  type="button"
                  class="btnicontext modal-pick-primary"
                  disabled={primaryFolderDisabled}
                  on:click={folderOk}
                >
                  {m.positiveLabel ??
                    (!(m.dirsOnly ?? true)
                      ? $t("Modal_Button_Select")
                      : $t("Modal_SetUserdata_ChooseFolder"))}
                </button>
              </div>
              <PathPickerTree
                selectedPath={folderPath}
                dirsOnly={m.dirsOnly ?? true}
                soughtFilename={m.soughtFilename?.trim() ?? ""}
                onPick={pickTreePath}
              />
            </div>
          {/if}
        {/key}
      </div>
    </div>
  </div>
{/if}

<style lang="scss">
  .modalBG {
    position: absolute;
    inset: 0;
    z-index: 50;
    background: rgba(0, 0, 0, 0.55);
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 1rem;
    box-sizing: border-box;
  }

  .modalFG {
    display: flex;
    flex-direction: column;
    width: min(720px, 100%);
    max-height: min(900px, calc(100% - 2rem));
    background: var(--program-bg, #1b2636);
    border: var(--border-bar-size, 1px) solid var(--border-bar-bg, #3b4853);
    box-shadow: 0 12px 40px rgba(0, 0, 0, 0.45);
    overflow: hidden;
    visibility: hidden;
    opacity: 0;
    transform: translateY(4px);
    transition:
      opacity 0.1s ease,
      transform 0.1s ease;
  }

  .modalFG.modalFG--ready {
    visibility: visible;
    opacity: 1;
    transform: translateY(0);
  }

  .modal-headerbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    height: 32px;
    min-height: 32px;
    background: var(--border-bar-bg, #3b4853);
    color: #fff;
    flex-shrink: 0;
    user-select: none;
  }

  .modal-title-left {
    display: flex;
    align-items: center;
    height: 100%;
  }

  .modal-headerbar .header_icon {
    height: 10px;
    margin: 0 12px;
    display: block;
    fill: white;
  }

  .modal-title-drag {
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

  .modal-window-controls {
    display: flex;
    height: 100%;
  }

  .modal-window-controls .win-btn {
    border-radius: 0;
    background: none;
    border: 0;
    margin: 0;
    display: flex;
    justify-content: center;
    align-items: center;
    width: 46px;
    height: 100%;
    cursor: pointer;
    padding: 0;
    &:hover {
      background: #3b4853;
    }
  }

  .modal-window-controls .win-btn-close:hover {
    background: #d51426;
  }

  .modal-scroll {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding: 1.25rem 1.5rem;
  }

  .modal-block {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }

  .modal-input-row {
    display: flex;
    width: 100%;
  }

  .modal-input {
    flex: 1;
    padding: 8px;
    margin: 0;
    width: 100%;
    box-sizing: border-box;
    background: #070a0d;
    border: 1px solid var(--button-bg, #2c3e50);
    color: #fff;
    font: inherit;
    &:focus {
      outline: 1px solid var(--accent);
      outline-offset: -1px;
      border-color: var(--accent);
    }
  }

  .modal-input--multiline {
    min-height: 7.5rem;
    resize: vertical;
    line-height: 1.35;
    white-space: pre-wrap;
  }

  .modal-inline-actions {
    display: flex;
    flex-direction: row;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.35rem;
    width: 100%;
    margin-top: 0.15rem;

    button {
      min-width: 7.5rem;
    }
  }

  .modal-actions-spacer {
    flex: 1;
    min-width: 0;
  }

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
    background: #090;
    transition: background 0.15s ease;
  }

  .folder_indicator.notfound {
    background: #F00;
  }

  .folder_indicator_text {
    min-height: 0;
  }

  .folder_indicator_bg {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 0.35rem 1rem 0.35rem 0.75rem;
    background: #00990022;
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
    background: #FF000033;
  }

  :global(.modal-pathPicker.pathPicker) {
    min-width: 0;
    width: 100%;
    max-width: none;
    margin: 0.35rem 0 0;
  }

  .modal-inline-actions .btnicontext:disabled {
    opacity: 0.45;
    cursor: not-allowed;
    pointer-events: none;
  }
</style>

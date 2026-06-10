<script lang="ts">
  import { tick, onMount } from "svelte";
  import { get } from "svelte/store";
  import { fade, scale } from "svelte/transition";
  import { cubicOut } from "svelte/easing";
  import { Browser } from "@wailsio/runtime";
  import { activeModal, dismissModal, cancelActiveModal, type CrashReportChoice } from "../stores/modal";
  import { DUR, motionEnabled } from "../lib/animation";
  import { t } from "../stores/i18n";
  import { offlineMode } from "../stores/offlineMode";
  import { pushToast } from "../stores/toast";
  import PathPickerTree from "./modals/PathPickerTree.svelte";
  import ModalBodyShell from "./modals/ModalBodyShell.svelte";
  import UpdateModalBody from "./modals/UpdateModalBody.svelte";
  import { normalizeDisplayPath } from "../lib/fsPaths";
  import { formatToastWithError } from "../lib/formatWailsError";
  import { tooltip } from "../lib/actions/tooltip";
  import { FOLDER_PICKER_APPDATA, FOLDER_PICKER_PORTABLE } from "../stores/modal";
  import * as FilesystemService from "../../bindings/TcNo-Acc-Switcher/filesystemservice";
  import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
  import {
    MODAL_FRAME_MIN_H,
    MODAL_FRAME_MIN_W,
    MODAL_FRAME_MIN_W_FOLDER,
    MODAL_RESIZE_EDGES,
    MODAL_RESIZE_CURSOR,
    centerFrame,
    clampRect,
    getModalBounds,
    startModalDrag,
    startModalResize,
    type ModalFrameRect,
    type ResizeEdge,
  } from "../lib/modalFrame";

  const DISCORD_URL = "https://s.tcno.co/AccSwitcherDiscord";
  const FEEDBACK_MAX_LENGTH = 2000;

  $: m = $activeModal;

  let folderPath = "";
  let promptValue = "";
  let feedbackValue = "";

  let lastModalSyncId = -1;
  let folderStatSeq = 0;
  let folderPathStat: { exists: boolean; isDir: boolean } | null = null;

  $: modalMinSize = {
    minW: m?.kind === "folder" ? MODAL_FRAME_MIN_W_FOLDER : MODAL_FRAME_MIN_W,
    minH: MODAL_FRAME_MIN_H,
  };

  $: if (m?.id !== undefined && m.id !== lastModalSyncId) {
    lastModalSyncId = m.id;
    frameReady = false;
    folderPathStat = null;
    if (m.kind === "prompt") {
      promptValue = m.initialValue ?? "";
    }
    if (m.kind === "feedback") {
      feedbackValue = "";
    }
    if (m.kind === "crashReport") {
      void tick().then(() =>
        requestAnimationFrame(() => {
          crashReportYesEl?.focus();
        }),
      );
    }
    if (m.kind === "folder") {
      folderPath = normalizeDisplayPath(m.initialPath ?? "");
    }
    void tick().then(() =>
      requestAnimationFrame(() => {
        if (get(activeModal)?.id === m.id) {
          initModalFrame(m.id);
          focusAndSelectActiveInput();
        }
      }),
    );
  }

  $: if (!m) {
    lastModalSyncId = -1;
    frameReady = false;
    folderPathStat = null;
  }

  function initModalFrame(expectedId: number): void {
    if (!backdropEl || !modalFgEl || get(activeModal)?.id !== expectedId) return;
    const bounds = getModalBounds(backdropEl);
    const naturalW = modalFgEl.offsetWidth;
    const naturalH = modalFgEl.offsetHeight;
    modalFrame = centerFrame(naturalW, naturalH, bounds, modalMinSize);
    frameReady = true;
  }

  function reclampModalFrame(): void {
    if (!frameReady || !backdropEl) return;
    modalFrame = clampRect(modalFrame, getModalBounds(backdropEl), modalMinSize);
  }

  function onHeaderPointerDown(e: PointerEvent): void {
    if (!frameReady || !headerEl || !backdropEl) return;
    startModalDrag(e, headerEl, modalFrame, {
      bounds: getModalBounds(backdropEl),
      minSize: modalMinSize,
      onUpdate: (rect) => {
        modalFrame = rect;
      },
    });
  }

  function onResizePointerDown(e: PointerEvent, edge: ResizeEdge): void {
    if (!frameReady || !backdropEl) return;
    const handle = e.currentTarget;
    if (!(handle instanceof HTMLElement)) return;
    startModalResize(e, handle, edge, modalFrame, {
      bounds: getModalBounds(backdropEl),
      minSize: modalMinSize,
      onUpdate: (rect) => {
        modalFrame = rect;
      },
    });
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
  let modalFgEl: HTMLDivElement | undefined;
  let headerEl: HTMLElement | undefined;
  let modalFrame: ModalFrameRect = { left: 0, top: 0, width: 0, height: 0 };
  let frameReady = false;

  let promptEl: HTMLInputElement | HTMLTextAreaElement | undefined;
  let feedbackEl: HTMLTextAreaElement | undefined;
  let folderInputEl: HTMLInputElement | undefined;
  let crashReportYesEl: HTMLButtonElement | undefined;

  $: feedbackSubmitDisabled = feedbackValue.trim().length === 0;

  function focusAndSelectActiveInput(): void {
    if (!m) return;
    if (m.kind === "prompt") {
      promptEl?.focus();
      promptEl?.select?.();
      return;
    }
    if (m.kind === "feedback") {
      feedbackEl?.focus();
      return;
    }
    if (m.kind === "folder") {
      folderInputEl?.focus();
      folderInputEl?.select?.();
    }
  }

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
    window.addEventListener("resize", reclampModalFrame);
    return () => {
      window.removeEventListener("keydown", onKeydown);
      window.removeEventListener("resize", reclampModalFrame);
    };
  });

  function confirmPositive(): void {
    if (!m || m.kind !== "confirm") return;
    dismissModal(true);
  }

  function confirmNegative(): void {
    if (!m || m.kind !== "confirm") return;
    dismissModal(false);
  }

  function crashReportChoice(choice: CrashReportChoice): void {
    if (!m || m.kind !== "crashReport") return;
    dismissModal(choice);
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

  function folderPortable(): void {
    if (!m || m.kind !== "folder") return;
    dismissModal(FOLDER_PICKER_PORTABLE);
  }

  function folderAppData(): void {
    if (!m || m.kind !== "folder") return;
    dismissModal(FOLDER_PICKER_APPDATA);
  }

  function pickTreePath(p: string, _entryIsDir: boolean): void {
    folderPath = normalizeDisplayPath(p);
  }

  async function feedbackSubmit(): Promise<void> {
    if (!m || m.kind !== "feedback" || feedbackSubmitDisabled) return;
    const text = feedbackValue.trim();
    const kind = m.mode === "issue" ? "switch_issue" : "feature_suggestion";
    try {
      await PlatformService.SubmitFeedback(kind, m.platform ?? "", text);
      dismissModal(text);
      pushToast({
        type: "success",
        message: get(t)("Toast_FeedbackThanks"),
        duration: 4000,
      });
    } catch (e) {
      pushToast({
        type: "error",
        message: formatToastWithError(get(t)("Toast_FeedbackSubmitFailed"), e),
        duration: 8000,
      });
    }
  }

  function openDiscordLink(e: MouseEvent): void {
    e.preventDefault();
    if (get(offlineMode)) {
      pushToast({
        type: "info",
        message: get(t)("Toast_OfflineModeNoLinks"),
        duration: 5000,
      });
      return;
    }
    void Browser.OpenURL(DISCORD_URL);
  }
</script>

{#if m}
  <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
  <div
    class="modalBG"
    bind:this={backdropEl}
    on:mousedown={onBackdropDown}
    role="presentation"
    in:fade={{ duration: motionEnabled() ? DUR.fast : 0, easing: cubicOut }}
    out:fade={{ duration: motionEnabled() ? DUR.fast : 0, easing: cubicOut }}
  >
    <div
      class="modalFG"
      class:modalFG--ready={frameReady}
      class:modalFilePicker={m.kind === "folder"}
      bind:this={modalFgEl}
      role="dialog"
      aria-modal="true"
      aria-labelledby="modal-title-label"
      style={frameReady
        ? `left:${modalFrame.left}px;top:${modalFrame.top}px;width:${modalFrame.width}px;height:${modalFrame.height}px`
        : undefined}
      in:scale={{ start: 0.96, duration: motionEnabled() ? DUR.normal : 0, easing: cubicOut, delay: motionEnabled() ? 20 : 0 }}
      out:scale={{ start: 0.96, duration: motionEnabled() ? DUR.fast : 0, easing: cubicOut }}
    >
      {#each MODAL_RESIZE_EDGES as edge (edge)}
        <div
          class="modal-resize-handle modal-resize-{edge}"
          style:cursor={MODAL_RESIZE_CURSOR[edge]}
          role="presentation"
          on:pointerdown={(e) => onResizePointerDown(e, edge)}
        ></div>
      {/each}
      <div class="modalFG-inner">
      <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
      <header class="modal-headerbar" bind:this={headerEl} on:pointerdown={onHeaderPointerDown}>
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
        <span id="modal-title-label" class="modal-title-drag">
          {#if m.kind === "update"}
            {$t("Heading_UpdateAvailable")}
          {:else if m.kind === "feedback"}
            {m.mode === "issue" ? $t("Feedback_Issue_Title") : $t("Feedback_Suggestion_Title")}
          {:else if m.kind === "crashReport"}
            {$t("Modal_CrashReport_Title")}
          {:else}
            {m.title}
          {/if}
        </span>
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
                  <button type="button" class="btnicontext modal-primary" on:click={() => dismissModal()}>
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
                <button type="button" class="btnicontext modal-primary" on:click={confirmPositive}>
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
                <button type="button" class="btnicontext modal-primary" on:click={promptOk}>
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
                  bind:this={folderInputEl}
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
                {#if m.showPortableButton}
                  <button
                    type="button"
                    class="btnicontext"
                    on:click={folderAppData}
                  >
                    AppData
                  </button>
                  <button
                    type="button"
                    class="btnicontext"
                    use:tooltip={$t("Tooltip_PortableMode")}
                    on:click={folderPortable}
                  >
                    {$t("Button_PortableMode")}
                  </button>
                {/if}
                <button
                  type="button"
                  class="btnicontext modal-primary"
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
          {:else if m.kind === "feedback"}
            <div class="modal-block">
              <p class="modal-feedback-body">
                {m.mode === "issue" ? $t("Feedback_Issue_Body") : $t("Feedback_Suggestion_Body")}
              </p>
              <div class="modal-input-row modal-feedback-input-row">
                <textarea
                  bind:this={feedbackEl}
                  bind:value={feedbackValue}
                  class="modal-input modal-input--multiline"
                  rows="6"
                  maxlength={FEEDBACK_MAX_LENGTH}
                  spellcheck="true"
                  autocomplete="off"
                  aria-describedby="feedback-char-count"
                ></textarea>
              </div>
              <div id="feedback-char-count" class="modal-feedback-char-count" aria-live="polite">
                {feedbackValue.length} / {FEEDBACK_MAX_LENGTH}
              </div>
              <div class="modal-inline-actions settingsCol inputAndButton">
                <span class="modal-actions-spacer"></span>
                <button type="button" class="btnicontext" on:click={() => cancelActiveModal()}>
                  {$t("Button_Cancel")}
                </button>
                <button
                  type="button"
                  class="btnicontext modal-primary"
                  disabled={feedbackSubmitDisabled}
                  on:click={feedbackSubmit}
                >
                  {$t("Feedback_Submit")}
                </button>
              </div>
              <div class="modal-feedback-footer">
                <button type="button" class="fancyLink modal-feedback-discord" on:click={openDiscordLink}>
                  {$t("Feedback_DiscordLink")}
                </button>
              </div>
            </div>
          {:else if m.kind === "crashReport"}
            <div class="modal-block">
              <p class="modal-crash-report-body">{$t("Modal_CrashReport_Body")}</p>
              <div class="modal-inline-actions settingsCol inputAndButton">
                <span class="modal-actions-spacer"></span>
                <button type="button" class="btnicontext" on:click={() => crashReportChoice("no")}>
                  {$t("No")}
                </button>
                <button
                  type="button"
                  class="btnicontext modal-primary"
                  bind:this={crashReportYesEl}
                  on:click={() => crashReportChoice("yes")}
                >
                  {$t("Yes")}
                </button>
                <button type="button" class="btnicontext" on:click={() => crashReportChoice("always")}>
                  {$t("Button_Always")}
                </button>
              </div>
            </div>
          {:else if m.kind === "update"}
            <div class="modal-block">
              <UpdateModalBody message={m.message} downloadUrl={m.downloadUrl} />
            </div>
          {/if}
        {/key}
      </div>
      </div>
    </div>
  </div>
{/if}

<style lang="scss">
  .modalBG {
    position: absolute;
    inset: 0;
    z-index: 50;
    background: var(--modal-scrim, var(--backdrop-scrim-55));
    padding: 1rem;
    box-sizing: border-box;
  }

  .modalFG {
    display: flex;
    flex-direction: column;
    position: absolute;
    background: var(--modal-bg, var(--mainContentBackground, var(--program-bg)));
    border: var(--border-bar-size, 1px) solid var(--border-bar-bg);
    box-shadow: 0 12px 40px var(--shadow-color-45);
    overflow: visible;
    box-sizing: border-box;
  }

  .modalFG-inner {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-height: 0;
    height: 100%;
    overflow: hidden;
  }

  .modalFG:not(.modalFG--ready) {
    left: 50%;
    top: 50%;
    transform: translate(-50%, -50%);
    width: auto;
    min-width: min(320px, calc(100% - 2rem));
    max-width: calc(100% - 2rem);
    max-height: min(900px, calc(100% - 2rem));
    height: auto;
  }

  .modalFG.modalFG--ready {
    transform: none;
    min-width: 0;
    max-width: none;
    max-height: none;
  }

  .modalFG.modalFilePicker:not(.modalFG--ready) {
    min-width: min(720px, calc(100% - 2rem));
  }

  $modal-resize-handle: 16px;
  $modal-resize-outset: 8px;

  .modal-resize-handle {
    position: absolute;
    z-index: 3;
    touch-action: none;
  }

  .modal-resize-n,
  .modal-resize-s {
    left: -$modal-resize-outset;
    right: -$modal-resize-outset;
    height: $modal-resize-handle;
  }

  .modal-resize-n {
    top: -$modal-resize-outset;
    cursor: ns-resize;
  }

  .modal-resize-s {
    bottom: -$modal-resize-outset;
    cursor: ns-resize;
  }

  .modal-resize-e,
  .modal-resize-w {
    top: -$modal-resize-outset;
    bottom: -$modal-resize-outset;
    width: $modal-resize-handle;
  }

  .modal-resize-e {
    right: -$modal-resize-outset;
    cursor: ew-resize;
  }

  .modal-resize-w {
    left: -$modal-resize-outset;
    cursor: ew-resize;
  }

  .modal-resize-ne,
  .modal-resize-nw,
  .modal-resize-se,
  .modal-resize-sw {
    width: $modal-resize-handle;
    height: $modal-resize-handle;
    z-index: 4;
  }

  .modal-resize-ne {
    top: -$modal-resize-outset;
    right: -$modal-resize-outset;
    cursor: nesw-resize;
  }

  .modal-resize-nw {
    top: -$modal-resize-outset;
    left: -$modal-resize-outset;
    cursor: nwse-resize;
  }

  .modal-resize-se {
    bottom: -$modal-resize-outset;
    right: -$modal-resize-outset;
    cursor: nwse-resize;
  }

  .modal-resize-sw {
    bottom: -$modal-resize-outset;
    left: -$modal-resize-outset;
    cursor: nesw-resize;
  }

  .modal-headerbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    height: 32px;
    min-height: 32px;
    background: var(--modal-header-bg, var(--border-bar-bg));
    color: var(--modal-header-fg, var(--whiteSecondary));
    flex-shrink: 0;
    user-select: none;
    cursor: grab;
    position: relative;
    z-index: 2;
    touch-action: none;

    &:active {
      cursor: grabbing;
    }
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
    fill: var(--modal-header-fg, var(--whiteSecondary));
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
    cursor: default;
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
    color: var(--modal-header-fg, var(--whiteSecondary));
    &:hover {
      background: var(--window-control-hover-bg);
    }
  }

  .modal-window-controls .win-btn-close:hover {
    background: var(--window-close-hover);
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
    background: var(--modal-input-bg, var(--even-darker-code-background));
    border: 1px solid var(--modal-input-border, var(--button-bg));
    color: var(--modal-body-fg, var(--whiteSecondary));
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

  .modal-crash-report-body {
    margin: 0;
    white-space: pre-line;
    line-height: 1.45;
    color: var(--modal-body-fg, var(--whiteSecondary));
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
  }

  .modal-inline-actions .btnicontext:disabled {
    opacity: 0.45;
    cursor: not-allowed;
    pointer-events: none;
  }

  .modal-feedback-body {
    margin: 0;
    color: var(--modal-body-fg, var(--whiteSecondary, #fff));
    line-height: 1.4;
  }

  .modal-feedback-char-count {
    margin: 0.2rem 0 0;
    font-size: 0.85rem;
    color: var(--modal-muted-fg, var(--blackTernary, #a7abbe));
    text-align: right;
  }

  .modal-feedback-footer {
    margin-top: 0.35rem;
    text-align: center;
  }

  .modal-feedback-discord {
    font-size: 0.95rem;
  }
</style>

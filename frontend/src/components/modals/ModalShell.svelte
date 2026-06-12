<script lang="ts">
  import { tick, onMount } from "svelte";
  import { createEventDispatcher } from "svelte";
  import { fade, scale } from "svelte/transition";
  import { cubicOut } from "svelte/easing";
  import { DUR, motionEnabled } from "../../lib/animation";
  import { t } from "../../stores/i18n";
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
  } from "../../lib/modalFrame";

  export let kind = "";
  export let title = "";
  export let modalId = 0;

  const dispatch = createEventDispatcher<{ cancel: void }>();

  $: modalMinSize = {
    minW: kind === "folder" ? MODAL_FRAME_MIN_W_FOLDER : MODAL_FRAME_MIN_W,
    minH: MODAL_FRAME_MIN_H,
  };

  let backdropEl: HTMLDivElement | undefined;
  let modalFgEl: HTMLDivElement | undefined;
  let headerEl: HTMLElement | undefined;
  let modalFrame: ModalFrameRect = { left: 0, top: 0, width: 0, height: 0 };
  let frameReady = false;

  let lastInitId = -1;

  function initModalFrame(): void {
    if (!backdropEl || !modalFgEl) return;
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

  $: if (modalId !== lastInitId) {
    lastInitId = modalId;
    frameReady = false;
    void tick().then(() =>
      requestAnimationFrame(() => {
        if (!backdropEl) return;
        initModalFrame();
      }),
    );
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

  function onBackdropDown(e: MouseEvent): void {
    if (e.target === backdropEl) dispatch("cancel");
  }

  function onCancel(): void {
    dispatch("cancel");
  }

  function onKeydown(e: KeyboardEvent): void {
    if (e.key === "Escape") {
      e.preventDefault();
      dispatch("cancel");
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
</script>

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
    class:modalFilePicker={kind === "folder"}
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
          {title}
        </span>
        <span class="modal-window-controls" role="toolbar">
          <button
            type="button"
            class="win-btn win-btn-close"
            aria-label={$t("Button_Close")}
            on:click={onCancel}
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
        <slot />
      </div>
    </div>
  </div>
</div>

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

  :global(.modal-block) {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }

  :global(.modal-inline-actions) {
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

  :global(.modal-inline-actions .btnicontext:disabled) {
    opacity: 0.45;
    cursor: not-allowed;
    pointer-events: none;
  }

  :global(.modal-actions-spacer) {
    flex: 1;
    min-width: 0;
  }

  :global(.modal-input-row) {
    display: flex;
    width: 100%;
  }

  :global(.modal-input) {
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

  :global(.modal-input--multiline) {
    min-height: 7.5rem;
    resize: vertical;
    line-height: 1.35;
    white-space: pre-wrap;
  }
</style>

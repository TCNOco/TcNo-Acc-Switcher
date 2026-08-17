<script lang="ts">
  import { tick, onMount } from "svelte";
  import { createEventDispatcher } from "svelte";
  import { get } from "svelte/store";
  import { fade, scale } from "svelte/transition";
  import { cubicOut } from "svelte/easing";
  import { DUR, motionEnabled } from "../../lib/animation";
  import { modalFocus } from "../../lib/modalFocus";
  import { t } from "../../stores/i18n";
  import { modalBusy } from "../../stores/modal";
  import { modalBackAction } from "../../stores/modalBack";
  import {
    MODAL_FRAME_MIN_H,
    MODAL_FRAME_MIN_W,
    MODAL_FRAME_MIN_W_FOLDER,
    MODAL_FRAME_MIN_W_STEAM_GUARD,
    MODAL_FRAME_MIN_H_FOLDER,
    MODAL_RESIZE_EDGES,
    MODAL_RESIZE_CURSOR,
    clampRect,
    fitFrameToContent,
    getModalBounds,
    modalAutoFitRequests,
    startModalDrag,
    startModalResize,
    type ModalFrameRect,
    type ResizeEdge,
  } from "../../lib/modalFrame";

  export let kind = "";
  export let title = "";
  export let modalId = 0;

  const dispatch = createEventDispatcher<{ cancel: void }>();

  /**
   * Kinds whose body is a line or two of text and a button row. The frame has a
   * minimum height, so these leave slack; centring puts it above and below
   * rather than dumping it all under the content.
   *
   * Deliberately not every kind: the file picker, Steam Guard and the
   * component-bodied dialogs fill the frame themselves.
   */
  const SHORT_BODY_KINDS = new Set(["alert", "confirm", "prompt"]);

  function minFrameWidth(modalKind: string): number {
    if (modalKind === "folder") return MODAL_FRAME_MIN_W_FOLDER;
    if (modalKind === "steamGuard") return MODAL_FRAME_MIN_W_STEAM_GUARD;
    return MODAL_FRAME_MIN_W;
  }

  $: modalMinSize = {
    minW: minFrameWidth(kind),
    minH: kind === "folder" ? MODAL_FRAME_MIN_H_FOLDER : MODAL_FRAME_MIN_H,
  };

  let backdropEl: HTMLDivElement | undefined;
  let modalFgEl: HTMLDivElement | undefined;
  let headerEl: HTMLElement | undefined;
  let modalFrame: ModalFrameRect = { left: 0, top: 0, width: 0, height: 0 };
  let frameReady = false;

  /**
   * Keeps initial focus off the header close-X: an explicitly marked target wins, then the
   * first enabled control of the body itself.
   */
  function preferredInitialFocus(): HTMLElement | null {
    const scroll = modalFgEl?.querySelector(".modal-scroll");
    if (!(scroll instanceof HTMLElement)) return null;
    const marked = scroll.querySelector<HTMLElement>(
      "[data-modal-autofocus], [data-steamguard-autofocus], [data-steamguard-focus]",
    );
    if (marked) return marked;
    return scroll.querySelector<HTMLElement>(
      "input:not([type='hidden']):not(:disabled), textarea:not(:disabled), select:not(:disabled), button:not(:disabled), a[href]",
    );
  }

  let lastInitId = -1;
  // Seeded from the store: it outlives individual modals, so a stale count would
  // fire a pointless fit on the next modal's first render.
  let lastAutoFitRequest = get(modalAutoFitRequests);
  let contentResizeObserver: ResizeObserver | undefined;
  let refitQueued = false;
  let userAdjustedFrame = false;
  let titleId = "";

  function initModalFrame(): void {
    if (!backdropEl || !modalFgEl) return;
    modalFrame = fitFrameToContent(
      modalFgEl,
      getModalBounds(backdropEl),
      modalMinSize,
    );
    frameReady = true;
    attachContentObservers();
  }

  function refitModalFrame(): void {
    if (!frameReady || !backdropEl || !modalFgEl || userAdjustedFrame) return;
    contentResizeObserver?.disconnect();
    const next = fitFrameToContent(
      modalFgEl,
      getModalBounds(backdropEl),
      modalMinSize,
    );
    if (
      next.width !== modalFrame.width ||
      next.height !== modalFrame.height ||
      next.left !== modalFrame.left ||
      next.top !== modalFrame.top
    ) {
      modalFrame = next;
    }
    attachContentObservers();
  }

  function queueRefitModalFrame(): void {
    if (!frameReady || refitQueued || userAdjustedFrame) return;
    refitQueued = true;
    requestAnimationFrame(() => {
      refitQueued = false;
      refitModalFrame();
    });
  }

  function attachContentObservers(): void {
    contentResizeObserver?.disconnect();
    if (!modalFgEl) return;

    const scroll = modalFgEl.querySelector(".modal-scroll");
    if (!(scroll instanceof HTMLElement)) return;

    contentResizeObserver = new ResizeObserver(() => queueRefitModalFrame());
    for (const child of scroll.children) {
      if (child instanceof HTMLElement) {
        contentResizeObserver.observe(child);
      }
    }
  }

  function detachContentObservers(): void {
    contentResizeObserver?.disconnect();
    contentResizeObserver = undefined;
  }

  function reclampModalFrame(): void {
    if (!frameReady || !backdropEl) return;
    modalFrame = clampRect(modalFrame, getModalBounds(backdropEl), modalMinSize);
  }

  // A body that switched page asks for a fresh fit: the new page holds different
  // content, so a size the user picked for the previous one no longer applies.
  $: if ($modalAutoFitRequests !== lastAutoFitRequest) {
    lastAutoFitRequest = $modalAutoFitRequests;
    userAdjustedFrame = false;
    void tick().then(() => queueRefitModalFrame());
  }

  $: if (modalId !== lastInitId) {
    lastInitId = modalId;
    titleId = `modal-title-label-${modalId}`;
    detachContentObservers();
    frameReady = false;
    userAdjustedFrame = false;
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
        userAdjustedFrame = true;
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
        userAdjustedFrame = true;
        modalFrame = rect;
      },
    });
  }

  function onBackdropDown(e: MouseEvent): void {
    if ($modalBusy) return;
    if (e.target === backdropEl) dispatch("cancel");
  }

  // Escape, the backdrop and the close button are all the same way out, and a
  // body that has withheld its back action because it is mid-operation cannot
  // survive any of them: the work carries on with nothing left to report to.
  function onCancel(): void {
    if ($modalBusy) return;
    dispatch("cancel");
  }

  onMount(() => {
    window.addEventListener("resize", reclampModalFrame);
    return () => {
      window.removeEventListener("resize", reclampModalFrame);
      detachContentObservers();
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
    class:modalSteamGuard={kind === "steamGuard"}
    class:modalShortBody={SHORT_BODY_KINDS.has(kind)}
    bind:this={modalFgEl}
    use:modalFocus={{ onEscape: onCancel, initialFocus: preferredInitialFocus }}
    role="dialog"
    aria-modal="true"
    aria-labelledby={titleId}
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
          {#if $modalBackAction}
            <!-- Sits where the window back button does, so leaving a screen is
                 in the same place whether you are in a modal or not. The body
                 publishes the action, so this mirrors that screen's own back,
                 close or show-all control rather than adding a second meaning. -->
            <button
              type="button"
              class="win-btn win-btn-back"
              title={$modalBackAction.label}
              aria-label={$modalBackAction.label}
              on:click={() => $modalBackAction?.run()}
            >
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 320 512" aria-hidden="true"><path d="M34.52 239.03L228.87 44.69c9.37-9.37 24.57-9.37 33.94 0l22.67 22.67c9.36 9.36 9.37 24.52.04 33.9L131.49 256l154.02 154.75c9.34 9.38 9.32 24.54-.04 33.9l-22.67 22.67c-9.37 9.37-24.57 9.37-33.94 0L34.52 272.97c-9.37-9.37-9.37-24.57 0-33.94z" /></svg>
            </button>
          {/if}
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
        <span id={titleId} class="modal-title-drag">
          {title}
        </span>
        <span class="modal-window-controls" role="toolbar">
          <button
            type="button"
            class="win-btn win-btn-close"
            aria-label={$t("Button_Close")}
            disabled={$modalBusy}
            on:click={onCancel}
          >
            <svg class="win-btn__glyph win-btn__glyph--close" viewBox="0 0 10 10" aria-hidden="true">
              <path d="M2 2l6 6" />
              <path d="M8 2L2 8" />
            </svg>
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
    border: var(--modal-frame-border-width, 1px) solid var(--modal-border, var(--border-bar-bg));
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

  /*
   * Centred with `translate`, not `transform`. Svelte's `scale` transition snapshots
   * `getComputedStyle(node).transform` when the modal mounts — while it is still
   * un-ready — and replays that snapshot for the whole animation. A transform-based
   * centre resolved to a pixel matrix of half the un-ready size, and because an
   * animation outranks a normal declaration, `.modalFG--ready`'s `transform: none`
   * could not clear it: the modal sat up and to the left until the animation ended,
   * then snapped to centre. `translate` is its own property, so the snapshot is
   * `none` and the animation only ever scales.
   */
  .modalFG:not(.modalFG--ready) {
    left: 50%;
    top: 50%;
    translate: -50% -50%;
    width: max-content;
    min-width: min(320px, calc(100% - 2rem));
    max-width: min(80%, calc(100% - 2rem));
    max-height: min(75%, calc(100% - 2rem));
    height: auto;

    .modalFG-inner {
      flex: none;
      height: auto;
      overflow: visible;
    }

    .modal-scroll {
      flex: none;
      min-height: auto;
      overflow: visible;
    }
  }

  .modalFG.modalFG--ready {
    translate: none;
    min-width: 0;
    max-width: none;
    max-height: none;
  }

  /*
   * Steam Guard bodies size themselves per screen: the shell keeps the single scrollbar
   * (`.modal-scroll` already scrolls) and only becomes a flex column so the body can centre
   * itself vertically with auto margins when the frame is taller than the screen's content.
   */
  .modalFG.modalSteamGuard .modal-scroll {
    display: flex;
    flex-direction: column;
  }

  /* `safe` so that a body which turns out to be taller than the frame still
     scrolls from its top: centred overflow otherwise puts the first line above
     the scroll origin, out of reach. */
  .modalFG.modalShortBody .modal-scroll {
    display: flex;
    flex-direction: column;
    justify-content: safe center;
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

  /* Centred on the bar, not on the space left between the flanking controls, so
     the title sits where the window titlebar puts it instead of drifting right
     by half the back button. Taken out of the flow for that, which is also why
     the reserve below is a fixed number: the widest flank is the back button
     plus the icon, and the themes only ever make these controls smaller. */
  .modal-title-drag {
    position: absolute;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    max-width: calc(100% - 200px);
    font-family: "Segoe UI", sans-serif;
    font-size: 12px;
    font-weight: 500;
    text-align: center;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    pointer-events: none;
  }

  .modal-window-controls {
    display: flex;
    height: 100%;
    cursor: default;
  }

  /* Geometry from the window controls, glyph from the titlebar back button, so
     it reads as the same control in both places. */
  .modal-title-left .win-btn-back {
    --wails-draggable: no-drag;
    border-radius: 0;
    background: none;
    border: 0;
    margin: 0;
    display: flex;
    justify-content: center;
    align-items: center;
    width: 40px;
    height: 100%;
    padding: 0;
    cursor: pointer;
    color: var(--modal-header-fg, var(--whiteSecondary));

    svg {
      display: block;
      height: 0.8rem;
      fill: currentColor;
    }

    &:hover {
      background: var(--window-control-hover-bg);
    }
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

    &:disabled {
      cursor: default;
      opacity: 0.4;
      background: none;
    }
  }

  .modal-window-controls .win-btn__glyph {
    width: 10px;
    height: 10px;
    display: block;
    overflow: visible;
    color: currentColor;
    fill: none;
    stroke: currentColor;
    stroke-width: 1.2;
    stroke-linecap: square;
    stroke-linejoin: miter;
    vector-effect: non-scaling-stroke;
    forced-color-adjust: auto;
  }

  .modal-window-controls .win-btn-close:hover:not(:disabled) {
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

<script lang="ts">
  import { createEventDispatcher, onDestroy, onMount } from "svelte";
  import {
    insertionIndexFromTileHover,
    moveItem,
    previewSlots,
  } from "../lib/reorderList";

  /** Unique string ids; order is the canonical list. */
  export let items: string[] = [];
  /** When true, drag-reorder is disabled (e.g. while a tag filter is active). */
  export let reorderDisabled = false;
  export let dragThresholdPx = 8;
  /** Classes on the scroll/flex container (e.g. `platform_list`). */
  export let listClass = "";
  /** Classes on each draggable cell wrapper (e.g. `platform_list_item`). */
  export let itemClass = "";
  export let placeholderClass = "";
  export let ghostClass = "";
  export let ariaLabel: string | undefined = undefined;

  const dispatch = createEventDispatcher<{
    reorder: { items: string[] };
    itemclick: { id: string };
  }>();

  /** Avoid stale closures in window listeners. */
  let itemsSnap: string[] = [];
  $: itemsSnap = items;

  let dragIndex: number | null = null;
  let dragOverIndex: number | null = null;

  let pendingDrag: {
    fromIndex: number;
    id: string;
    startX: number;
    startY: number;
    grabOffsetX: number;
    grabOffsetY: number;
    width: number;
    height: number;
    /** Live cell element — clone before dragIndex updates, or the node is destroyed by previewSlots. */
    cellEl: HTMLElement;
  } | null = null;

  /** Deep clone of the dragged cell for the floating ghost (pixel-accurate to the real tile). */
  let dragVisualClone: HTMLElement | null = null;

  let ghostX = 0;
  let ghostY = 0;
  let ghostW = 0;
  let ghostH = 0;
  let draggingId: string | null = null;

  let listEl: HTMLDivElement | null = null;

  $: displaySlots =
    dragIndex === null
      ? itemsSnap.map((id): string | null => id)
      : previewSlots(
          itemsSnap,
          dragIndex,
          dragOverIndex ?? dragIndex,
        );

  let suppressNextClick = false;
  /** If no synthetic post-drag click fires, expire suppress so the next real click is not eaten. */
  let suppressClickExpire: ReturnType<typeof setTimeout> | null = null;

  function armSuppressClickAfterDrag(): void {
    if (suppressClickExpire) {
      clearTimeout(suppressClickExpire);
      suppressClickExpire = null;
    }
    suppressNextClick = true;
    suppressClickExpire = setTimeout(() => {
      suppressNextClick = false;
      suppressClickExpire = null;
    }, 0);
  }

  function hitTestDragOver(clientX: number, clientY: number): void {
    if (dragIndex === null || !listEl) return;
    const el = document.elementFromPoint(clientX, clientY);
    const cell = el?.closest("[data-dnd-cell]");
    if (!cell || !listEl.contains(cell)) return;
    const isGap = (cell as HTMLElement).dataset.dndGap === "true";
    if (isGap) {
      const visual = Number((cell as HTMLElement).dataset.dndVisual);
      if (!Number.isNaN(visual)) dragOverIndex = visual;
      return;
    }
    const id = (cell as HTMLElement).dataset.dndName ?? "";
    if (!id || dragIndex === null) return;
    dragOverIndex = insertionIndexFromTileHover(
      itemsSnap,
      dragIndex,
      id,
      clientX,
      cell as HTMLElement,
    );
  }

  function stripCellForGhostClone(el: HTMLElement): void {
    el.removeAttribute("data-dnd-cell");
    el.removeAttribute("data-dnd-visual");
    el.removeAttribute("data-dnd-name");
    el.removeAttribute("tabindex");
    el.removeAttribute("role");
    el.classList.remove("reorder-pointer-grid__cell");
    el.style.cursor = "grabbing";
    el.style.width = "100%";
    el.style.height = "100%";
    el.style.boxSizing = "border-box";
    /* Grid layout margins (e.g. .platform_list_item { margin: 10px }) must not apply inside the fixed-size ghost. */
    el.style.margin = "0";
    el.style.maxWidth = "none";
    el.style.maxHeight = "none";
    // Avoid duplicate ids / radio group side effects in the floating copy
    el.querySelectorAll("input").forEach((n) => n.remove());
    el.querySelectorAll("[id]").forEach((n) => n.removeAttribute("id"));
    el.querySelectorAll("label[for]").forEach((n) => n.removeAttribute("for"));
  }

  function beginPointerDrag(
    e: PointerEvent,
    pd: NonNullable<typeof pendingDrag>,
  ): void {
    const visual = pd.cellEl.cloneNode(true) as HTMLElement;
    stripCellForGhostClone(visual);
    dragVisualClone = visual;

    dragIndex = pd.fromIndex;
    dragOverIndex = pd.fromIndex;
    ghostW = pd.width;
    ghostH = pd.height;
    draggingId = pd.id;
    ghostX = e.clientX - pd.grabOffsetX;
    ghostY = e.clientY - pd.grabOffsetY;
    document.body.style.userSelect = "none";
    hitTestDragOver(e.clientX, e.clientY);
  }

  function endPointerDrag(commit: boolean): void {
    if (pendingDrag && dragIndex === null) {
      pendingDrag = null;
      return;
    }
    if (dragIndex !== null) {
      const from = dragIndex;
      const to = dragOverIndex ?? from;
      armSuppressClickAfterDrag();
      dragIndex = null;
      dragOverIndex = null;
      pendingDrag = null;
      draggingId = null;
      dragVisualClone = null;
      document.body.style.userSelect = "";
      if (commit && from !== to) {
        dispatch("reorder", { items: moveItem(itemsSnap, from, to) });
      }
    } else {
      pendingDrag = null;
    }
  }

  function onWindowPointerMove(e: PointerEvent): void {
    if (!pendingDrag && dragIndex === null) return;

    if (pendingDrag && dragIndex === null) {
      const dx = e.clientX - pendingDrag.startX;
      const dy = e.clientY - pendingDrag.startY;
      if (dx * dx + dy * dy < dragThresholdPx * dragThresholdPx) return;
      beginPointerDrag(e, pendingDrag);
    }

    if (dragIndex !== null && pendingDrag) {
      ghostX = e.clientX - pendingDrag.grabOffsetX;
      ghostY = e.clientY - pendingDrag.grabOffsetY;
      hitTestDragOver(e.clientX, e.clientY);
    }
  }

  function onWindowPointerUp(_e: PointerEvent): void {
    if (pendingDrag && dragIndex === null) {
      pendingDrag = null;
      return;
    }
    endPointerDrag(true);
  }

  function onWindowPointerCancel(_e: PointerEvent): void {
    endPointerDrag(false);
  }

  function onWindowKeyDown(e: KeyboardEvent): void {
    if (e.key !== "Escape") return;
    if (dragIndex === null && !pendingDrag) return;
    e.preventDefault();
    dragIndex = null;
    dragOverIndex = null;
    pendingDrag = null;
    draggingId = null;
    dragVisualClone = null;
    document.body.style.userSelect = "";
    armSuppressClickAfterDrag();
  }

  function onCellPointerDown(e: PointerEvent, id: string): void {
    if (reorderDisabled) {
      return;
    }
    if (e.button !== 0) return;
    const from = itemsSnap.indexOf(id);
    if (from < 0) return;
    const el = e.currentTarget as HTMLElement;
    const r = el.getBoundingClientRect();
    pendingDrag = {
      fromIndex: from,
      id,
      startX: e.clientX,
      startY: e.clientY,
      grabOffsetX: e.clientX - r.left,
      grabOffsetY: e.clientY - r.top,
      width: r.width,
      height: r.height,
      cellEl: e.currentTarget as HTMLElement,
    };
  }

  /** Mount cloned tile into the ghost layer (arbitrary DOM from cloneNode). */
  function ghostCloneMount(node: HTMLElement, clone: HTMLElement | null) {
    function apply(c: HTMLElement | null) {
      node.replaceChildren();
      if (c) node.appendChild(c);
    }
    apply(clone);
    return {
      update(next: HTMLElement | null) {
        apply(next);
      },
      destroy() {
        node.replaceChildren();
      },
    };
  }

  function onCellClick(id: string): void {
    if (suppressClickExpire) {
      clearTimeout(suppressClickExpire);
      suppressClickExpire = null;
    }
    if (suppressNextClick) {
      suppressNextClick = false;
      return;
    }
    dispatch("itemclick", { id });
  }

  let teardown: (() => void) | undefined;

  onMount(() => {
    const move = (e: PointerEvent) => onWindowPointerMove(e);
    const up = (e: PointerEvent) => onWindowPointerUp(e);
    const cancel = (e: PointerEvent) => onWindowPointerCancel(e);
    const key = (e: KeyboardEvent) => onWindowKeyDown(e);
    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", up);
    window.addEventListener("pointercancel", cancel);
    window.addEventListener("keydown", key, true);
    teardown = () => {
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", up);
      window.removeEventListener("pointercancel", cancel);
      window.removeEventListener("keydown", key, true);
    };
  });

  onDestroy(() => {
    teardown?.();
    document.body.style.userSelect = "";
    if (suppressClickExpire) clearTimeout(suppressClickExpire);
  });
</script>

<div
  bind:this={listEl}
  class="reorder-pointer-grid {listClass}"
  role="list"
  aria-label={ariaLabel}
>
  {#each displaySlots as slot, i (slot === null ? `gap-${i}` : slot)}
    {#if slot === null}
      <div
        class="reorder-pointer-grid__gap {placeholderClass}"
        role="presentation"
        data-dnd-cell
        data-dnd-visual={i}
        data-dnd-gap="true"
      ></div>
    {:else}
      <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
      <!-- svelte-ignore a11y-no-noninteractive-tabindex -->
      <div
        class="reorder-pointer-grid__cell {itemClass}"
        class:dragging={draggingId === slot}
        role="listitem"
        tabindex="0"
        data-dnd-cell
        data-dnd-visual={i}
        data-dnd-name={slot}
        on:pointerdown={(e) => onCellPointerDown(e, slot)}
        on:keydown={(e) => e.key === "Enter" && onCellClick(slot)}
        on:click={() => onCellClick(slot)}
      >
        <slot name="item" rowId={slot} index={i} />
      </div>
    {/if}
  {/each}
</div>

{#if draggingId && dragVisualClone}
  <div
    class="reorder-pointer-grid__ghost {ghostClass}"
    style:left="{ghostX}px"
    style:top="{ghostY}px"
    style:width="{ghostW}px"
    style:height="{ghostH}px"
    aria-hidden="true"
  >
    <div
      class="reorder-pointer-grid__ghost-mirror"
      use:ghostCloneMount={dragVisualClone}
    ></div>
  </div>
{:else if draggingId && $$slots.ghost}
  <div
    class="reorder-pointer-grid__ghost {ghostClass}"
    style:left="{ghostX}px"
    style:top="{ghostY}px"
    style:width="{ghostW}px"
    style:height="{ghostH}px"
    aria-hidden="true"
  >
    <slot name="ghost" rowId={draggingId} />
  </div>
{:else if draggingId}
  <div
    class="reorder-pointer-grid__ghost {ghostClass}"
    style:left="{ghostX}px"
    style:top="{ghostY}px"
    style:width="{ghostW}px"
    style:height="{ghostH}px"
    aria-hidden="true"
  >
    <span class="reorder-pointer-grid__ghost-fallback">{draggingId}</span>
  </div>
{/if}

<style lang="scss">
  .reorder-pointer-grid__cell {
    cursor: grab;
    touch-action: none;
  }

  .reorder-pointer-grid__ghost {
    position: fixed;
    margin: 0;
    z-index: 10000;
    pointer-events: none;
    opacity: 0.96;
    cursor: grabbing;
    box-shadow: 0 12px 36px var(--shadow-color-50);
    transition: none;
    left: 0;
    top: 0;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    /* Same as .acc_list / list chrome — cloned tiles are often transparent (Steam label.acc). */
    background: var(--mainContentBackground);
    border-radius: 6px;

    &:hover {
      transform: none !important;
      animation: none !important;
      filter: none !important;
    }
  }

  .reorder-pointer-grid__ghost-mirror {
    flex: 1;
    min-height: 0;
    min-width: 0;
    width: 100%;
    height: 100%;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    background: transparent;
    border-radius: inherit;
  }

  .reorder-pointer-grid__ghost-fallback {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 100%;
    width: 100%;
    font-weight: 700;
    color: var(--accent);
    padding: 0.5rem;
    text-align: center;
    word-break: break-word;
  }

  [data-dnd-cell] {
    transition: transform 0.15s ease-out;
  }

  [data-dnd-cell].dragging {
    transition: none;
  }
</style>

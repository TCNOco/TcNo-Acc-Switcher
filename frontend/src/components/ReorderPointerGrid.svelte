<script lang="ts">
  import { createEventDispatcher, onDestroy, onMount } from "svelte";
  import {
    insertionIndexFromTileHover,
    moveItem,
    previewSlots,
  } from "../lib/reorderList";

  /** Unique string ids; order is the canonical list. */
  export let items: string[] = [];
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
  } | null = null;

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

  function beginPointerDrag(
    e: PointerEvent,
    pd: NonNullable<typeof pendingDrag>,
  ): void {
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
      suppressNextClick = true;
      dragIndex = null;
      dragOverIndex = null;
      pendingDrag = null;
      draggingId = null;
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
    document.body.style.userSelect = "";
    suppressNextClick = true;
  }

  function onCellPointerDown(e: PointerEvent, id: string): void {
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
    };
  }

  function onCellClick(id: string): void {
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

{#if draggingId}
  <div
    class="reorder-pointer-grid__ghost {ghostClass}"
    style:left="{ghostX}px"
    style:top="{ghostY}px"
    style:width="{ghostW}px"
    style:height="{ghostH}px"
    aria-hidden="true"
  >
    {#if $$slots.ghost}
      <slot name="ghost" rowId={draggingId} />
    {:else}
      <span class="reorder-pointer-grid__ghost-fallback">{draggingId}</span>
    {/if}
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
    box-shadow: 0 12px 36px rgba(0, 0, 0, 0.5);
    transition: none;
    left: 0;
    top: 0;

    &:hover {
      transform: none !important;
      animation: none !important;
      filter: none !important;
    }
  }

  .reorder-pointer-grid__ghost-fallback {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 100%;
    width: 100%;
    font-weight: 700;
    color: var(--accent, #80ffea);
    padding: 0.5rem;
    text-align: center;
    word-break: break-word;
  }
</style>

<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import { get } from "svelte/store";
  import ActionBar from "../components/ActionBar.svelte";
  import { route, appBarTitle } from "../stores/nav";
  import { t } from "../stores/i18n";
  import { openConfirm, openFolderPicker } from "../stores/modal";
  import { pushToast } from "../stores/toast";
  import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
  import type { PlatformStartup } from "../../bindings/TcNo-Acc-Switcher/internal/platform/models.js";
  import { platformIconFgHref } from "../lib/platformIcon";
  import "../styles/HomePlatforms.scss";

  const DRAG_THRESHOLD_PX = 8;

  let startup: PlatformStartup | null = null;
  let homeOrder: string[] = [];
  let loadError: string | null = null;
  let dragIndex: number | null = null;
  let dragOverIndex: number | null = null;

  /** Pointer gesture: pending until movement crosses threshold (HTML5 DnD is unreliable in WebView). */
  let pendingDrag: {
    fromIndex: number;
    name: string;
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
  let draggingName: string | null = null;

  let platformListEl: HTMLDivElement | null = null;

  /** While dragging: order with `null` = empty slot (dotted) so neighbors shift. */
  function previewSlots(
    order: string[],
    from: number,
    to: number,
  ): (string | null)[] {
    const n = order.length;
    const without = order.filter((_, i) => i !== from);
    const out: (string | null)[] = [];
    let wi = 0;
    for (let i = 0; i < n; i++) {
      if (i === to) {
        out.push(null);
      } else {
        out.push(without[wi++]!);
      }
    }
    return out;
  }

  /**
   * Insertion index into `homeOrder` with `from` removed — same as `splice(to,0,moved)` after `splice(from,1)`.
   * Left half of a tile: before that item; right half: after that item.
   */
  function insertionIndexFromTileHover(
    slot: string,
    clientX: number,
    cell: HTMLElement,
  ): number {
    if (dragIndex === null) return homeOrder.indexOf(slot);
    const short = homeOrder.filter((_, i) => i !== dragIndex);
    const refInShort = short.indexOf(slot);
    if (refInShort < 0) return 0;
    const rect = cell.getBoundingClientRect();
    const after = clientX >= rect.left + rect.width / 2;
    let pos = after ? refInShort + 1 : refInShort;
    return Math.max(0, Math.min(pos, short.length));
  }

  $: displaySlots =
    dragIndex === null
      ? homeOrder.map((name): string | null => name)
      : previewSlots(
          homeOrder,
          dragIndex,
          dragOverIndex ?? dragIndex,
        );

  /** After a successful reorder drop, ignore the synthetic click that follows. */
  let suppressNextClick = false;
  let navigating = false;

  $: appBarTitle.set("TcNo Account Switcher");

  function textClass(name: string): string {
    const n = name.length;
    if (n < 7) return "shortText";
    if (n > 12) return "longText";
    return "";
  }

  async function promptMissingPlatformsFile(): Promise<void> {
    const tt = get(t);
    const locate = await openConfirm({
      title: tt("Modal_Title_MissingPlatformsJson"),
      body: `<p>${tt("Modal_Body_MissingPlatformsJson")}</p>`,
      style: "yesno",
      positiveLabel: tt("Button_LocatePlatformsJson"),
      negativeLabel: tt("Button_RestoreBundledPlatforms"),
    });
    if (locate) {
      const path = await PlatformService.PickPlatformsJSON();
      if (path) await PlatformService.ApplyPlatformsJSONFile(path);
    } else {
      await PlatformService.RestoreDefaultPlatformsJSON();
    }
  }

  async function refreshStartup(skipMissingPrompt = false): Promise<void> {
    loadError = null;
    try {
      const s = await PlatformService.GetStartup();
      if (s.platformsFileMissing && !skipMissingPrompt) {
        await promptMissingPlatformsFile();
        await refreshStartup(true);
        return;
      }
      startup = s;
      homeOrder = s.homePlatformOrder ?? [];
      if (!s.platformsFileMissing && s.allPlatformNames?.length && homeOrder.length === 0) {
        route.set({ page: "manage-platforms" });
      }
    } catch (e) {
      loadError = e instanceof Error ? e.message : String(e);
    }
  }

  async function openPlatform(name: string): Promise<void> {
    if (navigating) return;
    navigating = true;
    try {
      const r = await PlatformService.ResolvePlatformLaunch(name);
      if (r.ok) {
        if (r.foundViaShortcut) {
          pushToast({
            type: "success",
            title: "",
            message: get(t)("Toast_FoundExeViaShortcut"),
            duration: 30000,
          });
        }
        route.set({ page: "platform", platformName: name });
        return;
      }
      if (r.needsManualLocate) {
        const tt = get(t);
        const picked = await openFolderPicker({
          title: tt("Modal_Title_LocatePlatform", { platform: name }),
          body: `<p>${tt("Modal_LocatePlatform", { platformExe: r.soughtExeName })}</p>`,
          initialPath: r.initialPath ?? "",
          dirsOnly: false,
          soughtFilename: r.soughtExeName,
          positiveLabel: tt("Modal_Button_Select"),
        });
        if (picked) {
          await PlatformService.ConfirmPlatformExePath(name, picked);
          route.set({ page: "platform", platformName: name });
        }
      }
    } catch (e) {
      loadError = e instanceof Error ? e.message : String(e);
    } finally {
      navigating = false;
    }
  }

  function hitTestDragOver(clientX: number, clientY: number): void {
    if (dragIndex === null || !platformListEl) return;
    const el = document.elementFromPoint(clientX, clientY);
    const cell = el?.closest("[data-dnd-cell]");
    if (!cell || !platformListEl.contains(cell)) return;
    const visual = Number((cell as HTMLElement).dataset.dndVisual);
    if (Number.isNaN(visual)) return;
    const isGap = (cell as HTMLElement).dataset.dndGap === "true";
    if (isGap) {
      dragOverIndex = visual;
      return;
    }
    const name = (cell as HTMLElement).dataset.dndName ?? "";
    if (!name) return;
    dragOverIndex = insertionIndexFromTileHover(
      name,
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
    draggingName = pd.name;
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
      draggingName = null;
      document.body.style.userSelect = "";
      if (commit && from !== to) {
        const next = [...homeOrder];
        const [moved] = next.splice(from, 1);
        next.splice(to, 0, moved);
        homeOrder = next;
        void PlatformService.SaveHomeOrder(next).catch(() => {});
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
      if (dx * dx + dy * dy < DRAG_THRESHOLD_PX * DRAG_THRESHOLD_PX) return;
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
    draggingName = null;
    document.body.style.userSelect = "";
    suppressNextClick = true;
  }

  function onTilePointerDown(e: PointerEvent, name: string): void {
    if (e.button !== 0) return;
    const from = homeOrder.indexOf(name);
    if (from < 0) return;
    const el = e.currentTarget as HTMLElement;
    const r = el.getBoundingClientRect();
    pendingDrag = {
      fromIndex: from,
      name,
      startX: e.clientX,
      startY: e.clientY,
      grabOffsetX: e.clientX - r.left,
      grabOffsetY: e.clientY - r.top,
      width: r.width,
      height: r.height,
    };
  }

  function onTileClick(name: string): void {
    if (suppressNextClick) {
      suppressNextClick = false;
      return;
    }
    void openPlatform(name);
  }

  let teardownWindowListeners: (() => void) | undefined;

  onMount(() => {
    void refreshStartup();
    const move = (e: PointerEvent) => onWindowPointerMove(e);
    const up = (e: PointerEvent) => onWindowPointerUp(e);
    const cancel = (e: PointerEvent) => onWindowPointerCancel(e);
    const key = (e: KeyboardEvent) => onWindowKeyDown(e);
    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", up);
    window.addEventListener("pointercancel", cancel);
    window.addEventListener("keydown", key, true);
    teardownWindowListeners = () => {
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", up);
      window.removeEventListener("pointercancel", cancel);
      window.removeEventListener("keydown", key, true);
    };
  });

  onDestroy(() => {
    teardownWindowListeners?.();
    document.body.style.userSelect = "";
  });
</script>

<div class="main-content home-root">
  {#if loadError}
    <p class="home-msg">{loadError}</p>
  {:else if !startup}
    <p class="home-msg">…</p>
  {:else if startup.platformsFileMissing}
    <p class="home-msg">{$t("Modal_Body_MissingPlatformsJson")}</p>
  {:else}
    <div class="platformTable">
      <div
        bind:this={platformListEl}
        class="platform_list"
        role="list"
        aria-label={$t("Preview_Platforms")}
      >
        {#each displaySlots as slot, i (slot === null ? `gap-${i}` : slot)}
          {#if slot === null}
            <div
              class="platform_list_item platform_list_placeholder"
              role="presentation"
              data-dnd-cell
              data-dnd-visual={i}
              data-dnd-gap="true"
            ></div>
          {:else}
            <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
            <!-- svelte-ignore a11y-no-noninteractive-tabindex -->
            <div
              class="platform_list_item platform_list_item--draggable"
              role="listitem"
              tabindex="0"
              data-dnd-cell
              data-dnd-visual={i}
              data-dnd-name={slot}
              on:pointerdown={(e) => onTilePointerDown(e, slot)}
              on:keydown={(e) => e.key === "Enter" && onTileClick(slot)}
              on:click={() => onTileClick(slot)}
            >
              <div class="fgText {textClass(slot)}">
                <p>{slot.toUpperCase()}</p>
              </div>
              <div class="fgImg" aria-hidden="true">
                <svg viewBox="0 0 500 500" aria-hidden="true">
                  <use
                    href={platformIconFgHref(slot)}
                    class="icoFG"
                  />
                </svg>
              </div>
              <svg viewBox="0 0 2084 2084" class="icoBG" aria-hidden="true">
                <use
                  href="img/platform/glass.svg#GLASS"
                  class="icoGlass"
                ></use>
              </svg>
            </div>
          {/if}
        {/each}
      </div>
    </div>
  {/if}

  {#if draggingName}
    <div
      class="platform_list_item platform_list_item--ghost"
      style:left="{ghostX}px"
      style:top="{ghostY}px"
      style:width="{ghostW}px"
      style:height="{ghostH}px"
      aria-hidden="true"
    >
      <div class="fgText {textClass(draggingName)}">
        <p>{draggingName.toUpperCase()}</p>
      </div>
      <div class="fgImg" aria-hidden="true">
        <svg viewBox="0 0 500 500" aria-hidden="true">
          <use
            href={platformIconFgHref(draggingName)}
            class="icoFG"
          />
        </svg>
      </div>
      <svg viewBox="0 0 2084 2084" class="icoBG" aria-hidden="true">
        <use href="img/platform/glass.svg#GLASS" class="icoGlass"></use>
      </svg>
    </div>
  {/if}
</div>
<ActionBar />

<style lang="scss">
  .home-root {
    display: flex;
    flex-direction: column;
    min-height: 0;
  }
  .home-msg {
    padding: 1rem;
    color: var(--white);
  }

  :global(.platform_list_item--draggable) {
    cursor: grab;
    touch-action: none;
  }

  :global(.platform_list_item--ghost) {
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
      box-shadow: 0 12px 36px rgba(0, 0, 0, 0.5);
    }
  }
</style>

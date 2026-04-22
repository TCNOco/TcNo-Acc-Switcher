<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import { get } from "svelte/store";
  import { Events } from "@wailsio/runtime";
  import * as Shortcuts from "../../bindings/TcNo-Acc-Switcher/internal/shortcuts/service.js";
  import { ListPayload, ShortcutDTO } from "../../bindings/TcNo-Acc-Switcher/internal/shortcuts/models.js";
  import { platformExeIconUrl, triggerPlatformAction, selectedAccount } from "../stores/platformPage";
  import { tooltip } from "../lib/actions/tooltip";
  import { contextMenu } from "../lib/actions/contextMenu";
  import type { MenuItemDef } from "../stores/contextMenu";
  import { t } from "../stores/i18n";
  import { pushToast } from "../stores/toast";
  import { HasShortcutMainExe, LaunchPlatformAs } from "../lib/platformBindings";
  import { formatToastWithError } from "../lib/formatWailsError";
  import {
    insertionIndexExternalDrag,
    insertionIndexFromTileHover,
    moveItem,
    previewInsertGap,
    previewSlots,
  } from "../lib/reorderList";

  type Row = InstanceType<typeof ShortcutDTO>;
  type Zone = "pinned" | "dropdown";

  export let platformName: string;

  let includeMainExe = false;
  let pinNames: string[] = [];
  let dropNames: string[] = [];
  let meta: Record<string, Row> = {};
  let ddOpen = false;
  let iconBroken = false;

  let pinListEl: HTMLDivElement | null = null;
  let dropListEl: HTMLDivElement | null = null;

  let dragSourceZone: Zone | null = null;
  let dragFromIndex: number | null = null;
  let dragOverZone: Zone | null = null;
  let dragOverIndex: number | null = null;
  let draggingId: string | null = null;

  let pendingDrag: {
    zone: Zone;
    fromIndex: number;
    id: string;
    startX: number;
    startY: number;
    grabOffsetX: number;
    grabOffsetY: number;
    width: number;
    height: number;
    cellEl: HTMLElement;
  } | null = null;

  let dragVisualClone: HTMLElement | null = null;
  let ghostX = 0;
  let ghostY = 0;
  let ghostW = 0;
  let ghostH = 0;

  const dragThresholdPx = 8;

  let suppressNextClick = false;
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

  /** Wails / JSON may use PascalCase; generated ShortcutDTO only defaults camelCase keys. */
  function readStrLoose(o: Record<string, unknown>, ...keys: string[]): string {
    for (const k of keys) {
      const v = o[k];
      if (typeof v === "string" && v.trim() !== "") {
        return v.trim();
      }
    }
    return "";
  }

  function readBoolLoose(o: Record<string, unknown>, ...keys: string[]): boolean {
    for (const k of keys) {
      const v = o[k];
      if (typeof v === "boolean") {
        return v;
      }
    }
    return false;
  }

  function normalizeShortcutRow(raw: unknown): Row | null {
    const o = (raw ?? {}) as Record<string, unknown>;
    const fn =
      readStrLoose(o, "fileName", "FileName", "file_name") ||
      (typeof (raw as Row)?.fileName === "string"
        ? String((raw as Row).fileName).trim()
        : "");
    if (!fn) {
      return null;
    }
    const stem = fn.replace(/\.(lnk|url)$/i, "");
    const low = fn.toLowerCase();
    const isUrl =
      readBoolLoose(o, "isUrl", "IsUrl") || low.endsWith(".url");
    return ShortcutDTO.createFrom({
      fileName: fn,
      displayName: readStrLoose(o, "displayName", "DisplayName") || stem,
      iconUrl: readStrLoose(o, "iconUrl", "IconURL", "iconURL"),
      pinned: readBoolLoose(o, "pinned", "Pinned"),
      isPlatformExe: readBoolLoose(o, "isPlatformExe", "IsPlatformExe"),
      isUrl,
    });
  }

  function applyRows(list: unknown[]): void {
    const m: Record<string, Row> = {};
    const p: string[] = [];
    const d: string[] = [];
    for (const raw of list) {
      const r = normalizeShortcutRow(raw);
      if (!r) {
        continue;
      }
      m[r.fileName] = r;
      if (r.pinned) {
        p.push(r.fileName);
      } else {
        d.push(r.fileName);
      }
    }
    meta = m;
    pinNames = p;
    dropNames = d;
  }

  async function refreshFromServer(): Promise<void> {
    try {
      const list = await Shortcuts.ListShortcuts(platformName);
      applyRows(list as unknown[]);
    } catch {
      applyRows([]);
    }
  }

  async function persist(): Promise<void> {
    try {
      await Shortcuts.SaveShortcutOrder(platformName, pinNames, dropNames);
    } catch {
      await refreshFromServer();
    }
  }

  /** Params must match `$:` args so Svelte subscribes when pin/drop lists or drag state change. */
  function computeDisplayPinned(
    pins: string[],
    draggingId: string | null,
    dragSourceZone: Zone | null,
    dragFromIndex: number | null,
    dragOverZone: Zone | null,
    dragOverIndex: number | null,
  ): (string | null)[] {
    if (
      draggingId == null ||
      dragSourceZone == null ||
      dragFromIndex == null
    ) {
      return [...pins];
    }
    const overIdx = dragOverIndex ?? dragFromIndex;
    const overZ = dragOverZone ?? dragSourceZone;
    if (dragSourceZone === "pinned") {
      if (overZ === "pinned") {
        return previewSlots(pins, dragFromIndex, overIdx);
      }
      return pins.filter((id) => id !== draggingId);
    }
    if (overZ === "pinned") {
      return previewInsertGap(pins, overIdx);
    }
    return [...pins];
  }

  function computeDisplayDrop(
    drops: string[],
    draggingId: string | null,
    dragSourceZone: Zone | null,
    dragFromIndex: number | null,
    dragOverZone: Zone | null,
    dragOverIndex: number | null,
  ): (string | null)[] {
    if (
      draggingId == null ||
      dragSourceZone == null ||
      dragFromIndex == null
    ) {
      return [...drops];
    }
    const overIdx = dragOverIndex ?? dragFromIndex;
    const overZ = dragOverZone ?? dragSourceZone;
    if (dragSourceZone === "dropdown") {
      if (overZ === "dropdown") {
        return previewSlots(drops, dragFromIndex, overIdx);
      }
      return drops.filter((id) => id !== draggingId);
    }
    if (overZ === "dropdown") {
      return previewInsertGap(drops, overIdx);
    }
    return [...drops];
  }

  $: displayPinned = computeDisplayPinned(
    pinNames,
    draggingId,
    dragSourceZone,
    dragFromIndex,
    dragOverZone,
    dragOverIndex,
  );
  $: displayDrop = computeDisplayDrop(
    dropNames,
    draggingId,
    dragSourceZone,
    dragFromIndex,
    dragOverZone,
    dragOverIndex,
  );

  function stripCellForGhostClone(el: HTMLElement): void {
    el.removeAttribute("data-dnd-cell");
    el.removeAttribute("data-dnd-visual");
    el.removeAttribute("data-dnd-name");
    el.removeAttribute("data-dnd-gap");
    el.removeAttribute("tabindex");
    el.removeAttribute("role");
    el.classList.remove("shortcutDndCell");
    el.style.cursor = "grabbing";
    el.style.width = "100%";
    el.style.height = "100%";
    el.style.boxSizing = "border-box";
    el.style.margin = "0";
    el.style.maxWidth = "none";
    el.style.maxHeight = "none";
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

    dragSourceZone = pd.zone;
    dragFromIndex = pd.fromIndex;
    draggingId = pd.id;
    dragOverZone = pd.zone;
    dragOverIndex = pd.fromIndex;
    ghostW = pd.width;
    ghostH = pd.height;
    ghostX = e.clientX - pd.grabOffsetX;
    ghostY = e.clientY - pd.grabOffsetY;
    document.body.style.userSelect = "none";
    hitTestDragOver(e.clientX, e.clientY);
  }

  function commitDrag(): void {
    if (
      dragSourceZone == null ||
      dragFromIndex == null ||
      dragOverZone == null ||
      dragOverIndex == null ||
      draggingId == null
    ) {
      return;
    }
    const fromZ = dragSourceZone;
    const toZ = dragOverZone;
    const from = dragFromIndex;
    const to = dragOverIndex;
    const id = draggingId;

    if (fromZ === toZ) {
      if (from === to) {
        return;
      }
      if (fromZ === "pinned") {
        pinNames = moveItem(pinNames, from, to);
      } else {
        dropNames = moveItem(dropNames, from, to);
      }
    } else {
      let p = [...pinNames];
      let d = [...dropNames];
      if (fromZ === "pinned") {
        p.splice(from, 1);
        d.splice(to, 0, id);
      } else {
        d.splice(from, 1);
        p.splice(to, 0, id);
      }
      pinNames = p;
      dropNames = d;
    }
    void persist();
  }

  function endPointerDrag(commit: boolean): void {
    if (pendingDrag && dragSourceZone === null) {
      pendingDrag = null;
      return;
    }
    if (dragSourceZone !== null) {
      const shouldCommit =
        commit &&
        dragOverZone != null &&
        dragOverIndex != null &&
        (dragSourceZone !== dragOverZone ||
          dragFromIndex !== dragOverIndex);
      armSuppressClickAfterDrag();
      if (shouldCommit) {
        commitDrag();
      }
      dragSourceZone = null;
      dragFromIndex = null;
      dragOverZone = null;
      dragOverIndex = null;
      draggingId = null;
      dragVisualClone = null;
      pendingDrag = null;
      document.body.style.userSelect = "";
    } else {
      pendingDrag = null;
    }
  }

  function pointInRect(
    x: number,
    y: number,
    r: DOMRect,
    inset = 0,
  ): boolean {
    return (
      x >= r.left - inset &&
      x <= r.right + inset &&
      y >= r.top - inset &&
      y <= r.bottom + inset
    );
  }

  /**
   * Hit-test using cell bounding rects (not elementFromPoint). The open dropdown
   * stacks above the pinned strip; geometry still matches the real tiles underneath.
   */
  function hitTestDragOver(clientX: number, clientY: number): void {
    if (dragSourceZone === null || dragFromIndex === null || !draggingId) {
      return;
    }
    const fromIdx = dragFromIndex;
    const srcZone = dragSourceZone;

    const tryZone = (zone: Zone, root: HTMLDivElement | null, names: string[]): boolean => {
      if (!root) {
        return false;
      }
      const cells = root.querySelectorAll("[data-dnd-cell]");
      for (const cell of [...cells]) {
        const r = cell.getBoundingClientRect();
        if (!pointInRect(clientX, clientY, r)) {
          continue;
        }
        const h = cell as HTMLElement;
        if (h.dataset.dndGap === "true") {
          const visual = Number(h.dataset.dndVisual);
          if (!Number.isNaN(visual)) {
            dragOverZone = zone;
            dragOverIndex = visual;
            return true;
          }
          continue;
        }
        const id = h.dataset.dndName ?? "";
        if (!id) {
          continue;
        }
        dragOverZone = zone;
        if (zone === "pinned") {
          if (srcZone === "pinned") {
            dragOverIndex = insertionIndexFromTileHover(
              names,
              fromIdx,
              id,
              clientX,
              h,
            );
          } else {
            dragOverIndex = insertionIndexExternalDrag(names, id, clientX, h);
          }
        } else if (srcZone === "dropdown") {
          dragOverIndex = insertionIndexFromTileHover(
            names,
            fromIdx,
            id,
            clientX,
            h,
          );
        } else {
          dragOverIndex = insertionIndexExternalDrag(names, id, clientX, h);
        }
        return true;
      }
      const br = root.getBoundingClientRect();
      if (pointInRect(clientX, clientY, br, 4)) {
        if (
          zone === "pinned" &&
          ddOpen &&
          dropListEl &&
          pointInRect(clientX, clientY, dropListEl.getBoundingClientRect())
        ) {
          return false;
        }
        dragOverZone = zone;
        if (srcZone === zone) {
          dragOverIndex = fromIdx;
        } else if (names.length === 0) {
          dragOverIndex = 0;
        } else {
          dragOverIndex = names.length;
        }
        return true;
      }
      return false;
    };

    if (tryZone("pinned", pinListEl, pinNames)) {
      return;
    }
    if (ddOpen && tryZone("dropdown", dropListEl, dropNames)) {
      return;
    }
  }

  function onWindowPointerMove(e: PointerEvent): void {
    if (!pendingDrag && dragSourceZone === null) {
      return;
    }

    if (pendingDrag && dragSourceZone === null) {
      const dx = e.clientX - pendingDrag.startX;
      const dy = e.clientY - pendingDrag.startY;
      if (dx * dx + dy * dy < dragThresholdPx * dragThresholdPx) {
        return;
      }
      beginPointerDrag(e, pendingDrag);
    }

    if (dragSourceZone !== null && pendingDrag) {
      ghostX = e.clientX - pendingDrag.grabOffsetX;
      ghostY = e.clientY - pendingDrag.grabOffsetY;
      hitTestDragOver(e.clientX, e.clientY);
    }
  }

  function onWindowPointerUp(_e: PointerEvent): void {
    if (pendingDrag && dragSourceZone === null) {
      pendingDrag = null;
      return;
    }
    endPointerDrag(true);
  }

  function onWindowPointerCancel(_e: PointerEvent): void {
    endPointerDrag(false);
  }

  function onWindowKeyDown(e: KeyboardEvent): void {
    if (e.key !== "Escape") {
      return;
    }
    if (dragSourceZone === null && !pendingDrag) {
      return;
    }
    e.preventDefault();
    endPointerDrag(false);
  }

  function onCellPointerDown(e: PointerEvent, zone: Zone, id: string): void {
    if (e.button !== 0) {
      return;
    }
    const arr = zone === "pinned" ? pinNames : dropNames;
    const from = arr.indexOf(id);
    if (from < 0) {
      return;
    }
    const el = e.currentTarget as HTMLElement;
    const r = el.getBoundingClientRect();
    pendingDrag = {
      zone,
      fromIndex: from,
      id,
      startX: e.clientX,
      startY: e.clientY,
      grabOffsetX: e.clientX - r.left,
      grabOffsetY: e.clientY - r.top,
      width: r.width,
      height: r.height,
      cellEl: el,
    };
  }

  function onShortcutClick(row: Row): void {
    if (suppressClickExpire) {
      clearTimeout(suppressClickExpire);
      suppressClickExpire = null;
    }
    if (suppressNextClick) {
      suppressNextClick = false;
      return;
    }
    void runRow(row, false);
  }

  function ghostCloneMount(node: HTMLElement, clone: HTMLElement | null) {
    function apply(c: HTMLElement | null) {
      node.replaceChildren();
      if (c) {
        node.appendChild(c);
      }
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

  function ctxItemsFor(fn: string): () => MenuItemDef[] {
    return () => {
      const row = meta[fn];
      if (!row) {
        return [];
      }
      const tr = get(t);
      return [
        {
          label: tr("Context_RunAdmin"),
          action: () => {
            void runRow(row, true);
          },
        },
        {
          label: tr("Context_Hide"),
          action: () => {
            void hideRow(row.fileName);
          },
        },
      ];
    };
  }

  function ctxPlatformItems(): MenuItemDef[] {
    const tr = get(t);
    return [
      {
        label: tr("Context_RunAdmin"),
        action: () => {
          void LaunchPlatformAs(platformName, true).catch((e: unknown) => {
            pushToast({
              type: "error",
              message: formatToastWithError(tr("Toast_LaunchFailed"), e),
              duration: 8000,
            });
          });
        },
      },
    ];
  }

  async function runRow(row: Row, admin: boolean): Promise<void> {
    let a = admin;
    const tr = get(t);
    if (a && row.isUrl) {
      pushToast({
        type: "warning",
        message: tr("Toast_UrlAdminErr"),
        duration: 8000,
      });
      a = false;
    }
    try {
      const sel = get(selectedAccount);
      const uid =
        sel.platformKey && sel.platformKey === platformName ? sel.uniqueId : "";
      await Shortcuts.RunShortcut(platformName, row.fileName, a, uid);
    } catch (e) {
      pushToast({
        type: "error",
        message: formatToastWithError(tr("Toast_LaunchFailed"), e),
        duration: 8000,
      });
    }
  }

  async function hideRow(fileName: string): Promise<void> {
    const tr = get(t);
    try {
      await Shortcuts.HideShortcut(platformName, fileName);
      await refreshFromServer();
    } catch (e) {
      pushToast({
        type: "error",
        message: formatToastWithError(tr("Toast_SwitchFailed"), e),
        duration: 8000,
      });
    }
  }

  async function openFolder(): Promise<void> {
    const tr = get(t);
    try {
      await Shortcuts.OpenShortcutFolder(platformName);
      pushToast({
        type: "info",
        message: tr("Toast_PlaceShortcutFiles"),
        duration: 8000,
      });
    } catch (e) {
      pushToast({
        type: "error",
        message: formatToastWithError(tr("Toast_LaunchFailed"), e),
        duration: 8000,
      });
    }
  }

  let offEv: (() => void) | undefined;
  let teardownDnd: (() => void) | undefined;

  onMount(() => {
    void (async () => {
      try {
        includeMainExe = await HasShortcutMainExe(platformName);
      } catch {
        includeMainExe = false;
      }
      await refreshFromServer();
      void Shortcuts.ScanShortcuts(platformName);
    })();

    offEv = Events.On("shortcuts-updated", (ev) => {
      const raw = ev.data;
      const p =
        raw instanceof ListPayload
          ? raw
          : ListPayload.createFrom(raw as Record<string, unknown>);
      if (p.platformKey !== platformName) {
        return;
      }
      applyRows(p.shortcuts ?? []);
    });

    const move = (e: PointerEvent) => onWindowPointerMove(e);
    const up = (e: PointerEvent) => onWindowPointerUp(e);
    const cancel = (e: PointerEvent) => onWindowPointerCancel(e);
    const key = (e: KeyboardEvent) => onWindowKeyDown(e);
    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", up);
    window.addEventListener("pointercancel", cancel);
    window.addEventListener("keydown", key, true);
    teardownDnd = () => {
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", up);
      window.removeEventListener("pointercancel", cancel);
      window.removeEventListener("keydown", key, true);
    };
  });

  onDestroy(() => {
    offEv?.();
    teardownDnd?.();
    document.body.style.userSelect = "";
    if (suppressClickExpire) {
      clearTimeout(suppressClickExpire);
    }
  });
</script>

<div class="gameShortcutBar">
  <div
    bind:this={pinListEl}
    class="shortcuts shortcutDndGrid"
    class:expandShortcuts={ddOpen && pinNames.length === 0 && dropNames.length > 0}
    role="list"
    aria-label={$t("Stats_GameShortcuts")}
  >
    {#each displayPinned as slot, i (slot === null ? `pg-${i}` : `p-${i}-${slot}`)}
      {#if slot === null}
        <div
          class="shortcutDndGap shortcutPlaceholder"
          role="presentation"
          data-dnd-cell
          data-dnd-gap="true"
          data-dnd-visual={i}
        ></div>
      {:else}
        {@const row = meta[slot]}
        {#if row}
          <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
          <div
            class="shortcutDndCell"
            role="listitem"
            data-dnd-cell
            data-dnd-visual={i}
            data-dnd-name={slot}
            on:pointerdown={(e) => onCellPointerDown(e, "pinned", slot)}
          >
            <button
              type="button"
              class="HasContextMenu"
              aria-label={row.displayName}
              use:tooltip={row.displayName}
              use:contextMenu={ctxItemsFor(slot)}
              on:click={() => onShortcutClick(row)}
            >
              <img src={row.iconUrl} alt="" draggable="false" />
            </button>
          </div>
        {/if}
      {/if}
    {/each}
  </div>

  <div class="shortcutDropdownWrap">
    <button
      type="button"
      id="shortcutDropdownBtn"
      class="square"
      class:flip={ddOpen}
      aria-expanded={ddOpen}
      aria-label={$t("Tooltip_ExpandShortcuts")}
      use:tooltip={$t("Tooltip_ExpandShortcuts")}
      on:click={() => (ddOpen = !ddOpen)}
    >
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 448 512" aria-hidden="true"
        ><path
          d="M201.4 137.4c12.5-12.5 32.8-12.5 45.3 0l160 160c12.5 12.5 12.5 32.8 0 45.3s-32.8 12.5-45.3 0L224 218.7 86.6 342.6c-12.5 12.5-32.8 12.5-45.3 0s-12.5-32.8 0-45.3l160-160z"
        /></svg
      >
    </button>

    {#if ddOpen}
      <div class="shortcutDropdown gameShortcuts open" id="shortcutDropdown">
        <div
          bind:this={dropListEl}
          class="shortcutDropdownItems shortcutDndGrid"
          role="list"
          aria-label={$t("Stats_GameShortcuts")}
        >
          {#each displayDrop as slot, i (slot === null ? `dg-${i}` : `d-${i}-${slot}`)}
            {#if slot === null}
              <div
                class="shortcutDndGap shortcutPlaceholder"
                role="presentation"
                data-dnd-cell
                data-dnd-gap="true"
                data-dnd-visual={i}
              ></div>
            {:else}
              {@const row = meta[slot]}
              {#if row}
                <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
                <div
                  class="shortcutDndCell"
                  role="listitem"
                  data-dnd-cell
                  data-dnd-visual={i}
                  data-dnd-name={slot}
                  on:pointerdown={(e) => onCellPointerDown(e, "dropdown", slot)}
                >
                  <button
                    type="button"
                    class="HasContextMenu"
                    aria-label={row.displayName}
                    use:tooltip={row.displayName}
                    use:contextMenu={ctxItemsFor(slot)}
                    on:click={() => onShortcutClick(row)}
                  >
                    <img src={row.iconUrl} alt="" draggable="false" />
                  </button>
                </div>
              {/if}
            {/if}
          {/each}
        </div>
        <button
          type="button"
          id="btnOpenShortcutFolder"
          aria-label={$t("Tooltip_ShortcutFolder")}
          use:tooltip={$t("Tooltip_ShortcutFolder")}
          on:click={() => openFolder()}
        >
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 448 512" aria-hidden="true"
            ><path
              d="M416 208H272V64c0-17.67-14.33-32-32-32h-32c-17.67 0-32 14.33-32 32v144H32c-17.67 0-32 14.33-32 32v32c0 17.67 14.33 32 32 32h144v144c0 17.67 14.33 32 32 32h32c17.67 0 32-14.33 32-32V304h144c17.67 0 32-14.33 32-32v-32c0-17.67-14.33-32-32-32z"
            /></svg
          >
        </button>
      </div>
    {/if}
  </div>

  {#if includeMainExe}
    <button
      type="button"
      id="btnStartPlat"
      class="square actionbar__launch"
      aria-label={$t("Button_Launch")}
      use:tooltip={$t("Tooltip_Launch")}
      use:contextMenu={() => ctxPlatformItems()}
      on:click={() => triggerPlatformAction("launch")}
    >
      {#if $platformExeIconUrl && !iconBroken}
        <img
          class="actionbar__exeicon"
          src={$platformExeIconUrl}
          alt=""
          on:error={() => (iconBroken = true)}
        />
      {:else}
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" aria-hidden="true"
          ><path
            d="M288 32c0-17.7-14.3-32-32-32s-32 14.3-32 32V274.7l-73.4-73.4c-12.5-12.5-32.8-12.5-45.3 0s-12.5 32.8 0 45.3l128 128c12.5 12.5 32.8 12.5 45.3 0l128-128c12.5-12.5 12.5-32.8 0-45.3s-32.8-12.5-45.3 0L288 274.7V32z"
          /></svg
        >
      {/if}
    </button>
  {:else}
    <button
      type="button"
      class="actionbar__launch square"
      aria-label={$t("Button_Launch")}
      use:tooltip={$t("Tooltip_Launch")}
      use:contextMenu={() => ctxPlatformItems()}
      on:click={() => triggerPlatformAction("launch")}
    >
      {#if $platformExeIconUrl && !iconBroken}
        <img
          class="actionbar__exeicon"
          src={$platformExeIconUrl}
          alt=""
          on:error={() => (iconBroken = true)}
        />
      {:else}
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" aria-hidden="true"
          ><path
            d="M288 32c0-17.7-14.3-32-32-32s-32 14.3-32 32V274.7l-73.4-73.4c-12.5-12.5-32.8-12.5-45.3 0s-12.5 32.8 0 45.3l128 128c12.5 12.5 32.8 12.5 45.3 0l128-128c12.5-12.5 12.5-32.8 0-45.3s-32.8-12.5-45.3 0L288 274.7V32z"
          /></svg
        >
      {/if}
    </button>
  {/if}
</div>

{#if draggingId && dragVisualClone}
  <div
    class="shortcutDndGhost"
    style:left="{ghostX}px"
    style:top="{ghostY}px"
    style:width="{ghostW}px"
    style:height="{ghostH}px"
    aria-hidden="true"
  >
    <div class="shortcutDndGhostMirror" use:ghostCloneMount={dragVisualClone}></div>
  </div>
{/if}

<style lang="scss">
  .gameShortcutBar {
    position: relative;
    display: flex;
    flex-direction: row;
    align-items: center;
    height: 100%;
    gap: 0;
  }

  .shortcutDropdownWrap {
    position: relative;
    display: flex;
    align-items: center;
  }

  #shortcutDropdown.open {
    display: flex;
    flex-direction: column;
    bottom: calc(100% + 10px);
    left: 50%;
    transform: translateX(-50%);
    min-width: 168px;
    /* Let pointer events reach the pinned strip underneath empty panel area */
    pointer-events: none;
    .shortcutDropdownItems,
    #btnOpenShortcutFolder {
      pointer-events: auto;
    }
  }

  .shortcutDndGrid {
    display: flex;
    flex-flow: row wrap;
    align-content: flex-start;
    align-items: center;
    min-width: 0;
  }

  .shortcutDndCell {
    cursor: grab;
    touch-action: none;
    flex: 0 0 auto;
    width: 32px;
    height: 32px;
    box-sizing: border-box;

    button {
      width: 100%;
      height: 100%;
      margin: 0;
    }
  }

  .shortcutDndGap {
    flex: 0 0 auto;
    width: 32px;
    height: 32px;
    box-sizing: border-box;
    pointer-events: auto;
  }

  .shortcutDndGhost {
    position: fixed;
    margin: 0;
    z-index: 10001;
    pointer-events: none;
    opacity: 0.96;
    cursor: grabbing;
    box-shadow: 0 12px 36px rgba(0, 0, 0, 0.5);
    left: 0;
    top: 0;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    background: var(--accountListBackground, #11181d);
    border-radius: 6px;
  }

  .shortcutDndGhostMirror {
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
    > :global(*) {
      box-sizing: border-box;
      min-width: 0;
      min-height: 0;
      width: 100%;
      height: 100%;
      margin: 0;
      flex: 1 1 auto;
    }

    :global(button) {
      display: flex;
      align-items: center;
      justify-content: center;
      width: 100%;
      height: 100%;
      margin: 0;
      padding: 3px;
      box-sizing: border-box;
      min-width: 0;
      min-height: 0;
    }

    :global(img) {
      width: 100%;
      height: 100%;
      max-width: 100%;
      max-height: 100%;
      object-fit: contain;
      display: block;
      flex: 0 1 auto;
    }
  }

  #shortcutDropdownBtn svg {
    width: 1rem;
    height: 1rem;
    fill: currentColor;
  }

  #btnOpenShortcutFolder svg {
    width: 0.85rem;
    height: 0.85rem;
    fill: currentColor;
  }

  .actionbar__launch {
    min-width: 36px;
    min-height: 36px;
    padding: 4px;
  }

  .actionbar__exeicon {
    width: 1.75rem;
    height: 1.75rem;
    object-fit: contain;
    display: block;
    border-radius: 4px;
  }
</style>

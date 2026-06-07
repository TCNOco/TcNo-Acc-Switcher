<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import { get } from "svelte/store";
  import { Events } from "@wailsio/runtime";
  import * as Shortcuts from "wails-shortcuts-service";
  import { ImportDroppedShortcuts } from "../../bindings/TcNo-Acc-Switcher/internal/shortcuts/service.js";
  import {
    ListPayload,
    ShortcutDTO,
  } from "../../bindings/TcNo-Acc-Switcher/internal/shortcuts/models.js";
  import {
    platformActionBusy,
    selectedAccount,
    platformLiveSessionId,
    requestPlatformAccountsRefresh,
  } from "../stores/platformPage";
  import { t } from "../stores/i18n";
  import { pushToast } from "../stores/toast";
  import { HasShortcutMainExe } from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
  import { formatToastWithError, formatWailsError } from "../lib/formatWailsError";
  import { fileDropAcceptor, type FileDropAcceptor } from "../stores/fileDrop";
  import { pathsAreOnlyProfileMedia } from "../lib/profileImageDrop";
  import { runShortcut, hideShortcut, openShortcutFolder, buildShortcutContextMenu, buildPlatformContextMenu } from "../lib/shortcutActions";
  import {
    insertionIndexExternalDrag,
    insertionIndexFromTileHover,
    moveItem,
    previewInsertGap,
    previewSlots,
  } from "../lib/reorderList";
  import { tooltip } from "../lib/actions/tooltip";
  import { contextMenu } from "../lib/actions/contextMenu";
  import {
    platformExeIconUrl,
    triggerPlatformAction,
  } from "../stores/platformPage";
  import ShortcutZone from "./ShortcutZone.svelte";
  import "../styles/gameshortcutbar.scss";

  export let platformName: string;

  let resolvedSwapAccountLabelForMenu = "";
  let resolveSwapMenuLabelSeq = 0;

  $: {
    const sel = $selectedAccount;
    const uid = String(sel.uniqueId ?? "").trim();
    if (sel.platformKey !== platformName || !uid) {
      resolvedSwapAccountLabelForMenu = "";
    } else {
      const seq = ++resolveSwapMenuLabelSeq;
      void Shortcuts.ResolveAccountShortcutStem(
        platformName,
        uid,
        sel.displayName,
        sel.accountLogin,
      ).then((name: string) => {
        if (seq === resolveSwapMenuLabelSeq) {
          resolvedSwapAccountLabelForMenu = name;
        }
      });
    }
  }

  $: launchTooltipText = (() => {
    const sel = $selectedAccount;
    const live = $platformLiveSessionId;
    const uid = String(sel.uniqueId ?? "").trim();
    const onPlatform = sel.platformKey === platformName;
    const hasSel = onPlatform && uid !== "";
    const liveUid = String(live.uniqueId ?? "").trim();
    const liveHere = live.platformKey === platformName && liveUid !== "";
    if (!hasSel || (liveHere && liveUid === uid)) {
      return $t("Tooltip_Launch");
    }
    const rawName =
      hasSel
        ? resolvedSwapAccountLabelForMenu ||
          String(sel.displayName ?? "").trim() ||
          uid
        : "";
    const name =
      rawName.trim() !== ""
        ? rawName.trim()
        : $t("Tooltip_LaunchAfterSwitch_NameFallback");
    return $t("Tooltip_LaunchAfterSwitch", { name });
  })();
  $: isActionBusy = $platformActionBusy.busy;

  type Row = InstanceType<typeof ShortcutDTO>;
  type Zone = "pinned" | "dropdown";

  let prevPlatform = platformName;
  $: if (platformName !== prevPlatform) {
    prevPlatform = platformName;
    iconBroken = false;
    ddOpen = false;
    dragSourceZone = null;
    dragFromIndex = null;
    dragOverZone = null;
    dragOverIndex = null;
    draggingId = null;
    dragVisualClone = null;
    pendingDrag = null;
    void (async () => {
      try {
        includeMainExe = await HasShortcutMainExe(platformName);
      } catch {
        includeMainExe = false;
      }
      await refreshFromServer();
      void Shortcuts.ScanShortcuts(platformName);
    })();
  }

  let includeMainExe = false;
  let iconBroken = false;

  $: ctxPlatformItems = buildPlatformContextMenu(platformName);
  let pinNames: string[] = [];
  let dropNames: string[] = [];
  let meta: Record<string, Row> = {};
  let ddOpen = false;

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

  function readStrLoose(o: Record<string, unknown>, ...keys: string[]): string {
    for (const k of keys) {
      const v = o[k];
      if (typeof v === "string" && v.trim() !== "") return v.trim();
    }
    return "";
  }

  function readBoolLoose(
    o: Record<string, unknown>,
    ...keys: string[]
  ): boolean {
    for (const k of keys) {
      const v = o[k];
      if (typeof v === "boolean") return v;
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
    if (!fn) return null;
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
      if (!r) continue;
      m[r.fileName] = r;
      if (r.pinned) p.push(r.fileName);
      else d.push(r.fileName);
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
    el.querySelectorAll("label[for]").forEach((n) =>
      n.removeAttribute("for"),
    );
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
    document.body.dataset.dragging = "true";
    hitTestDragOver(e.clientX, e.clientY);
  }

  function moveWithinZone(zone: string, from: number, to: number): void {
    if (zone === "pinned") pinNames = moveItem(pinNames, from, to);
    else dropNames = moveItem(dropNames, from, to);
  }

  function moveAcrossZones(
    fromZ: string,
    toZ: string,
    from: number,
    to: number,
    id: string,
  ): void {
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

  function commitDrag(): void {
    if (
      dragSourceZone == null ||
      dragFromIndex == null ||
      dragOverZone == null ||
      dragOverIndex == null ||
      draggingId == null
    )
      return;

    if (dragSourceZone === dragOverZone) {
      if (dragFromIndex === dragOverIndex) return;
      moveWithinZone(dragSourceZone, dragFromIndex, dragOverIndex);
    } else {
      moveAcrossZones(
        dragSourceZone,
        dragOverZone,
        dragFromIndex,
        dragOverIndex,
        draggingId,
      );
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
      if (shouldCommit) commitDrag();
      dragSourceZone = null;
      dragFromIndex = null;
      dragOverZone = null;
      dragOverIndex = null;
      draggingId = null;
      dragVisualClone = null;
      pendingDrag = null;
      document.body.style.userSelect = "";
      delete document.body.dataset.dragging;
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

  function hitTestDragOver(clientX: number, clientY: number): void {
    if (dragSourceZone === null || dragFromIndex === null || !draggingId)
      return;
    const fromIdx = dragFromIndex;
    const srcZone = dragSourceZone;

    function computeDropIndex(
      zone: Zone,
      srcZone: Zone,
      names: string[],
      fromIdx: number,
      id: string,
      cell: HTMLElement,
    ): number {
      const sameSource =
        (zone === "pinned" && srcZone === "pinned") ||
        (zone === "dropdown" && srcZone === "dropdown");
      if (sameSource)
        return insertionIndexFromTileHover(
          names,
          fromIdx,
          id,
          clientX,
          cell,
        );
      return insertionIndexExternalDrag(names, id, clientX, cell);
    }

    function tryBoundary(
      zone: Zone,
      srcZone: Zone,
      fromIdx: number,
      namesLen: number,
      root: HTMLDivElement,
    ): boolean {
      const br = root.getBoundingClientRect();
      if (!pointInRect(clientX, clientY, br, 4)) return false;

      if (
        zone === "pinned" &&
        ddOpen &&
        dropListEl &&
        pointInRect(clientX, clientY, dropListEl.getBoundingClientRect())
      )
        return false;

      dragOverZone = zone;
      if (srcZone === zone) dragOverIndex = fromIdx;
      else if (namesLen === 0) dragOverIndex = 0;
      else dragOverIndex = namesLen;
      return true;
    }

    const tryZone = (
      zone: Zone,
      root: HTMLDivElement | null,
      names: string[],
    ): boolean => {
      if (!root) return false;

      const cells = root.querySelectorAll("[data-dnd-cell]");
      for (const cell of [...cells]) {
        const r = cell.getBoundingClientRect();
        if (!pointInRect(clientX, clientY, r)) continue;

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
        if (!id) continue;

        dragOverZone = zone;
        dragOverIndex = computeDropIndex(
          zone,
          srcZone,
          names,
          fromIdx,
          id,
          h,
        );
        return true;
      }

      return tryBoundary(zone, srcZone, fromIdx, names.length, root);
    };

    if (tryZone("pinned", pinListEl, pinNames)) return;
    if (ddOpen && tryZone("dropdown", dropListEl, dropNames)) return;
  }

  function onWindowPointerMove(e: PointerEvent): void {
    if (!pendingDrag && dragSourceZone === null) return;

    if (pendingDrag && dragSourceZone === null) {
      const dx = e.clientX - pendingDrag.startX;
      const dy = e.clientY - pendingDrag.startY;
      if (dx * dx + dy * dy < dragThresholdPx * dragThresholdPx) return;
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
    if (e.key !== "Escape") return;
    if (dragSourceZone === null && !pendingDrag) return;
    e.preventDefault();
    endPointerDrag(false);
  }

  function onCellPointerDown(
    e: PointerEvent,
    z: string,
    id: string,
  ): void {
    if (e.button !== 0) return;
    const zone = z as Zone;
    const arr = zone === "pinned" ? pinNames : dropNames;
    const from = arr.indexOf(id);
    if (from < 0) return;
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
    if (isActionBusy) return;
    if (suppressClickExpire) {
      clearTimeout(suppressClickExpire);
      suppressClickExpire = null;
    }
    if (suppressNextClick) {
      suppressNextClick = false;
      return;
    }
    void runShortcut(
      platformName,
      row.fileName,
      false,
      row.isUrl,
      () => requestPlatformAccountsRefresh(platformName),
    );
  }

  function ghostCloneMount(
    node: HTMLElement,
    clone: HTMLElement | null,
  ) {
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

  function ctxMenuForFile(fn: string): () => import("../stores/contextMenu").MenuItemDef[] {
    const r = meta[fn];
    return buildShortcutContextMenu({
      platformName,
      fileName: fn,
      swapLabel: resolvedSwapAccountLabelForMenu,
      onRunAsAdmin: () => {
        if (r)
          void runShortcut(
            platformName,
            fn,
            true,
            r.isUrl,
            () => requestPlatformAccountsRefresh(platformName),
          );
      },
      onHide: () => {
        void hideShortcut(platformName, fn, () => refreshFromServer());
      },
    });
  }

  function isNoShortcutFilesDrop(err: unknown): boolean {
    const s = formatWailsError(err) || String(err);
    return (
      s.trim() === "Toast_ShortcutImportUnsupported" ||
      s.includes("Toast_ShortcutImportUnsupported")
    );
  }

  const shortcutFileDropAcceptor: FileDropAcceptor = {
    labelKey: "DropOverlay_CopyShortcut",
    handle: async (paths: string[]) => {
      const tr = get(t);
      try {
        const n = await ImportDroppedShortcuts(platformName, paths);
        ddOpen = true;
        pushToast({
          type: "success",
          message: tr("Toast_ShortcutImported", { count: n }),
          duration: 6000,
        });
      } catch (e: unknown) {
        if (isNoShortcutFilesDrop(e)) {
          if (pathsAreOnlyProfileMedia(paths)) return;
          pushToast({
            type: "warning",
            message: tr("Toast_ShortcutImportUnsupported"),
            duration: 8000,
          });
        } else {
          pushToast({
            type: "error",
            message: formatToastWithError(
              tr("Toast_ShortcutImportFailed"),
              e,
            ),
            duration: 8000,
          });
        }
      }
    },
  };

  let offEv: (() => void) | undefined;
  let teardownDnd: (() => void) | undefined;

  onMount(() => {
    fileDropAcceptor.set(shortcutFileDropAcceptor);

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
      if (p.platformKey !== platformName) return;
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
    resolveSwapMenuLabelSeq++;
    fileDropAcceptor.update((cur) =>
      cur === shortcutFileDropAcceptor ? null : cur,
    );
    offEv?.();
    teardownDnd?.();
    document.body.style.userSelect = "";
    delete document.body.dataset.dragging;
    if (suppressClickExpire) {
      clearTimeout(suppressClickExpire);
    }
  });
</script>

<div class="gameShortcutBar">
  <ShortcutZone
    bind:el={pinListEl}
    zone="pinned"
    zoneClass="shortcuts shortcutDndGrid"
    expandClass={ddOpen && pinNames.length === 0 && dropNames.length > 0}
    displaySlots={displayPinned}
    {meta}
    contextMenuFor={ctxMenuForFile}
    onTileClick={onShortcutClick}
    onCellPointerDown={onCellPointerDown}
  />

  <div class="shortcutDropdownWrap">
    <button
      type="button"
      id="shortcutDropdownBtn"
      class="square"
      class:flip={ddOpen}
      aria-expanded={ddOpen}
      aria-label={$t("Tooltip_ExpandShortcuts")}
      on:click={() => (ddOpen = !ddOpen)}
    >
      <svg
        xmlns="http://www.w3.org/2000/svg"
        viewBox="0 0 448 512"
        aria-hidden="true"
        ><path
          d="M201.4 137.4c12.5-12.5 32.8-12.5 45.3 0l160 160c12.5 12.5 12.5 32.8 0 45.3s-32.8 12.5-45.3 0L224 218.7 86.6 342.6c-12.5 12.5-32.8 12.5-45.3 0s-12.5-32.8 0-45.3l160-160z"
        /></svg
      >
    </button>

    {#if ddOpen}
      <div class="shortcutDropdown gameShortcuts open" id="shortcutDropdown">
        <ShortcutZone
          bind:el={dropListEl}
          zone="dropdown"
          zoneClass="shortcutDropdownItems shortcutDndGrid"
          displaySlots={displayDrop}
          {meta}
          contextMenuFor={ctxMenuForFile}
          onTileClick={onShortcutClick}
          onCellPointerDown={onCellPointerDown}
        />
        <button
          type="button"
          id="btnOpenShortcutFolder"
          aria-label={$t("Tooltip_ShortcutFolder")}
          on:click={() => openShortcutFolder(platformName)}
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            viewBox="0 0 448 512"
            aria-hidden="true"
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
      disabled={isActionBusy}
      use:tooltip={launchTooltipText || $t("Button_Launch")}
      use:contextMenu={ctxPlatformItems}
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
      disabled={isActionBusy}
      use:tooltip={launchTooltipText || $t("Button_Launch")}
      use:contextMenu={ctxPlatformItems}
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
    <div
      class="shortcutDndGhostMirror"
      use:ghostCloneMount={dragVisualClone}
    ></div>
  </div>
{/if}

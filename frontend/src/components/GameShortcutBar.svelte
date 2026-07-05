<script context="module" lang="ts">
  import type { ShortcutReorderCommand as PaletteShortcutReorderCommand } from "../lib/dragReorderShortcuts";

  export type ShortcutCommandPaletteItem = {
    fileName: string;
    displayName: string;
  };

  export type ShortcutCommandPaletteState = {
    pinned: ShortcutCommandPaletteItem[];
    dropdown: ShortcutCommandPaletteItem[];
  };

  type ShortcutCommandHandler = (fileName: string, command: PaletteShortcutReorderCommand) => Promise<void>;

  const shortcutPaletteStateByPlatform = new Map<string, ShortcutCommandPaletteState>();
  const shortcutCommandHandlerByPlatform = new Map<string, ShortcutCommandHandler>();

  function cloneItems(items: ShortcutCommandPaletteItem[]): ShortcutCommandPaletteItem[] {
    return items.map((item) => ({ ...item }));
  }

  export function getShortcutCommandPaletteState(platformName: string): ShortcutCommandPaletteState {
    const state = shortcutPaletteStateByPlatform.get(platformName);
    if (!state) {
      return { pinned: [], dropdown: [] };
    }
    return {
      pinned: cloneItems(state.pinned),
      dropdown: cloneItems(state.dropdown),
    };
  }

  export function setShortcutCommandPaletteState(platformName: string, state: ShortcutCommandPaletteState): void {
    shortcutPaletteStateByPlatform.set(platformName, {
      pinned: cloneItems(state.pinned),
      dropdown: cloneItems(state.dropdown),
    });
  }

  export function clearShortcutCommandPaletteState(platformName: string): void {
    shortcutPaletteStateByPlatform.delete(platformName);
    shortcutCommandHandlerByPlatform.delete(platformName);
  }

  export function registerShortcutCommandHandler(platformName: string, handler: ShortcutCommandHandler): void {
    shortcutCommandHandlerByPlatform.set(platformName, handler);
  }

  export async function runShortcutCommandPaletteAction(
    platformName: string,
    fileName: string,
    command: PaletteShortcutReorderCommand,
  ): Promise<void> {
    const handler = shortcutCommandHandlerByPlatform.get(platformName);
    if (!handler) return;
    await handler(fileName, command);
  }
</script>

<script lang="ts">
  import { onDestroy, onMount, tick } from "svelte";
  import { Events } from "@wailsio/runtime";
  import * as Shortcuts from "wails-shortcuts-service";
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
  import { HasShortcutMainExe } from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
  import { fileDropAcceptor } from "../stores/fileDrop";
  import { runShortcut, hideShortcut, openShortcutFolder, buildShortcutContextMenu, buildPlatformContextMenu } from "../lib/shortcutActions";
  import { tooltip } from "../lib/actions/tooltip";
  import { contextMenu } from "../lib/actions/contextMenu";
  import {
    focusShortcutArrowNavigationTarget,
    shortcutArrowNavigation,
  } from "../lib/actions/shortcutArrowNavigation";
  import {
    platformExeIconUrl,
    triggerPlatformAction,
  } from "../stores/platformPage";
  import { pushToast } from "../stores/toast";
  import ShortcutZone from "./ShortcutZone.svelte";
  import "../styles/gameshortcutbar.scss";
  import {
    createDragReorderController,
    computeDisplayPinned,
    computeDisplayDrop,
    reorderShortcutByCommand,
    type ShortcutReorderCommand,
    type Zone,
  } from "../lib/dragReorderShortcuts";
  import { createShortcutFileDropAcceptor } from "../lib/shortcutFileDropAcceptor";
  import { ghostCloneMount } from "../lib/actions/ghostCloneMount";

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

  let prevPlatform = platformName;
  $: if (platformName !== prevPlatform) {
    prevPlatform = platformName;
    iconBroken = false;
    ddOpen = false;
    dnd.resetState();
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
  let dropdownEl: HTMLDivElement | null = null;

  const dnd = createDragReorderController({
    getPins: () => pinNames,
    getDrops: () => dropNames,
    setPins: (p) => { pinNames = p; },
    setDrops: (d) => { dropNames = d; },
    isDropdownOpen: () => ddOpen,
    onPersist: async () => { await persist(); },
    afterDrag: armSuppressClickAfterDrag,
  });

  $: dnd.setPinListEl(pinListEl);
  $: dnd.setDropListEl(dropListEl);

  let suppressNextClick = false;
  let suppressClickExpire: ReturnType<typeof setTimeout> | null = null;
  let registeredPlatformName = "";

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

  async function persist(): Promise<boolean> {
    try {
      await Shortcuts.SaveShortcutOrder(platformName, pinNames, dropNames);
      return true;
    } catch {
      await refreshFromServer();
      return false;
    }
  }

  $: displayPinned = computeDisplayPinned(
    pinNames,
    $dnd.draggingId,
    $dnd.dragSourceZone,
    $dnd.dragFromIndex,
    $dnd.dragOverZone,
    $dnd.dragOverIndex,
  );
  $: displayDrop = computeDisplayDrop(
    dropNames,
    $dnd.draggingId,
    $dnd.dragSourceZone,
    $dnd.dragFromIndex,
    $dnd.dragOverZone,
    $dnd.dragOverIndex,
  );

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
      row.displayName,
    );
  }

  function shortcutDisplayName(fn: string): string {
    return meta[fn]?.displayName?.trim() || fn.replace(/\.(lnk|url)$/i, "").trim() || fn;
  }

  function buildShortcutCommandPaletteState(): ShortcutCommandPaletteState {
    return {
      pinned: pinNames.map((fileName) => ({
        fileName,
        displayName: shortcutDisplayName(fileName),
      })),
      dropdown: dropNames.map((fileName) => ({
        fileName,
        displayName: shortcutDisplayName(fileName),
      })),
    };
  }

  function shortcutZone(fn: string): Zone | null {
    if (pinNames.includes(fn)) return "pinned";
    if (dropNames.includes(fn)) return "dropdown";
    return null;
  }

  async function applyShortcutReorderCommand(fn: string, command: ShortcutReorderCommand): Promise<void> {
    const zone = shortcutZone(fn);
    if (!zone) return;
    const result = reorderShortcutByCommand(pinNames, dropNames, zone, fn, command);
    if (!result.moved) return;
    pinNames = result.pins;
    dropNames = result.drops;
    const saved = await persist();
    if (!saved) return;
    const listName = result.zone === "pinned" ? $t("Shortcut_ListPinned") : $t("Shortcut_ListDropdown");
    pushToast({
      type: "success",
      message: $t("Toast_MovedItemToListPosition", {
        item: shortcutDisplayName(fn),
        list: listName,
        position: result.position,
        total: result.total,
      }),
      duration: 3500,
    });
  }

  $: {
    if (registeredPlatformName && registeredPlatformName !== platformName) {
      clearShortcutCommandPaletteState(registeredPlatformName);
      registeredPlatformName = "";
    }
    if (platformName) {
      setShortcutCommandPaletteState(platformName, buildShortcutCommandPaletteState());
      registerShortcutCommandHandler(platformName, applyShortcutReorderCommand);
      registeredPlatformName = platformName;
    }
  }

  function shortcutReorderMenu(fn: string) {
    const zone = shortcutZone(fn);
    const order = zone === "pinned" ? pinNames : zone === "dropdown" ? dropNames : [];
    const index = order.indexOf(fn);
    return {
      canMoveLeft: index > 0,
      canMoveRight: index >= 0 && index < order.length - 1,
      canPin: zone === "dropdown",
      canUnpin: zone === "pinned",
      canMoveToPinned: zone === "dropdown",
      canMoveToDropdown: zone === "pinned",
      onMoveLeft: () => { void applyShortcutReorderCommand(fn, "left"); },
      onMoveRight: () => { void applyShortcutReorderCommand(fn, "right"); },
      onPin: () => { void applyShortcutReorderCommand(fn, "pin"); },
      onUnpin: () => { void applyShortcutReorderCommand(fn, "unpin"); },
      onMoveToPinned: () => { void applyShortcutReorderCommand(fn, "move-to-pinned"); },
      onMoveToDropdown: () => { void applyShortcutReorderCommand(fn, "move-to-dropdown"); },
    };
  }

  function ctxMenuForFile(fn: string): () => import("../stores/contextMenu").MenuItemDef[] {
    const r = meta[fn];
    return buildShortcutContextMenu({
      platformName,
      fileName: fn,
      swapLabel: resolvedSwapAccountLabelForMenu,
      reorder: () => shortcutReorderMenu(fn),
      onRunAsAdmin: () => {
        if (r)
          void runShortcut(
            platformName,
            fn,
            true,
            r.isUrl,
            () => requestPlatformAccountsRefresh(platformName),
            r.displayName,
          );
      },
      onHide: () => {
        void hideShortcut(platformName, fn, () => refreshFromServer());
      },
    });
  }

  const shortcutFileDropAcceptor = createShortcutFileDropAcceptor(
    () => platformName,
    () => { ddOpen = true; },
  );

  async function focusDropdownShortcut(edge: "first" | "last" = "first"): Promise<void> {
    await tick();
    focusShortcutArrowNavigationTarget(dropdownEl ?? dropListEl, edge);
  }

  function onShortcutDropdownButtonKeydown(e: KeyboardEvent): void {
    if (e.key !== "ArrowDown" && e.key !== "ArrowUp") {
      return;
    }
    e.preventDefault();
    e.stopPropagation();
    ddOpen = true;
    void focusDropdownShortcut(e.key === "ArrowUp" ? "last" : "first");
  }

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

    const move = (e: PointerEvent) => dnd.onPointerMove(e);
    const up = (e: PointerEvent) => dnd.onPointerUp(e);
    const cancel = (e: PointerEvent) => dnd.onPointerCancel(e);
    const key = (e: KeyboardEvent) => dnd.onKeyDown(e);
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
    if (registeredPlatformName) {
      clearShortcutCommandPaletteState(registeredPlatformName);
      registeredPlatformName = "";
    }
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

<div class="gameShortcutBar" use:shortcutArrowNavigation={{ capture: true }}>
  <ShortcutZone
    bind:el={pinListEl}
    zone="pinned"
    zoneClass="shortcuts shortcutDndGrid"
    expandClass={ddOpen && pinNames.length === 0 && dropNames.length > 0}
    ariaLabel={$t("Stats_GameShortcutsHotbar")}
    displaySlots={displayPinned}
    {meta}
    contextMenuFor={ctxMenuForFile}
    onTileClick={onShortcutClick}
    onCellPointerDown={dnd.onCellPointerDown}
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
      on:keydown={onShortcutDropdownButtonKeydown}
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
      <div bind:this={dropdownEl} class="shortcutDropdown gameShortcuts open" id="shortcutDropdown">
        <ShortcutZone
          bind:el={dropListEl}
          zone="dropdown"
          zoneClass="shortcutDropdownItems shortcutDndGrid"
          ariaLabel={$t("Stats_GameShortcuts")}
          displaySlots={displayDrop}
          {meta}
          contextMenuFor={ctxMenuForFile}
          onTileClick={onShortcutClick}
          onCellPointerDown={dnd.onCellPointerDown}
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

{#if $dnd.draggingId && $dnd.dragVisualClone}
  <div
    class="shortcutDndGhost"
    style:left="{$dnd.ghostX}px"
    style:top="{$dnd.ghostY}px"
    style:width="{$dnd.ghostW}px"
    style:height="{$dnd.ghostH}px"
    aria-hidden="true"
  >
    <div
      class="shortcutDndGhostMirror"
      use:ghostCloneMount={$dnd.dragVisualClone}
    ></div>
  </div>
{/if}

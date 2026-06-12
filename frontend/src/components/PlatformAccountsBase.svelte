<script lang="ts" context="module">
  export type { GameStatMetricDTO } from "./PlatformAccountAdapter";
</script>

<script lang="ts" generics="TAccount">
  import { onDestroy, onMount } from "svelte";
  import { get } from "svelte/store";
  import { Events } from "@wailsio/runtime";
  import AccountImagePickOverlay from "./AccountImagePickOverlay.svelte";
  import ReorderPointerGrid from "./ReorderPointerGrid.svelte";
  import TagFilterBar from "./TagFilterBar.svelte";
  import AccountTagBubbles from "./AccountTagBubbles.svelte";
  import SearchOverlay, { type SearchResultRow } from "./SearchOverlay.svelte";
  import { route, previousPage, appBarTitle } from "../stores/nav";
  import {
    platformExeIconUrl,
    platformAction,
    platformActionBusy,
    selectedAccount as selectedAccountStore,
    platformLiveSessionId,
    platformAccountsRefresh,
  } from "../stores/platformPage";
  import {
    getPlatformAccountsCache,
    platformAccountCounts,
    setPlatformAccountsCache,
  } from "../stores/platformAccountsCache";
  import { pushToast } from "../stores/toast";
  import { activeModal } from "../stores/modal";
  import { locale, t } from "../stores/i18n";
  import * as BasicService from "../../bindings/TcNo-Acc-Switcher/internal/basic/basicservice.js";
  import { GetPlatformExeIcon } from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
  import { formatWailsError } from "../lib/formatWailsError";
  import {
    preflightAdminForPlatform,
    reportLaunchFailure,
  } from "../lib/adminFlow";
  import { tooltip } from "../lib/actions/tooltip";
  import { contextMenu as ctxMenuAction } from "../lib/actions/contextMenu";
  import type { MenuItemDef } from "../stores/contextMenu";
  import { offlineMode, offlineSafeImageSrc, withAssetCacheBust } from "../stores/offlineMode";
  import { fuzzyWordsMatch } from "../lib/searchFuzzy";
  import { formatLastLoginForLocale } from "../lib/formatLastLogin";
  import {
    openTagFilterMenu,
    type TagDefRow,
    type TagFilterMode,
  } from "../lib/accountTagsContext";
  import { closeSearchOverlay, searchOverlayCtrl } from "../stores/searchOverlay";
  import { platformListSort, type PlatformSortKind } from "../stores/platformListSort";
  import { actionBarStatus, fileDropInterceptor, accountProfileImageDropActive } from "../stores/fileDrop";
  import type { PlatformAccountAdapter } from "./PlatformAccountAdapter";
  import "../styles/gamestats.scss";
  import "../styles/platformAccountsShared.scss";

  // ---- Extracted pure logic modules ----
  import { shouldBumpEpoch } from "../lib/accounts/epochManager";
  import type { EpochCheckRow } from "../lib/accounts/types";
  import {
    mergeListIntoExisting,
    mergeEnrichmentIntoExisting,
    applyLoadedAccounts,
    type ApplyLoadedAccountsState,
  } from "../lib/accounts/mergePipeline";
  import { buildSharedItems, type ContextMenuContext } from "../lib/accounts/contextMenuBuilder";
  import { fileDropIntercept, type FileDropContext } from "../lib/accounts/fileDropHandler";
  import {
    loadTagDefs,
    refreshGameStatsMarkup,
    refreshGameStatsSupport,
    openGameStatsModal,
  } from "../lib/accounts/gameStatsWorker";
  import {
    swapToLogin as swapToLoginFn,
    handlePlatformActionKind,
    type AccountActionsContext,
  } from "../lib/accounts/accountActions";

  export let name: string;
  export let adapter: PlatformAccountAdapter<TAccount>;

  const SEARCH_MAX = 5;

  let accounts: TAccount[] = [];
  let accountIds: string[] = [];
  $: accountMap = new Map(accounts.map((a) => [adapter.id(a), a] as const));
  let accountsLoading = false;
  let loadError = "";
  let selectedId = "";
  let isActionBusyValue = false;

  let refreshTimers: ReturnType<typeof setTimeout>[] = [];
  let acclistEl: HTMLDivElement | undefined;
  let overlayQuery = "";
  let overlayQueryDebounceTimer: ReturnType<typeof setTimeout> | null = null;
  let debouncedOverlayQuery = "";
  let avatarEpoch: Record<string, number> = {};
  let tagDefs: TagDefRow[] = [];
  let tagFilterMode: TagFilterMode = { kind: "all" };
  let imagePick = { open: false, accountId: "", displayName: "", manual: false };
  let fileDragHoverRowId = "";
  let offPlatformAction: (() => void) | undefined;
  let offAccountsRefresh: (() => void) | undefined;
  let offUpdateEvent: (() => void) | undefined;
  let offGameStatsUpdated: (() => void) | undefined;
  let offSort: (() => void) | undefined;
  let lastHandledActionId = 0;
  let lastHandledSortId = 0;

  let gameStatsByAccount: Record<string, Record<string, Record<string, { statValue: string; indicatorMarkup: string }>>> = {};
  let hasGameStatsSupport = false;

  // Reactive declarations
  $: so = $searchOverlayCtrl;
  $: appBarTitle.set(name || "TcNo Account Switcher");
  $: isActionBusy = isActionBusyValue;

  $: {
    const q = overlayQuery;
    if (overlayQueryDebounceTimer) clearTimeout(overlayQueryDebounceTimer);
    overlayQueryDebounceTimer = setTimeout(() => { debouncedOverlayQuery = q; }, 150);
  }

  $: if (name) {
    route.set({ page: "platform", platformName: name });
  }

  $: searchPrimary = buildAccountRows(debouncedOverlayQuery, avatarEpoch);

  $: selectedAccountStore.set({
    platformKey: name,
    uniqueId: selectedId,
    displayName: accountById(selectedId) ? adapter.name(accountById(selectedId)!) : "",
    accountLogin: accountById(selectedId) ? adapter.accountLogin(accountById(selectedId)!) : "",
  });

  $: tagFilterBarLabel =
    tagFilterMode.kind === "all"
      ? $t("Tags_All")
      : tagFilterMode.kind === "untagged"
        ? $t("Tags_Filter_Untagged")
        : tagFilterMode.name;

  $: displayIds = (() => {
    const ids = accountIds;
    if (tagFilterMode.kind === "all") return ids;
    if (tagFilterMode.kind === "untagged") {
      return ids.filter((id) => (accountMap.get(id) ? (adapter.tags(accountMap.get(id)!) ?? []).length === 0 : true));
    }
    const tid = tagFilterMode.id;
    return ids.filter((id) => {
      const a = accountMap.get(id);
      return a ? (adapter.tags(a) ?? []).some((x) => x.id === tid) : false;
    });
  })();

  $: reorderDisabled = tagFilterMode.kind !== "all";

  $: skeletonCount = Math.max(1, Math.min(24, $platformAccountCounts[name] ?? 3));
  $: skeletonSlots = Array.from({ length: skeletonCount });

  $: {
    if (selectedId && displayIds.length > 0 && !displayIds.includes(selectedId)) {
      selectedId = "";
      touchStatus();
    }
  }

  // ---- Helpers ----
  function accountById(id: string): TAccount | undefined {
    return accounts.find((a) => adapter.id(a) === id);
  }

  function slotKey(x: string | null | undefined): string {
    return x ?? "";
  }

  function accountRowEqual(a: TAccount, b: TAccount): boolean {
    return JSON.stringify(a) === JSON.stringify(b);
  }

  function touchStatus(): void {
    if (isActionBusy) return;
    const acc = accountById(selectedId);
    actionBarStatus.set(acc ? $t("Status_SelectedAccount", { name: adapter.name(acc) || adapter.id(acc) }) : "");
  }

  function clearSelection(): void {
    if (!selectedId) return;
    selectedId = "";
    touchStatus();
  }

  function onAccountsAreaClick(e: MouseEvent): void {
    const target = e.target as HTMLElement | null;
    if (!target) return;
    if (target.closest("[data-dnd-cell]")) return;
    clearSelection();
  }

  function onWindowKeyDown(e: KeyboardEvent): void {
    if (e.key !== "Escape") return;
    if (get(activeModal)) return;
    if (imagePick.open) { e.preventDefault(); closeImagePick(); return; }
    clearSelection();
  }

  function onItemClick(e: CustomEvent<{ id: string }>): void {
    selectedId = e.detail.id;
    touchStatus();
  }

  function scheduleAccountsRefresh(): void {
    for (const t of refreshTimers) clearTimeout(t);
    refreshTimers = [];
    void loadAccounts();
    refreshTimers.push(
      setTimeout(() => void loadAccounts(), 700),
      setTimeout(() => void loadAccounts(), 2200),
    );
  }

  function bumpAvatarEpoch(id: string): void {
    const uid = id.trim();
    if (!uid) return;
    avatarEpoch = { ...avatarEpoch, [uid]: (avatarEpoch[uid] ?? 0) + 1 };
  }

  // ---- Context factories for extracted modules ----
  function getAccountActionsCtx(): AccountActionsContext {
    return {
      name,
      adapter,
      get selectedId() { return selectedId; },
      get isActionBusyValue() { return isActionBusyValue; },
      accountById,
      scheduleAccountsRefresh,
      touchStatus,
      setIsActionBusy: (v: boolean) => { isActionBusyValue = v; },
    };
  }

  function createFileDropCtx(): FileDropContext {
    return {
      adapter,
      get imagePick() { return imagePick; },
      get fileDragHoverRowId() { return fileDragHoverRowId; },
      tr: get(t),
      loadAccounts,
      bumpAvatarEpoch,
      closeImagePick,
      clearFileDragHover: () => { fileDragHoverRowId = ""; },
    };
  }

  // ---- Wrappers for extracted functions that return values ----
  async function loadTagDefsInternal(): Promise<void> {
    tagDefs = await loadTagDefs(name);
  }

  async function refreshGameStatsMarkupInternal(acctIds: string[]): Promise<void> {
    gameStatsByAccount = await refreshGameStatsMarkup(name, acctIds);
  }

  async function refreshGameStatsSupportInternal(): Promise<void> {
    hasGameStatsSupport = await refreshGameStatsSupport(name);
  }

  async function swapToLogin(): Promise<void> {
    await swapToLoginFn(getAccountActionsCtx());
  }

  // ---- Sort ----
  function displayKeyForSort(uid: string): string {
    const a = accountById(uid);
    return (a ? adapter.name(a) : uid).trim().toLowerCase();
  }

  function lastUsedMsForSort(uid: string): number {
    const a = accountById(uid);
    if (!a) return 0;
    const t = Date.parse((adapter.lastUsed(a) ?? "").trim());
    return Number.isNaN(t) ? 0 : t;
  }

  function accountNameSortKey(uid: string): string {
    const a = accountById(uid);
    return a ? adapter.accountLogin(a).trim().toLowerCase() : uid.toLowerCase();
  }

  function applyPlatformSort(kind: PlatformSortKind): void {
    const ids = [...accountIds];
    const cmpAlpha = (x: string, y: string) => displayKeyForSort(x).localeCompare(displayKeyForSort(y));
    const cmpUser = (x: string, y: string) => accountNameSortKey(x).localeCompare(accountNameSortKey(y));

    switch (kind) {
      case "alpha_asc": ids.sort(cmpAlpha); break;
      case "alpha_desc": ids.sort((x, y) => -cmpAlpha(x, y)); break;
      case "steam_user_asc": ids.sort(cmpUser); break;
      case "steam_user_desc": ids.sort((x, y) => -cmpUser(x, y)); break;
      case "lastused_new_old": case "date_new_old":
        ids.sort((x, y) => lastUsedMsForSort(y) - lastUsedMsForSort(x) || cmpAlpha(x, y));
        break;
      case "lastused_old_new": case "date_old_new":
        ids.sort((x, y) => lastUsedMsForSort(x) - lastUsedMsForSort(y) || cmpAlpha(x, y));
        break;
      default: return;
    }
    accountIds = ids;
    adapter.saveOrder(ids).catch(() => {});
  }

  // ---- Tag filter ----
  function onTagFilterBarClick(ev: Event): void {
    openTagFilterMenu({
      ev: ev as MouseEvent,
      tagDefs,
      tr: get(t),
      onPick: (mode) => { tagFilterMode = mode; },
    });
  }

  // ---- File drop drag ----
  function hasFilesDragType(ev: DragEvent): boolean {
    const types = ev.dataTransfer?.types;
    return !!(types && Array.from(types as unknown as Iterable<string>).includes("Files"));
  }

  function onAccDragOver(ev: DragEvent, rid: string): void {
    if (!hasFilesDragType(ev)) return;
    ev.preventDefault();
    fileDragHoverRowId = rid;
  }

  function onAccDragLeave(ev: DragEvent, rid: string): void {
    if (!hasFilesDragType(ev)) return;
    const rel = ev.relatedTarget as Node | null;
    const cur = ev.currentTarget as HTMLElement | null;
    if (!rel || !cur?.contains(rel)) {
      if (fileDragHoverRowId === rid) fileDragHoverRowId = "";
    }
  }

  function onAccListDragLeave(ev: DragEvent): void {
    if (!hasFilesDragType(ev)) return;
    const rel = ev.relatedTarget as Node | null;
    if (!acclistEl || rel === null || !acclistEl.contains(rel)) {
      fileDragHoverRowId = "";
    }
  }

  // ---- Image pick ----
  function closeImagePick(): void {
    imagePick = { open: false, accountId: "", displayName: "", manual: false };
    fileDragHoverRowId = "";
  }

  function openImagePick(rowId: string): void {
    const acc = accountById(rowId);
    imagePick = {
      open: true,
      accountId: rowId,
      displayName: (acc ? adapter.name(acc) : rowId).trim(),
      manual: acc ? adapter.manualProfileImage(acc) : false,
    };
  }

  async function applyImageFromOverlay(path: string): Promise<void> {
    const id = imagePick.accountId.trim();
    if (!id) return;
    await adapter.changeImage(id, path);
    await loadAccounts();
    bumpAvatarEpoch(id);
    pushToast({ type: "success", message: $t("Toast_AccountSaved"), duration: 4000 });
  }

  async function removeImageFromOverlay(): Promise<void> {
    const id = imagePick.accountId.trim();
    if (!id) return;
    await adapter.clearManualImage(id);
    await loadAccounts();
    bumpAvatarEpoch(id);
    scheduleAccountsRefresh();
    pushToast({ type: "success", message: $t("Toast_AccountSaved"), duration: 4000 });
  }

  // ---- Load accounts ----
  function deferNonCriticalAccountWork(ids: string[]): void {
    void loadTagDefsInternal();
    void refreshGameStatsMarkupInternal(ids);
    void BasicService.StartGameStatsRefresh(name);
  }

  async function loadAccounts(): Promise<void> {
    loadError = "";
    accountsLoading = true;

    const cached = getPlatformAccountsCache(name);
    if (cached && cached.accounts.length > 0) {
      accounts = cached.accounts as TAccount[];
      accountIds = [...cached.accountIds];
      const liveRow = accounts.find((r) => adapter.currentSession(r));
      platformLiveSessionId.set({ platformKey: name, uniqueId: liveRow ? adapter.id(liveRow) : "" });
      touchStatus();
    }

    const prevById = new Map(accounts.map((a) => [adapter.id(a), a]));
    const hadCachedAccounts = !!(cached && cached.accounts.length > 0);
    try {
      const list = await adapter.loadAccountsList();
      const mergedList = mergeListIntoExisting(adapter, accounts, list);
      const state: ApplyLoadedAccountsState<TAccount> = {
        avatarEpoch, accounts, accountIds, selectedId,
      };
      const listChanged = applyLoadedAccounts(adapter, name, mergedList, prevById, state, touchStatus);
      ({ avatarEpoch, accounts, accountIds, selectedId } = state);
      accountsLoading = false;
      if (listChanged) deferNonCriticalAccountWork(accountIds);

      void (async () => {
        try {
          const enrich = await adapter.loadAccountsEnrichment();
          const enrichPrev = new Map(accounts.map((a) => [adapter.id(a), a]));
          const merged = mergeEnrichmentIntoExisting(adapter, accounts, enrich);
          const enrichState: ApplyLoadedAccountsState<TAccount> = {
            avatarEpoch, accounts, accountIds, selectedId,
          };
          const enrichChanged = applyLoadedAccounts(adapter, name, merged, enrichPrev, enrichState, touchStatus);
          ({ avatarEpoch, accounts, accountIds, selectedId } = enrichState);
          if (enrichChanged) deferNonCriticalAccountWork(accountIds);
          await adapter.onAfterLoad?.(accounts, { hadCachedAccounts, enrichChanged });
        } catch {
          await adapter.onAfterLoad?.(accounts, { hadCachedAccounts, enrichChanged: false });
        }
      })();
    } catch (e) {
      accountsLoading = false;
      if (!cached || cached.accounts.length === 0) {
        loadError = formatWailsError(e) || String(e);
        accounts = []; accountIds = []; selectedId = "";
        gameStatsByAccount = {};
        actionBarStatus.set("");
        platformLiveSessionId.set({ platformKey: "", uniqueId: "" });
      }
    }
  }

  // ---- Apply real-time patches ----
  function applyPatchFromEvent(raw: unknown): void {
    const patch = adapter.buildPatch(raw);
    const targetId = adapter.patchTargetId(patch);
    const idx = accounts.findIndex((r) => adapter.id(r) === targetId);
    if (idx < 0) return;

    const prev = accounts[idx];
    const next = adapter.applyPatch(patch, prev);
    if (accountRowEqual(prev, next)) return;

    accounts = accounts.map((r, i) => (i === idx ? next : r));
    if (shouldBumpEpoch(prev as unknown as EpochCheckRow, next as unknown as EpochCheckRow)) {
      bumpAvatarEpoch(targetId);
    }
    setPlatformAccountsCache(name, { accounts, accountIds });
    if (targetId === selectedId) touchStatus();
  }

  // ---- Search ----
  function buildAccountRows(q: string, epochs: Record<string, number>): SearchResultRow[] {
    const tr = get(t);
    const trimmed = q.trim();
    const hay = (acc: TAccount) => fuzzyWordsMatch(trimmed, adapter.searchHay(acc, trimmed));
    let list = trimmed ? accounts.filter((a) => hay(a)) : accounts.slice(0, SEARCH_MAX);
    if (trimmed) list = list.slice(0, SEARCH_MAX);
    return list.map((a) => ({
      key: `a:${adapter.id(a)}`,
      title: adapter.name(a) || adapter.id(a),
      badge: tr("Search_Section_Account"),
      accountIconUrl: offlineSafeImageSrc(
        get(offlineMode),
        withAssetCacheBust(
          adapter.imageUrl(a) && !adapter.imagePending(a) ? adapter.imageUrl(a) : undefined,
          epochs[adapter.id(a)] ?? 0,
        ),
        adapter.profileFallback,
      ),
    }));
  }

  async function onSearchPick(ev: CustomEvent<SearchResultRow>): Promise<void> {
    const row = ev.detail;
    closeSearchOverlay();
    if (row.key.startsWith("a:")) {
      const id = row.key.slice(2);
      selectedId = id;
      touchStatus();
      await swapToLogin();
    }
    if (row.key.startsWith("g:") && adapter.gameSearchRows) {
      const appId = row.key.slice(2);
      if (!selectedId) {
        pushToast({ type: "error", message: $t("Toast_NoValidSteamId"), duration: 5000 });
        return;
      }
      try {
        if (adapter.loginAndLaunchGame) await adapter.loginAndLaunchGame(selectedId, appId);
        scheduleAccountsRefresh();
        pushToast({
          type: "success",
          message: $t("Toast_StartedGame", { program: row.title }),
          duration: 4000,
        });
      } catch (e) { await reportLaunchFailure(e, name); }
    }
  }

  // ---- Context menu ----
  function ctxMenu(rowId: string): () => MenuItemDef[] {
    return () => {
      const acc = accountById(rowId);
      if (!acc) return [];
      const ctx: ContextMenuContext = {
        name,
        adapter,
        get isActionBusy() { return isActionBusyValue; },
        get hasGameStatsSupport() { return hasGameStatsSupport; },
        tr: get(t),
        get tagDefs() { return tagDefs; },
        openImagePick,
        swapToLogin,
        loadAccounts,
        scheduleAccountsRefresh,
        loadTagDefs: loadTagDefsInternal,
        openGameStatsModal: (rid: string) => openGameStatsModal(rid, adapter, name, accountById, () => void loadAccounts()),
        onSelectedIdChanged: (id: string) => { selectedId = id; touchStatus(); },
      };
      const shared = buildSharedItems(acc, rowId, ctx);
      return adapter.buildMenu(acc, shared);
    };
  }

  // ---- Lifecycle ----
  onMount(() => {
    previousPage.set({ page: "home" });
    void preflightAdminForPlatform(name);
    void refreshGameStatsSupportInternal();
    void loadAccounts();
    void GetPlatformExeIcon(name).then((u: string) => platformExeIconUrl.set(u ?? ""));

    offSort = platformListSort.subscribe((sig) => {
      if (!sig || sig.id <= lastHandledSortId) return;
      lastHandledSortId = sig.id;
      applyPlatformSort(sig.kind);
    });

    offPlatformAction = platformAction.subscribe((v) => {
      if (!v || v.id === lastHandledActionId) return;
      lastHandledActionId = v.id;
      void handlePlatformActionKind(v.kind, getAccountActionsCtx());
    });

    offAccountsRefresh = platformAccountsRefresh.subscribe((p) => {
      if (p.seq === 0 || p.platformKey !== name) return;
      scheduleAccountsRefresh();
    });

    offUpdateEvent = Events.On(adapter.updateEventName, (ev) => {
      applyPatchFromEvent(ev.data);
    });

    offGameStatsUpdated = Events.On("basic-game-stats-updated", (ev) => {
      const p = ev.data as { platformKey?: string; uniqueId?: string };
      if ((p.platformKey ?? "").trim() !== name.trim()) return;
      const uid = (p.uniqueId ?? "").trim();
      if (uid) void refreshGameStatsMarkupInternal([uid]);
    });

    fileDropInterceptor.set((paths: string[]) => fileDropIntercept(paths, createFileDropCtx()));
  });

  onDestroy(() => {
    for (const t of refreshTimers) clearTimeout(t);
    refreshTimers = [];
    if (overlayQueryDebounceTimer) clearTimeout(overlayQueryDebounceTimer);
    selectedAccountStore.set({ platformKey: "", uniqueId: "", displayName: "", accountLogin: "" });
    platformLiveSessionId.set({ platformKey: "", uniqueId: "" });
    platformAction.set(null);
    offPlatformAction?.();
    offSort?.();
    offAccountsRefresh?.();
    offUpdateEvent?.();
    offGameStatsUpdated?.();
    accountProfileImageDropActive.set(false);
    fileDropInterceptor.set(null);
    platformAccountsRefresh.set({ seq: 0, platformKey: "" });
    platformExeIconUrl.set("");
    actionBarStatus.set("");
    adapter.onCleanup?.();
  });
</script>

<div class="main-content platform-accounts-root">
  {#if name}
    <div class="platformTableHost">
      <SearchOverlay
        open={so.open}
        syncNonce={so.nonce}
        initialQuery={so.initialQuery}
        bind:query={overlayQuery}
        primaryRows={searchPrimary}
        categoryRows={[]}
        categoryHint=""
        gameRows={adapter.gameSearchRows?.(debouncedOverlayQuery) ?? []}
        gameHint={adapter.gameSearchHint ?? ""}
        on:close={() => closeSearchOverlay()}
        on:pick={(e) => void onSearchPick(e)}
      />
      <div class="platformTable">
      {#if loadError}
        <p class="platform-accounts-hint">{loadError}</p>
      {/if}
      <!-- svelte-ignore a11y-click-events-have-key-events -->
      <!-- svelte-ignore a11y-no-static-element-interactions -->
      <div
        class="steam-acclist"
        bind:this={acclistEl}
        on:click={onAccountsAreaClick}
        on:dragleave={onAccListDragLeave}
      >
        {#if tagDefs.length > 0}
          <TagFilterBar label={tagFilterBarLabel} onClick={onTagFilterBarClick} />
        {/if}
        {#if accountsLoading && displayIds.length === 0}
          <div class="acc_list acc_list--skeleton" aria-busy="true" aria-label={$t("Button_Loading")}>
            {#each skeletonSlots as _, i (i)}
              <div class="acc_list_item acc_list_item--skeleton" aria-hidden="true">
                <div class="acc_skeleton_avatar"></div>
                <div class="acc_skeleton_lines">
                  <div class="acc_skeleton_line acc_skeleton_line--name"></div>
                </div>
              </div>
            {/each}
          </div>
        {:else}
        <ReorderPointerGrid
          items={displayIds}
          reorderDisabled={reorderDisabled}
          listClass="acc_list"
          itemClass="acc_list_item acc_list_item--drag"
          placeholderClass="acc_list_item placeHolderAcc"
          ghostClass="acc_list_item acc_list_item--ghost"
          ariaLabel="Accounts"
          on:reorder={(e) => { accountIds = e.detail.items; adapter.saveOrder(e.detail.items).catch(() => {}); }}
          on:itemclick={onItemClick}
        >
          <svelte:fragment slot="item" let:rowId>
            {@const rid = slotKey(rowId)}
            {@const acc = accountMap.get(rid)}
            {@const radioId = `acc-${adapter.platformKey}-${rid}`}
            {#if acc}
              {#key `${rid}-${avatarEpoch[rid] ?? 0}`}
              <div class="acc_list_item_inner">
                <input
                  type="radio"
                  class="acc"
                  id={radioId}
                  name={`${adapter.platformKey}-accounts`}
                  value={rid}
                  bind:group={selectedId}
                  on:change={touchStatus}
                />
                <label
                  for={radioId}
                  class="acc"
                  class:currentAcc={adapter.currentSession(acc)}
                  class:acc--profile-drop-target={$accountProfileImageDropActive && !imagePick.open}
                  class:acc--drop-target={fileDragHoverRowId === rid}
                  on:dragover={(e) => onAccDragOver(e, rid)}
                  on:dragleave={(e) => onAccDragLeave(e, rid)}
                  use:ctxMenuAction={{
                    items: ctxMenu(rid),
                    beforeOpen: () => { selectedId = rid; touchStatus(); },
                  }}
                  use:tooltip={adapter.currentSession(acc)
                    ? { text: $t("Tooltip_CurrentAccount"), placement: "right", boundary: acclistEl }
                    : undefined}
                  on:dblclick|preventDefault={() => {
                    if (isActionBusy) return;
                    selectedId = rid;
                    touchStatus();
                    void swapToLogin();
                  }}
                >
                  {#if $accountProfileImageDropActive && !imagePick.open}
                    <div class="acc_profile_drop_overlay" class:acc_profile_drop_overlay--hover={fileDragHoverRowId === rid} aria-hidden="true">
                      <div class="acc_profile_drop_overlay__center">
                        <div class="acc_profile_drop_overlay__icon" aria-hidden="true">
                          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path fill="currentColor" d="M5 20h14v-2H5v2zM19 9h-4V3H9v6H5l7 7 7-7z"/></svg>
                        </div>
                        <span class="acc_profile_drop_overlay__label">{$t("Drop_SetAccountIcon")}</span>
                      </div>
                    </div>
                  {/if}

                  <slot name="account-avatar" {acc} epoch={avatarEpoch[rid] ?? 0} fallback={adapter.profileFallback}>
                    <img
                      src={offlineSafeImageSrc($offlineMode, withAssetCacheBust(
                        adapter.imageUrl(acc) && !adapter.imagePending(acc) ? adapter.imageUrl(acc) : undefined,
                        avatarEpoch[rid] ?? 0,
                      ), adapter.profileFallback)}
                      alt="" draggable="false"
                    />
                  </slot>

                  <slot name="account-before-name" {acc} />

                  <h6 class="displayName">{adapter.name(acc)}</h6>

                  <AccountTagBubbles tags={adapter.tags(acc) ?? []} />

                  {#if adapter.shouldShowNote(acc)}
                    <p class="acc_note">{adapter.note(acc)}</p>
                  {/if}

                  {#if gameStatsByAccount[rid] && Object.keys(gameStatsByAccount[rid]).length > 0}
                    <div class="acc_inline_gamestats" aria-label={$t("Context_ManageGameStats")}>
                      <span class="acc_inline_gamestats_metrics">
                        {#each Object.values(gameStatsByAccount[rid]) as metrics}
                          {#each Object.values(metrics) as dto}
                            <span class="acc_inline_gamestats_metric">
                              {#if dto.indicatorMarkup}
                                <span class="acc_inline_gamestats_ind">{@html dto.indicatorMarkup}</span>
                              {/if}
                              <span class="acc_inline_gamestats_val">{@html dto.statValue}</span>
                            </span>
                          {/each}
                        {/each}
                      </span>
                    </div>
                  {/if}

                  <slot name="account-after-stats" {acc} />

                  {#if adapter.shouldShowLastUsed(acc)}
                    <p class="acc_lastused">{formatLastLoginForLocale(adapter.lastUsed(acc), $locale)}</p>
                  {/if}

                  <slot name="account-footer" {acc} />
                </label>
              </div>
              {/key}
            {/if}
          </svelte:fragment>
        </ReorderPointerGrid>
        {/if}
      </div>
    </div>
    </div>
    <AccountImagePickOverlay
      bind:open={imagePick.open}
      accountDisplayName={imagePick.displayName}
      showRemoveButton={imagePick.manual}
      onClose={closeImagePick}
      onApplyPath={applyImageFromOverlay}
      onRemoveManual={removeImageFromOverlay}
    />
  {/if}
</div>
<svelte:window on:keydown={onWindowKeyDown} />

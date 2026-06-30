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
  import AccountLiveSessionIndicator from "./AccountLiveSessionIndicator.svelte";
  import AccountListSkeleton from "./AccountListSkeleton.svelte";
  import SearchOverlay, { type SearchResultRow } from "./SearchOverlay.svelte";
  import { route, previousPage, appBarTitle } from "../stores/nav";
  import {
    platformExeIconUrl,
    platformAction,
    platformActionBusy,
    selectedAccount as selectedAccountStore,
    platformLiveSessionId,
    platformAccountsRefresh,
    requestPlatformAccountsRefresh,
  } from "../stores/platformPage";
  import {
    getPlatformAccountsCache,
    platformAccountCounts,
    platformTagCounts,
    setPlatformAccountsCache,
  } from "../stores/platformAccountsCache";
  import { pushToast } from "../stores/toast";
  import { activeModal, openConfirm, openPrompt } from "../stores/modal";
  import { locale, t } from "../stores/i18n";
  import * as BasicService from "../../bindings/TcNo-Acc-Switcher/internal/basic/basicservice.js";
  import { GetPlatformExeIcon } from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
  import { formatToastWithError, formatWailsError } from "../lib/formatWailsError";
  import {
    preflightAdminForPlatform,
    reportLaunchFailure,
  } from "../lib/adminFlow";
  import { contextMenu as ctxMenuAction } from "../lib/actions/contextMenu";
  import type { MenuItemDef } from "../stores/contextMenu";
  import { offlineMode, offlineSafeImageSrc, withAssetCacheBust } from "../stores/offlineMode";
  import { formatLastLoginForLocale } from "../lib/formatLastLogin";
  import {
    openTagFilterMenu,
    type TagDefRow,
    type TagFilterMode,
  } from "../lib/accountTagsContext";
  import { closeSearchOverlay, searchOverlayCtrl } from "../stores/searchOverlay";
  import { platformListSort, type PlatformSortKind } from "../stores/platformListSort";
  import { actionBarStatus, fileDropInterceptor, accountProfileImageDropActive } from "../stores/fileDrop";
  import { sanitizeHtml } from "../lib/sanitizeHtml";
  import type { PlatformAccountAdapter } from "./PlatformAccountAdapter";
  import {
    commandRows,
    isCommandQuery,
    runCommand,
    type CommandPaletteCommand,
  } from "../lib/commandPalette";
  import "../styles/gamestats.scss";
  import "../styles/platformAccountsShared.scss";

  // ---- Extracted pure logic modules ----
  import {
    mergeListIntoExisting,
    mergeEnrichmentIntoExisting,
    applyLoadedAccounts,
    type ApplyLoadedAccountsState,
  } from "../lib/accounts/mergePipeline";
  import {
    applyAccountPatch,
    buildAccountMap,
    buildAccountSearchRows,
    createImagePickState,
    displayIdsForTagFilter,
    sortAccountIds,
    type AccountImagePickState,
    type SearchHayCache,
  } from "../lib/accounts/accountPageModel";
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
    runPlatformActionLocked,
    type AccountActionsContext,
  } from "../lib/accounts/accountActions";

  export let name: string;
  export let adapter: PlatformAccountAdapter<TAccount>;

  const SEARCH_MAX = 5;

  let accounts: TAccount[] = [];
  let accountIds: string[] = [];
  $: accountMap = buildAccountMap(accounts, adapter);
  const searchHayCache: SearchHayCache = new Map();
  let accountsLoading = false;
  let tagDefsLoading = false;
  let loadError = "";
  let selectedId = "";
  let isActionBusyValue = false;

  let refreshTimers: ReturnType<typeof setTimeout>[] = [];
  let acclistEl: HTMLDivElement | undefined;
  let overlayQuery = "";
  let overlayQueryDebounceTimer: ReturnType<typeof setTimeout> | null = null;
  let debouncedOverlayQuery = "";
  let avatarEpoch: Record<string, number> = {};
  let rowVersions: Record<string, number> = {};
  let tagDefs: TagDefRow[] = [];
  let tagFilterMode: TagFilterMode = { kind: "all" };
  let imagePick: AccountImagePickState = { open: false, accountId: "", displayName: "", manual: false };
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

  $: commandMode = isCommandQuery(overlayQuery);
  $: searchPrimary = commandMode
    ? commandRows(overlayQuery, buildPlatformCommands(), get(t)("Search_Section_Command"))
    : buildAccountRows(debouncedOverlayQuery, avatarEpoch);

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

  $: displayIds = displayIdsForTagFilter(accountIds, accountMap, adapter, tagFilterMode);

  $: reorderDisabled = tagFilterMode.kind !== "all";

  $: knownAccountCount = $platformAccountCounts[name];
  $: knownTagCount = $platformTagCounts[name];
  $: hasTags = knownTagCount ? knownTagCount.tagCount > 0 || knownTagCount.taggedAccountCount > 0 : false;
  $: skeletonCount =
    knownAccountCount !== undefined
      ? Math.min(24, Math.max(0, knownAccountCount))
      : 3;

  $: {
    if (selectedId && displayIds.length > 0 && !displayIds.includes(selectedId)) {
      selectedId = "";
      touchStatus();
    }
  }

  // ---- Helpers ----
  function accountById(id: string): TAccount | undefined {
    return accountMap.get(id);
  }

  function slotKey(x: string | null | undefined): string {
    return x ?? "";
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

  const REFRESH_DEBOUNCE_MS = 300;

  function scheduleAccountsRefresh(): void {
    for (const t of refreshTimers) clearTimeout(t);
    refreshTimers = [];
    refreshTimers.push(
      setTimeout(() => {
        refreshTimers = [];
        void loadAccounts();
      }, REFRESH_DEBOUNCE_MS),
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
    tagDefsLoading = true;
    try {
      tagDefs = await loadTagDefs(name);
    } finally {
      tagDefsLoading = false;
    }
  }

  async function refreshGameStatsMarkupInternal(acctIds: string[]): Promise<void> {
    gameStatsByAccount = await refreshGameStatsMarkup(name, acctIds);
  }

  async function refreshGameStatsSupportInternal(): Promise<void> {
    hasGameStatsSupport = await refreshGameStatsSupport(name);
  }

  async function swapToLogin(): Promise<void> {
    const acc = accountById(selectedId);
    if (acc && adapter.savedDataBroken?.(acc)) {
      pushToast({ type: "error", message: $t("Security_AccountDataBroken"), duration: 6000 });
      return;
    }
    await runPlatformActionLocked(async () => {
      await swapToLoginFn(getAccountActionsCtx());
    }, getAccountActionsCtx());
  }

  // ---- Sort ----
  function applyPlatformSort(kind: PlatformSortKind): void {
    const ids = sortAccountIds(accountIds, accountById, adapter, kind);
    if (!ids) return;
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
    imagePick = createImagePickState(rowId, accountById(rowId), adapter);
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

  function visibleAccountIds(): string[] {
    return displayIds.filter((id) => !!accountById(id));
  }

  async function clearAccountTagsForPlatform(): Promise<void> {
    const ok = await openConfirm({
      title: $t("Command_ClearAccountTags"),
      body: $t("Command_ClearAccountTagsConfirm", { platform: name }),
      positiveLabel: $t("Command_ClearAccountTags"),
      negativeLabel: $t("Button_Cancel"),
      style: "okcancel",
    });
    if (!ok) return;
    try {
      await BasicService.ClearAccountTags(name);
      tagFilterMode = { kind: "all" };
      await loadAccounts();
      await loadTagDefsInternal();
      pushToast({ type: "success", message: $t("Toast_AccountTagsCleared"), duration: 4000 });
    } catch (e) {
      pushToast({ type: "error", message: formatToastWithError($t("Toast_SaveFailed"), e), duration: 8000 });
    }
  }

  async function tagVisibleAccounts(): Promise<void> {
    const ids = visibleAccountIds();
    if (ids.length === 0) {
      pushToast({ type: "info", message: $t("Toast_NoVisibleAccounts"), duration: 4000 });
      return;
    }
    const tagName = await openPrompt({
      title: $t("Command_TagVisibleAccounts"),
      body: $t("Command_TagVisibleAccountsPrompt", { count: ids.length }),
      positiveLabel: $t("Tags_Add"),
      negativeLabel: $t("Button_Cancel"),
    });
    const trimmed = String(tagName ?? "").trim();
    if (tagName === null || !trimmed) return;
    try {
      await BasicService.AddTagToAccounts(name, ids, trimmed);
      await loadAccounts();
      await loadTagDefsInternal();
      pushToast({
        type: "success",
        message: $t("Toast_TaggedVisibleAccounts", { count: ids.length, tag: trimmed }),
        duration: 4000,
      });
    } catch (e) {
      pushToast({ type: "error", message: formatToastWithError($t("Toast_SaveFailed"), e), duration: 8000 });
    }
  }

  async function clearAllProfileImages(): Promise<void> {
    const ok = await openConfirm({
      title: $t("Command_ClearAllImages"),
      body: $t("Command_ClearAllImagesConfirm", { platform: name }),
      positiveLabel: $t("Command_ClearAllImages"),
      negativeLabel: $t("Button_Cancel"),
      style: "okcancel",
    });
    if (!ok) return;
    try {
      await BasicService.ClearAllBasicProfileImages(name);
      requestPlatformAccountsRefresh(name);
      for (const id of accountIds) bumpAvatarEpoch(id);
      await loadAccounts();
      pushToast({ type: "success", message: $t("Toast_ProfileImagesCleared"), duration: 4000 });
    } catch (e) {
      pushToast({ type: "error", message: formatToastWithError($t("Toast_SaveFailed"), e), duration: 8000 });
    }
  }

  async function refreshAllProfileImages(): Promise<void> {
    try {
      await BasicService.RefreshAllBasicProfileImages(name);
      requestPlatformAccountsRefresh(name);
      for (const id of accountIds) bumpAvatarEpoch(id);
      await loadAccounts();
      pushToast({ type: "success", message: $t("Toast_ImagesRefreshing"), duration: 5000 });
    } catch (e) {
      pushToast({ type: "error", message: formatToastWithError($t("Toast_SaveFailed"), e), duration: 8000 });
    }
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
    const prev = accountById(targetId);
    if (!prev) return;
    const result = applyAccountPatch(accounts, rowVersions, avatarEpoch, adapter, targetId, adapter.applyPatch(patch, prev));
    if (!result.changed) return;
    accounts = result.accounts;
    rowVersions = result.rowVersions;
    avatarEpoch = result.avatarEpoch;
    setPlatformAccountsCache(name, { accounts, accountIds });
    if (targetId === selectedId) touchStatus();
  }

  // ---- Search ----
  function buildAccountRows(q: string, epochs: Record<string, number>): SearchResultRow[] {
    return buildAccountSearchRows({
      accounts,
      rows: adapter,
      query: q,
      max: SEARCH_MAX,
      rowVersions,
      avatarEpoch: epochs,
      searchHayCache,
      offlineMode: get(offlineMode),
      profileFallback: adapter.profileFallback,
      accountBadge: get(t)("Search_Section_Account"),
    });
  }

  function buildPlatformCommands(): CommandPaletteCommand[] {
    const tr = get(t);
    const commands: CommandPaletteCommand[] = [
      {
        id: "login",
        title: tr("Command_LoginSelected"),
        run: () => handlePlatformActionKind("login", getAccountActionsCtx()),
      },
      {
        id: "add-new",
        title: tr("Command_AddNewAccount"),
        run: () => handlePlatformActionKind("addNew", getAccountActionsCtx()),
      },
      {
        id: "refresh-accounts",
        title: tr("Command_RefreshAccounts"),
        run: () => loadAccounts(),
      },
      {
        id: "clear-account-tags",
        title: tr("Command_ClearAccountTags"),
        run: () => clearAccountTagsForPlatform(),
      },
      {
        id: "tag-visible-accounts",
        title: tr("Command_TagVisibleAccounts"),
        run: () => tagVisibleAccounts(),
      },
      {
        id: "clear-all-images",
        title: tr("Command_ClearAllImages"),
        run: () => clearAllProfileImages(),
      },
      {
        id: "refresh-all-images",
        title: tr("Command_RefreshAllImages"),
        run: () => refreshAllProfileImages(),
      },
      {
        id: "platform-settings",
        title: tr("Command_OpenPlatformSettings", { platform: name }),
        run: () => route.set({ page: "platform-settings", platformName: name }),
      },
      {
        id: "home",
        title: tr("Command_GoHome"),
        run: () => route.set({ page: "home" }),
      },
    ];
    if (name !== "Steam") {
      commands.splice(2, 0, {
        id: "save-current",
        title: tr("Command_SaveCurrentAccount"),
        run: () => handlePlatformActionKind("saveCurrent", getAccountActionsCtx()),
      });
    }
    return commands;
  }

  async function onSearchPick(ev: CustomEvent<SearchResultRow>): Promise<void> {
    const row = ev.detail;
    closeSearchOverlay();
    if (row.key.startsWith("cmd:")) {
      runCommand(buildPlatformCommands(), row.key);
      return;
    }
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
        placeholder={commandMode ? $t("Command_SearchPlaceholder") : $t("Context_Search")}
        primaryRows={searchPrimary}
        categoryRows={[]}
        categoryHint=""
        gameRows={commandMode ? [] : (adapter.gameSearchRows?.(debouncedOverlayQuery) ?? [])}
        gameHint={commandMode ? "" : (adapter.gameSearchHint ?? "")}
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
        {#if hasTags || tagDefs.length > 0}
          <TagFilterBar label={tagFilterBarLabel} onClick={onTagFilterBarClick} disabled={tagDefs.length === 0} />
        {/if}
        {#if accountsLoading && displayIds.length === 0 && skeletonCount > 0}
          <AccountListSkeleton count={skeletonCount} />
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
              {#key `${rid}-${avatarEpoch[rid] ?? 0}-${rowVersions[rid] ?? 0}`}
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
                  class:acc--broken={adapter.savedDataBroken?.(acc) === true}
                  class:acc--profile-drop-target={$accountProfileImageDropActive && !imagePick.open}
                  class:acc--drop-target={fileDragHoverRowId === rid}
                  on:dragover={(e) => onAccDragOver(e, rid)}
                  on:dragleave={(e) => onAccDragLeave(e, rid)}
                  use:ctxMenuAction={{
                    items: ctxMenu(rid),
                    beforeOpen: () => { selectedId = rid; touchStatus(); },
                  }}
                  on:dblclick|preventDefault={() => {
                    if (isActionBusy) return;
                    if (adapter.savedDataBroken?.(acc)) return;
                    selectedId = rid;
                    touchStatus();
                    void swapToLogin();
                  }}
                >
                  <AccountLiveSessionIndicator
                    active={adapter.currentSession(acc)}
                    tooltipText={$t("Tooltip_CurrentAccount")}
                    boundary={acclistEl}
                  />
                  {#if adapter.savedDataBroken?.(acc)}
                    <span class="acc_broken_badge">{$t("Security_AccountDataBroken")}</span>
                  {/if}
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
                                <span class="acc_inline_gamestats_ind">{@html sanitizeHtml(dto.indicatorMarkup, "gameStats")}</span>
                              {/if}
                              <span class="acc_inline_gamestats_val">{@html sanitizeHtml(dto.statValue, "gameStats")}</span>
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

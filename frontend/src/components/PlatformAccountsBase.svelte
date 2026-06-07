<script lang="ts" context="module">
  export type { GameStatMetricDTO } from "./PlatformAccountAdapter";
</script>

<script lang="ts" generics="TAccount">
  import { onDestroy, onMount } from "svelte";
  import { Events } from "@wailsio/runtime";
  import ActionBar from "./ActionBar.svelte";
  import AccountImagePickOverlay from "./AccountImagePickOverlay.svelte";
  import ReorderPointerGrid from "./ReorderPointerGrid.svelte";
  import TagFilterBar from "./TagFilterBar.svelte";
  import AccountTagBubbles from "./AccountTagBubbles.svelte";
  import { route, previousPage, appBarTitle } from "../stores/nav";
  import { buildEpochMap } from "../lib/accountEpoch";
  import SearchOverlay, { type SearchResultRow } from "./SearchOverlay.svelte";
  import {
    platformExeIconUrl,
    platformAction,
    platformActionBusy,
    selectedAccount as selectedAccountStore,
    platformLiveSessionId,
    platformAccountsRefresh,
  } from "../stores/platformPage";
  import { pushToast } from "../stores/toast";
  import { activeModal, openAlertNoButton, openConfirm, openPrompt } from "../stores/modal";
  import { locale, t } from "../stores/i18n";
  import * as BasicService from "../../bindings/TcNo-Acc-Switcher/internal/basic/basicservice.js";
  import { GetPlatformExeIcon } from "../lib/platformBindings";
  import { formatToastWithError, formatWailsError } from "../lib/formatWailsError";
  import {
    isNeedsAdminError,
    offerRestartIfNeedsAdmin,
    preflightAdminForPlatform,
    reportLaunchFailure,
  } from "../lib/adminFlow";
  import { tooltip } from "../lib/actions/tooltip";
  import { contextMenu as ctxMenuAction } from "../lib/actions/contextMenu";
  import type { MenuItemDef } from "../stores/contextMenu";
  import * as Shortcuts from "wails-shortcuts-service";
  import { get } from "svelte/store";
  import { offlineMode, offlineSafeImageSrc, withAssetCacheBust } from "../stores/offlineMode";
  import { fuzzyWordsMatch } from "../lib/searchFuzzy";
  import { formatLastLoginForLocale } from "../lib/formatLastLogin";
  import {
    buildTagsSectionMenuItem,
    openTagFilterMenu,
    type TagDefRow,
    type TagFilterMode,
  } from "../lib/accountTagsContext";
  import { closeSearchOverlay, searchOverlayCtrl } from "../stores/searchOverlay";
  import { platformListSort, type PlatformSortKind } from "../stores/platformListSort";
  import { actionBarStatus, fileDropInterceptor, accountProfileImageDropActive } from "../stores/fileDrop";
  import { firstProfileImagePath } from "../lib/profileImageDrop";
  import GameStatsSetupModalBody from "./modals/GameStatsSetupModalBody.svelte";
  import type { PlatformAccountAdapter } from "./PlatformAccountAdapter";
  import "../styles/gamestats.scss";
  import "../styles/platformAccountsShared.scss";

  export let name: string;
  export let adapter: PlatformAccountAdapter<TAccount>;

  const SEARCH_MAX = 5;

  let accounts: TAccount[] = [];
  let accountIds: string[] = [];
  $: accountMap = new Map(accounts.map((a) => [adapter.id(a), a] as const));
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

  // ---- Game stats ----
  async function loadTagDefs(): Promise<void> {
    try {
      const rows = await BasicService.ListTagDefinitions(name);
      tagDefs = (rows as { id: string; name: string; color: string }[]).map((r) => ({ id: r.id, name: r.name, color: r.color }));
    } catch { tagDefs = []; }
  }

  async function refreshGameStatsMarkup(acctIds: string[]): Promise<void> {
    if (!name.trim() || acctIds.length === 0) { gameStatsByAccount = {}; return; }
    try {
      const pairs = await Promise.all(
        acctIds.map(async (uid) => {
          try { const m = await BasicService.GetUserStatsAllGamesMarkup(name, uid); return [uid, m ?? {}] as const; }
          catch { return [uid, {}] as const; }
        }),
      );
      gameStatsByAccount = Object.fromEntries(pairs) as Record<string, Record<string, Record<string, { statValue: string; indicatorMarkup: string }>>>;
    } catch { gameStatsByAccount = {}; }
  }

  async function refreshGameStatsSupport(): Promise<void> {
    try { const games = await BasicService.GetAvailableGames(name); hasGameStatsSupport = (games?.length ?? 0) > 0; }
    catch { hasGameStatsSupport = false; }
  }

  function openGameStatsModal(rowId: string): void {
    const acc = accountById(rowId);
    void openAlertNoButton({
      title: get(t)("Context_ManageGameStats"),
      bodyComponent: GameStatsSetupModalBody,
      bodyProps: {
        platformKey: name,
        uniqueId: rowId,
        displayName: (acc ? adapter.name(acc) : rowId).trim(),
        onApplied: () => void loadAccounts(),
      },
    });
  }

  // ---- Load accounts ----
  async function loadAccounts(): Promise<void> {
    loadError = "";
    const prevById = new Map(accounts.map((a) => [adapter.id(a), a]));
    try {
      const rows = await adapter.loadAccounts();
      avatarEpoch = buildEpochMap(rows as unknown as Record<string, unknown>[], prevById as unknown as Map<string, Record<string, unknown>>, (r: unknown) => adapter.id(r as TAccount), avatarEpoch);
      accounts = rows;
      accountIds = rows.map((r) => adapter.id(r));
      const liveRow = rows.find((r) => adapter.currentSession(r));
      platformLiveSessionId.set({ platformKey: name, uniqueId: liveRow ? adapter.id(liveRow) : "" });
      const stillValid = selectedId && rows.some((r) => adapter.id(r) === selectedId);
      selectedId = stillValid ? selectedId : "";
      touchStatus();
      await loadTagDefs();
      await refreshGameStatsMarkup(rows.map((r) => adapter.id(r)));
      void adapter.onAfterLoad?.(rows);
    } catch (e) {
      loadError = formatWailsError(e) || String(e);
      accounts = []; accountIds = []; selectedId = "";
      gameStatsByAccount = {};
      actionBarStatus.set("");
      platformLiveSessionId.set({ platformKey: "", uniqueId: "" });
    }
  }

  // ---- File drop intercept ----
  async function fileDropIntercept(paths: string[]): Promise<boolean> {
    const img = firstProfileImagePath(paths);
    if (!img) return false;
    try {
      if (imagePick.open && imagePick.accountId.trim()) {
        const target = imagePick.accountId.trim();
        await adapter.changeImage(target, img);
        await loadAccounts();
        bumpAvatarEpoch(target);
        pushToast({ type: "success", message: $t("Toast_AccountSaved"), duration: 4000 });
        closeImagePick();
        return true;
      }
      const hover = fileDragHoverRowId.trim();
      if (hover) {
        await adapter.changeImage(hover, img);
        await loadAccounts();
        bumpAvatarEpoch(hover);
        pushToast({ type: "success", message: $t("Toast_AccountSaved"), duration: 4000 });
        fileDragHoverRowId = "";
        return true;
      }
    } catch (e) {
      pushToast({ type: "error", message: formatToastWithError($t("Toast_SaveFailed"), e), duration: 8000 });
      return true;
    }
    return false;
  }

  // ---- Apply real-time patches ----
  function applyPatchFromEvent(raw: unknown): void {
    const patch = adapter.buildPatch(raw);
    const targetId = adapter.patchTargetId(patch);
    let hit = false;
    accounts = accounts.map((r) => {
      if (adapter.id(r) !== targetId) return r;
      hit = true;
      return adapter.applyPatch(patch, r);
    });
    if (hit) bumpAvatarEpoch(targetId);
    if (targetId === selectedId) touchStatus();
  }

  // ---- Actions ----
  async function reportSwitchFailure(e: unknown): Promise<void> {
    await offerRestartIfNeedsAdmin(e, name);
    if (isNeedsAdminError(e)) return;
    pushToast({ type: "error", message: formatToastWithError($t("Toast_SwitchFailed"), e), duration: 8000 });
  }

  async function reportSaveFailure(e: unknown): Promise<void> {
    await offerRestartIfNeedsAdmin(e, name);
    if (isNeedsAdminError(e)) return;
    pushToast({ type: "error", message: formatToastWithError($t("Toast_SaveFailed"), e), duration: 8000 });
  }

  async function swapToLogin(): Promise<void> {
    if (!selectedId) return;
    try {
      await adapter.swapTo(selectedId);
      scheduleAccountsRefresh();
      pushToast({ type: "success", message: $t("Toast_AccountSwitched"), duration: 4000 });
    } catch (e) { await reportSwitchFailure(e); }
  }

  async function launchPlatformForSelection(): Promise<void> {
    const selected = accountById(selectedId);
    if (selectedId && selected && !adapter.currentSession(selected)) {
      try { await adapter.swapTo(selectedId); }
      catch (e) { await reportSwitchFailure(e); return; }
    }
    try {
      await adapter.launch();
      scheduleAccountsRefresh();
    } catch (e) { await reportLaunchFailure(e, name); }
  }

  async function runPlatformActionLocked(work: () => Promise<void>): Promise<void> {
    if (isActionBusy) return;
    isActionBusyValue = true;
    platformActionBusy.set({ busy: true, platformKey: name });
    try { await work(); }
    finally {
      isActionBusyValue = false;
      platformActionBusy.set({ busy: false, platformKey: "" });
      touchStatus();
    }
  }

  async function handlePlatformActionKind(kind: "login" | "addNew" | "launch" | "saveCurrent"): Promise<void> {
    await runPlatformActionLocked(async () => {
      if (kind === "launch") { await launchPlatformForSelection(); return; }
      if (kind === "addNew") {
        try {
          await adapter.addNew();
          scheduleAccountsRefresh();
          pushToast({ type: "success", message: $t("Toast_AccountSwitched"), duration: 4000 });
        } catch (e) { await reportSwitchFailure(e); }
        return;
      }
      if (kind === "saveCurrent") {
        if (adapter.saveCurrent) {
          actionBarStatus.set($t("Status_ActionBar_PreparingSave"));
          await adapter.saveCurrent();
        }
        return;
      }
      if (kind === "login") { await swapToLogin(); }
    });
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
        pushToast({ type: "success", message: $t("Toast_GameLaunchRequested"), duration: 4000 });
      } catch (e) { await reportLaunchFailure(e, name); }
    }
  }

  // ---- Context menu ----
  function buildSharedItems(acc: TAccount, rowId: string): {
    swapTo: MenuItemDef;
    changeName: MenuItemDef;
    createShortcut: MenuItemDef;
    changeImage: MenuItemDef;
    forget: MenuItemDef;
    notes: MenuItemDef;
    tags: MenuItemDef;
    gameStats: MenuItemDef | null;
  } {
    const tr = get(t);
    const imgUrl = (adapter.imageUrl(acc) ?? "").trim();
    const manual = adapter.manualProfileImage(acc);
    const openPick = () => { selectedId = rowId; touchStatus(); openImagePick(rowId); };

    return {
      swapTo: {
        label: tr("Context_SwapTo"),
        disabled: isActionBusy,
        action: () => {
          if (isActionBusy) return;
          selectedId = rowId;
          touchStatus();
          void swapToLogin();
        },
      },
      changeName: {
        label: tr("Context_ChangeName"),
        action: async () => {
          const next = await openPrompt({
            title: tr("Context_ChangeName"), body: "",
            positiveLabel: tr("Ok"), negativeLabel: tr("Button_Cancel"),
            initialValue: adapter.name(acc) ?? "",
          });
          if (next === null || !String(next).trim()) return;
          try {
            await adapter.rename(rowId, String(next).trim());
            await loadAccounts();
            pushToast({ type: "success", message: tr("Toast_AccountSaved"), duration: 3000 });
          } catch (e) {
            pushToast({ type: "error", message: formatToastWithError(tr("Toast_SaveFailed"), e), duration: 8000 });
          }
        },
      },
      createShortcut: {
        label: tr("Context_CreateShortcut"),
        action: async () => {
          try {
            const p = await Shortcuts.CreateAccountShortcut(
              name, rowId, adapter.name(acc) ?? rowId, "", "", adapter.accountLogin(acc) ?? "",
            );
            pushToast({ type: "success", message: `${tr("Toast_ShortcutCreated")}\n${p}`, duration: 6000 });
          } catch (e) {
            pushToast({ type: "error", message: formatToastWithError(tr("Toast_SwitchFailed"), e), duration: 8000 });
          }
        },
      },
      changeImage: (!imgUrl || !manual)
        ? { label: tr("Context_ChangeImage"), action: openPick }
        : {
            label: tr("Context_ChangeImage"), action: openPick,
            children: [
              { label: tr("Context_ChooseProfileImage"), action: openPick },
              {
                label: tr("Context_RemoveProfileImage"),
                action: async () => {
                  try {
                    await adapter.clearManualImage(rowId);
                    scheduleAccountsRefresh();
                    pushToast({ type: "success", message: tr("Toast_AccountSaved"), duration: 3000 });
                  } catch (e) {
                    pushToast({ type: "error", message: formatToastWithError(tr("Toast_SaveFailed"), e), duration: 8000 });
                  }
                },
              },
            ],
          },
      forget: {
        label: tr("Forget"),
        action: async () => {
          const ok = await openConfirm({
            title: tr("Forget"), body: tr("Prompt_ForgetAccount", { platform: name }), style: "yesno",
          });
          if (!ok) return;
          try {
            await adapter.forget(rowId);
            await loadAccounts();
            pushToast({ type: "success", message: tr("Toast_AccountSaved"), duration: 3000 });
          } catch (e) {
            pushToast({ type: "error", message: formatToastWithError(tr("Toast_SaveFailed"), e), duration: 8000 });
          }
        },
      },
      notes: {
        label: tr("Notes"),
        action: async () => {
          const cur = await adapter.getNote(rowId);
          const note = await openPrompt({
            title: tr("Notes"),
            body: tr("Modal_Title_AccountNotes", { accountName: adapter.name(acc) ?? rowId }),
            positiveLabel: tr("Ok"), negativeLabel: tr("Button_Cancel"),
            initialValue: cur ?? "", multiline: true,
          });
          if (note === null) return;
          try {
            await adapter.setNote(rowId, String(note));
            await loadAccounts();
            pushToast({ type: "success", message: tr("Toast_AccountSaved"), duration: 3000 });
          } catch (e) {
            pushToast({ type: "error", message: formatToastWithError(tr("Toast_SaveFailed"), e), duration: 8000 });
          }
        },
      },
      tags: buildTagsSectionMenuItem({
        platformKey: name, uniqueId: rowId,
        assignedTags: (adapter.tags(acc) ?? []) as TagDefRow[],
        tagDefs, tr,
        afterChange: async () => { await loadAccounts(); await loadTagDefs(); },
        onSuccess: () => pushToast({ type: "success", message: tr("Toast_AccountSaved"), duration: 2500 }),
        onError: (e) => pushToast({ type: "error", message: formatToastWithError(tr("Toast_SaveFailed"), e), duration: 8000 }),
      }),
      gameStats: hasGameStatsSupport
        ? { label: tr("Context_ManageGameStats"), action: () => openGameStatsModal(rowId) }
        : null,
    };
  }

  function ctxMenu(rowId: string): () => MenuItemDef[] {
    return () => {
      const acc = accountById(rowId);
      if (!acc) return [];
      const shared = buildSharedItems(acc, rowId);
      return adapter.buildMenu(acc, shared);
    };
  }

  // ---- Lifecycle ----
  onMount(() => {
    previousPage.set({ page: "home" });
    void preflightAdminForPlatform(name);
    void refreshGameStatsSupport();
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
      void handlePlatformActionKind(v.kind);
    });

    offAccountsRefresh = platformAccountsRefresh.subscribe((p) => {
      if (p.seq === 0 || p.platformKey !== name) return;
      scheduleAccountsRefresh();
    });

    offUpdateEvent = Events.On(adapter.updateEventName, (ev) => {
      applyPatchFromEvent(ev.data);
    });

    fileDropInterceptor.set(fileDropIntercept);
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
                      {#each Object.entries(gameStatsByAccount[rid]) as [gameTitle, metrics]}
                        <div class="acc_inline_gamestats_row">
                          <span class="acc_inline_gamestats_game">{gameTitle}</span>
                          <span class="acc_inline_gamestats_metrics">
                            {#each Object.values(metrics) as dto}
                              <span class="acc_inline_gamestats_metric">
                                {#if dto.indicatorMarkup}
                                  <span class="acc_inline_gamestats_ind">{@html dto.indicatorMarkup}</span>
                                {/if}
                                <span class="acc_inline_gamestats_val">{@html dto.statValue}</span>
                              </span>
                            {/each}
                          </span>
                        </div>
                      {/each}
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
<ActionBar />

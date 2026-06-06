<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import { Events } from "@wailsio/runtime";
  import ActionBar from "../components/ActionBar.svelte";
  import AccountImagePickOverlay from "../components/AccountImagePickOverlay.svelte";
  import ReorderPointerGrid from "../components/ReorderPointerGrid.svelte";
  import TagFilterBar from "../components/TagFilterBar.svelte";
  import AccountTagBubbles from "../components/AccountTagBubbles.svelte";
  import { route, previousPage, appBarTitle } from "../stores/nav";
  import { buildEpochMap } from "../lib/accountEpoch";
  import SearchOverlay, { type SearchResultRow } from "../components/SearchOverlay.svelte";
  import { actionBarStatus } from "../stores/actionBarStatus";
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
  import { AccountDTO, AccountImagePatch } from "../../bindings/TcNo-Acc-Switcher/internal/basic/models.js";
  import { GetPlatformExeIcon, LaunchPlatform } from "../lib/platformBindings";
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
  import { fileDropInterceptor } from "../stores/fileDropInterceptor";
  import { accountProfileImageDropActive } from "../stores/accountProfileImageDropUi";
  import { firstProfileImagePath } from "../lib/profileImageDrop";
  import GameStatsSetupModalBody from "../components/modals/GameStatsSetupModalBody.svelte";
  import "../styles/gamestats.scss";
  import "../styles/platformAccountsShared.scss";

  const PROFILE_FALLBACK = "/img/BasicDefault.webp";

  type GameStatMetricDTO = { statValue: string; indicatorMarkup: string };

  /** Bindings class typing can lag `currentSession`; keep explicit for the list row. */
  type BasicRow = InstanceType<typeof AccountDTO> & {
    currentSession?: boolean;
    avatarPending?: boolean;
    tags?: TagDefRow[];
  };

  export let name: string;

  let accounts: BasicRow[] = [];
  let accountIds: string[] = [];
  $: accountMap = new Map(accounts.map((a) => [a.uniqueId, a]));
  let loadError = "";
  let selectedUniqueId = "";
  let offPlatformAction: (() => void) | undefined;
  let offAccountsRefresh: (() => void) | undefined;
  let offBasicImageEvent: (() => void) | undefined;
  let lastHandledActionId = 0;
  let basicListRefreshTimers: ReturnType<typeof setTimeout>[] = [];
  let basicAcclistEl: HTMLDivElement | undefined;
  let overlayQuery = "";
  let overlayQueryDebounceTimer: ReturnType<typeof setTimeout> | null = null;
  let debouncedOverlayQuery = "";
  let offSort: (() => void) | undefined;
  let lastHandledSortId = 0;
  let basicAvatarEpoch: Record<string, number> = {};

  /** Per account: game title → metric key → markup from stats cache. */
  let gameStatsMarkupByAccount: Record<string, Record<string, Record<string, GameStatMetricDTO>>> = {};
  let hasGameStatsSupport = false;

  let accImagePick = { open: false, uniqueId: "", displayName: "", manual: false };
  let fileDragHoverRowId = "";

  let tagDefs: TagDefRow[] = [];
  let tagFilterMode: TagFilterMode = { kind: "all" };

  const SEARCH_MAX = 5;

  $: so = $searchOverlayCtrl;
  $: appBarTitle.set(name || "TcNo Account Switcher");
  $: isActionBusy = $platformActionBusy.busy;
  $: {
    const q = overlayQuery;
    if (overlayQueryDebounceTimer) clearTimeout(overlayQueryDebounceTimer);
    overlayQueryDebounceTimer = setTimeout(() => {
      debouncedOverlayQuery = q;
    }, 150);
  }
  $: basicSearchPrimary = buildBasicAccountRows(debouncedOverlayQuery, basicAvatarEpoch);
  $: if (name) {
    route.set({ page: "platform", platformName: name });
  }

  $: selectedAccountStore.set({
    platformKey: name,
    uniqueId: selectedUniqueId,
    displayName: accountById(selectedUniqueId)?.displayName ?? "",
    accountLogin: "",
  });

  function accountById(id: string): BasicRow | undefined {
    return accounts.find((a) => a.uniqueId === id);
  }

  function closeImagePick(): void {
    accImagePick = { open: false, uniqueId: "", displayName: "", manual: false };
    fileDragHoverRowId = "";
  }

  function openImagePick(rowId: string): void {
    const acc = accountById(rowId);
    accImagePick = {
      open: true,
      uniqueId: rowId,
      displayName: (acc?.displayName ?? rowId).trim(),
      manual: !!acc?.manualProfileImage,
    };
  }

  async function applyImageFromOverlay(path: string): Promise<void> {
    const uid = accImagePick.uniqueId.trim();
    if (!uid || !name) {
      return;
    }
    await BasicService.ChangeAccountImage(name, uid, path);
    await loadAccounts();
    bumpBasicAvatarEpoch(uid);
    pushToast({
      type: "success",
      message: get(t)("Toast_AccountSaved"),
      duration: 3000,
    });
  }

  async function removeImageFromOverlay(): Promise<void> {
    const uid = accImagePick.uniqueId.trim();
    if (!uid || !name) {
      return;
    }
    await BasicService.ClearManualAccountProfileImage(name, uid);
    await loadAccounts();
    bumpBasicAvatarEpoch(uid);
    scheduleAccountsRefresh();
    pushToast({
      type: "success",
      message: get(t)("Toast_AccountSaved"),
      duration: 3000,
    });
  }

  function hasFilesDragType(ev: DragEvent): boolean {
    const types = ev.dataTransfer?.types;
    return !!(types && Array.from(types as unknown as Iterable<string>).includes("Files"));
  }

  function onBasicAccDragOver(ev: DragEvent, rid: string): void {
    if (!hasFilesDragType(ev)) {
      return;
    }
    ev.preventDefault();
    fileDragHoverRowId = rid;
  }

  function onBasicAccDragLeave(ev: DragEvent, rid: string): void {
    if (!hasFilesDragType(ev)) {
      return;
    }
    const rel = ev.relatedTarget as Node | null;
    const cur = ev.currentTarget as HTMLElement | null;
    if (!rel || !cur?.contains(rel)) {
      if (fileDragHoverRowId === rid) {
        fileDragHoverRowId = "";
      }
    }
  }

  function onBasicAccListDragLeave(ev: DragEvent): void {
    if (!hasFilesDragType(ev)) {
      return;
    }
    const rel = ev.relatedTarget as Node | null;
    const root = basicAcclistEl;
    if (!root || rel === null || !root.contains(rel)) {
      fileDragHoverRowId = "";
    }
  }

  async function basicPlatformFileDropIntercept(paths: string[]): Promise<boolean> {
    const img = firstProfileImagePath(paths);
    if (!img || !name) {
      return false;
    }
    try {
      if (accImagePick.open && accImagePick.uniqueId.trim()) {
        const target = accImagePick.uniqueId.trim();
        await BasicService.ChangeAccountImage(name, target, img);
        await loadAccounts();
        bumpBasicAvatarEpoch(target);
        pushToast({
          type: "success",
          message: get(t)("Toast_AccountSaved"),
          duration: 3000,
        });
        closeImagePick();
        return true;
      }
      const hover = fileDragHoverRowId.trim();
      if (hover) {
        await BasicService.ChangeAccountImage(name, hover, img);
        await loadAccounts();
        bumpBasicAvatarEpoch(hover);
        pushToast({
          type: "success",
          message: get(t)("Toast_AccountSaved"),
          duration: 3000,
        });
        fileDragHoverRowId = "";
        return true;
      }
    } catch (e) {
      pushToast({
        type: "error",
        message: formatToastWithError(get(t)("Toast_SaveFailed"), e),
        duration: 8000,
      });
      return true;
    }
    return false;
  }

  function applyBasicImagePatch(p: AccountImagePatch): void {
    const plat = String(p.platformKey ?? "").trim();
    const uid = String(p.uniqueId ?? "").trim();
    if (!plat || !uid || plat !== name) {
      return;
    }
    let hit = false;
    accounts = accounts.map((r) => {
      if (r.uniqueId !== uid) {
        return r;
      }
      hit = true;
      const prevUrl = (r.imageUrl ?? "").trim();
      // Allow explicit "" from backend/events (clear avatar); old logic kept stale URL whenever imageUrl was empty.
      const nextUrl = p.imageUrl != null ? String(p.imageUrl).trim() : prevUrl;
      const nextPending =
        typeof p.avatarPending === "boolean" ? p.avatarPending : (r.avatarPending ?? false);
      const nextManual =
        typeof p.manualProfileImage === "boolean" ? p.manualProfileImage : (r.manualProfileImage ?? false);
      return {
        ...r,
        imageUrl: nextUrl,
        avatarPending: nextPending,
        manualProfileImage: nextManual,
      } as BasicRow;
    });
    if (hit) {
      basicAvatarEpoch = { ...basicAvatarEpoch, [uid]: (basicAvatarEpoch[uid] ?? 0) + 1 };
    }
  }

  $: tagFilterBarLabel =
    tagFilterMode.kind === "all"
      ? $t("Tags_All")
      : tagFilterMode.kind === "untagged"
        ? $t("Tags_Filter_Untagged")
        : tagFilterMode.name;

  $: displayAccountIds = (() => {
    const ids = accountIds;
    if (tagFilterMode.kind === "all") {
      return ids;
    }
    if (tagFilterMode.kind === "untagged") {
      return ids.filter((id) => (accountMap.get(id)?.tags?.length ?? 0) === 0);
    }
    const tid = tagFilterMode.id;
    return ids.filter((id) => accountMap.get(id)?.tags?.some((x) => x.id === tid));
  })();

  $: reorderDisabled = tagFilterMode.kind !== "all";

  $: {
    if (
      selectedUniqueId &&
      displayAccountIds.length > 0 &&
      !displayAccountIds.includes(selectedUniqueId)
    ) {
      selectedUniqueId = "";
      touchStatus();
    }
  }

  function touchStatus(): void {
    if (isActionBusy) {
      return;
    }
    const acc = accountById(selectedUniqueId);
    actionBarStatus.set(
      acc ? $t("Status_SelectedAccount", { name: acc.displayName || acc.uniqueId }) : "",
    );
  }

  function scheduleAccountsRefresh(): void {
    for (const t of basicListRefreshTimers) clearTimeout(t);
    basicListRefreshTimers = [];
    void loadAccounts();
    basicListRefreshTimers.push(
      setTimeout(() => void loadAccounts(), 700),
      setTimeout(() => void loadAccounts(), 2200),
    );
  }

  /** Cache file is overwritten in place — public imageUrl often stays identical; bump so `{#key}` remounts `<img>`. */
  function bumpBasicAvatarEpoch(uniqueId: string): void {
    const uid = uniqueId.trim();
    if (!uid) {
      return;
    }
    basicAvatarEpoch = { ...basicAvatarEpoch, [uid]: (basicAvatarEpoch[uid] ?? 0) + 1 };
  }

  async function loadTagDefs(): Promise<void> {
    try {
      const rows = await BasicService.ListTagDefinitions(name);
      tagDefs = (rows as { id: string; name: string; color: string }[]).map((r) => ({
        id: r.id,
        name: r.name,
        color: r.color,
      }));
    } catch {
      tagDefs = [];
    }
  }

  async function refreshGameStatsMarkup(accountIds: string[]): Promise<void> {
    if (!name.trim() || accountIds.length === 0) {
      gameStatsMarkupByAccount = {};
      return;
    }
    try {
      const pairs = await Promise.all(
        accountIds.map(async (uid) => {
          try {
            const m = await BasicService.GetUserStatsAllGamesMarkup(name, uid);
            return [uid, m ?? {}] as const;
          } catch {
            return [uid, {}] as const;
          }
        }),
      );
      gameStatsMarkupByAccount = Object.fromEntries(pairs) as Record<
        string,
        Record<string, Record<string, GameStatMetricDTO>>
      >;
    } catch {
      gameStatsMarkupByAccount = {};
    }
  }

  async function refreshGameStatsSupport(): Promise<void> {
    try {
      const games = await BasicService.GetAvailableGames(name);
      hasGameStatsSupport = (games?.length ?? 0) > 0;
    } catch {
      hasGameStatsSupport = false;
    }
  }

  function openGameStatsModal(rowId: string): void {
    const acc = accountById(rowId);
    void openAlertNoButton({
      title: get(t)("Context_ManageGameStats"),
      bodyComponent: GameStatsSetupModalBody,
      bodyProps: {
        platformKey: name,
        uniqueId: rowId,
        displayName: (acc?.displayName ?? rowId).trim(),
        onApplied: () => void loadAccounts(),
      },
    });
  }

  async function loadAccounts(): Promise<void> {
    loadError = "";
    const prevById = new Map(accounts.map((a) => [a.uniqueId, a]));
    try {
      const rows = (await BasicService.GetAccounts(name)) as BasicRow[];
      basicAvatarEpoch = buildEpochMap(rows, prevById, (r) => r.uniqueId, basicAvatarEpoch);
      accounts = rows;
      accountIds = rows.map((r) => r.uniqueId);
      const liveRow = rows.find((r) => r.currentSession);
      platformLiveSessionId.set({
        platformKey: name,
        uniqueId: (liveRow?.uniqueId ?? "").trim(),
      });
      const stillValid = selectedUniqueId && rows.some((r) => r.uniqueId === selectedUniqueId);
      selectedUniqueId = stillValid ? selectedUniqueId : "";
      touchStatus();
      await loadTagDefs();
      await refreshGameStatsMarkup(rows.map((r) => r.uniqueId));
    } catch (e) {
      loadError = formatWailsError(e) || String(e);
      accounts = [];
      accountIds = [];
      selectedUniqueId = "";
      gameStatsMarkupByAccount = {};
      actionBarStatus.set("");
      platformLiveSessionId.set({ platformKey: "", uniqueId: "" });
    }
  }

  function onItemClick(e: CustomEvent<{ id: string }>): void {
    selectedUniqueId = e.detail.id;
    touchStatus();
  }

  function clearSelection(): void {
    if (!selectedUniqueId) {
      return;
    }
    selectedUniqueId = "";
    touchStatus();
  }

  function onAccountsAreaClick(e: MouseEvent): void {
    const target = e.target as HTMLElement | null;
    if (!target) {
      return;
    }
    if (target.closest("[data-dnd-cell]")) {
      return;
    }
    clearSelection();
  }

  function onWindowKeyDown(e: KeyboardEvent): void {
    if (e.key !== "Escape") {
      return;
    }
    if (get(activeModal)) {
      return;
    }
    if (accImagePick.open) {
      e.preventDefault();
      closeImagePick();
      return;
    }
    clearSelection();
  }

  function onReorder(e: CustomEvent<{ items: string[] }>): void {
    accountIds = e.detail.items;
    BasicService.SaveAccountOrder(name, e.detail.items).catch(() => {});
  }

  function displayKeyForSort(uid: string): string {
    const a = accountById(uid);
    return (a?.displayName ?? uid).trim().toLowerCase();
  }

  function lastUsedMsForSort(uid: string): number {
    const a = accountById(uid);
    const t = Date.parse((a?.lastUsed ?? "").trim());
    return Number.isNaN(t) ? 0 : t;
  }

  function applyPlatformSort(kind: PlatformSortKind): void {
    if (kind === "steam_user_asc" || kind === "steam_user_desc") {
      return;
    }
    const ids = [...accountIds];
    const cmpAlpha = (x: string, y: string) =>
      displayKeyForSort(x).localeCompare(displayKeyForSort(y));
    switch (kind) {
      case "alpha_asc":
        ids.sort(cmpAlpha);
        break;
      case "alpha_desc":
        ids.sort((x, y) => -cmpAlpha(x, y));
        break;
      case "lastused_new_old":
      case "date_new_old":
        ids.sort(
          (x, y) =>
            lastUsedMsForSort(y) - lastUsedMsForSort(x) || cmpAlpha(x, y),
        );
        break;
      case "lastused_old_new":
      case "date_old_new":
        ids.sort(
          (x, y) =>
            lastUsedMsForSort(x) - lastUsedMsForSort(y) || cmpAlpha(x, y),
        );
        break;
      default:
        return;
    }
    accountIds = ids;
    BasicService.SaveAccountOrder(name, ids).catch(() => {});
  }

  function buildBasicAccountRows(
    q: string,
    epochs: Record<string, number>,
  ): SearchResultRow[] {
    const tr = get(t);
    const trimmed = q.trim();
    const hay = (acc: BasicRow) => {
      const blob = `${acc.displayName}\n${acc.uniqueId}\n${acc.note ?? ""}`;
      return fuzzyWordsMatch(trimmed, blob);
    };
    let list = trimmed ? accounts.filter((a) => hay(a)) : accounts.slice(0, SEARCH_MAX);
    if (trimmed) {
      list = list.slice(0, SEARCH_MAX);
    }
    return list.map((a) => ({
      key: `a:${a.uniqueId}`,
      title: a.displayName || a.uniqueId,
      badge: tr("Search_Section_Account"),
      accountIconUrl: offlineSafeImageSrc(
        get(offlineMode),
        withAssetCacheBust(
          a.imageUrl && !(a as { avatarPending?: boolean }).avatarPending ? a.imageUrl : undefined,
          epochs[a.uniqueId] ?? 0,
        ),
        PROFILE_FALLBACK,
      ),
    }));
  }

  async function onSearchPick(ev: CustomEvent<SearchResultRow>): Promise<void> {
    const row = ev.detail;
    closeSearchOverlay();
    if (row.key.startsWith("a:")) {
      const uid = row.key.slice(2);
      selectedUniqueId = uid;
      touchStatus();
      await swapToLogin();
    }
  }

  async function reportBasicSwitchFailure(e: unknown): Promise<void> {
    await offerRestartIfNeedsAdmin(e, name);
    if (isNeedsAdminError(e)) {
      return;
    }
    pushToast({
      type: "error",
      message: formatToastWithError($t("Toast_SwitchFailed"), e),
      duration: 8000,
    });
  }

  async function reportBasicSaveFailure(e: unknown): Promise<void> {
    await offerRestartIfNeedsAdmin(e, name);
    if (isNeedsAdminError(e)) {
      return;
    }
    pushToast({
      type: "error",
      message: formatToastWithError($t("Toast_SaveFailed"), e),
      duration: 8000,
    });
  }

  async function swapToLogin(): Promise<void> {
    if (!selectedUniqueId) {
      return;
    }
    try {
      await BasicService.SwapToAccount(name, selectedUniqueId, []);
      scheduleAccountsRefresh();
      pushToast({
        type: "success",
        message: $t("Toast_AccountSwitched"),
        duration: 4000,
      });
    } catch (e) {
      await reportBasicSwitchFailure(e);
    }
  }

  async function launchPlatformForSelection(): Promise<void> {
    const selected = accountById(selectedUniqueId);
    if (selectedUniqueId && selected && !selected.currentSession) {
      try {
        await BasicService.SwapToAccount(name, selectedUniqueId, []);
      } catch (e) {
        await reportBasicSwitchFailure(e);
        return;
      }
    }
    try {
      await LaunchPlatform(name);
      scheduleAccountsRefresh();
    } catch (e) {
      await reportLaunchFailure(e, name);
    }
  }

  async function saveCurrentPrompt(): Promise<void> {
    let suggestedName = "";
    try {
      suggestedName = await (
        BasicService as unknown as { SuggestedSaveAccountName: (platformKey: string) => Promise<string> }
      ).SuggestedSaveAccountName(name);
    } catch (e) {
      await offerRestartIfNeedsAdmin(e, name);
      if (isNeedsAdminError(e)) {
        return;
      }
      suggestedName = "";
    }
    const displayName = await openPrompt({
      title: $t("Modal_SaveCurrent_Title"),
      body: $t("Modal_SaveCurrent_Body"),
      positiveLabel: $t("Button_SaveCurrent"),
      negativeLabel: $t("Button_Cancel"),
      initialValue: suggestedName || "",
    });
    if (displayName === null || !String(displayName).trim()) {
      return;
    }
    try {
      await BasicService.SaveCurrent(name, String(displayName).trim());
      await loadAccounts();
      pushToast({
        type: "success",
        message: $t("Toast_AccountSaved"),
        duration: 4000,
      });
    } catch (e) {
      await reportBasicSaveFailure(e);
    }
  }

  async function runPlatformActionLocked(work: () => Promise<void>): Promise<void> {
    if (isActionBusy) {
      return;
    }
    platformActionBusy.set({ busy: true, platformKey: name });
    try {
      await work();
    } finally {
      platformActionBusy.set({ busy: false, platformKey: "" });
      touchStatus();
    }
  }

  async function handlePlatformActionKind(
    kind: "login" | "addNew" | "launch" | "saveCurrent",
  ): Promise<void> {
    await runPlatformActionLocked(async () => {
      if (kind === "launch") {
        await launchPlatformForSelection();
        return;
      }
      if (kind === "addNew") {
        try {
          await BasicService.AddNew(name);
          scheduleAccountsRefresh();
          pushToast({
            type: "success",
            message: $t("Toast_AccountSwitched"),
            duration: 4000,
          });
        } catch (e) {
          await reportBasicSwitchFailure(e);
        }
        return;
      }
      if (kind === "saveCurrent") {
        actionBarStatus.set($t("Status_ActionBar_PreparingSave"));
        await saveCurrentPrompt();
        return;
      }
      if (kind === "login") {
        await swapToLogin();
      }
    });
  }

  function slotKey(x: string | null | undefined): string {
    return x ?? "";
  }

  function onTagFilterBarClick(ev: Event): void {
    openTagFilterMenu({
      ev: ev as MouseEvent,
      tagDefs,
      tr: get(t),
      onPick: (mode) => {
        tagFilterMode = mode;
      },
    });
  }

  function basicCtxMenu(rowId: string): () => MenuItemDef[] {
    return () => {
      const tr = get(t);
      const acc = accounts.find((a) => a.uniqueId === rowId);
      if (!acc) {
        return [];
      }
      return [
        {
          label: tr("Context_SwapTo"),
          disabled: isActionBusy,
          action: () => {
            if (isActionBusy) {
              return;
            }
            selectedUniqueId = rowId;
            touchStatus();
            void swapToLogin();
          },
        },
        {
          label: tr("Context_ChangeName"),
          action: async () => {
            const next = await openPrompt({
              title: tr("Context_ChangeName"),
              body: "",
              positiveLabel: tr("Ok"),
              negativeLabel: tr("Button_Cancel"),
              initialValue: acc.displayName ?? "",
            });
            if (next === null || !String(next).trim()) {
              return;
            }
            try {
              await BasicService.RenameAccount(name, rowId, String(next).trim());
              await loadAccounts();
              pushToast({
                type: "success",
                message: tr("Toast_AccountSaved"),
                duration: 3000,
              });
            } catch (e) {
              pushToast({
                type: "error",
                message: formatToastWithError(tr("Toast_SaveFailed"), e),
                duration: 8000,
              });
            }
          },
        },
        {
          label: tr("Context_CreateShortcut"),
          action: async () => {
            try {
              const p = await Shortcuts.CreateAccountShortcut(
                name,
                rowId,
                acc.displayName ?? rowId,
                "",
                "",
                "",
              );
              pushToast({
                type: "success",
                message: `${tr("Toast_ShortcutCreated")}\n${p}`,
                duration: 6000,
              });
            } catch (e) {
              pushToast({
                type: "error",
                message: formatToastWithError(tr("Toast_SwitchFailed"), e),
                duration: 8000,
              });
            }
          },
        },
        (() => {
          const imgUrl = (acc?.imageUrl ?? "").trim();
          const manual = !!acc?.manualProfileImage;
          const openPick = () => {
            selectedUniqueId = rowId;
            touchStatus();
            openImagePick(rowId);
          };
          if (!imgUrl || !manual) {
            return {
              label: tr("Context_ChangeImage"),
              action: openPick,
            };
          }
          return {
            label: tr("Context_ChangeImage"),
            action: openPick,
            children: [
              {
                label: tr("Context_ChooseProfileImage"),
                action: openPick,
              },
              {
                label: tr("Context_RemoveProfileImage"),
                action: async () => {
                  try {
                    await BasicService.ClearManualAccountProfileImage(name, rowId);
                    scheduleAccountsRefresh();
                    pushToast({
                      type: "success",
                      message: tr("Toast_AccountSaved"),
                      duration: 3000,
                    });
                  } catch (e) {
                    pushToast({
                      type: "error",
                      message: formatToastWithError(tr("Toast_SaveFailed"), e),
                      duration: 8000,
                    });
                  }
                },
              },
            ],
          };
        })(),
        {
          label: tr("Forget"),
          action: async () => {
            const ok = await openConfirm({
              title: tr("Forget"),
              body: tr("Prompt_ForgetAccount", { platform: name }),
              style: "yesno",
            });
            if (!ok) {
              return;
            }
            try {
              await BasicService.ForgetAccount(name, rowId);
              await loadAccounts();
              pushToast({
                type: "success",
                message: tr("Toast_AccountSaved"),
                duration: 3000,
              });
            } catch (e) {
              pushToast({
                type: "error",
                message: formatToastWithError(tr("Toast_SaveFailed"), e),
                duration: 8000,
              });
            }
          },
        },
        buildTagsSectionMenuItem({
          platformKey: name,
          uniqueId: rowId,
          assignedTags: (acc.tags ?? []) as TagDefRow[],
          tagDefs,
          tr,
          afterChange: async () => {
            await loadAccounts();
            await loadTagDefs();
          },
          onSuccess: () =>
            pushToast({
              type: "success",
              message: tr("Toast_AccountSaved"),
              duration: 2500,
            }),
          onError: (e) =>
            pushToast({
              type: "error",
              message: formatToastWithError(tr("Toast_SaveFailed"), e),
              duration: 8000,
            }),
        }),
        ...(hasGameStatsSupport
          ? ([
              {
                label: tr("Context_ManageGameStats"),
                action: () => openGameStatsModal(rowId),
              },
            ] as MenuItemDef[])
          : []),
        {
          label: tr("Notes"),
          action: async () => {
            const cur = await BasicService.GetAccountNote(name, rowId);
            const note = await openPrompt({
              title: tr("Notes"),
              body: tr("Modal_Title_AccountNotes", {
                accountName: acc.displayName ?? rowId,
              }),
              positiveLabel: tr("Ok"),
              negativeLabel: tr("Button_Cancel"),
              initialValue: cur ?? "",
              multiline: true,
            });
            if (note === null) {
              return;
            }
            try {
              await BasicService.SetAccountNote(name, rowId, String(note));
              await loadAccounts();
              pushToast({
                type: "success",
                message: tr("Toast_AccountSaved"),
                duration: 3000,
              });
            } catch (e) {
              pushToast({
                type: "error",
                message: formatToastWithError(tr("Toast_SaveFailed"), e),
                duration: 8000,
              });
            }
          },
        },
      ];
    };
  }

  onMount(() => {
    previousPage.set({ page: "home" });
    void preflightAdminForPlatform(name);
    void refreshGameStatsSupport();
    void loadAccounts();
    void GetPlatformExeIcon(name).then((u: string) => platformExeIconUrl.set(u ?? ""));
    offSort = platformListSort.subscribe((sig) => {
      if (!sig || sig.id <= lastHandledSortId) {
        return;
      }
      lastHandledSortId = sig.id;
      applyPlatformSort(sig.kind);
    });
    offPlatformAction = platformAction.subscribe((v) => {
      if (!v || v.id === lastHandledActionId) {
        return;
      }
      lastHandledActionId = v.id;
      void handlePlatformActionKind(v.kind);
    });
    offAccountsRefresh = platformAccountsRefresh.subscribe((p) => {
      if (p.seq === 0 || p.platformKey !== name) {
        return;
      }
      scheduleAccountsRefresh();
    });
    offBasicImageEvent = Events.On("basic-account-image-updated", (ev) => {
      const raw = ev.data;
      const p =
        raw instanceof AccountImagePatch
          ? raw
          : AccountImagePatch.createFrom(raw as Record<string, unknown>);
      applyBasicImagePatch(p);
    });
    void (async () => {
      try {
        const remote = await BasicService.PlatformUsesRemoteProfileImages(name);
        if (remote) {
          void BasicService.StartBasicProfileImageRefresh(name);
        }
      } catch {
        /* ignore */
      }
    })();
    fileDropInterceptor.set(basicPlatformFileDropIntercept);
  });

  onDestroy(() => {
    for (const t of basicListRefreshTimers) clearTimeout(t);
    basicListRefreshTimers = [];
    if (overlayQueryDebounceTimer) clearTimeout(overlayQueryDebounceTimer);
    selectedAccountStore.set({ platformKey: "", uniqueId: "", displayName: "", accountLogin: "" });
    platformLiveSessionId.set({ platformKey: "", uniqueId: "" });
    platformAction.set(null);
    offPlatformAction?.();
    offSort?.();
    offAccountsRefresh?.();
    offBasicImageEvent?.();
    accountProfileImageDropActive.set(false);
    fileDropInterceptor.set(null);
    platformAccountsRefresh.set({ seq: 0, platformKey: "" });
    platformExeIconUrl.set("");
    actionBarStatus.set("");
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
        primaryRows={basicSearchPrimary}
        categoryRows={[]}
        categoryHint=""
        gameRows={[]}
        gameHint=""
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
        bind:this={basicAcclistEl}
        on:click={onAccountsAreaClick}
        on:dragleave={onBasicAccListDragLeave}
      >
        {#if tagDefs.length > 0}
          <TagFilterBar label={tagFilterBarLabel} onClick={onTagFilterBarClick} />
        {/if}
        <ReorderPointerGrid
          items={displayAccountIds}
          reorderDisabled={reorderDisabled}
          listClass="acc_list"
          itemClass="acc_list_item acc_list_item--drag"
          placeholderClass="acc_list_item placeHolderAcc"
          ghostClass="acc_list_item acc_list_item--ghost"
          ariaLabel="Accounts"
          on:reorder={onReorder}
          on:itemclick={onItemClick}
        >
          <svelte:fragment slot="item" let:rowId>
            {@const rid = slotKey(rowId)}
            {@const acc = accountMap.get(rid)}
            {@const radioId = `basic-acc-${rid}`}
            <div class="acc_list_item_inner">
              <input
                type="radio"
                class="acc"
                id={radioId}
                name="basic-accounts"
                value={rid}
                bind:group={selectedUniqueId}
                on:change={touchStatus}
              />
              <label
                for={radioId}
                class="acc"
                class:currentAcc={acc?.currentSession}
                class:acc--profile-drop-target={$accountProfileImageDropActive && !accImagePick.open}
                class:acc--drop-target={fileDragHoverRowId === rid}
                on:dragover={(e) => onBasicAccDragOver(e, rid)}
                on:dragleave={(e) => onBasicAccDragLeave(e, rid)}
                use:ctxMenuAction={{
                  items: basicCtxMenu(rid),
                  beforeOpen: () => {
                    selectedUniqueId = rid;
                    touchStatus();
                  },
                }}
                use:tooltip={acc?.currentSession
                  ? {
                      text: $t("Tooltip_CurrentAccount"),
                      placement: "right",
                      boundary: basicAcclistEl,
                    }
                  : undefined}
                on:dblclick|preventDefault={() => {
                  if (isActionBusy) {
                    return;
                  }
                  selectedUniqueId = rid;
                  touchStatus();
                  void swapToLogin();
                }}
              >
                {#if $accountProfileImageDropActive && !accImagePick.open}
                  <div
                    class="acc_profile_drop_overlay"
                    class:acc_profile_drop_overlay--hover={fileDragHoverRowId === rid}
                    aria-hidden="true"
                  >
                    <div class="acc_profile_drop_overlay__center">
                      <div class="acc_profile_drop_overlay__icon" aria-hidden="true">
                        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"
                          ><path
                            fill="currentColor"
                            d="M5 20h14v-2H5v2zM19 9h-4V3H9v6H5l7 7 7-7z"
                          /></svg
                        >
                      </div>
                      <span class="acc_profile_drop_overlay__label">{$t("Drop_SetAccountIcon")}</span>
                    </div>
                  </div>
                {/if}
                {#key `${rid}:${basicAvatarEpoch[rid] ?? 0}`}
                  <img
                    src={offlineSafeImageSrc(
                      $offlineMode,
                      withAssetCacheBust(
                        acc?.imageUrl && !acc?.avatarPending ? acc.imageUrl : undefined,
                        basicAvatarEpoch[rid] ?? 0,
                      ),
                      PROFILE_FALLBACK,
                    )}
                    alt=""
                    draggable="false"
                  />
                {/key}
                <h6 class="displayName">{acc?.displayName ?? rid}</h6>
                <AccountTagBubbles tags={acc?.tags ?? []} />
                {#if acc?.note}
                  <p class="acc_note">{acc.note}</p>
                {/if}
                {#if gameStatsMarkupByAccount[rid] && Object.keys(gameStatsMarkupByAccount[rid]).length > 0}
                  <div class="acc_inline_gamestats" aria-label={$t("Context_ManageGameStats")}>
                    {#each Object.entries(gameStatsMarkupByAccount[rid]) as [gameTitle, metrics]}
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
                {#if acc?.showLastUsed && (acc?.lastUsed ?? "").trim()}
                  <p class="acc_lastused">
                    {formatLastLoginForLocale(acc.lastUsed, $locale)}
                  </p>
                {/if}
              </label>
            </div>
          </svelte:fragment>
        </ReorderPointerGrid>
      </div>
    </div>
    </div>
    <AccountImagePickOverlay
      bind:open={accImagePick.open}
      accountDisplayName={accImagePick.displayName}
      showRemoveButton={accImagePick.manual}
      onClose={closeImagePick}
      onApplyPath={applyImageFromOverlay}
      onRemoveManual={removeImageFromOverlay}
    />
  {/if}
</div>
<svelte:window on:keydown={onWindowKeyDown} />
<ActionBar />

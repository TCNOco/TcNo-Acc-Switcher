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
    selectedAccount as selectedAccountStore,
    platformLiveSessionId,
    platformAccountsRefresh,
  } from "../stores/platformPage";
  import { pushToast } from "../stores/toast";
  import * as SteamService from "../../bindings/TcNo-Acc-Switcher/internal/steam/steamservice.js";
  import { GetPlatformExeIcon } from "../lib/platformBindings";
  import { AccountDTO, AccountPatch } from "../../bindings/TcNo-Acc-Switcher/internal/steam/models.js";
  import { locale, t } from "../stores/i18n";
  import { formatLastLoginForLocale } from "../lib/formatLastLogin";
  import { formatToastWithError, formatWailsError } from "../lib/formatWailsError";
  import {
    isNeedsAdminError,
    offerRestartIfNeedsAdmin,
    preflightAdminForPlatform,
    reportLaunchFailure,
  } from "../lib/adminFlow";
  import { tooltip } from "../lib/actions/tooltip";
  import { miniProfileHover } from "../lib/actions/miniProfileHover";
  import "../styles/miniprofile.scss";
  import "../styles/gamestats.scss";
  import "../styles/platformAccountsShared.scss";
  import { contextMenu as ctxMenuAction } from "../lib/actions/contextMenu";
  import type { MenuItemDef } from "../stores/contextMenu";
  import * as BasicService from "../../bindings/TcNo-Acc-Switcher/internal/basic/basicservice.js";
  import * as Shortcuts from "wails-shortcuts-service";
  import { ListPayload } from "../../bindings/TcNo-Acc-Switcher/internal/shortcuts/models.js";
  import { get } from "svelte/store";
  import { activeModal, openAlertNoButton, openConfirm, openPrompt } from "../stores/modal";
  import { offlineMode, offlineSafeImageSrc, withAssetCacheBust } from "../stores/offlineMode";
  import { fuzzyWordsMatch } from "../lib/searchFuzzy";
  import { closeSearchOverlay, searchOverlayCtrl } from "../stores/searchOverlay";
  import { platformListSort, type PlatformSortKind } from "../stores/platformListSort";
  import { fileDropInterceptor } from "../stores/fileDropInterceptor";
  import { accountProfileImageDropActive } from "../stores/accountProfileImageDropUi";
  import { firstProfileImagePath } from "../lib/profileImageDrop";
  import {
    buildTagsSectionMenuItem,
    openTagFilterMenu,
    type TagDefRow,
    type TagFilterMode,
  } from "../lib/accountTagsContext";
  import GameStatsSetupModalBody from "../components/modals/GameStatsSetupModalBody.svelte";

  type GameStatMetricDTO = { statValue: string; indicatorMarkup: string };

  const STEAM_USERDATA_ERR_KEYS = new Set([
    "Toast_NoValidSteamId",
    "Toast_SameAccount",
    "Toast_NoFindSteamUserdata",
    "Toast_NoFindGameBackup",
  ]);

  function mapSteamUserdataI18nError(
    err: unknown,
    tr: (k: string, v?: Record<string, string | number>) => string,
  ): string {
    return formatWailsError(err, {
      translateMessage: (k) => tr(k),
      i18nFirstLineKeys: STEAM_USERDATA_ERR_KEYS,
    });
  }

  /** Bindings row + client-only syncError; some JSON fields are explicit — TS sometimes omits quoted keys on generated classes. */
  type SteamAccountRow = InstanceType<typeof AccountDTO> & {
    tags?: TagDefRow[];
    syncError?: string;
    currentSession: boolean;
    showShortNotes: boolean;
    note: string;
    avatarFrameUrl?: string;
    miniProfileHtml?: string;
    showMiniProfile?: boolean;
    showAvatarFrame?: boolean;
  };

  /** Patch fields used by mini profile / frame (explicit for TS + Wails bindings). */
  type SteamAccountPatch = AccountPatch & {
    avatarFrameUrl?: string;
    miniProfileHtml?: string;
    showMiniProfile?: boolean;
    showAvatarFrame?: boolean;
  };

  /** Vite `public/img/` → served at `/img/`; shown until Steam profile image is ready */
  const PROFILE_PLACEHOLDER = "/img/BasicDefault.webp";
  /** Same as `GameShortcutBar` when a shortcut has no usable icon. */
  const SHORTCUT_ICON_FALLBACK = "/img/icons/file.svg";

  /**
   * App IDs omitted:
   * 228980 = Steamworks Common Redistributables.
   */
  const STEAM_CONTEXT_MENU_HIDDEN_APP_IDS = new Set<string>(["228980"]);

  export let name: string;

  let steamAccounts: SteamAccountRow[] = [];
  /** Bumped when a row receives a patch so `{#key}` remounts that tile (avoids stuck "Updating…" on last row). */
  let rowEpoch: Record<string, number> = {};
  $: steamAccountMap = new Map(steamAccounts.map((a) => [a.steamId64, a]));
  /** Scroll/content box for Steam tooltips (keeps popovers off the window chrome). */
  let steamAcclistEl: HTMLDivElement | null = null;
  /** `.main-content` root for mini profile clamping. */
  let steamMainEl: HTMLDivElement | null = null;
  let steamIds: string[] = [];
  let steamLoadError = "";
  /** Per Steam id: game title → metric key → stats cache markup (Basic game stats for Steam platform). */
  let steamGameStatsByAccount: Record<string, Record<string, Record<string, GameStatMetricDTO>>> = {};
  let hasGameStatsSupport = false;
  let offSteamEvent: (() => void) | undefined;
  let offShortcutsUpdated: (() => void) | undefined;
  let offPlatformAction: (() => void) | undefined;
  let offAccountsRefresh: (() => void) | undefined;
  let lastHandledActionId = 0;
  /** Cleared on destroy; staggered reloads pick up Steam writing loginusers after swap/launch. */
  let steamListRefreshTimers: ReturnType<typeof setTimeout>[] = [];
  /** Selected row for switching (radio group); drives footer status. */
  let selectedSteamId = "";

  let installedGames: { appId: string; name: string }[] = [];
  /** Maps from `Shortcuts.ListShortcuts` / `shortcuts-updated` — same PNG paths as the footer bar. */
  let steamShortcutIconByAppId: Record<string, string> = {};
  let steamShortcutIconByStemKey: Record<string, string> = {};
  /** Per account: app IDs with `userdata/.../id` and with `Backups/Steam/.../id` (drives Game data submenu). */
  let gameDataBySteamId: Record<string, { userdata: Set<string>; backup: Set<string> }> = {};

  let overlayQuery = "";
  let overlayQueryDebounceTimer: ReturnType<typeof setTimeout> | null = null;
  let debouncedOverlayQuery = "";
  let offSort: (() => void) | undefined;
  let lastHandledSortId = 0;

  let tagDefs: TagDefRow[] = [];
  let tagFilterMode: TagFilterMode = { kind: "all" };

  let steamImagePick = { open: false, steamId: "", displayName: "", manual: false };
  let steamFileDragHoverRowId = "";

  const SEARCH_MAX = 5;

  $: so = $searchOverlayCtrl;
  $: {
    const q = overlayQuery;
    if (overlayQueryDebounceTimer) clearTimeout(overlayQueryDebounceTimer);
    overlayQueryDebounceTimer = setTimeout(() => {
      debouncedOverlayQuery = q;
    }, 150);
  }
  $: steamSearchPrimary = buildSteamAccountRows(debouncedOverlayQuery, rowEpoch);
  $: steamSearchGames = buildSteamGameRows(overlayQuery);

  $: if (name) {
    void refreshSteamGameShortcutIcons();
  }

  $: appBarTitle.set(name || "TcNo Account Switcher");
  $: if (name) {
    route.set({ page: "platform", platformName: name });
  }

  $: selSteamAcc = accountBySteamId(selectedSteamId);
  $: selectedAccountStore.set({
    platformKey: name,
    uniqueId: selectedSteamId,
    displayName: (() => {
      const p = (selSteamAcc?.personaName ?? "").trim();
      if (p) {
        return p;
      }
      return (selSteamAcc?.displayName ?? "").trim();
    })(),
    accountLogin: (selSteamAcc?.accountName ?? "").trim(),
  });

  function accountBySteamId(id: string): SteamAccountRow | undefined {
    return steamAccounts.find((a) => a.steamId64 === id);
  }

  $: tagFilterBarLabel =
    tagFilterMode.kind === "all"
      ? $t("Tags_All")
      : tagFilterMode.kind === "untagged"
        ? $t("Tags_Filter_Untagged")
        : tagFilterMode.name;

  $: displaySteamIds = (() => {
    const ids = steamIds;
    if (tagFilterMode.kind === "all") {
      return ids;
    }
    if (tagFilterMode.kind === "untagged") {
      return ids.filter((id) => (steamAccountMap.get(id)?.tags?.length ?? 0) === 0);
    }
    const tid = tagFilterMode.id;
    return ids.filter((id) => steamAccountMap.get(id)?.tags?.some((x) => x.id === tid));
  })();

  $: steamReorderDisabled = tagFilterMode.kind !== "all";

  $: {
    if (
      selectedSteamId &&
      displaySteamIds.length > 0 &&
      !displaySteamIds.includes(selectedSteamId)
    ) {
      selectedSteamId = "";
      touchSteamActionBar();
    }
  }

  function personaForStatus(acc: SteamAccountRow): string {
    const p = (acc.personaName ?? "").trim();
    if (p) return p;
    const a = (acc.accountName ?? "").trim();
    if (a) return a;
    return acc.steamId64;
  }

  function touchSteamActionBar(): void {
    const acc = accountBySteamId(selectedSteamId);
    actionBarStatus.set(
      acc ? $t("Status_SelectedAccount", { name: personaForStatus(acc) }) : "",
    );
  }

  function onSteamItemClick(e: CustomEvent<{ id: string }>): void {
    selectedSteamId = e.detail.id;
    touchSteamActionBar();
  }

  function clearSteamSelection(): void {
    if (!selectedSteamId) {
      return;
    }
    selectedSteamId = "";
    touchSteamActionBar();
  }

  function onSteamAccountsAreaClick(e: MouseEvent): void {
    const target = e.target as HTMLElement | null;
    if (!target) {
      return;
    }
    if (target.closest("[data-dnd-cell]")) {
      return;
    }
    clearSteamSelection();
  }

  function onWindowKeyDown(e: KeyboardEvent): void {
    if (e.key !== "Escape") {
      return;
    }
    if (get(activeModal)) {
      return;
    }
    if (steamImagePick.open) {
      e.preventDefault();
      closeSteamImagePick();
      return;
    }
    clearSteamSelection();
  }

  function closeSteamImagePick(): void {
    steamImagePick = { open: false, steamId: "", displayName: "", manual: false };
    steamFileDragHoverRowId = "";
  }

  function openSteamImagePick(steamId64: string): void {
    const acc = accountBySteamId(steamId64);
    const dn = (acc?.personaName ?? acc?.displayName ?? steamId64).trim();
    steamImagePick = {
      open: true,
      steamId: steamId64,
      displayName: dn,
      manual: !!(acc as SteamAccountRow | undefined)?.manualProfileImage,
    };
  }

  async function applySteamImageFromOverlay(path: string): Promise<void> {
    const sid = steamImagePick.steamId.trim();
    if (!sid) {
      return;
    }
    await SteamService.ChangeAccountImage(sid, path);
    await loadSteamAccounts();
    bumpSteamRowEpoch(sid);
    pushToast({
      type: "success",
      message: $t("Toast_AccountSaved"),
      duration: 4000,
    });
  }

  async function removeSteamImageFromOverlay(): Promise<void> {
    const sid = steamImagePick.steamId.trim();
    if (!sid) {
      return;
    }
    await SteamService.ClearManualAccountProfileImage(sid);
    await loadSteamAccounts();
    bumpSteamRowEpoch(sid);
    scheduleSteamAccountsRefresh();
    pushToast({
      type: "success",
      message: $t("Toast_AccountSaved"),
      duration: 4000,
    });
  }

  function steamHasFilesDragType(ev: DragEvent): boolean {
    const types = ev.dataTransfer?.types;
    return !!(types && Array.from(types as unknown as Iterable<string>).includes("Files"));
  }

  function onSteamAccDragOver(ev: DragEvent, rid: string): void {
    if (!steamHasFilesDragType(ev)) {
      return;
    }
    ev.preventDefault();
    steamFileDragHoverRowId = rid;
  }

  function onSteamAccDragLeave(ev: DragEvent, rid: string): void {
    if (!steamHasFilesDragType(ev)) {
      return;
    }
    const rel = ev.relatedTarget as Node | null;
    const cur = ev.currentTarget as HTMLElement | null;
    if (!rel || !cur?.contains(rel)) {
      if (steamFileDragHoverRowId === rid) {
        steamFileDragHoverRowId = "";
      }
    }
  }

  function onSteamAccListDragLeave(ev: DragEvent): void {
    if (!steamHasFilesDragType(ev)) {
      return;
    }
    const rel = ev.relatedTarget as Node | null;
    const root = steamAcclistEl;
    if (!root || rel === null || !root.contains(rel)) {
      steamFileDragHoverRowId = "";
    }
  }

  async function steamPlatformFileDropIntercept(paths: string[]): Promise<boolean> {
    const img = firstProfileImagePath(paths);
    if (!img) {
      return false;
    }
    try {
      if (steamImagePick.open && steamImagePick.steamId.trim()) {
        const target = steamImagePick.steamId.trim();
        await SteamService.ChangeAccountImage(target, img);
        await loadSteamAccounts();
        bumpSteamRowEpoch(target);
        pushToast({
          type: "success",
          message: $t("Toast_AccountSaved"),
          duration: 4000,
        });
        closeSteamImagePick();
        return true;
      }
      const hover = steamFileDragHoverRowId.trim();
      if (hover) {
        await SteamService.ChangeAccountImage(hover, img);
        await loadSteamAccounts();
        bumpSteamRowEpoch(hover);
        pushToast({
          type: "success",
          message: $t("Toast_AccountSaved"),
          duration: 4000,
        });
        steamFileDragHoverRowId = "";
        return true;
      }
    } catch (e) {
      pushToast({
        type: "error",
        message: formatToastWithError($t("Toast_SaveFailed"), e),
        duration: 8000,
      });
      return true;
    }
    return false;
  }

  function applySteamPatch(p: SteamAccountPatch): void {
    const id = String(p.steamId64 ?? "");
    let hit = false;
    steamAccounts = steamAccounts.map((r) => {
      if (r.steamId64 !== id) return r;
      hit = true;
      const errMsg = typeof p.error === "string" ? p.error : r.syncError ?? "";
      const nextManual =
        typeof p.manualProfileImage === "boolean" ? p.manualProfileImage : (r.manualProfileImage ?? false);
      return {
        ...r,
        imageUrl: p.imageUrl || r.imageUrl,
        vac: p.vac,
        ltd: p.ltd,
        avatarPending: p.avatarPending,
        metaPending: p.metaPending,
        manualProfileImage: nextManual,
        syncError: errMsg,
        displayName:
          typeof p.displayName === "string" && p.displayName.trim() !== ""
            ? p.displayName.trim()
            : r.displayName ?? "",
        avatarFrameUrl:
          typeof p.avatarFrameUrl === "string" && p.avatarFrameUrl.trim() !== ""
            ? p.avatarFrameUrl.trim()
            : r.avatarFrameUrl ?? "",
        miniProfileHtml:
          typeof p.miniProfileHtml === "string" && p.miniProfileHtml.trim() !== ""
            ? p.miniProfileHtml.trim()
            : r.miniProfileHtml ?? "",
        showMiniProfile: typeof p.showMiniProfile === "boolean" ? p.showMiniProfile : r.showMiniProfile,
        showAvatarFrame:
          typeof p.showAvatarFrame === "boolean" ? p.showAvatarFrame : r.showAvatarFrame,
      } as SteamAccountRow;
    });
    if (hit) {
      rowEpoch = { ...rowEpoch, [id]: (rowEpoch[id] ?? 0) + 1 };
    }
    if (id === selectedSteamId) {
      touchSteamActionBar();
    }
  }

  function scheduleSteamAccountsRefresh(): void {
    for (const t of steamListRefreshTimers) clearTimeout(t);
    steamListRefreshTimers = [];
    void loadSteamAccounts();
    steamListRefreshTimers.push(
      setTimeout(() => void loadSteamAccounts(), 700),
      setTimeout(() => void loadSteamAccounts(), 2200),
    );
  }

  /** Same public URL after cache overwrite — force `{#key}` remount so `<img>` reloads. */
  function bumpSteamRowEpoch(steamId64: string): void {
    const id = steamId64.trim();
    if (!id) {
      return;
    }
    rowEpoch = { ...rowEpoch, [id]: (rowEpoch[id] ?? 0) + 1 };
  }

  async function refreshGameDataAppSets(): Promise<void> {
    const ids = steamIds;
    if (ids.length === 0) {
      gameDataBySteamId = {};
      return;
    }
    try {
      const parts: [string, { userdata: Set<string>; backup: Set<string> }][] = await Promise.all(
        ids.map(async (id) => {
          try {
            const s = await SteamService.GetSteamGameDataAppIDSets(id);
            return [
              id,
              {
                userdata: new Set(s.userdataAppIds.map((x: string) => String(x).trim())),
                backup: new Set(s.backupAppIds.map((x: string) => String(x).trim())),
              },
            ] as [string, { userdata: Set<string>; backup: Set<string> }];
          } catch {
            return [
              id,
              { userdata: new Set<string>(), backup: new Set<string>() },
            ] as [string, { userdata: Set<string>; backup: Set<string> }];
          }
        }),
      );
      gameDataBySteamId = Object.fromEntries(parts);
    } catch {
      gameDataBySteamId = {};
    }
  }

  async function loadSteamTagDefs(): Promise<void> {
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

  async function refreshSteamGameStatsMarkup(accountIds: string[]): Promise<void> {
    if (!name.trim() || accountIds.length === 0) {
      steamGameStatsByAccount = {};
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
      steamGameStatsByAccount = Object.fromEntries(pairs) as Record<
        string,
        Record<string, Record<string, GameStatMetricDTO>>
      >;
    } catch {
      steamGameStatsByAccount = {};
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

  async function loadSteamAccounts(): Promise<void> {
    steamLoadError = "";
    const prevById = new Map(steamAccounts.map((a) => [a.steamId64, a]));
    try {
      const rows = (await SteamService.GetSteamAccounts()) as SteamAccountRow[];
      rowEpoch = buildEpochMap(rows, prevById, (r) => r.steamId64, rowEpoch);
      steamAccounts = rows;
      steamIds = rows.map((r) => r.steamId64);
      const liveRow = rows.find((r) => r.currentSession);
      platformLiveSessionId.set({
        platformKey: name,
        uniqueId: (liveRow?.steamId64 ?? "").trim(),
      });
      const stillValid =
        selectedSteamId && rows.some((r) => r.steamId64 === selectedSteamId);
      selectedSteamId = stillValid
        ? selectedSteamId
        : "";
      touchSteamActionBar();
      SteamService.StartSteamProfileRefresh();
      await refreshGameDataAppSets();
      await loadSteamTagDefs();
      await refreshSteamGameStatsMarkup(rows.map((r) => r.steamId64));
    } catch (e) {
      steamLoadError = formatWailsError(e) || String(e);
      steamAccounts = [];
      steamIds = [];
      gameDataBySteamId = {};
      steamGameStatsByAccount = {};
      selectedSteamId = "";
      actionBarStatus.set("");
      platformLiveSessionId.set({ platformKey: "", uniqueId: "" });
    }
  }

  function onAccountReorder(e: CustomEvent<{ items: string[] }>): void {
    steamIds = e.detail.items;
    SteamService.SaveSteamAccountOrder(e.detail.items).catch(() => {});
  }

  function steamDisplaySortKey(uid: string): string {
    const a = accountBySteamId(uid);
    const p = (a?.personaName ?? "").trim();
    if (p) {
      return p.toLowerCase();
    }
    const d = (a?.displayName ?? "").trim();
    if (d) {
      return d.toLowerCase();
    }
    return uid.toLowerCase();
  }

  function steamAccountNameSortKey(uid: string): string {
    const a = accountBySteamId(uid);
    const n = (a?.accountName ?? "").trim().toLowerCase();
    return n || "\uffff";
  }

  function steamLastLoginMs(uid: string): number {
    const a = accountBySteamId(uid);
    const t = Date.parse((a?.lastLogin ?? "").trim());
    return Number.isNaN(t) ? 0 : t;
  }

  function applySteamSort(kind: PlatformSortKind): void {
    const ids = [...steamIds];
    const cmpPersona = (x: string, y: string) =>
      steamDisplaySortKey(x).localeCompare(steamDisplaySortKey(y));
    const cmpUser = (x: string, y: string) =>
      steamAccountNameSortKey(x).localeCompare(steamAccountNameSortKey(y));
    switch (kind) {
      case "alpha_asc":
        ids.sort(cmpPersona);
        break;
      case "alpha_desc":
        ids.sort((x, y) => -cmpPersona(x, y));
        break;
      case "steam_user_asc":
        ids.sort(cmpUser);
        break;
      case "steam_user_desc":
        ids.sort((x, y) => -cmpUser(x, y));
        break;
      case "lastused_new_old":
      case "date_new_old":
        ids.sort(
          (x, y) =>
            steamLastLoginMs(y) - steamLastLoginMs(x) || cmpPersona(x, y),
        );
        break;
      case "lastused_old_new":
      case "date_old_new":
        ids.sort(
          (x, y) =>
            steamLastLoginMs(x) - steamLastLoginMs(y) || cmpPersona(x, y),
        );
        break;
      default:
        return;
    }
    steamIds = ids;
    SteamService.SaveSteamAccountOrder(ids).catch(() => {});
  }

  function steamAccountSearchHay(acc: SteamAccountRow, trimmed: string): string {
    const parts: string[] = [
      acc.personaName ?? "",
      acc.displayName ?? "",
      acc.note ?? "",
    ];
    if (acc.showAccUsername) {
      parts.push(acc.accountName ?? "");
    }
    const wantsSteamIdMatch = trimmed
      .toLowerCase()
      .split(/\s+/)
      .filter((w) => w.length > 0)
      .some((w) => /^\d{5,}$/.test(w));
    if (wantsSteamIdMatch) {
      parts.push(acc.steamId64 ?? "");
    }
    return parts.join("\n");
  }

  function buildSteamAccountRows(
    q: string,
    epochs: Record<string, number>,
  ): SearchResultRow[] {
    const tr = get(t);
    const trimmed = q.trim();
    const hay = (acc: SteamAccountRow) => {
      const blob = steamAccountSearchHay(acc, trimmed);
      return fuzzyWordsMatch(trimmed, blob);
    };
    let list = trimmed
      ? steamAccounts.filter((a) => hay(a))
      : steamAccounts.slice(0, SEARCH_MAX);
    if (trimmed) {
      list = list.slice(0, SEARCH_MAX);
    }
    return list.map((a) => ({
      key: `a:${a.steamId64}`,
      title: (a.personaName || a.displayName || a.steamId64).trim(),
      badge: tr("Search_Section_Account"),
      accountIconUrl: offlineSafeImageSrc(
        get(offlineMode),
        withAssetCacheBust(
          a.imageUrl && !a.avatarPending ? a.imageUrl : undefined,
          epochs[a.steamId64] ?? 0,
        ),
        PROFILE_PLACEHOLDER,
      ),
    }));
  }

  /** Mirrors `internal/exeicon.SafeFolderName` for `/img/shortcuts/{folder}/…` paths. */
  function safeFolderName(platformKey: string): string {
    let b = "";
    for (const r of platformKey.trim().toLowerCase()) {
      if (r === " " || r === "/" || r === "\\") {
        b += "_";
      } else if (/[a-z0-9\-_]/.test(r)) {
        b += r;
      }
    }
    return b || "unknown";
  }

  function normalizeGameSearchKey(s: string): string {
    return s
      .toLowerCase()
      .replace(/[™©®]/g, "")
      .replace(/[^a-z0-9]+/g, " ")
      .trim()
      .replace(/\s+/g, " ");
  }

  function resolveShortcutIconUrl(o: Record<string, unknown>, stemLow: string, platFolder: string): string {
    let iconRaw = String(o.iconUrl ?? o.IconURL ?? o.iconURL ?? "").trim();
    if (!iconRaw) iconRaw = `/img/shortcuts/${platFolder}/${stemLow}.png`;
    return offlineSafeImageSrc(get(offlineMode), iconRaw, SHORTCUT_ICON_FALLBACK);
  }

  function extractShortcutRecord(raw: unknown): { stem: string; stemLow: string } | null {
    const o = (raw ?? {}) as Record<string, unknown>;
    const fn = String(o.fileName ?? o.FileName ?? "").trim();
    if (!fn) return null;
    const stem = fn.replace(/\.(lnk|url)$/i, "").trim();
    return { stem, stemLow: stem.toLowerCase() };
  }

  function applyShortcutIconsFromShortcutList(list: unknown[]): void {
    const byAppId: Record<string, string> = {};
    const byStemKey: Record<string, string> = {};
    const platFolder = safeFolderName(name);
    for (const raw of list) {
      const rec = extractShortcutRecord(raw);
      if (!rec) continue;
      const iconUrl = resolveShortcutIconUrl((raw ?? {}) as Record<string, unknown>, rec.stemLow, platFolder);
      if (/^\d+$/.test(rec.stem)) byAppId[rec.stem] = iconUrl;
      const nk = normalizeGameSearchKey(rec.stem);
      if (nk) byStemKey[nk] = iconUrl;
    }
    steamShortcutIconByAppId = byAppId;
    steamShortcutIconByStemKey = byStemKey;
  }


  function resolveSteamGameSearchIcon(g: { appId: string; name: string }): string {
    const id = String(g.appId).trim();
    const fromId = steamShortcutIconByAppId[id];
    if (fromId) {
      return fromId;
    }
    const nameKey = normalizeGameSearchKey(g.name);
    const fromName = steamShortcutIconByStemKey[nameKey];
    if (fromName) {
      return fromName;
    }
    const guessed = `/img/shortcuts/${safeFolderName(name)}/${id.toLowerCase()}.png`;
    return offlineSafeImageSrc(get(offlineMode), guessed, SHORTCUT_ICON_FALLBACK);
  }

  async function refreshSteamGameShortcutIcons(): Promise<void> {
    const plat = typeof name === "string" ? name.trim() : "";
    if (!plat) {
      steamShortcutIconByAppId = {};
      steamShortcutIconByStemKey = {};
      return;
    }
    try {
      const list = await Shortcuts.ListShortcuts(plat);
      applyShortcutIconsFromShortcutList(list as unknown[]);
    } catch {
      steamShortcutIconByAppId = {};
      steamShortcutIconByStemKey = {};
    }
  }

  function buildSteamGameRows(q: string): SearchResultRow[] {
    const tr = get(t);
    const trimmed = q.trim();
    if (!trimmed) {
      return [];
    }
    return installedGames
      .filter((g) => fuzzyWordsMatch(trimmed, g.name))
      .slice(0, SEARCH_MAX)
      .map((g) => ({
        key: `g:${String(g.appId).trim()}`,
        title: g.name,
        badge: tr("Search_Section_Game"),
        isCategory: true,
        accountIconUrl: resolveSteamGameSearchIcon(g),
      }));
  }

  async function onSteamSearchPick(ev: CustomEvent<SearchResultRow>): Promise<void> {
    const row = ev.detail;
    closeSearchOverlay();
    if (row.key.startsWith("a:")) {
      const id = row.key.slice(2);
      selectedSteamId = id;
      touchSteamActionBar();
      await steamLoginSelected();
      return;
    }
    if (row.key.startsWith("g:")) {
      const appId = row.key.slice(2);
      if (!selectedSteamId) {
        pushToast({
          type: "error",
          message: $t("Toast_NoValidSteamId"),
          duration: 5000,
        });
        return;
      }
      try {
        await SteamService.LoginAndLaunchGame(selectedSteamId, -1, appId);
        scheduleSteamAccountsRefresh();
        pushToast({
          type: "success",
          message: $t("Toast_GameLaunchRequested"),
          duration: 4000,
        });
      } catch (e) {
        await reportLaunchFailure(e, name);
      }
    }
  }

  async function reportSteamSwitchFailure(e: unknown): Promise<void> {
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

  async function steamLoginSelected(): Promise<void> {
    if (!selectedSteamId) {
      return;
    }
    try {
      await SteamService.SwapToSteamAccount(selectedSteamId, -1, []);
      scheduleSteamAccountsRefresh();
      pushToast({
        type: "success",
        message: $t("Toast_AccountSwitched"),
        duration: 4000,
      });
    } catch (e) {
      await reportSteamSwitchFailure(e);
    }
  }

  async function launchSteamForSelection(): Promise<void> {
    const selected = accountBySteamId(selectedSteamId);
    if (selectedSteamId && selected && !selected.currentSession) {
      try {
        await SteamService.SwapToSteamAccount(selectedSteamId, -1, []);
      } catch (e) {
        await reportSteamSwitchFailure(e);
        return;
      }
    }
    try {
      await SteamService.LaunchSteam();
      scheduleSteamAccountsRefresh();
    } catch (e) {
      await reportLaunchFailure(e, name);
    }
  }

  async function handlePlatformActionKind(
    kind: "login" | "addNew" | "launch" | "saveCurrent",
  ): Promise<void> {
    if (kind === "saveCurrent") {
      return;
    }
    if (kind === "launch") {
      await launchSteamForSelection();
      return;
    }
    if (kind === "addNew") {
      try {
        await SteamService.SteamAddNew();
        scheduleSteamAccountsRefresh();
        pushToast({
          type: "success",
          message: $t("Toast_AccountSwitched"),
          duration: 4000,
        });
      } catch (e) {
        await reportSteamSwitchFailure(e);
      }
      return;
    }
    if (kind === "login") {
      await steamLoginSelected();
    }
  }

  function slotKey(x: string | null | undefined): string {
    return x ?? "";
  }

  function onSteamTagFilterBarClick(ev: Event): void {
    openTagFilterMenu({
      ev: ev as MouseEvent,
      tagDefs,
      tr: get(t),
      onPick: (mode) => {
        tagFilterMode = mode;
      },
    });
  }

  async function clipboardWrite(text: string): Promise<void> {
    try {
      await navigator.clipboard.writeText(text);
      pushToast({ type: "success", message: $t("Toast_Copied"), duration: 2500 });
    } catch {
      pushToast({
        type: "error",
        message: $t("Toast_CopyFailed"),
        duration: 4000,
      });
    }
  }

  function steamCtxMenu(rid: string): () => MenuItemDef[] {
    return () => {
      const tr = get(t);
      const acc = steamAccounts.find((a) => a.steamId64 === rid);
      if (!acc) {
        return [];
      }

      const loginStates: { st: number; lab: string }[] = [
        { st: 7, lab: tr("Invisible") },
        { st: 0, lab: tr("Offline") },
        { st: 1, lab: tr("Online") },
        { st: 2, lab: tr("Busy") },
        { st: 3, lab: tr("Away") },
        { st: 4, lab: tr("Snooze") },
        { st: 5, lab: tr("LookingToTrade") },
        { st: 6, lab: tr("LookingToPlay") },
      ];

      const loginAsChildren: MenuItemDef[] = [
        { type: "search", label: tr("Context_Search") },
        ...loginStates.map((x) => ({
          label: x.lab,
          action: async () => {
            try {
              await SteamService.SwapToSteamAccount(rid, x.st, []);
              scheduleSteamAccountsRefresh();
              pushToast({
                type: "success",
                message: $t("Toast_AccountSwitched"),
                duration: 4000,
              });
            } catch (e) {
              await reportSteamSwitchFailure(e);
            }
          },
        })),
      ];

      const copyChildren: MenuItemDef[] = [
        {
          label: tr("Context_CommunityUrl"),
          action: () =>
            void clipboardWrite(`https://steamcommunity.com/profiles/${rid}`),
        },
        {
          label: tr("Context_CommunityUsername"),
          action: () =>
            void clipboardWrite(
              (acc.displayName ?? "").trim() || (acc.personaName ?? "").trim() || rid,
            ),
        },
        {
          label: tr("Context_LoginUsername"),
          action: () =>
            void clipboardWrite((acc.accountName ?? "").trim() || rid),
        },
        {
          label: tr("Context_CopySteamIdSubmenu"),
          children: [
            {
              label: tr("Context_Steam_Id64"),
              action: async () => {
                try {
                  const f = await SteamService.GetSteamIDFormats(rid);
                  void clipboardWrite(f["ID64"] ?? rid);
                } catch {
                  void clipboardWrite(rid);
                }
              },
            },
            {
              label: tr("Context_Steam_Id3"),
              action: async () => {
                try {
                  const f = await SteamService.GetSteamIDFormats(rid);
                  void clipboardWrite(f["ID3"] ?? "");
                } catch {
                  /* ignore */
                }
              },
            },
            {
              label: tr("Context_Steam_Id32"),
              action: async () => {
                try {
                  const f = await SteamService.GetSteamIDFormats(rid);
                  void clipboardWrite(f["ID32"] ?? "");
                } catch {
                  /* ignore */
                }
              },
            },
          ],
        },
      ];

      const shortcutChildren: MenuItemDef[] = [
        { type: "search", label: tr("Context_Search") },
        {
          label: tr("Context_CreateShortcut"),
          action: async () => {
            try {
              const p = await Shortcuts.CreateAccountShortcut(
                "Steam",
                rid,
                acc.personaName ?? rid,
                "",
                "",
                (acc.accountName ?? "").trim(),
              );
              pushToast({
                type: "success",
                message: `${tr("Toast_ShortcutCreated")}\n${p}`,
                duration: 6000,
              });
            } catch (e) {
              pushToast({
                type: "error",
                message: formatToastWithError($t("Toast_SwitchFailed"), e),
                duration: 8000,
              });
            }
          },
        },
        ...loginStates.map((x) => ({
          label: `${tr("Context_CreateShortcut")} (${x.lab})`,
          action: async () => {
            try {
              const p = await Shortcuts.CreateAccountShortcut(
                "Steam",
                rid,
                acc.personaName ?? rid,
                String(x.st),
                x.lab,
                (acc.accountName ?? "").trim(),
              );
              pushToast({
                type: "success",
                message: `${tr("Toast_ShortcutCreated")}\n${p}`,
                duration: 6000,
              });
            } catch (e) {
              pushToast({
                type: "error",
                message: formatToastWithError($t("Toast_SwitchFailed"), e),
                duration: 8000,
              });
            }
          },
        })),
      ];

      const gsets = gameDataBySteamId[rid];
      const gameDataSubmenuItems: MenuItemDef[] = [];
      for (const g of installedGames) {
        const aid = String(g.appId).trim();
        const hasUser = gsets?.userdata.has(aid) ?? false;
        const hasBackup = gsets?.backup.has(aid) ?? false;
        if (!hasUser && !hasBackup) {
          continue;
        }
        const children: MenuItemDef[] = [
          {
            label: "Open folder",
            action: async () => {
              try {
                await SteamService.OpenSteamGameDataFolder(rid, g.appId);
              } catch (e) {
                pushToast({
                  type: "error",
                  message: mapSteamUserdataI18nError(e, tr),
                  duration: 8000,
                });
              }
            },
          },
        ];
        if (hasUser) {
          children.push({
            label: tr("Context_Game_CopySettingsFrom"),
            action: async () => {
              try {
                await SteamService.CopySteamGameSettingsFrom(rid, g.appId);
                pushToast({
                  type: "success",
                  message: tr("Toast_SettingsCopied"),
                  duration: 5000,
                });
                void refreshGameDataAppSets();
              } catch (e) {
                pushToast({
                  type: "error",
                  message: mapSteamUserdataI18nError(e, tr),
                  duration: 8000,
                });
              }
            },
          });
        }
        if (hasBackup) {
          children.push({
            label: tr("Context_Game_RestoreSettingsTo"),
            action: async () => {
              try {
                await SteamService.RestoreSteamGameSettingsTo(rid, g.appId);
                pushToast({
                  type: "success",
                  message: tr("Toast_GameDataRestored"),
                  duration: 5000,
                });
                void refreshGameDataAppSets();
              } catch (e) {
                pushToast({
                  type: "error",
                  message: mapSteamUserdataI18nError(e, tr),
                  duration: 8000,
                });
              }
            },
          });
        }
        if (hasUser) {
          children.push({
            label: tr("Context_Game_BackupData"),
            action: async () => {
              try {
                const folder = await SteamService.BackupSteamGameData(rid, g.appId);
                pushToast({
                  type: "success",
                  message: tr("Toast_GameBackupDone", { folderLocation: folder }),
                  duration: 8000,
                });
                void refreshGameDataAppSets();
              } catch (e) {
                pushToast({
                  type: "error",
                  message: mapSteamUserdataI18nError(e, tr),
                  duration: 8000,
                });
              }
            },
          });
        }
        gameDataSubmenuItems.push({ label: g.name, children });
      }

      const gameChildren: MenuItemDef[] =
        gameDataSubmenuItems.length === 0
          ? [
              {
                type: "item",
                label: tr("Context_GameData_NoFolders"),
              },
            ]
          : [{ type: "search", label: tr("Context_Search") }, ...gameDataSubmenuItems];

      const launchChildren: MenuItemDef[] = [
        { type: "search", label: tr("Context_Search") },
        ...installedGames.map((g) => ({
          label: g.name,
          action: async () => {
            try {
              await SteamService.LoginAndLaunchGame(rid, -1, g.appId);
              scheduleSteamAccountsRefresh();
              pushToast({
                type: "success",
                message: $t("Toast_GameLaunchRequested"),
                duration: 4000,
              });
            } catch (e) {
              await reportLaunchFailure(e, name);
            }
          },
        })),
      ];

      return [
        {
          label: tr("Context_SwapTo"),
          action: async () => {
            selectedSteamId = rid;
            touchSteamActionBar();
            await steamLoginSelected();
          },
        },
        {
          label: tr("Context_Game_LoginAndLaunch"),
          children: launchChildren,
        },
        {
          label: tr("Context_LoginAsSubmenu"),
          children: loginAsChildren,
        },
        {
          label: tr("Context_CopySubmenu"),
          children: copyChildren,
        },
        {
          label: tr("Context_CreateShortcut"),
          children: shortcutChildren,
        },
        {
          label: tr("Forget"),
          action: async () => {
            const ok = await openConfirm({
              title: tr("Forget"),
              body: tr("Prompt_ForgetSteam"),
              style: "yesno",
            });
            if (!ok) {
              return;
            }
            try {
              await SteamService.ForgetSteamAccount(rid);
              scheduleSteamAccountsRefresh();
              pushToast({
                type: "success",
                message: $t("Toast_AccountSaved"),
                duration: 4000,
              });
            } catch (e) {
              pushToast({
                type: "error",
                message: formatToastWithError($t("Toast_SaveFailed"), e),
                duration: 8000,
              });
            }
          },
        },
        {
          label: tr("Notes"),
          action: async () => {
            const cur = await BasicService.GetAccountNote("Steam", rid);
            const note = await openPrompt({
              title: tr("Notes"),
              body: tr("Modal_Title_AccountNotes", {
                accountName: acc.personaName ?? rid,
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
              await BasicService.SetAccountNote("Steam", rid, String(note));
              await loadSteamAccounts();
              pushToast({
                type: "success",
                message: $t("Toast_AccountSaved"),
                duration: 4000,
              });
            } catch (e) {
              pushToast({
                type: "error",
                message: formatToastWithError($t("Toast_SaveFailed"), e),
                duration: 8000,
              });
            }
          },
        },
        buildTagsSectionMenuItem({
          platformKey: name,
          uniqueId: rid,
          assignedTags: (acc.tags ?? []) as TagDefRow[],
          tagDefs,
          tr,
          afterChange: async () => {
            await loadSteamAccounts();
            await loadSteamTagDefs();
          },
          onSuccess: () =>
            pushToast({
              type: "success",
              message: $t("Toast_AccountSaved"),
              duration: 2500,
            }),
          onError: (e) =>
            pushToast({
              type: "error",
              message: formatToastWithError($t("Toast_SaveFailed"), e),
              duration: 8000,
            }),
        }),
        {
          label: tr("Context_ManageSubmenu"),
          children: [
            {
              label: tr("Context_GameDataSubmenu"),
              children: gameChildren,
            },
            ...(hasGameStatsSupport
              ? ([
                  {
                    label: tr("Context_ManageGameStats"),
                    action: () => {
                      void openAlertNoButton({
                        title: get(t)("Context_ManageGameStats"),
                        bodyComponent: GameStatsSetupModalBody,
                        bodyProps: {
                          platformKey: name,
                          uniqueId: rid,
                          displayName: (
                            (acc?.personaName ?? "").trim() ||
                            (acc?.displayName ?? "").trim() ||
                            rid
                          ).trim(),
                          onApplied: () => void loadSteamAccounts(),
                        },
                      });
                    },
                  },
                ] as MenuItemDef[])
              : []),
            {
              label: tr("Context_Steam_OpenUserdata"),
              action: async () => {
                try {
                  await SteamService.OpenUserdataFolder(rid);
                } catch (e) {
                  pushToast({
                    type: "error",
                    message: formatToastWithError($t("Toast_LaunchFailed"), e),
                    duration: 8000,
                  });
                }
              },
            },
            (() => {
              const imgUrl = (acc?.imageUrl ?? "").trim();
              const manual = !!acc?.manualProfileImage;
              const openPick = () => {
                selectedSteamId = rid;
                touchSteamActionBar();
                openSteamImagePick(rid);
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
                        await SteamService.ClearManualAccountProfileImage(rid);
                        await loadSteamAccounts();
                        pushToast({
                          type: "success",
                          message: $t("Toast_AccountSaved"),
                          duration: 4000,
                        });
                      } catch (e) {
                        pushToast({
                          type: "error",
                          message: formatToastWithError($t("Toast_SaveFailed"), e),
                          duration: 8000,
                        });
                      }
                    },
                  },
                ],
              };
            })(),
          ],
        },
      ];
    };
  }

  onMount(() => {
    previousPage.set({ page: "home" });
    void preflightAdminForPlatform(name);
    void refreshGameStatsSupport();
    void (async () => {
      await loadSteamAccounts();
      try {
        const rows = await SteamService.GetInstalledGames();
        installedGames = rows
          .filter((r) => !STEAM_CONTEXT_MENU_HIDDEN_APP_IDS.has(String(r.appId).trim()))
          .map((r) => ({
            appId: r.appId,
            name: r.name,
          }));
      } catch {
        installedGames = [];
      }
    })();
    void GetPlatformExeIcon(name).then((u: string) => platformExeIconUrl.set(u ?? ""));
    offSort = platformListSort.subscribe((sig) => {
      if (!sig || sig.id <= lastHandledSortId) {
        return;
      }
      lastHandledSortId = sig.id;
      applySteamSort(sig.kind);
    });
    offSteamEvent = Events.On("steam-account-updated", (ev) => {
      const raw = ev.data;
      const p =
        raw instanceof AccountPatch
          ? raw
          : AccountPatch.createFrom(raw as Record<string, unknown>);
      applySteamPatch(p as SteamAccountPatch);
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
      scheduleSteamAccountsRefresh();
    });
    offShortcutsUpdated = Events.On("shortcuts-updated", (ev) => {
      const raw = ev.data;
      const p =
        raw instanceof ListPayload
          ? raw
          : ListPayload.createFrom(raw as Record<string, unknown>);
      if (p.platformKey !== name) {
        return;
      }
      applyShortcutIconsFromShortcutList(p.shortcuts ?? []);
    });
    fileDropInterceptor.set(steamPlatformFileDropIntercept);
  });

  onDestroy(() => {
    for (const t of steamListRefreshTimers) clearTimeout(t);
    steamListRefreshTimers = [];
    if (overlayQueryDebounceTimer) clearTimeout(overlayQueryDebounceTimer);
    selectedAccountStore.set({ platformKey: "", uniqueId: "", displayName: "", accountLogin: "" });
    platformLiveSessionId.set({ platformKey: "", uniqueId: "" });
    platformAction.set(null);
    offSteamEvent?.();
    offShortcutsUpdated?.();
    offSort?.();
    offPlatformAction?.();
    offAccountsRefresh?.();
    accountProfileImageDropActive.set(false);
    fileDropInterceptor.set(null);
    platformAccountsRefresh.set({ seq: 0, platformKey: "" });
    platformExeIconUrl.set("");
    actionBarStatus.set("");
  });
</script>

<div class="main-content platform-accounts-root" bind:this={steamMainEl}>
  {#if name}
    <div class="platformTableHost">
      <SearchOverlay
        open={so.open}
        syncNonce={so.nonce}
        initialQuery={so.initialQuery}
        bind:query={overlayQuery}
        primaryRows={steamSearchPrimary}
        categoryRows={[]}
        categoryHint=""
        gameRows={steamSearchGames}
        gameHint={$t("Search_Hint_Games")}
        on:close={() => closeSearchOverlay()}
        on:pick={(e) => void onSteamSearchPick(e)}
      />
      <div class="platformTable">
      {#if steamLoadError}
        <p class="platform-accounts-hint">{steamLoadError}</p>
      {/if}
      <!-- svelte-ignore a11y-click-events-have-key-events -->
      <!-- svelte-ignore a11y-no-static-element-interactions -->
      <div
        class="steam-acclist"
        bind:this={steamAcclistEl}
        on:click={onSteamAccountsAreaClick}
        on:dragleave={onSteamAccListDragLeave}
      >
        {#if tagDefs.length > 0}
          <TagFilterBar label={tagFilterBarLabel} onClick={onSteamTagFilterBarClick} />
        {/if}
        <ReorderPointerGrid
          items={displaySteamIds}
          reorderDisabled={steamReorderDisabled}
          listClass="acc_list"
          itemClass="acc_list_item acc_list_item--drag"
          placeholderClass="acc_list_item placeHolderAcc"
          ghostClass="acc_list_item acc_list_item--ghost"
          ariaLabel="Steam accounts"
          on:reorder={onAccountReorder}
          on:itemclick={onSteamItemClick}
        >
          <svelte:fragment slot="item" let:rowId>
            {@const rid = slotKey(rowId)}
            {@const acc = steamAccountMap.get(rid)}
            {@const radioId = `steam-acc-${rid}`}
            {#key `${rid}-${rowEpoch[rid] ?? 0}`}
            <div class="acc_list_item_inner">
              <input
                type="radio"
                class="acc"
                id={radioId}
                name="steam-accounts"
                value={rid}
                bind:group={selectedSteamId}
                on:change={touchSteamActionBar}
              />
              <label
                for={radioId}
                class="acc"
                class:currentAcc={acc?.currentSession}
                class:acc--profile-drop-target={$accountProfileImageDropActive && !steamImagePick.open}
                class:acc--drop-target={steamFileDragHoverRowId === rid}
                on:dragover={(e) => onSteamAccDragOver(e, rid)}
                on:dragleave={(e) => onSteamAccDragLeave(e, rid)}
                use:ctxMenuAction={{
                  items: steamCtxMenu(rid),
                  beforeOpen: () => {
                    selectedSteamId = rid;
                    touchSteamActionBar();
                  },
                }}
                use:tooltip={acc?.currentSession
                  ? {
                      text: $t("Tooltip_CurrentAccount"),
                      placement: "right",
                      boundary: steamAcclistEl,
                    }
                  : undefined}
                on:dblclick|preventDefault={() => {
                  selectedSteamId = rid;
                  touchSteamActionBar();
                  void steamLoginSelected();
                }}
              >
                {#if $accountProfileImageDropActive && !steamImagePick.open}
                  <div
                    class="acc_profile_drop_overlay"
                    class:acc_profile_drop_overlay--hover={steamFileDragHoverRowId === rid}
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
                <span class="steam-acc-avatar-wrap">
                  <img
                    class="steam-acc-avatar"
                    class:status_vac={acc?.showVac && acc?.vac}
                    class:status_limited={acc?.showLimited && acc?.ltd}
                    src={offlineSafeImageSrc(
                      $offlineMode,
                      withAssetCacheBust(
                        acc?.imageUrl && !acc?.avatarPending ? acc.imageUrl : undefined,
                        rowEpoch[rid] ?? 0,
                      ),
                      PROFILE_PLACEHOLDER,
                    )}
                    alt=""
                    draggable="false"
                    use:miniProfileHover={{
                      html: acc?.miniProfileHtml ?? "",
                      boundary: steamMainEl,
                      offline: $offlineMode,
                      enabled: !!(
                        acc?.showMiniProfile &&
                        (acc?.miniProfileHtml ?? "").trim() !== ""
                      ),
                    }}
                  />
                  {#if acc?.showAvatarFrame && (acc?.avatarFrameUrl ?? "").trim() !== "" && !$offlineMode}
                    <img
                      class="steam-acc-avatar-frame"
                      src={offlineSafeImageSrc($offlineMode, acc.avatarFrameUrl, PROFILE_PLACEHOLDER)}
                      alt=""
                      draggable="false"
                    />
                  {/if}
                </span>
                {#if acc?.showAccUsername && acc?.accountName}
                  <p class="streamerCensor">{acc.accountName}</p>
                {/if}
                <h6 class="displayName">{acc?.personaName ?? rid}</h6>
                <AccountTagBubbles tags={acc?.tags ?? []} />
                {#if acc?.showShortNotes && acc?.note?.trim()}
                  <p class="acc_note">{acc.note}</p>
                {/if}
                {#if steamGameStatsByAccount[rid] && Object.keys(steamGameStatsByAccount[rid]).length > 0}
                  <div class="acc_inline_gamestats" aria-label={$t("Context_ManageGameStats")}>
                    {#each Object.entries(steamGameStatsByAccount[rid]) as [gameTitle, metrics]}
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
                {#if acc?.showSteamId}
                  <p class="streamerCensor steamId">{acc.steamId64}</p>
                {/if}
                {#if acc?.showLastLogin && acc?.lastLogin}
                  <p class="lastLogin">{formatLastLoginForLocale(acc.lastLogin, $locale)}</p>
                {/if}
                {#if acc?.syncError}
                  <div class="steam_meta_err" title={acc.syncError}>{acc.syncError}</div>
                {:else if acc?.avatarPending}
                  <div class="steam_meta_pending">{$t("Status_Updating")}</div>
                {/if}
              </label>
            </div>
            {/key}
          </svelte:fragment>
        </ReorderPointerGrid>
      </div>
    </div>
    </div>
    <AccountImagePickOverlay
      bind:open={steamImagePick.open}
      accountDisplayName={steamImagePick.displayName}
      showRemoveButton={steamImagePick.manual}
      onClose={closeSteamImagePick}
      onApplyPath={applySteamImageFromOverlay}
      onRemoveManual={removeSteamImageFromOverlay}
    />
  {/if}
</div>
<svelte:window on:keydown={onWindowKeyDown} />
<ActionBar />

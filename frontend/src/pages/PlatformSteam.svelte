<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import { Events } from "@wailsio/runtime";
  import ActionBar from "../components/ActionBar.svelte";
  import ReorderPointerGrid from "../components/ReorderPointerGrid.svelte";
  import TagFilterBar from "../components/TagFilterBar.svelte";
  import AccountTagBubbles from "../components/AccountTagBubbles.svelte";
  import { route, previousPage, appBarTitle } from "../stores/nav";
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
  import { contextMenu as ctxMenuAction } from "../lib/actions/contextMenu";
  import type { MenuItemDef } from "../stores/contextMenu";
  import * as BasicService from "../../bindings/TcNo-Acc-Switcher/internal/basic/basicservice.js";
  import * as Shortcuts from "wails-shortcuts-service";
  import { ListPayload } from "../../bindings/TcNo-Acc-Switcher/internal/shortcuts/models.js";
  import { get } from "svelte/store";
  import { openConfirm, openPrompt } from "../stores/modal";
  import { offlineMode, offlineSafeImageSrc } from "../stores/offlineMode";
  import { fuzzyWordsMatch } from "../lib/searchFuzzy";
  import { closeSearchOverlay, searchOverlayCtrl } from "../stores/searchOverlay";
  import { platformListSort, type PlatformSortKind } from "../stores/platformListSort";
  import {
    buildTagsSectionMenuItem,
    openTagFilterMenu,
    type TagDefRow,
    type TagFilterMode,
  } from "../lib/accountTagsContext";

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
  /** Scroll/content box for Steam tooltips (keeps popovers off the window chrome). */
  let steamAcclistEl: HTMLDivElement | null = null;
  /** `.main-content` root for mini profile clamping. */
  let steamMainEl: HTMLDivElement | null = null;
  let steamIds: string[] = [];
  let steamLoadError = "";
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
  let offSort: (() => void) | undefined;
  let lastHandledSortId = 0;

  let tagDefs: TagDefRow[] = [];
  let tagFilterMode: TagFilterMode = { kind: "all" };

  const SEARCH_MAX = 5;

  $: so = $searchOverlayCtrl;
  $: steamSearchPrimary = buildSteamAccountRows(overlayQuery);
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
      return ids.filter((id) => (accountBySteamId(id)?.tags?.length ?? 0) === 0);
    }
    const tid = tagFilterMode.id;
    return ids.filter((id) => accountBySteamId(id)?.tags?.some((x) => x.id === tid));
  })();

  $: steamReorderDisabled = tagFilterMode.kind !== "all";

  $: {
    if (
      selectedSteamId &&
      displaySteamIds.length > 0 &&
      !displaySteamIds.includes(selectedSteamId)
    ) {
      selectedSteamId = displaySteamIds[0] ?? "";
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

  function applySteamPatch(p: SteamAccountPatch): void {
    const id = String(p.steamId64 ?? "");
    let hit = false;
    steamAccounts = steamAccounts.map((r) => {
      if (r.steamId64 !== id) return r;
      hit = true;
      const errMsg = typeof p.error === "string" ? p.error : r.syncError ?? "";
      return {
        ...r,
        imageUrl: p.imageUrl || r.imageUrl,
        vac: p.vac,
        ltd: p.ltd,
        avatarPending: p.avatarPending,
        metaPending: p.metaPending,
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

  async function loadSteamAccounts(): Promise<void> {
    steamLoadError = "";
    try {
      const rows = (await SteamService.GetSteamAccounts()) as SteamAccountRow[];
      rowEpoch = {};
      steamAccounts = rows;
      steamIds = rows.map((r) => r.steamId64);
      const liveRow = rows.find((r) => r.currentSession);
      platformLiveSessionId.set({
        platformKey: name,
        uniqueId: (liveRow?.steamId64 ?? "").trim(),
      });
      const active = liveRow;
      const firstId = rows[0]?.steamId64 ?? "";
      const stillValid =
        selectedSteamId && rows.some((r) => r.steamId64 === selectedSteamId);
      selectedSteamId = stillValid
        ? selectedSteamId
        : active
          ? active.steamId64
          : firstId;
      touchSteamActionBar();
      SteamService.StartSteamProfileRefresh();
      await refreshGameDataAppSets();
      await loadSteamTagDefs();
    } catch (e) {
      steamLoadError = formatWailsError(e) || String(e);
      steamAccounts = [];
      steamIds = [];
      gameDataBySteamId = {};
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

  function buildSteamAccountRows(q: string): SearchResultRow[] {
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
        a.imageUrl && !a.avatarPending ? a.imageUrl : undefined,
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

  function applyShortcutIconsFromShortcutList(list: unknown[]): void {
    const byAppId: Record<string, string> = {};
    const byStemKey: Record<string, string> = {};
    const platFolder = safeFolderName(name);
    for (const raw of list) {
      const o = (raw ?? {}) as Record<string, unknown>;
      const fn = String(o.fileName ?? o.FileName ?? "").trim();
      if (!fn) {
        continue;
      }
      const stem = fn.replace(/\.(lnk|url)$/i, "").trim();
      const stemLow = stem.toLowerCase();
      let iconRaw = String(o.iconUrl ?? o.IconURL ?? o.iconURL ?? "").trim();
      if (!iconRaw) {
        iconRaw = `/img/shortcuts/${platFolder}/${stemLow}.png`;
      }
      const iconUrl = offlineSafeImageSrc(get(offlineMode), iconRaw, SHORTCUT_ICON_FALLBACK);
      if (/^\d+$/.test(stem)) {
        byAppId[stem] = iconUrl;
      }
      const nk = normalizeGameSearchKey(stem);
      if (nk) {
        byStemKey[nk] = iconUrl;
      }
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
            {
              label: tr("Context_ChangeImage"),
              action: async () => {
                const path = await openPrompt({
                  title: tr("Context_ChangeImage"),
                  body: "",
                  positiveLabel: tr("Ok"),
                  negativeLabel: tr("Button_Cancel"),
                  initialValue: "",
                });
                if (path === null || !String(path).trim()) {
                  return;
                }
                try {
                  await SteamService.ChangeAccountImage(rid, String(path).trim());
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
        },
      ];
    };
  }

  onMount(() => {
    previousPage.set({ page: "home" });
    void preflightAdminForPlatform(name);
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
  });

  onDestroy(() => {
    for (const t of steamListRefreshTimers) clearTimeout(t);
    steamListRefreshTimers = [];
    selectedAccountStore.set({ platformKey: "", uniqueId: "", displayName: "", accountLogin: "" });
    platformLiveSessionId.set({ platformKey: "", uniqueId: "" });
    platformAction.set(null);
    offSteamEvent?.();
    offShortcutsUpdated?.();
    offSort?.();
    offPlatformAction?.();
    offAccountsRefresh?.();
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
      <div class="steam-acclist" bind:this={steamAcclistEl}>
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
            <!-- Inline .find so updates to steamAccounts re-run this block (helpers are not dependency-tracked). -->
            {@const acc = steamAccounts.find((a) => a.steamId64 === rid)}
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
                <span class="steam-acc-avatar-wrap">
                  <img
                    class="steam-acc-avatar"
                    class:status_vac={acc?.showVac && acc?.vac}
                    class:status_limited={acc?.showLimited && acc?.ltd}
                    src={offlineSafeImageSrc(
                      $offlineMode,
                      acc?.imageUrl && !acc?.avatarPending ? acc.imageUrl : undefined,
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
                {#if acc?.showSteamId}
                  <p class="streamerCensor steamId">{acc.steamId64}</p>
                {/if}
                {#if acc?.showLastLogin && acc?.lastLogin}
                  <p class="lastLogin">{formatLastLoginForLocale(acc.lastLogin, $locale)}</p>
                {/if}
                {#if acc?.syncError}
                  <div class="steam_meta_err" title={acc.syncError}>{acc.syncError}</div>
                {:else if acc?.avatarPending}
                  <div class="steam_meta_pending">Updating…</div>
                {/if}
              </label>
            </div>
            {/key}
          </svelte:fragment>
        </ReorderPointerGrid>
      </div>
    </div>
    </div>
  {/if}
</div>
<ActionBar />

<style lang="scss">
  .platform-accounts-root {
    display: flex;
    flex-direction: column;
    min-height: 0;
    flex: 1;
  }

  .platform-accounts-hint {
    margin: 0.75rem 1rem 0;
    font-size: 0.85rem;
    color: var(--white, #fff);
    opacity: 0.85;
  }

  .platformTableHost {
    position: relative;
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
  }
</style>

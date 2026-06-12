<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import { get } from "svelte/store";
  import { Events } from "@wailsio/runtime";
  import SteamAccountAvatar from "../components/SteamAccountAvatar.svelte";
  import PlatformAccountsBase from "../components/PlatformAccountsBase.svelte";
  import type { PlatformAccountAdapter, SharedMenuItems } from "../components/PlatformAccountAdapter";
  import type { TagDefRow } from "../lib/accountTagsContext";
  import type { MenuItemDef } from "../stores/contextMenu";
  import type { PlatformSortKind } from "../stores/platformListSort";
  import { pushToast } from "../stores/toast";
  import { t } from "../stores/i18n";
  import { openConfirm, openPrompt } from "../stores/modal";
  import * as SteamService from "../../bindings/TcNo-Acc-Switcher/internal/steam/steamservice.js";
  import {
    AccountDTO,
    AccountPatch,
    SteamAccountEnrichmentDTO,
    SteamAccountListItemDTO,
  } from "../../bindings/TcNo-Acc-Switcher/internal/steam/models.js";
  import * as Shortcuts from "wails-shortcuts-service";
  import { ListPayload } from "../../bindings/TcNo-Acc-Switcher/internal/shortcuts/models.js";
  import { offlineMode, offlineSafeImageSrc, withAssetCacheBust } from "../stores/offlineMode";
  import { formatToastWithError } from "../lib/formatWailsError";
  import * as BasicService from "../../bindings/TcNo-Acc-Switcher/internal/basic/basicservice.js";
  import { buildSteamExtraMenu, type SteamMenuDeps } from "../lib/steam/contextMenuBuilder";
  import type { SteamAccountRow } from "../lib/steam/types";
  import { reportLaunchFailure } from "../lib/adminFlow";
  import { fuzzyWordsMatch } from "../lib/searchFuzzy";
  import { formatLastLoginForLocale } from "../lib/formatLastLogin";
  import "../styles/miniprofile.scss";
  import "../styles/platformAccountsShared.scss";

  const PROFILE_PLACEHOLDER = "/img/BasicDefault.webp";
  const SHORTCUT_ICON_FALLBACK = "/img/icons/file.svg";


  const STEAM_CONTEXT_MENU_HIDDEN_APP_IDS = new Set(["228980"]);




  type SteamAccountPatch = AccountPatch & {
    avatarFrameUrl?: string; miniProfileHtml?: string;
    showMiniProfile?: boolean; showAvatarFrame?: boolean;
  };

  export let name: string;

  let installedGames: { appId: string; name: string }[] = [];
  let steamShortcutIconByAppId: Record<string, string> = {};
  let steamShortcutIconByStemKey: Record<string, string> = {};
  let gameDataBySteamId: Record<string, { userdata: Set<string>; backup: Set<string> }> = {};
  let steamMainEl: HTMLDivElement | null = null;
  let offShortcutsUpdated: (() => void) | undefined;

  function safeFolderName(platformKey: string): string {
    let b = "";
    for (const r of platformKey.trim().toLowerCase()) {
      if (r === " " || r === "/" || r === "\\") b += "_";
      else if (/[a-z0-9\-_]/.test(r)) b += r;
    }
    return b || "unknown";
  }

  function normalizeGameSearchKey(s: string): string {
    return s.toLowerCase().replace(/[™©®]/g, "").replace(/[^a-z0-9]+/g, " ").trim().replace(/\s+/g, " ");
  }

  function resolveShortcutIconUrl(o: Record<string, unknown>, stemLow: string, platFolder: string): string {
    let iconRaw = String(o.iconUrl ?? o.IconURL ?? o.iconURL ?? "").trim();
    if (!iconRaw) iconRaw = `/img/shortcuts/${platFolder}/${stemLow}.png`;
    return offlineSafeImageSrc(get(offlineMode), iconRaw, SHORTCUT_ICON_FALLBACK);
  }

  function applyShortcutIconsFromShortcutList(list: unknown[]): void {
    const byAppId: Record<string, string> = {};
    const byStemKey: Record<string, string> = {};
    const platFolder = safeFolderName(name);
    for (const raw of list) {
      const o = (raw ?? {}) as Record<string, unknown>;
      const fn = String(o.fileName ?? o.FileName ?? "").trim();
      if (!fn) continue;
      const stem = fn.replace(/\.(lnk|url)$/i, "").trim();
      const stemLow = stem.toLowerCase();
      const iconUrl = resolveShortcutIconUrl(o, stemLow, platFolder);
      if (/^\d+$/.test(stem)) byAppId[stem] = iconUrl;
      const nk = normalizeGameSearchKey(stem);
      if (nk) byStemKey[nk] = iconUrl;
    }
    steamShortcutIconByAppId = byAppId;
    steamShortcutIconByStemKey = byStemKey;
  }

  function resolveSteamGameSearchIcon(g: { appId: string; name: string }): string {
    const id = String(g.appId).trim();
    if (steamShortcutIconByAppId[id]) return steamShortcutIconByAppId[id];
    const nk = normalizeGameSearchKey(g.name);
    if (steamShortcutIconByStemKey[nk]) return steamShortcutIconByStemKey[nk];
    return offlineSafeImageSrc(get(offlineMode), `/img/shortcuts/${safeFolderName(name)}/${id.toLowerCase()}.png`, SHORTCUT_ICON_FALLBACK);
  }

  async function refreshGameDataAppSets(steamIds: string[]): Promise<void> {
    if (steamIds.length === 0) { gameDataBySteamId = {}; return; }
    try {
      const parts = await Promise.all(steamIds.map(async (id) => {
        try {
          const s = await SteamService.GetSteamGameDataAppIDSets(id);
          return [id, { userdata: new Set(s.userdataAppIds.map((x: string) => String(x).trim())), backup: new Set(s.backupAppIds.map((x: string) => String(x).trim())) }] as const;
        } catch { return [id, { userdata: new Set<string>(), backup: new Set<string>() }] as const; }
      }));
      gameDataBySteamId = Object.fromEntries(parts);
    } catch { gameDataBySteamId = {}; }
  }


  function buildSteamExtraMenuAdapter(acc: SteamAccountRow, shared: SharedMenuItems): MenuItemDef[] {
    return buildSteamExtraMenu(acc, shared, getSteamMenuDeps());
  }

  function getSteamMenuDeps(): SteamMenuDeps {
    return {
      name,
      installedGames,
      gameDataBySteamId,
      steamIds,
      refreshGameDataAppSets,
    };
  }

  let steamIds: string[] = [];

  $: adapter = {
    platformKey: "Steam",
    profileFallback: PROFILE_PLACEHOLDER,

    id: (a: SteamAccountRow) => a.steamId64,
    name: (a: SteamAccountRow) => a.displayName?.trim() || a.personaName?.trim() || a.steamId64,
    imageUrl: (a: SteamAccountRow) => a.imageUrl,
    imagePending: (a: SteamAccountRow) => a.avatarPending ?? false,
    currentSession: (a: SteamAccountRow) => a.currentSession ?? false,
    manualProfileImage: (a: SteamAccountRow) => a.manualProfileImage ?? false,
    tags: (a: SteamAccountRow) => a.tags,
    note: (a: SteamAccountRow) => a.note ?? "",
    shouldShowNote: (a: SteamAccountRow) => a.showShortNotes === true && !!(a.note ?? "").trim(),
    shouldShowLastUsed: (a: SteamAccountRow) => a.showLastLogin === true && !!(a.lastLogin ?? "").trim(),
    lastUsed: (a: SteamAccountRow) => a.lastLogin ?? "",
    accountLogin: (a: SteamAccountRow) => (a.accountName ?? "").trim(),

    loadAccountsList: async () => {
      const rows = await SteamService.GetSteamAccountsList();
      return rows.map((r: SteamAccountListItemDTO) => ({
        steamId64: r.steamId64,
        personaName: r.personaName,
        displayName: r.displayName,
        accountName: r.accountName,
        currentSession: r.currentSession ?? false,
      })) as SteamAccountRow[];
    },
    loadAccountsEnrichment: async () => {
      const rows = await SteamService.GetSteamAccountsEnrichment();
      return rows.map((r: SteamAccountEnrichmentDTO) => ({
        steamId64: r.steamId64,
        displayName: r.displayName,
        lastLogin: r.lastLogin,
        offline: r.offline ?? false,
        imageUrl: r.imageUrl,
        staticImageUrl: r.staticImageUrl,
        avatarPending: r.avatarPending ?? false,
        metaPending: r.metaPending ?? false,
        vac: r.vac ?? false,
        ltd: r.ltd ?? false,
        showSteamId: r.showSteamId ?? false,
        showVac: r.showVac ?? false,
        showLimited: r.showLimited ?? false,
        showLastLogin: r.showLastLogin ?? false,
        showAccUsername: r.showAccUsername ?? false,
        collectInfo: r.collectInfo ?? false,
        showShortNotes: r.showShortNotes ?? false,
        note: r.note ?? "",
        avatarFrameUrl: r.avatarFrameUrl,
        miniProfileHtml: r.miniProfileHtml,
        showMiniProfile: r.showMiniProfile ?? false,
        showAvatarFrame: r.showAvatarFrame ?? false,
        syncError: r.syncError ?? "",
        tags: r.tags,
        manualProfileImage: r.manualProfileImage ?? false,
      })) as SteamAccountRow[];
    },
    swapTo: (id: string) => SteamService.SwapToSteamAccount(id, -1, []),
    saveOrder: (ids: string[]) => SteamService.SaveSteamAccountOrder(ids),
    addNew: () => SteamService.SteamAddNew(),
    forget: (id: string) => SteamService.ForgetSteamAccount(id),
    rename: async (id: string, newName: string) => {
      await BasicService.RenameAccount("Steam", id, newName);
    },
    changeImage: (id: string, path: string) => SteamService.ChangeAccountImage(id, path),
    clearManualImage: (id: string) => SteamService.ClearManualAccountProfileImage(id),
    getNote: (id: string) => BasicService.GetAccountNote("Steam", id),
    setNote: (id: string, note: string) => BasicService.SetAccountNote("Steam", id, note),
    launch: () => SteamService.LaunchSteam(),

    buildMenu: (_acc, shared) => buildSteamExtraMenuAdapter(_acc as SteamAccountRow, shared),

    updateEventName: "steam-account-updated",
    buildPatch: (raw: unknown) =>
      raw instanceof AccountPatch ? raw : AccountPatch.createFrom(raw as Record<string, unknown>),
    patchTargetId: (patch: unknown) => {
      const p = patch as { steamId64?: string };
      return (p.steamId64 ?? "").trim();
    },
    applyPatch: (patch: unknown, account: SteamAccountRow) => {
      const p = patch as SteamAccountPatch;
      const nextUrl = p.imageUrl || account.imageUrl;
      const errMsg = typeof p.error === "string" ? p.error : account.syncError ?? "";
      const nextManual = typeof p.manualProfileImage === "boolean" ? p.manualProfileImage : (account.manualProfileImage ?? false);
      return {
        ...account,
        imageUrl: nextUrl, vac: p.vac, ltd: p.ltd,
        avatarPending: p.avatarPending, metaPending: p.metaPending,
        manualProfileImage: nextManual, syncError: errMsg,
        displayName: typeof p.displayName === "string" && p.displayName.trim() !== "" ? p.displayName.trim() : account.displayName ?? "",
        staticImageUrl: typeof p.staticImageUrl === "string" && p.staticImageUrl.trim() !== "" ? p.staticImageUrl.trim() : account.staticImageUrl ?? "",
        avatarFrameUrl: typeof p.avatarFrameUrl === "string" && p.avatarFrameUrl.trim() !== "" ? p.avatarFrameUrl.trim() : account.avatarFrameUrl ?? "",
        miniProfileHtml: typeof p.miniProfileHtml === "string" && p.miniProfileHtml.trim() !== "" ? p.miniProfileHtml.trim() : account.miniProfileHtml ?? "",
        showMiniProfile: typeof p.showMiniProfile === "boolean" ? p.showMiniProfile : account.showMiniProfile,
        showAvatarFrame: typeof p.showAvatarFrame === "boolean" ? p.showAvatarFrame : account.showAvatarFrame,
      } as SteamAccountRow;
    },

    searchHay: (a: SteamAccountRow, trimmed: string) => {
      const parts = [a.displayName ?? "", a.personaName ?? "", a.note ?? ""];
      if (a.showAccUsername) parts.push(a.accountName ?? "");
      if (trimmed.toLowerCase().split(/\s+/).some((w) => /^\d{5,}$/.test(w))) parts.push(a.steamId64 ?? "");
      return parts.join("\n");
    },

    gameSearchRows: (q: string) => {
      const trimmed = q.trim();
      if (!trimmed) return [];
      return installedGames
        .filter((g) => fuzzyWordsMatch(trimmed, g.name))
        .slice(0, 5)
        .map((g) => ({ key: `g:${String(g.appId).trim()}`, title: g.name, badge: get(t)("Search_Section_Game"), isCategory: true, accountIconUrl: resolveSteamGameSearchIcon(g) }));
    },
    gameSearchHint: get(t)("Search_Hint_Games"),

    loginAndLaunchGame: async (accountId: string, appId: string) => {
      await SteamService.LoginAndLaunchGame(accountId, -1, appId);
    },

    onAfterLoad: async (accounts: SteamAccountRow[], ctx) => {
      steamIds = accounts.map((r) => r.steamId64);
      if (!ctx?.hadCachedAccounts || ctx.enrichChanged) {
        SteamService.StartSteamProfileRefresh();
      }
      try { await refreshGameDataAppSets(steamIds); } catch {}
    },
  } satisfies PlatformAccountAdapter<SteamAccountRow>;

  onMount(() => {
    void (async () => {
      try {
        const rows = await SteamService.GetInstalledGames();
        installedGames = rows
          .filter((r) => !STEAM_CONTEXT_MENU_HIDDEN_APP_IDS.has(String(r.appId).trim()))
          .map((r) => ({ appId: r.appId, name: r.name }));
      } catch { installedGames = []; }
    })();

    void (async () => {
      try {
        const list = await Shortcuts.ListShortcuts("Steam");
        applyShortcutIconsFromShortcutList(list as unknown[]);
      } catch {}
    })();

    offShortcutsUpdated = Events.On("shortcuts-updated", (ev) => {
      try {
        const raw = ev.data;
        const p = raw instanceof ListPayload ? raw : ListPayload.createFrom(raw as Record<string, unknown>);
        if (p.platformKey !== name) return;
        applyShortcutIconsFromShortcutList(p.shortcuts ?? []);
      } catch {}
    });
  });

  onDestroy(() => {
    offShortcutsUpdated?.();
  });
</script>

<div class="main-content platform-accounts-root" bind:this={steamMainEl}>
  <PlatformAccountsBase {name} {adapter}>
    <svelte:fragment slot="account-avatar" let:acc let:epoch let:fallback>
      <SteamAccountAvatar account={acc} {epoch} {fallback} boundary={steamMainEl} />
    </svelte:fragment>

    <svelte:fragment slot="account-before-name" let:acc>
      {@const a = acc}
      {#if a.showAccUsername && a.accountName}
        <p class="streamerCensor">{a.accountName}</p>
      {/if}
    </svelte:fragment>

    <svelte:fragment slot="account-after-stats" let:acc>
      {@const a = acc}
      {#if a.showSteamId}
        <p class="streamerCensor steamId">{a.steamId64}</p>
      {/if}
    </svelte:fragment>

    <svelte:fragment slot="account-footer" let:acc>
      {@const a = acc}
      {#if a.syncError}
        <div class="steam_meta_err" title={a.syncError}>{a.syncError}</div>
      {:else if a.avatarPending}
        <div class="steam_meta_pending">{$t("Status_Updating")}</div>
      {/if}
    </svelte:fragment>
  </PlatformAccountsBase>
</div>

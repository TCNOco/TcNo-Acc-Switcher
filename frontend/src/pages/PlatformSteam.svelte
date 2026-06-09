<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import { get } from "svelte/store";
  import { Events } from "@wailsio/runtime";
  import PlatformAccountsBase from "../components/PlatformAccountsBase.svelte";
  import type { PlatformAccountAdapter, SharedMenuItems } from "../components/PlatformAccountAdapter";
  import type { TagDefRow } from "../lib/accountTagsContext";
  import type { MenuItemDef } from "../stores/contextMenu";
  import type { PlatformSortKind } from "../stores/platformListSort";
  import { pushToast } from "../stores/toast";
  import { t } from "../stores/i18n";
  import { openConfirm, openPrompt } from "../stores/modal";
  import * as SteamService from "../../bindings/TcNo-Acc-Switcher/internal/steam/steamservice.js";
  import { AccountDTO, AccountPatch } from "../../bindings/TcNo-Acc-Switcher/internal/steam/models.js";
  import * as Shortcuts from "wails-shortcuts-service";
  import { ListPayload } from "../../bindings/TcNo-Acc-Switcher/internal/shortcuts/models.js";
  import { offlineMode, offlineSafeImageSrc, withAssetCacheBust } from "../stores/offlineMode";
  import { isProfileVideoUrl } from "../lib/profileImageDrop";
  import { miniProfileHover } from "../lib/actions/miniProfileHover";
  import { formatToastWithError, formatWailsError } from "../lib/formatWailsError";
  import * as BasicService from "../../bindings/TcNo-Acc-Switcher/internal/basic/basicservice.js";
  import { reportLaunchFailure } from "../lib/adminFlow";
  import { fuzzyWordsMatch } from "../lib/searchFuzzy";
  import { formatLastLoginForLocale } from "../lib/formatLastLogin";
  import "../styles/miniprofile.scss";
  import "../styles/platformAccountsShared.scss";

  const PROFILE_PLACEHOLDER = "/img/BasicDefault.webp";
  const SHORTCUT_ICON_FALLBACK = "/img/icons/file.svg";

  const STEAM_USERDATA_ERR_KEYS = new Set([
    "Toast_NoValidSteamId", "Toast_SameAccount", "Toast_NoFindSteamUserdata", "Toast_NoFindGameBackup",
  ]);

  const STEAM_CONTEXT_MENU_HIDDEN_APP_IDS = new Set(["228980"]);

  function mapSteamUserdataI18nError(err: unknown, tr: (k: string, v?: Record<string, string | number>) => string): string {
    return formatWailsError(err, { translateMessage: (k) => tr(k), i18nFirstLineKeys: STEAM_USERDATA_ERR_KEYS });
  }

  type SteamAccountRow = InstanceType<typeof AccountDTO> & {
    tags?: TagDefRow[]; syncError?: string; currentSession: boolean;
    showShortNotes: boolean; note: string; staticImageUrl?: string;
    avatarFrameUrl?: string; miniProfileHtml?: string; showMiniProfile?: boolean; showAvatarFrame?: boolean;
  };

  function steamListAvatarUrl(acc: SteamAccountRow, offline: boolean): string | undefined {
    if (acc.avatarPending) return undefined;
    const primary = acc.imageUrl?.trim() || undefined;
    const fallback = acc.staticImageUrl?.trim() || undefined;
    if (offline) {
      if (fallback) return fallback;
      if (primary && !isProfileVideoUrl(primary)) return primary;
      return undefined;
    }
    return primary ?? fallback;
  }

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

  async function clipboardWrite(text: string): Promise<void> {
    try { await navigator.clipboard.writeText(text); pushToast({ type: "success", message: get(t)("Toast_Copied"), duration: 2500 }); }
    catch { pushToast({ type: "error", message: get(t)("Toast_CopyFailed"), duration: 4000 }); }
  }

  function buildSteamExtraMenu(acc: SteamAccountRow, shared: SharedMenuItems): MenuItemDef[] {
    const tr = get(t);
    const rid = acc.steamId64;

    const loginStates = [
      { st: 7, lab: tr("Invisible") }, { st: 0, lab: tr("Offline") }, { st: 1, lab: tr("Online") },
      { st: 2, lab: tr("Busy") }, { st: 3, lab: tr("Away") }, { st: 4, lab: tr("Snooze") },
      { st: 5, lab: tr("LookingToTrade") }, { st: 6, lab: tr("LookingToPlay") },
    ];

    const loginAsChildren: MenuItemDef[] = [
      { type: "search", label: tr("Context_Search") },
      ...loginStates.map((x) => ({
        label: x.lab,
        action: async () => {
          try { await SteamService.SwapToSteamAccount(rid, x.st, []); pushToast({ type: "success", message: tr("Toast_AccountSwitched"), duration: 4000 }); }
          catch (e) { pushToast({ type: "error", message: formatToastWithError(tr("Toast_SwitchFailed"), e), duration: 8000 }); }
        },
      })),
    ];

    const copyChildren: MenuItemDef[] = [
      { label: tr("Context_CommunityUrl"), action: () => void clipboardWrite(`https://steamcommunity.com/profiles/${rid}`) },
      { label: tr("Context_CommunityUsername"), action: () => void clipboardWrite((acc.displayName ?? "").trim() || (acc.personaName ?? "").trim() || rid) },
      { label: tr("Context_LoginUsername"), action: () => void clipboardWrite((acc.accountName ?? "").trim() || rid) },
      {
        label: tr("Context_CopySteamIdSubmenu"),
        children: [
          { label: tr("Context_Steam_Id64"), action: async () => {
            try { const f = await SteamService.GetSteamIDFormats(rid); void clipboardWrite(f["ID64"] ?? rid); } catch { void clipboardWrite(rid); }
          }},
          { label: tr("Context_Steam_Id3"), action: async () => {
            try { const f = await SteamService.GetSteamIDFormats(rid); void clipboardWrite(f["ID3"] ?? ""); } catch {}
          }},
          { label: tr("Context_Steam_Id32"), action: async () => {
            try { const f = await SteamService.GetSteamIDFormats(rid); void clipboardWrite(f["ID32"] ?? ""); } catch {}
          }},
        ],
      },
    ];

    const shortcutChildren: MenuItemDef[] = [
      { type: "search", label: tr("Context_Search") },
      ...loginStates.map((x) => ({
        label: `${tr("Context_CreateShortcut")} (${x.lab})`,
        action: async () => {
          try {
            const p = await Shortcuts.CreateAccountShortcut("Steam", rid, acc.displayName?.trim() || acc.personaName?.trim() || rid, String(x.st), x.lab, (acc.accountName ?? "").trim());
            pushToast({ type: "success", message: `${tr("Toast_ShortcutCreated")}\n${p}`, duration: 6000 });
          } catch (e) { pushToast({ type: "error", message: formatToastWithError(get(t)("Toast_SwitchFailed"), e), duration: 8000 }); }
        },
      })),
    ];

    const gsets = gameDataBySteamId[rid];
    const gameDataItems: MenuItemDef[] = [];
    for (const g of installedGames) {
      const aid = String(g.appId).trim();
      const hasUser = gsets?.userdata.has(aid) ?? false;
      const hasBackup = gsets?.backup.has(aid) ?? false;
      if (!hasUser && !hasBackup) continue;
      const children: MenuItemDef[] = [
        { label: "Open folder", action: async () => {
          try { await SteamService.OpenSteamGameDataFolder(rid, g.appId); }
          catch (e) { pushToast({ type: "error", message: mapSteamUserdataI18nError(e, tr), duration: 8000 }); }
        }},
      ];
      if (hasUser) {
        children.push({ label: tr("Context_Game_CopySettingsFrom"), action: async () => {
          try { await SteamService.CopySteamGameSettingsFrom(rid, g.appId); pushToast({ type: "success", message: tr("Toast_SettingsCopied"), duration: 5000 }); void refreshGameDataAppSets(steamIds); }
          catch (e) { pushToast({ type: "error", message: mapSteamUserdataI18nError(e, tr), duration: 8000 }); }
        }});
      }
      if (hasBackup) {
        children.push({ label: tr("Context_Game_RestoreSettingsTo"), action: async () => {
          try { await SteamService.RestoreSteamGameSettingsTo(rid, g.appId); pushToast({ type: "success", message: tr("Toast_GameDataRestored"), duration: 5000 }); void refreshGameDataAppSets(steamIds); }
          catch (e) { pushToast({ type: "error", message: mapSteamUserdataI18nError(e, tr), duration: 8000 }); }
        }});
      }
      if (hasUser) {
        children.push({ label: tr("Context_Game_BackupData"), action: async () => {
          try { const folder = await SteamService.BackupSteamGameData(rid, g.appId); pushToast({ type: "success", message: tr("Toast_GameBackupDone", { folderLocation: folder }), duration: 8000 }); void refreshGameDataAppSets(steamIds); }
          catch (e) { pushToast({ type: "error", message: mapSteamUserdataI18nError(e, tr), duration: 8000 }); }
        }});
      }
      gameDataItems.push({ label: g.name, children });
    }

    const gameChildren: MenuItemDef[] = gameDataItems.length === 0
      ? [{ type: "item", label: tr("Context_GameData_NoFolders") }]
      : [{ type: "search", label: tr("Context_Search") }, ...gameDataItems];

    const launchChildren: MenuItemDef[] = [
      { type: "search", label: tr("Context_Search") },
      ...installedGames.map((g) => ({
        label: g.name,
        action: async () => {
          try {
            await SteamService.LoginAndLaunchGame(rid, -1, g.appId);
            pushToast({
              type: "success",
              message: tr("Toast_StartedGame", { program: g.name }),
              duration: 4000,
            });
          }
          catch (e) { await reportLaunchFailure(e, name); }
        },
      })),
    ];

    return [
      shared.swapTo,
      { label: tr("Context_Game_LoginAndLaunch"), children: launchChildren },
      { label: tr("Context_LoginAsSubmenu"), children: loginAsChildren },
      { label: tr("Context_CopySubmenu"), children: copyChildren },
      shared.createShortcut,
      { label: `${tr("Context_CreateShortcut")} (${tr("PersonaState")})`, children: shortcutChildren },
      shared.forget,
      shared.notes,
      shared.tags,
      {
        label: tr("Context_ManageSubmenu"),
        children: ([
          { label: tr("Context_GameDataSubmenu"), children: gameChildren },
          shared.gameStats,
          { label: tr("Context_Steam_OpenUserdata"), action: async () => {
            try { await SteamService.OpenUserdataFolder(rid); }
            catch (e) { pushToast({ type: "error", message: formatToastWithError(get(t)("Toast_LaunchFailed"), e), duration: 8000 }); }
          }},
          shared.changeImage,
        ] as (MenuItemDef | null)[]).filter((x): x is MenuItemDef => x != null),
      },
    ];
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

    loadAccounts: () => SteamService.GetSteamAccounts() as Promise<SteamAccountRow[]>,
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

    buildMenu: (_acc, shared) => buildSteamExtraMenu(_acc as SteamAccountRow, shared),

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

    onAfterLoad: async (accounts: SteamAccountRow[]) => {
      steamIds = accounts.map((r) => r.steamId64);
      SteamService.StartSteamProfileRefresh();
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
      {@const a = acc}
      {@const avatarSrc = offlineSafeImageSrc($offlineMode, withAssetCacheBust(steamListAvatarUrl(a, $offlineMode), epoch), fallback)}
      {@const avatarIsVideo = !$offlineMode && isProfileVideoUrl(avatarSrc)}
      <span class="steam-acc-avatar-wrap">
        {#if avatarIsVideo}
          <video
            class="steam-acc-avatar"
            class:status_vac={a.showVac && a.vac}
            class:status_limited={a.showLimited && a.ltd}
            src={avatarSrc}
            autoplay loop muted playsinline
            aria-hidden="true" draggable="false"
            use:miniProfileHover={{
              html: a.miniProfileHtml ?? "",
              boundary: steamMainEl,
              offline: $offlineMode,
              enabled: !!(a.showMiniProfile && (a.miniProfileHtml ?? "").trim() !== ""),
            }}
          ></video>
        {:else}
          <img
            class="steam-acc-avatar"
            class:status_vac={a.showVac && a.vac}
            class:status_limited={a.showLimited && a.ltd}
            src={avatarSrc}
            alt="" draggable="false"
            use:miniProfileHover={{
              html: a.miniProfileHtml ?? "",
              boundary: steamMainEl,
              offline: $offlineMode,
              enabled: !!(a.showMiniProfile && (a.miniProfileHtml ?? "").trim() !== ""),
            }}
          />
        {/if}
        {#if a.showAvatarFrame && (a.avatarFrameUrl ?? "").trim() !== "" && !$offlineMode}
          <img class="steam-acc-avatar-frame" src={offlineSafeImageSrc($offlineMode, a.avatarFrameUrl, fallback)} alt="" draggable="false" />
        {/if}
      </span>
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

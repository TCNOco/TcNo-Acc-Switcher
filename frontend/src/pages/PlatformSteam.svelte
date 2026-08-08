<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import { get } from "svelte/store";
  import { Events } from "@wailsio/runtime";
  import SteamAccountAvatar from "../components/SteamAccountAvatar.svelte";
  import { cs2CooldownRemaining, cs2CooldownTooltip } from "../lib/steam/cs2Cooldown";
  import PlatformAccountsBase from "../components/PlatformAccountsBase.svelte";
  import type { AccountNameStatus, PlatformAccountAdapter, SharedMenuItems } from "../components/PlatformAccountAdapter";
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
  import { offlineMode } from "../stores/offlineMode";
  import { formatToastWithError } from "../lib/formatWailsError";
  import * as BasicService from "../../bindings/TcNo-Acc-Switcher/internal/basic/basicservice.js";
  import { buildSteamExtraMenu } from "../lib/steam/contextMenuBuilder";
  import type { SteamMenuDeps } from "../lib/steam/menuCommands";
  import type { SteamAccountRow } from "../lib/steam/types";
  import { steamAccountVisualKey } from "../lib/steam/accountVisualKey";
  import { emitSteamGuardMenuRequest } from "../lib/steam/steamGuardMenuRequest";
  import {
    refreshSteamGuardVaultUnlocked,
    steamGuardVaultUnlockedNow,
  } from "../lib/steam/steamGuardQuickCopy";
  import { reportLaunchFailure } from "../lib/adminFlow";
  import { fuzzyWordsMatch } from "../lib/searchFuzzy";
  import { formatLastLoginForLocale } from "../lib/formatLastLogin";
  import { shortcutIconIndexes, steamGameIconUrl } from "../lib/shortcutAssets";
  import {
    clearSteamGuardActionAccount,
    publishSteamGuardActionAccounts,
  } from "../stores/steamGuardAction";
  import "../styles/miniprofile.scss";
  import "../styles/platformAccountsShared.scss";

  const PROFILE_PLACEHOLDER = "/img/BasicDefault.webp";
  const SHORTCUT_ICON_FALLBACK = "/img/icons/file.svg";


  const STEAM_CONTEXT_MENU_HIDDEN_APP_IDS = new Set(["228980"]);




  type SteamAccountPatch = AccountPatch & {
    avatarFrameUrl?: string; miniProfileHtml?: string;
    showMiniProfile?: boolean; showAvatarFrame?: boolean;
    showSteamGuardLock?: boolean;
  };

  export let name: string;

  let installedGames: { appId: string; name: string }[] = [];
  let steamShortcutIconByAppId: Record<string, string> = {};
  let steamShortcutIconByStemKey: Record<string, string> = {};
  let gameDataBySteamId: Record<string, { userdata: Set<string>; backup: Set<string> }> = {};
  let steamMainEl: HTMLDivElement | null = null;
  let offShortcutsUpdated: (() => void) | undefined;

  function applyShortcutIconsFromShortcutList(list: unknown[]): void {
    const indexes = shortcutIconIndexes(list, name, SHORTCUT_ICON_FALLBACK, get(offlineMode));
    steamShortcutIconByAppId = indexes.byAppId;
    steamShortcutIconByStemKey = indexes.byStemKey;
  }

  function resolveSteamGameSearchIcon(g: { appId: string; name: string }): string {
    return steamGameIconUrl(
      g,
      name,
      { byAppId: steamShortcutIconByAppId, byStemKey: steamShortcutIconByStemKey },
      SHORTCUT_ICON_FALLBACK,
      get(offlineMode),
    );
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
    // This menu is built from the cached vault state; the refresh is for the next one.
    void refreshSteamGuardVaultUnlocked();
    return buildSteamExtraMenu(acc, shared, getSteamMenuDeps());
  }

  function getSteamMenuDeps(): SteamMenuDeps {
    return {
      name,
      installedGames,
      gameDataBySteamId,
      steamIds,
      refreshGameDataAppSets,
      openSteamGuard: emitSteamGuardMenuRequest,
      steamGuardVaultUnlocked: steamGuardVaultUnlockedNow(),
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
    nameStatus: (a: SteamAccountRow): AccountNameStatus | null => {
      // VAC outranks limited: it is the harder verdict, and an account can be both.
      if (a.showVac === true && a.vac === true) return { kind: "vac", label: $t("Steam_Status_VacBanned") };
      if (a.showLimited === true && a.ltd === true) return { kind: "limited", label: $t("Steam_Status_Limited") };
      return null;
    },

    extraA11yStatus: (a: SteamAccountRow): string[] => {
      if (a.showCs2Cooldown === false) return [];
      const cooldown = cs2CooldownRemaining(a.cs2CooldownExpiresAt, a.cs2CooldownPermanent, Date.now());
      return cooldown ? [cs2CooldownTooltip(cooldown, $t)] : [];
    },

    visualKey: steamAccountVisualKey,

    loadAccountsList: async () => {
      const rows = await SteamService.GetSteamAccountsList();
      // No displayName here on purpose — it is enrichment-owned. This row is spread over the
      // existing one, so carrying the key at all would overwrite the community name already on
      // screen with the vdf persona until the enrichment round-trip lands.
      return rows.map((r: SteamAccountListItemDTO) => ({
        steamId64: r.steamId64,
        personaName: r.personaName,
        accountName: r.accountName,
        currentSession: r.currentSession ?? false,
        hasSteamGuard: r.hasSteamGuard ?? false,
        steamGuardPending: r.steamGuardPending ?? false,
        steamGuardLoginOnly: r.steamGuardLoginOnly ?? false,
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
        hasVisibleBan: r.hasVisibleBan ?? false,
        banStatusHidden: r.banStatusHidden ?? false,
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
        showSteamGuardLock: r.showSteamGuardLock ?? true,
        syncError: r.syncError ?? "",
        tags: r.tags,
        manualProfileImage: r.manualProfileImage ?? false,
        cs2CooldownExpiresAt: r.cs2CooldownExpiresAt ?? "",
        cs2CooldownPermanent: r.cs2CooldownPermanent ?? false,
        showCs2Cooldown: r.showCs2Cooldown ?? true,
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
    closePlatform: () => SteamService.CloseSteam(),
    refreshOnWindowFocus: true,
    refreshAccounts: () => SteamService.RefreshAllSteamImages(),
    refreshAllProfileImages: () => SteamService.RefreshAllSteamImages(),

    buildMenu: (_acc, shared) => buildSteamExtraMenuAdapter(_acc as SteamAccountRow, shared),

    updateEventName: "steam-account-updated",

    // Its own event rather than an AccountPatch: that DTO's vac/ltd/imageUrl
    // fields carry no omitempty, so a cooldown-only patch would arrive with all
    // of them at their zero value and the merge would apply them.
    extraUpdateEvents: [
      {
        name: "steam-cs2-cooldown-updated",
        targetId: (raw: unknown) => ((raw as { steamId64?: string })?.steamId64 ?? "").trim(),
        apply: (raw: unknown, account: SteamAccountRow) => {
          const p = raw as {
            cs2CooldownExpiresAt?: string;
            cs2CooldownPermanent?: boolean;
            tags?: SteamAccountRow["tags"];
          };
          return {
            ...account,
            cs2CooldownExpiresAt: p.cs2CooldownExpiresAt ?? "",
            cs2CooldownPermanent: p.cs2CooldownPermanent === true,
            tags: p.tags ?? [],
          } as SteamAccountRow;
        },
      },
    ],
    buildPatch: (raw: unknown) =>
      raw instanceof AccountPatch ? raw : AccountPatch.createFrom(raw as Record<string, unknown>),
    patchTargetId: (patch: unknown) => {
      const p = patch as { steamId64?: string };
      return (p.steamId64 ?? "").trim();
    },
    applyPatch: (patch: unknown, account: SteamAccountRow) => {
      const p = patch as SteamAccountPatch;
      // AccountPatch.imageUrl has no omitempty and the generated constructor defaults it to "",
      // so a patch that never touched the avatar still arrives carrying a blank. Treat blank as
      // "not carried", like the string fields below — nothing clears an avatar by sending "".
      const patchUrl = typeof p.imageUrl === "string" ? p.imageUrl.trim() : "";
      const nextUrl = patchUrl !== "" ? patchUrl : account.imageUrl;
      const errMsg = typeof p.error === "string" ? p.error : account.syncError ?? "";
      const nextManual = typeof p.manualProfileImage === "boolean" ? p.manualProfileImage : (account.manualProfileImage ?? false);
      return {
        ...account,
        imageUrl: nextUrl,
        vac: typeof p.vac === "boolean" ? p.vac : account.vac,
        ltd: typeof p.ltd === "boolean" ? p.ltd : account.ltd,
        avatarPending: typeof p.avatarPending === "boolean" ? p.avatarPending : account.avatarPending,
        metaPending: typeof p.metaPending === "boolean" ? p.metaPending : account.metaPending,
        manualProfileImage: nextManual, syncError: errMsg,
        displayName: typeof p.displayName === "string" && p.displayName.trim() !== "" ? p.displayName.trim() : account.displayName ?? "",
        staticImageUrl: typeof p.staticImageUrl === "string" && p.staticImageUrl.trim() !== "" ? p.staticImageUrl.trim() : account.staticImageUrl ?? "",
        avatarFrameUrl: typeof p.avatarFrameUrl === "string" && p.avatarFrameUrl.trim() !== "" ? p.avatarFrameUrl.trim() : account.avatarFrameUrl ?? "",
        miniProfileHtml: typeof p.miniProfileHtml === "string" && p.miniProfileHtml.trim() !== "" ? p.miniProfileHtml.trim() : account.miniProfileHtml ?? "",
        showMiniProfile: typeof p.showMiniProfile === "boolean" ? p.showMiniProfile : account.showMiniProfile,
        showAvatarFrame: typeof p.showAvatarFrame === "boolean" ? p.showAvatarFrame : account.showAvatarFrame,
        showSteamGuardLock: typeof p.showSteamGuardLock === "boolean" ? p.showSteamGuardLock : account.showSteamGuardLock,
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
      publishSteamGuardActionAccounts(accounts);
      if (!ctx?.hadCachedAccounts || ctx.enrichChanged) {
        SteamService.StartSteamProfileRefresh();
      }
      try { await refreshGameDataAppSets(steamIds); } catch {}
      void refreshSteamGuardVaultUnlocked();
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
    clearSteamGuardActionAccount();
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

    <!-- No account-after-name fragment: the cooldown and Prime states are
         managed tags drawn by AccountTagBubbles, and the CS2 rank is a game
         stats metric drawn in the stats row with every other game's. -->

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
      {:else if a.metaPending || a.avatarPending}
        <div class="steam_meta_pending">{$t("Status_Updating")}</div>
      {/if}
    </svelte:fragment>
  </PlatformAccountsBase>
</div>

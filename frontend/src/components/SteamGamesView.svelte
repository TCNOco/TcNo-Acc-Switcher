<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import { get } from "svelte/store";
  import { Events } from "@wailsio/runtime";
  import SearchOverlay, { type SearchResultRow } from "./SearchOverlay.svelte";
  import * as SteamService from "../../bindings/TcNo-Acc-Switcher/internal/steam/steamservice.js";
  import type {
    SteamAccountEnrichmentDTO,
    SteamAccountListItemDTO,
    OwnedGameDTO,
  } from "../../bindings/TcNo-Acc-Switcher/internal/steam/models.js";
  import { t } from "../stores/i18n";
  import { pushToast } from "../stores/toast";
  import { actionBarStatus } from "../stores/fileDrop";
  import { platformActionBusy, requestPlatformAccountsRefresh } from "../stores/platformPage";
  import { closeSearchOverlay, searchOverlayCtrl } from "../stores/searchOverlay";
  import { platformListSort, type PlatformSortKind } from "../stores/platformListSort";
  import { offlineMode, offlineSafeImageSrc } from "../stores/offlineMode";
  import {
    clearSteamGamesBar,
    setSteamGamesAccountPickHandler,
    steamGamesBarAccounts,
    steamGamesLaunchOnSwitch,
    type SteamGamesBarAccount,
  } from "../stores/steamGamesBar";
  import { contextMenu as ctxMenuAction } from "../lib/actions/contextMenu";
  import { tooltip as tooltipAction } from "../lib/actions/tooltip";
  import { buildFilterMenuItems } from "../lib/filterMenu";
  import type { MenuItemDef } from "../stores/contextMenu";
  import { formatToastWithError } from "../lib/formatWailsError";
  import { reportLaunchFailure } from "../lib/adminFlow";
  import { isProfileVideoUrl } from "../lib/profileImageDrop";
  import {
    filterOwnedGames,
    sortOwnedGames,
    splitGameOwners,
    type OwnedGameRow,
  } from "../lib/steam/ownedGames";
  import "../styles/platformAccountsShared.scss";
  import "../styles/steamGames.scss";

  const PROFILE_PLACEHOLDER = "/img/BasicDefault.webp";
  const GAME_ICON_FALLBACK = "/img/icons/file.svg";
  /** Owner avatars drawn inline before the row collapses the rest into "+N". */
  const MAX_OWNER_AVATARS = 5;
  const GAMES_REFRESH_DEBOUNCE_MS = 400;
  const SEARCH_MAX = 8;

  export let name: string;

  type GamesAccount = {
    steamId64: string;
    displayName: string;
    avatarUrl: string;
    inVault: boolean;
  };

  let games: OwnedGameRow[] = [];
  let accounts: GamesAccount[] = [];
  let accountsLoaded = false;
  let gamesLoading = true;
  let loadError = "";
  let selectedAppId = "";
  let sortKind: PlatformSortKind = "alpha_asc";
  let listEl: HTMLDivElement | undefined;
  let overlayQuery = "";
  let overlayQueryDebounceTimer: ReturnType<typeof setTimeout> | null = null;
  let debouncedOverlayQuery = "";
  let gamesRefreshTimer: ReturnType<typeof setTimeout> | null = null;
  let offGamesUpdated: (() => void) | undefined;
  let offSort: (() => void) | undefined;
  let lastHandledSortId = 0;
  let busy = false;

  $: so = $searchOverlayCtrl;
  $: accountById = new Map(accounts.map((a) => [a.steamId64, a]));
  $: hasVaultAccounts = accounts.some((a) => a.inVault);
  $: sortedGames = sortOwnedGames(games, sortKind);
  $: selectedGame = sortedGames.find((g) => g.appId === selectedAppId) ?? null;

  $: {
    const q = overlayQuery;
    if (overlayQueryDebounceTimer) clearTimeout(overlayQueryDebounceTimer);
    overlayQueryDebounceTimer = setTimeout(() => { debouncedOverlayQuery = q; }, 150);
  }

  $: searchRows = buildGameSearchRows(debouncedOverlayQuery);

  function ownerLabel(steamId64: string): string {
    return accountById.get(steamId64)?.displayName || steamId64;
  }

  function ownerAvatar(steamId64: string, offline: boolean): string {
    return offlineSafeImageSrc(
      offline,
      accountById.get(steamId64)?.avatarUrl ?? "",
      PROFILE_PLACEHOLDER,
    );
  }

  function gameIcon(game: OwnedGameRow): string {
    return offlineSafeImageSrc($offlineMode, game.iconUrl, GAME_ICON_FALLBACK);
  }

  function ownersTooltip(owners: string[]): string {
    if (owners.length === 0) return $t("Steam_Games_OwnerUnknown");
    return owners.map(ownerLabel).join("\n");
  }

  function selectGame(appId: string): void {
    selectedAppId = appId;
    touchStatus();
  }

  function touchStatus(): void {
    if (busy) return;
    // Reads `games` rather than the reactive `selectedGame`, which is still a
    // frame behind when this runs straight after a selection change.
    const game = games.find((g) => g.appId === selectedAppId);
    actionBarStatus.set(game ? $t("Status_SelectedAccount", { name: game.name }) : "");
  }

  /**
   * The avatar the switcher already has. A video avatar cannot render inside a
   * 30px tile the way an image does, so the static copy wins when there is one.
   */
  function resolveAvatarUrl(imageUrl: string, staticImageUrl: string, pending: boolean): string {
    if (pending) return "";
    const primary = imageUrl.trim();
    const fallback = staticImageUrl.trim();
    if (isProfileVideoUrl(primary) && fallback) return fallback;
    return primary || fallback;
  }

  async function loadAccounts(): Promise<void> {
    const [list, enrichment] = await Promise.all([
      SteamService.GetSteamAccountsList(),
      SteamService.GetSteamAccountsEnrichment().catch(() => [] as SteamAccountEnrichmentDTO[]),
    ]);
    const enrichById = new Map(
      enrichment.map((row: SteamAccountEnrichmentDTO) => [row.steamId64, row]),
    );
    accounts = list.map((row: SteamAccountListItemDTO) => {
      const extra = enrichById.get(row.steamId64);
      // Stored raw, not offline-safed here: accounts load once and offline mode
      // can be switched on afterwards, which would leave these avatars pointing
      // at remote URLs the guard exists to suppress. Consumers apply it.
      const avatarUrl = resolveAvatarUrl(
        extra?.imageUrl ?? "",
        extra?.staticImageUrl ?? "",
        extra?.avatarPending ?? false,
      );
      return {
        steamId64: row.steamId64,
        displayName:
          (extra?.displayName ?? "").trim()
          || (row.personaName ?? "").trim()
          || (row.accountName ?? "").trim()
          || row.steamId64,
        avatarUrl,
        inVault: (row.hasSteamGuard ?? false) || (row.steamGuardLoginOnly ?? false),
      };
    });
    steamGamesBarAccounts.set(
      accounts.map(({ steamId64, displayName, avatarUrl }): SteamGamesBarAccount => ({
        steamId64,
        displayName,
        avatarUrl,
      })),
    );
    accountsLoaded = true;
  }

  async function loadGames(): Promise<void> {
    try {
      const rows = await SteamService.GetOwnedGamesList();
      games = rows.map((row: OwnedGameDTO) => ({
        appId: row.appId,
        name: row.name,
        iconUrl: row.iconUrl,
        owners: row.owners ?? [],
      }));
      loadError = "";
    } catch (e) {
      games = [];
      loadError = formatToastWithError($t("Steam_Games_LoadFailed"), e);
    } finally {
      gamesLoading = false;
    }
  }

  function scheduleGamesRefresh(): void {
    if (gamesRefreshTimer) clearTimeout(gamesRefreshTimer);
    gamesRefreshTimer = setTimeout(() => {
      gamesRefreshTimer = null;
      void loadGames();
    }, GAMES_REFRESH_DEBOUNCE_MS);
  }

  async function refreshOwnedGames(): Promise<void> {
    try {
      await SteamService.RefreshOwnedGames();
      pushToast({ type: "info", message: $t("Steam_Games_Refreshing"), duration: 4000 });
    } catch (e) {
      pushToast({
        type: "error",
        message: formatToastWithError($t("Steam_Games_LoadFailed"), e),
        duration: 8000,
      });
    }
  }

  async function pickAccount(steamId64: string): Promise<void> {
    if (busy) return;
    const account = accountById.get(steamId64);
    if (!account) return;
    const launch = get(steamGamesLaunchOnSwitch);
    const game = selectedGame;
    if (launch && !game) {
      pushToast({ type: "info", message: $t("Steam_Games_PickGameFirst"), duration: 5000 });
      return;
    }
    busy = true;
    platformActionBusy.set({ busy: true, platformKey: name });
    actionBarStatus.set($t("Status_SelectedAccount", { name: account.displayName }));
    try {
      if (launch && game) {
        await SteamService.LoginAndLaunchGame(steamId64, -1, game.appId);
        pushToast({
          type: "success",
          message: $t("Toast_StartedGame", { program: game.name }),
          duration: 4000,
        });
      } else {
        await SteamService.SwapToSteamAccount(steamId64, -1, []);
        pushToast({ type: "success", message: $t("Toast_AccountSwitched"), duration: 4000 });
      }
      requestPlatformAccountsRefresh(name);
    } catch (e) {
      await reportLaunchFailure(e, name);
    } finally {
      busy = false;
      platformActionBusy.set({ busy: false, platformKey: "" });
      touchStatus();
    }
  }

  function buildGameSearchRows(query: string): SearchResultRow[] {
    const trimmed = query.trim();
    if (!trimmed) return [];
    return filterOwnedGames(sortedGames, trimmed)
      .slice(0, SEARCH_MAX)
      .map((game): SearchResultRow => ({
        key: `g:${game.appId}`,
        title: game.name,
        badge: $t("Search_Section_Game"),
        accountIconUrl: gameIcon(game),
      }));
  }

  function onSearchPick(ev: CustomEvent<SearchResultRow>): void {
    const row = ev.detail;
    closeSearchOverlay();
    if (!row.key.startsWith("g:")) return;
    selectGame(row.key.slice(2));
    listEl
      ?.querySelector<HTMLElement>(`[data-app-id="${CSS.escape(row.key.slice(2))}"]`)
      ?.scrollIntoView({ block: "nearest" });
  }

  function backgroundMenu(): MenuItemDef[] {
    return [
      ...buildFilterMenuItems(name),
      { label: $t("Steam_Games_Refresh"), action: () => { void refreshOwnedGames(); } },
    ];
  }

  onMount(() => {
    setSteamGamesAccountPickHandler((steamId64) => { void pickAccount(steamId64); });
    void loadGames();
    void loadAccounts().catch(() => { accounts = []; accountsLoaded = true; });

    offGamesUpdated = Events.On("steam-owned-games-updated", () => {
      scheduleGamesRefresh();
    });

    offSort = platformListSort.subscribe((sig) => {
      if (!sig || sig.id <= lastHandledSortId) return;
      lastHandledSortId = sig.id;
      sortKind = sig.kind;
    });
  });

  onDestroy(() => {
    offGamesUpdated?.();
    offSort?.();
    if (gamesRefreshTimer) clearTimeout(gamesRefreshTimer);
    if (overlayQueryDebounceTimer) clearTimeout(overlayQueryDebounceTimer);
    clearSteamGamesBar();
    actionBarStatus.set("");
    platformActionBusy.set({ busy: false, platformKey: "" });
  });
</script>

<div class="platformTableHost">
  <SearchOverlay
    open={so.open}
    syncNonce={so.nonce}
    initialQuery={so.initialQuery}
    bind:query={overlayQuery}
    placeholder={$t("Context_Search")}
    primaryRows={searchRows}
    categoryRows={[]}
    categoryHint=""
    gameRows={[]}
    gameHint=""
    on:close={() => closeSearchOverlay()}
    on:pick={onSearchPick}
  />
  <div
    class="steamGames"
    bind:this={listEl}
    use:ctxMenuAction={{ items: backgroundMenu, paginate: false }}
  >
    {#if loadError}
      <p class="platform-accounts-hint">{loadError}</p>
    {/if}
    {#if accountsLoaded && !hasVaultAccounts}
      <p class="platform-accounts-hint steamGames__intro">{$t("Steam_Games_NoVaultAccounts")}</p>
    {/if}
    {#if sortedGames.length === 0}
      {#if !gamesLoading && !loadError && accountsLoaded && hasVaultAccounts}
        <p class="platform-accounts-hint">{$t("Steam_Games_Empty")}</p>
      {/if}
    {:else}
      <ul class="steamGames__list" aria-label={$t("Steam_Tab_Games")}>
        {#each sortedGames as game (game.appId)}
          {@const owners = splitGameOwners(game.owners, MAX_OWNER_AVATARS)}
          <li class="steamGames__row">
            <button
              type="button"
              class="steamGames__rowBtn"
              data-app-id={game.appId}
              aria-pressed={game.appId === selectedAppId}
              on:click={() => selectGame(game.appId)}
            >
              <img class="steamGames__icon" src={gameIcon(game)} alt="" draggable="false" />
              <span class="steamGames__name">{game.name}</span>
              <span class="steamGames__owners">
                {#if game.owners.length === 0}
                  <span
                    class="steamGames__ownerUnknown"
                    use:tooltipAction={{ text: $t("Steam_Games_OwnerUnknown"), boundary: listEl }}
                  >?</span>
                {:else}
                  {#each owners.shown as steamId64 (steamId64)}
                    <img
                      class="steamGames__ownerAvatar"
                      src={ownerAvatar(steamId64, $offlineMode)}
                      alt=""
                      draggable="false"
                      use:tooltipAction={{ text: ownerLabel(steamId64), boundary: listEl }}
                    />
                  {/each}
                  {#if owners.overflow > 0}
                    <span
                      class="steamGames__ownerOverflow"
                      use:tooltipAction={{ text: ownersTooltip(game.owners), boundary: listEl }}
                    >+{owners.overflow}</span>
                  {/if}
                {/if}
              </span>
              <span class="sr-only">
                {game.owners.length === 0
                  ? $t("Steam_Games_OwnerUnknown")
                  : $t("Steam_Games_OwnedBy", { names: game.owners.map(ownerLabel).join(", ") })}
              </span>
            </button>
          </li>
        {/each}
      </ul>
    {/if}
  </div>
</div>

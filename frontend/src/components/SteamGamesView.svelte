<script lang="ts">
  import { onDestroy, onMount, tick } from "svelte";
  import { get } from "svelte/store";
  import { Events } from "@wailsio/runtime";
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
  import { closeSearchOverlay } from "../stores/searchOverlay";
  import { setSteamGamesSearchFocusHandler } from "../stores/steamGamesSearch";
  import { platformListSort, type PlatformSortKind } from "../stores/platformListSort";
  import { offlineMode, offlineSafeImageSrc } from "../stores/offlineMode";
  import {
    clearSteamGamesBar,
    setSteamGamesAccountPickHandler,
    steamGamesBar,
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
    chunkOwnedGames,
    filterOwnedGames,
    gameOwnerAccounts,
    ownerDisplayName,
    ownersTooltipText,
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
  /** Rows per `content-visibility` block — see `chunkOwnedGames`. */
  const ROWS_PER_CHUNK = 25;
  const SEARCH_INPUT_ID = "steamGames_search";
  /**
   * Filtering itself is trivial, but a query that changes the row count by a thousand
   * makes the browser build that many rows: at 2000 games the icon and owner-avatar
   * `img` elements alone cost ~125ms, and the whole re-render ~250ms. Well past a
   * frame, so the list settles once a burst of typing stops. The field is never
   * debounced — only what the list derives from it.
   */
  const FILTER_DEBOUNCE_MS = 120;

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
  // Most-owned first is the point of this tab. Local to the games list — the account
  // switcher keeps its own default and only shares the `platformListSort` signal.
  let sortKind: PlatformSortKind = "owned_count_desc";
  let listEl: HTMLDivElement | undefined;
  let searchEl: HTMLInputElement | undefined;
  let query = "";
  let appliedQuery = "";
  let filterTimer: ReturnType<typeof setTimeout> | null = null;
  let gamesRefreshTimer: ReturnType<typeof setTimeout> | null = null;
  let offGamesUpdated: (() => void) | undefined;
  let offSort: (() => void) | undefined;
  let lastHandledSortId = 0;
  let busy = false;

  $: accountById = new Map(accounts.map((a) => [a.steamId64, a]));
  $: hasVaultAccounts = accounts.some((a) => a.inVault);
  $: sortedGames = sortOwnedGames(games, sortKind);
  $: scheduleFilter(query);
  $: visibleGames = filterOwnedGames(sortedGames, appliedQuery);
  $: gameChunks = chunkOwnedGames(visibleGames, ROWS_PER_CHUNK);
  $: selectedGame = sortedGames.find((g) => g.appId === selectedAppId) ?? null;

  $: barTiles = gameOwnerAccounts(selectedGame, accountById).map(
    ({ steamId64, displayName, avatarUrl }): SteamGamesBarAccount => ({
      steamId64,
      displayName,
      avatarUrl,
    }),
  );
  $: steamGamesBar.set({
    accounts: barTiles,
    reason: barTiles.length > 0 ? "" : selectedGame ? "owners-unknown" : "no-game",
  });

  // These take the account map rather than closing over it so that Svelte tracks it
  // as a dependency of every markup expression that calls them. Accounts resolve
  // after the games list does, and a call that only mentioned `steamId64` left the
  // row showing the raw id — the action or interpolation never re-ran.
  function ownerAvatar(
    byId: Map<string, GamesAccount>,
    steamId64: string,
    offline: boolean,
  ): string {
    return offlineSafeImageSrc(
      offline,
      byId.get(steamId64)?.avatarUrl ?? "",
      PROFILE_PLACEHOLDER,
    );
  }

  function gameIcon(game: OwnedGameRow): string {
    return offlineSafeImageSrc($offlineMode, game.iconUrl, GAME_ICON_FALLBACK);
  }

  // offlineSafeImageSrc only covers an empty or remote URL. A cached icon can
  // also be swept off disk between the list being built and the row painting,
  // and a 404 would otherwise leave the webview's broken-image glyph in place.
  function onGameIconError(event: Event): void {
    const img = event.currentTarget as HTMLImageElement;
    if (img.getAttribute("src") === GAME_ICON_FALLBACK) return;
    img.src = GAME_ICON_FALLBACK;
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

  /**
   * Type-anywhere, routed here by App rather than to the search overlay. The caller
   * has already swallowed the keystroke, so it is appended by hand; appending before
   * the focus lands is what keeps the first character from being dropped when the
   * user types faster than focus moves.
   */
  async function focusSearch(append: string): Promise<void> {
    query += append;
    await tick();
    searchEl?.focus();
    const end = searchEl?.value.length ?? 0;
    searchEl?.setSelectionRange(end, end);
  }

  function scheduleFilter(q: string): void {
    if (q === appliedQuery) return;
    if (filterTimer) clearTimeout(filterTimer);
    filterTimer = setTimeout(() => {
      filterTimer = null;
      appliedQuery = q;
    }, FILTER_DEBOUNCE_MS);
  }

  function onSearchKeydown(e: KeyboardEvent): void {
    if (e.key !== "Escape" || query === "") return;
    query = "";
    e.preventDefault();
    e.stopPropagation();
  }

  async function openStorePage(appId: string): Promise<void> {
    try {
      await SteamService.OpenGameStorePage(appId);
    } catch (e) {
      pushToast({
        type: "error",
        message: formatToastWithError($t("Steam_Games_OpenStoreFailed"), e),
        duration: 8000,
      });
    }
  }

  /**
   * Right-click on a game row. The action stops propagation, so this shadows
   * backgroundMenu wherever a row is the target - which on a full list is
   * everywhere except a few pixels of scroller padding, and nowhere at all by
   * keyboard. It therefore has to carry the background entries too, or a long
   * library would put Refresh games out of reach.
   */
  function gameMenu(appId: string): () => MenuItemDef[] {
    return () => [
      { label: $t("Steam_Games_OpenStore"), action: () => { void openStorePage(appId); } },
      ...backgroundMenu(),
    ];
  }

  function backgroundMenu(): MenuItemDef[] {
    return [
      ...buildFilterMenuItems(name),
      { label: $t("Steam_Games_Refresh"), action: () => { void refreshOwnedGames(); } },
    ];
  }

  onMount(() => {
    setSteamGamesAccountPickHandler((steamId64) => { void pickAccount(steamId64); });
    setSteamGamesSearchFocusHandler((append) => { void focusSearch(append); });
    // The overlay belongs to the switcher tab and is not mounted here. Toggling tabs
    // while it was open would otherwise leave the store open with nothing rendering
    // it, and App would keep routing keystrokes into it.
    closeSearchOverlay();
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
    if (filterTimer) clearTimeout(filterTimer);
    setSteamGamesSearchFocusHandler(null);
    clearSteamGamesBar();
    actionBarStatus.set("");
    platformActionBusy.set({ busy: false, platformKey: "" });
  });
</script>

<div class="main-content platform-accounts-root">
<div class="platformTableHost steamGames__host">
  <div class="steamGames__search">
    <label class="sr-only" for={SEARCH_INPUT_ID}>{$t("Steam_Games_SearchLabel")}</label>
    <input
      id={SEARCH_INPUT_ID}
      bind:this={searchEl}
      bind:value={query}
      type="search"
      class="steamGames__searchInput"
      placeholder={$t("Steam_Games_SearchLabel")}
      spellcheck="false"
      autocomplete="off"
      on:keydown={onSearchKeydown}
    />
    <button
      type="button"
      class="steamGames__searchBtn"
      aria-label={$t("Filter_Search")}
      on:click={() => searchEl?.focus()}
    >
      <svg viewBox="0 0 24 24" width="18" height="18" fill="currentColor" aria-hidden="true">
        <path d="M15.5 14h-.79l-.28-.27a6.47 6.47 0 0 0 1.57-4.23A6.5 6.5 0 1 0 9.5 16c1.61 0 3.09-.59 4.23-1.57l.27.28v.79l5 4.99L20.49 19l-4.99-5Zm-6 0C7.01 14 5 11.99 5 9.5S7.01 5 9.5 5 14 7.01 14 9.5 11.99 14 9.5 14Z" />
      </svg>
    </button>
  </div>
  <div class="platformTable steamGames__table">
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
    {#if visibleGames.length === 0}
      {#if sortedGames.length > 0}
        <p class="platform-accounts-hint">{$t("Steam_Games_SearchNoMatch")}</p>
      {:else if !gamesLoading && !loadError && accountsLoaded && hasVaultAccounts}
        <p class="platform-accounts-hint">{$t("Steam_Games_Empty")}</p>
      {/if}
    {:else}
      <ul class="steamGames__list" aria-label={$t("Steam_Tab_Games")}>
        <!-- The blocks carry the containment; they are not list items themselves, so
             the rows inside them keep the listitem role the `li` used to provide. -->
        {#each gameChunks as chunk (chunk.index)}
          <li class="steamGames__chunk" role="presentation" style="--steamGames-chunk-rows: {chunk.rows.length}">
            {#each chunk.rows as game (game.appId)}
              {@const owners = splitGameOwners(game.owners, MAX_OWNER_AVATARS)}
              <div
                class="steamGames__row"
                role="listitem"
                use:ctxMenuAction={{
                  items: gameMenu(game.appId),
                  beforeOpen: () => selectGame(game.appId),
                }}
              >
                <button
                  type="button"
                  class="steamGames__rowBtn"
                  data-app-id={game.appId}
                  aria-pressed={game.appId === selectedAppId}
                  on:click={() => selectGame(game.appId)}
                >
                  <img class="steamGames__icon" src={gameIcon(game)} alt="" draggable="false" on:error={onGameIconError} />
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
                          src={ownerAvatar(accountById, steamId64, $offlineMode)}
                          alt=""
                          draggable="false"
                          use:tooltipAction={{ text: ownerDisplayName(accountById, steamId64), boundary: listEl }}
                        />
                      {/each}
                      {#if owners.hidden.length > 0}
                        <!-- Only the owners with no avatar in this row: the badge counts
                             them, so naming the visible ones again would not say who "+N" is. -->
                        <span
                          class="steamGames__ownerOverflow"
                          use:tooltipAction={{ text: ownersTooltipText(accountById, owners.hidden, $t("Steam_Games_OwnerUnknown"), "\n"), boundary: listEl }}
                        >+{owners.hidden.length}</span>
                      {/if}
                    {/if}
                  </span>
                  <!-- Every owner, deliberately: this is the whole row read aloud, and the
                       avatars the "+N" badge counts against were never announced. -->
                  <span class="sr-only">
                    {game.owners.length === 0
                      ? $t("Steam_Games_OwnerUnknown")
                      : $t("Steam_Games_OwnedBy", {
                          names: ownersTooltipText(accountById, game.owners, "", ", "),
                        })}
                  </span>
                </button>
              </div>
            {/each}
          </li>
        {/each}
      </ul>
    {/if}
  </div>
  </div>
</div>
</div>

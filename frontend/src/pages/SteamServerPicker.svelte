<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { get } from "svelte/store";
  import { Events } from "@wailsio/runtime";
  import { previousPage, appBarTitle, navigateBackLikeButton } from "../stores/nav";
  import { t } from "../stores/i18n";
  import { offlineMode } from "../stores/offlineMode";
  import { activeModal, openAlert, openConfirm } from "../stores/modal";
  import { pushToast } from "../stores/toast";
  import { formatToastWithError } from "../lib/formatWailsError";
  import { isNeedsAdminError } from "../lib/adminFlow";
  import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
  import {
    Games,
    IsElevated,
    LoadServers,
    RefreshPings,
    SetGroupBlocked,
    SetManyBlocked,
  } from "../../bindings/TcNo-Acc-Switcher/internal/serverpicker/service.js";
  import type {
    GameDTO,
    ServerGroupDTO,
  } from "../../bindings/TcNo-Acc-Switcher/internal/serverpicker/models";
  import {
    DEFAULT_SORT,
    REGION_LABEL_KEYS,
    ariaSort,
    availableRegions,
    bulkAction,
    filterGroups,
    groupPing,
    isGroupPending,
    isPopPending,
    lossQuality,
    nextSort,
    pingQuality,
    shouldCaptureTypedKey,
    sortGroups,
    type PingMap,
    type Sort,
    type SortKey,
  } from "../lib/steam/serverPicker";
  import "../styles/Settings.scss";
  import "../styles/serverPicker.scss";

  const PING_EVENT = "serverpicker:ping";
  const PING_DONE_EVENT = "serverpicker:ping-done";
  const FLAG_FALLBACK = "/img/flags/xx.svg";

  let games: GameDTO[] = [];
  let gameId = "cs2";
  let groups: ServerGroupDTO[] = [];
  let pings: PingMap = {};
  let region = "";
  let query = "";
  let sort: Sort = DEFAULT_SORT;
  let expanded = new Set<string>();

  let elevated = false;
  let loading = true;
  let pinging = false;
  let busy = false;
  let loadError = "";
  let searchInput: HTMLInputElement | null = null;

  let offPing: (() => void) | null = null;
  let offPingDone: (() => void) | null = null;

  $: appBarTitle.set($t("ServerPicker_Title"));
  $: regionLabels = Object.fromEntries(
    Object.entries(REGION_LABEL_KEYS).map(([id, key]) => [id, $t(key)]),
  );
  $: regions = availableRegions(groups);
  $: visible = sortGroups(filterGroups(groups, { region, query, regionLabels }), pings, sort);
  $: nextBulk = bulkAction(visible);

  const COLUMNS: { key: SortKey; labelKey: string }[] = [
    { key: "server", labelKey: "ServerPicker_Col_Server" },
    { key: "id", labelKey: "ServerPicker_Col_ServerId" },
    { key: "ping", labelKey: "ServerPicker_Col_Ping" },
    { key: "loss", labelKey: "ServerPicker_Col_PacketLoss" },
  ];


  function flagSrc(code: string): string {
    return code ? `/img/flags/${code}.svg` : FLAG_FALLBACK;
  }

  function onFlagError(e: Event): void {
    const img = e.currentTarget as HTMLImageElement;
    if (img.src.endsWith(FLAG_FALLBACK)) return;
    img.src = FLAG_FALLBACK;
  }

  function formatPing(rttMs: number | null): string {
    return rttMs === null ? "—" : `${rttMs} ms`;
  }

  function formatLoss(loss: number | null): string {
    return loss === null ? "—" : `${Math.round(loss)}%`;
  }

  async function load(): Promise<void> {
    loading = true;
    loadError = "";
    try {
      // Ahead of the list so the banner is right even if the fetch fails.
      elevated = await IsElevated();
      const list = await LoadServers(gameId);
      groups = list.groups ?? [];
      elevated = list.elevated;
      pings = {};
      expanded = new Set();
      void refresh();
    } catch (e) {
      groups = [];
      loadError = formatToastWithError($t("ServerPicker_LoadFailed"), e);
    } finally {
      loading = false;
    }
  }

  async function refresh(): Promise<void> {
    if (groups.length === 0) return;
    pinging = true;
    try {
      await RefreshPings(gameId);
    } catch (e) {
      pinging = false;
      pushToast({ type: "error", message: formatToastWithError($t("ServerPicker_PingFailed"), e), duration: 8000 });
    }
  }

  async function onGameChange(): Promise<void> {
    await load();
  }

  // Blocked means Steam cannot reach the group's relays, so matchmaking skips
  // it. The checkbox reads the other way round — ticked is allowed — because
  // that is how the list looks before anyone changes anything.
  async function toggleGroup(group: ServerGroupDTO): Promise<void> {
    if (!elevated || busy) return;
    const nextBlocked = !group.blocked;
    busy = true;
    try {
      await SetGroupBlocked(gameId, group.id, nextBlocked);
      groups = groups.map((g) => (g.id === group.id ? { ...g, blocked: nextBlocked } : g));
    } catch (e) {
      await reportFailure(e);
    } finally {
      busy = false;
    }
  }

  async function applyBulk(): Promise<void> {
    if (!elevated || busy || visible.length === 0) return;
    const blocked = nextBulk === "disable";
    const ids = visible.map((g) => g.id);
    busy = true;
    try {
      await SetManyBlocked(gameId, ids, blocked);
      const touched = new Set(ids);
      groups = groups.map((g) => (touched.has(g.id) ? { ...g, blocked } : g));
    } catch (e) {
      await reportFailure(e);
    } finally {
      busy = false;
    }
  }

  async function reportFailure(e: unknown): Promise<void> {
    if (isNeedsAdminError(e)) {
      elevated = false;
      await offerRestart();
      return;
    }
    pushToast({ type: "error", message: formatToastWithError($t("ServerPicker_ApplyFailed"), e), duration: 8000 });
  }

  async function offerRestart(): Promise<void> {
    const ok = await openConfirm({
      title: $t("Modal_Title_ConfirmAction"),
      body: $t("Prompt_RestartAsAdmin"),
      style: "yesno",
      positiveLabel: $t("Ok"),
      negativeLabel: $t("No"),
    });
    if (!ok) return;
    // Land back here rather than on the Steam account list; the CLI maps this
    // sentinel to the server-picker route.
    await PlatformService.RestartAsAdmin(["--open-page=Steam/ServerPicker"]);
  }

  function toggleExpanded(id: string): void {
    const next = new Set(expanded);
    if (next.has(id)) {
      next.delete(id);
    } else {
      next.add(id);
    }
    expanded = next;
  }

  // Pops the history entry this page pushed, rather than pushing a second one
  // on top of it — otherwise Back from the settings page returns here.
  function onClose(): void {
    navigateBackLikeButton();
  }

  function isTypingTarget(target: EventTarget | null): boolean {
    const el = target as HTMLElement | null;
    if (!el) return false;
    return (
      el.tagName === "INPUT" ||
      el.tagName === "TEXTAREA" ||
      el.tagName === "SELECT" ||
      el.isContentEditable
    );
  }

  function onWindowKeyDown(e: KeyboardEvent): void {
    if (get(activeModal)) return;

    if (e.key === "Escape") {
      if (query) {
        e.preventDefault();
        query = "";
        searchInput?.blur();
        return;
      }
      e.preventDefault();
      onClose();
      return;
    }

    if (isTypingTarget(e.target)) return;
    if (!shouldCaptureTypedKey(e)) return;

    // The keystroke landed on the page, not the field — replay it rather than
    // letting the first character of what someone typed vanish.
    e.preventDefault();
    query += e.key;
    searchInput?.focus();
  }

  onMount(() => {
    previousPage.set({ page: "platform-settings", platformName: "Steam" });

    // A bookmarked hash reaches the page without passing the settings button,
    // so the offline gate has to hold here too.
    if (get(offlineMode)) {
      void openAlert({
        title: $t("ServerPicker_Offline_Title"),
        body: $t("ServerPicker_Offline_Body"),
      }).then(() => onClose());
      return;
    }

    offPing = Events.On(PING_EVENT, (ev) => {
      const data = ev.data as { popId?: string; reachable?: boolean; rttMs?: number; loss?: number };
      if (!data?.popId) return;
      pings = {
        ...pings,
        [data.popId]: {
          reachable: Boolean(data.reachable),
          rttMs: Number(data.rttMs ?? -1),
          loss: Number(data.loss ?? -1),
        },
      };
    });
    offPingDone = Events.On(PING_DONE_EVENT, () => {
      pinging = false;
    });

    void (async () => {
      try {
        games = await Games();
        if (games.length > 0 && !games.some((g) => g.id === gameId)) {
          gameId = games[0].id;
        }
      } catch {
        games = [];
      }
      await load();
    })();
  });

  onDestroy(() => {
    offPing?.();
    offPingDone?.();
  });
</script>

<svelte:window on:keydown={onWindowKeyDown} />

<div class="main-content main-spacing serverpicker-root">
  <h1 class="SettingsHeader">{$t("ServerPicker_Title")}</h1>
  <p class="serverpicker-note">{$t("ServerPicker_Note")}</p>

  {#if !elevated}
    <div class="serverpicker-banner" role="status">
      <span>{$t("ServerPicker_NeedsAdmin")}</span>
      <button type="button" on:click={() => void offerRestart()}>{$t("ServerPicker_RestartAsAdmin")}</button>
    </div>
  {/if}

  <div class="serverpicker-toolbar">
    <select
      class="serverpicker-select"
      aria-label={$t("ServerPicker_Game")}
      bind:value={gameId}
      disabled={loading || busy}
      on:change={() => void onGameChange()}
    >
      {#each games as g (g.id)}
        <option value={g.id}>{g.name}</option>
      {/each}
    </select>

    <select class="serverpicker-select" aria-label={$t("ServerPicker_Region")} bind:value={region}>
      <option value="">{$t("ServerPicker_Region_All")}</option>
      {#each regions as r (r)}
        <option value={r}>{regionLabels[r]}</option>
      {/each}
    </select>

    <input
      class="serverpicker-search"
      type="search"
      bind:this={searchInput}
      bind:value={query}
      placeholder={$t("ServerPicker_SearchPlaceholder")}
      aria-label={$t("ServerPicker_SearchPlaceholder")}
    />

    <button
      type="button"
      class="serverpicker-refresh"
      class:is-busy={pinging}
      title={$t("ServerPicker_Refresh")}
      aria-label={$t("ServerPicker_Refresh")}
      disabled={loading || groups.length === 0}
      on:click={() => void refresh()}
    >
      <svg viewBox="0 0 512 512" aria-hidden="true" width="16" height="16">
        <path
          fill="currentColor"
          d="M463.5 224H472c13.3 0 24-10.7 24-24V72c0-9.7-5.8-18.5-14.8-22.2s-19.3-1.7-26.2 5.2L413.4 96.6c-87.6-86.5-228.7-86.2-315.8 1c-87.5 87.5-87.5 229.3 0 316.8s229.3 87.5 316.8 0c12.5-12.5 12.5-32.8 0-45.3s-32.8-12.5-45.3 0c-62.5 62.5-163.8 62.5-226.3 0s-62.5-163.8 0-226.3c62.2-62.2 162.7-62.5 225.3-1l-30.4 30.4c-6.9 6.9-8.9 17.2-5.2 26.2s12.5 14.8 22.2 14.8H463.5z"
        />
      </svg>
    </button>
  </div>

  {#if loadError}
    <div class="serverpicker-error" role="alert">{loadError}</div>
  {/if}

  <div class="serverpicker-table" role="table" aria-label={$t("ServerPicker_Title")}>
    <div class="serverpicker-row serverpicker-row--head" role="row">
      <span role="columnheader">{$t("ServerPicker_Col_Enabled")}</span>
      {#each COLUMNS as col (col.key)}
        <span role="columnheader" aria-sort={ariaSort(sort, col.key)}>
          <button
            type="button"
            class="serverpicker-sort"
            class:is-active={sort.key === col.key}
            on:click={() => (sort = nextSort(sort, col.key))}
          >
            {$t(col.labelKey)}
            <span class="serverpicker-sortmark" aria-hidden="true">
              {sort.key === col.key ? (sort.dir === "asc" ? "▲" : "▼") : ""}
            </span>
          </button>
        </span>
      {/each}
    </div>

    {#if loading}
      <div class="serverpicker-empty" aria-live="polite">{$t("ServerPicker_Loading")}</div>
    {:else if visible.length === 0}
      <div class="serverpicker-empty">{$t("ServerPicker_NoResults")}</div>
    {:else}
      {#each visible as group (group.id)}
        {@const best = groupPing(group, pings)}
        {@const isOpen = expanded.has(group.id)}
        <div class="serverpicker-row" role="row" class:is-blocked={group.blocked}>
          <span role="cell" class="serverpicker-cell--check">
            <div class="form-check">
              <input
                id={"sp-" + group.id}
                type="checkbox"
                checked={!group.blocked}
                disabled={!elevated || busy}
                aria-label={group.label}
                on:change={() => void toggleGroup(group)}
              />
              <label class="form-check-label" for={"sp-" + group.id}></label>
            </div>
          </span>

          <span role="cell" class="serverpicker-cell--name">
            <img class="serverpicker-flag" src={flagSrc(group.country)} alt="" on:error={onFlagError} />
            <span class="serverpicker-name">{group.label}</span>
            {#if group.members.length > 1}
              <button
                type="button"
                class="serverpicker-expander"
                aria-expanded={isOpen}
                aria-label={$t("ServerPicker_ShowMembers", { count: group.members.length })}
                title={$t("ServerPicker_ShowMembers", { count: group.members.length })}
                on:click={() => toggleExpanded(group.id)}
              >
                <span class="serverpicker-count">{group.members.length}</span>
                <span class="serverpicker-chevron" class:is-open={isOpen} aria-hidden="true">▸</span>
              </button>
            {/if}
          </span>

          <span role="cell" class="serverpicker-mono">{group.members.map((m) => m.id).join(", ")}</span>
          {#if isGroupPending(group, pings, pinging)}
            <span role="cell" class="serverpicker-mono"><span class="serverpicker-spinner" aria-label={$t("ServerPicker_Measuring")}></span></span>
            <span role="cell" class="serverpicker-mono"><span class="serverpicker-spinner" aria-hidden="true"></span></span>
          {:else}
            <span role="cell" class="serverpicker-mono q-{pingQuality(best?.rttMs ?? null)}">
              {formatPing(best?.rttMs ?? null)}
            </span>
            <span role="cell" class="serverpicker-mono q-{lossQuality(best?.loss ?? null)}">
              {formatLoss(best?.loss ?? null)}
            </span>
          {/if}
        </div>

        {#if isOpen}
          {#each group.members as member (member.id)}
            {@const p = pings[member.id]}
            <div class="serverpicker-row serverpicker-row--member" role="row">
              <span role="cell"></span>
              <span role="cell" class="serverpicker-cell--name">
                <span class="serverpicker-name">{member.desc}</span>
              </span>
              <span role="cell" class="serverpicker-mono">{member.id}</span>
              {#if isPopPending(member.id, pings, pinging)}
                <span role="cell" class="serverpicker-mono"><span class="serverpicker-spinner" aria-label={$t("ServerPicker_Measuring")}></span></span>
                <span role="cell" class="serverpicker-mono"><span class="serverpicker-spinner" aria-hidden="true"></span></span>
              {:else}
                <span role="cell" class="serverpicker-mono q-{pingQuality(p?.reachable ? p.rttMs : null)}">
                  {formatPing(p?.reachable ? p.rttMs : null)}
                </span>
                <span role="cell" class="serverpicker-mono q-{lossQuality(p?.reachable ? p.loss : null)}">
                  {formatLoss(p?.reachable ? p.loss : null)}
                </span>
              {/if}
            </div>
          {/each}
        {/if}
      {/each}
    {/if}
  </div>

  <div class="buttoncol col_close serverpicker-footer">
    <button
      type="button"
      class="serverpicker-bulk"
      disabled={!elevated || busy || visible.length === 0}
      on:click={() => void applyBulk()}
    >
      <span>{$t(nextBulk === "disable" ? "ServerPicker_DisableAll" : "ServerPicker_EnableAll")}</span>
    </button>
    <button type="button" class="btn_close" on:click={onClose}><span>{$t("Button_Close")}</span></button>
  </div>
</div>

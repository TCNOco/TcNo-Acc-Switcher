<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import { Events } from "@wailsio/runtime";
  import ActionBar from "../components/ActionBar.svelte";
  import ReorderPointerGrid from "../components/ReorderPointerGrid.svelte";
  import { route, previousPage, appBarTitle } from "../stores/nav";
  import { actionBarStatus } from "../stores/actionBarStatus";
  import * as SteamService from "../../bindings/TcNo-Acc-Switcher/internal/steam/steamservice.js";
  import { AccountDTO, AccountPatch } from "../../bindings/TcNo-Acc-Switcher/internal/steam/models.js";
  import { locale, t } from "../stores/i18n";
  import { formatLastLoginForLocale } from "../lib/formatLastLogin";
  import { tooltip } from "../lib/actions/tooltip";

  /** Bindings row + client-only syncError; `currentSession` is explicit — TS sometimes omits quoted keys on generated classes. */
  type SteamAccountRow = InstanceType<typeof AccountDTO> & {
    syncError?: string;
    currentSession: boolean;
  };

  /** Vite `public/img/` → served at `/img/`; shown until Steam profile image is ready */
  const PROFILE_PLACEHOLDER = "/img/BasicDefault.webp";

  export let name: string;

  let steamAccounts: SteamAccountRow[] = [];
  /** Bumped when a row receives a patch so `{#key}` remounts that tile (avoids stuck “Updating…” on last row). */
  let rowEpoch: Record<string, number> = {};
  /** Scroll/content box for Steam tooltips (keeps popovers off the window chrome). */
  let steamAcclistEl: HTMLDivElement | null = null;
  let steamIds: string[] = [];
  let steamLoadError = "";
  let offSteamEvent: (() => void) | undefined;
  /** Selected row for switching (radio group); drives footer status. */
  let selectedSteamId = "";

  $: appBarTitle.set(name || "TcNo Account Switcher");
  $: if (name) {
    route.set({ page: "platform", platformName: name });
  }

  function accountBySteamId(id: string): SteamAccountRow | undefined {
    return steamAccounts.find((a) => a.steamId64 === id);
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

  function applySteamPatch(p: AccountPatch): void {
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
      } as SteamAccountRow;
    });
    if (hit) {
      rowEpoch = { ...rowEpoch, [id]: (rowEpoch[id] ?? 0) + 1 };
    }
    if (id === selectedSteamId) {
      touchSteamActionBar();
    }
  }

  async function loadSteamAccounts(): Promise<void> {
    steamLoadError = "";
    try {
      const rows = (await SteamService.GetSteamAccounts()) as SteamAccountRow[];
      rowEpoch = {};
      steamAccounts = rows;
      steamIds = rows.map((r) => r.steamId64);
      const active = rows.find((r) => r.currentSession);
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
    } catch (e) {
      steamLoadError = String(e);
      steamAccounts = [];
      steamIds = [];
      selectedSteamId = "";
      actionBarStatus.set("");
    }
  }

  function onAccountReorder(e: CustomEvent<{ items: string[] }>): void {
    steamIds = e.detail.items;
    SteamService.SaveSteamAccountOrder(e.detail.items).catch(() => {});
  }

  function slotKey(x: string | null | undefined): string {
    return x ?? "";
  }

  onMount(() => {
    previousPage.set({ page: "home" });
    void loadSteamAccounts();
    offSteamEvent = Events.On("steam-account-updated", (ev) => {
      const raw = ev.data;
      const p =
        raw instanceof AccountPatch
          ? raw
          : AccountPatch.createFrom(raw as Record<string, unknown>);
      applySteamPatch(p);
    });
  });

  onDestroy(() => {
    offSteamEvent?.();
    actionBarStatus.set("");
  });
</script>

<div class="main-content platform-accounts-root">
  {#if name}
    <div class="platformTable">
      {#if steamLoadError}
        <p class="platform-accounts-hint">{steamLoadError}</p>
      {/if}
      <div class="steam-acclist" bind:this={steamAcclistEl}>
        <ReorderPointerGrid
          items={steamIds}
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
                use:tooltip={acc?.currentSession
                  ? {
                      text: $t("Tooltip_CurrentAccount"),
                      placement: "right",
                      boundary: steamAcclistEl,
                    }
                  : undefined}
              >
                <img
                  class:status_vac={acc?.showVac && acc?.vac}
                  class:status_limited={acc?.showLimited && acc?.ltd}
                  src={acc?.imageUrl && !acc?.avatarPending ? acc.imageUrl : PROFILE_PLACEHOLDER}
                  alt=""
                  draggable="false"
                />
                {#if acc?.showAccUsername && acc?.accountName}
                  <p class="streamerCensor">{acc.accountName}</p>
                {/if}
                <h6 class="displayName">{acc?.personaName ?? rid}</h6>
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

</style>

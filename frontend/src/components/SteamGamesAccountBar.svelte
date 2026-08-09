<script lang="ts">
  import { t } from "../stores/i18n";
  import { tooltip } from "../lib/actions/tooltip";
  import { platformActionBusy } from "../stores/platformPage";
  import {
    pickSteamGamesAccount,
    steamGamesBarAccounts,
    steamGamesLaunchOnSwitch,
  } from "../stores/steamGamesBar";
  import "../styles/gameshortcutbar.scss";
  import "../styles/steamGames.scss";

  /**
   * Tiles the action bar shows before the rest fall into the dropdown. The strip
   * shares its row with the filter and settings buttons, so this is the count that
   * still leaves those reachable at the narrowest window the app allows.
   */
  const MAX_PINNED_TILES = 8;

  let ddOpen = false;

  $: pinned = $steamGamesBarAccounts.slice(0, MAX_PINNED_TILES);
  $: overflow = $steamGamesBarAccounts.slice(MAX_PINNED_TILES);
  $: isActionBusy = $platformActionBusy.busy;
  $: if (overflow.length === 0) ddOpen = false;

  function pick(steamId64: string): void {
    if (isActionBusy) return;
    ddOpen = false;
    pickSteamGamesAccount(steamId64);
  }
</script>

<div class="steamGamesBar" role="group" aria-label={$t("Steam_Games_AccountStrip")}>
  <label class="steamGamesBar__launch">
    <input type="checkbox" bind:checked={$steamGamesLaunchOnSwitch} />
    <span use:tooltip={$t("Steam_Games_LaunchOnSwitch_Tooltip")}>{$t("Steam_Games_LaunchOnSwitch")}</span>
  </label>

  <div class="shortcuts shortcutDndGrid" role="list" aria-label={$t("Steam_Games_AccountStrip")}>
    {#each pinned as account (account.steamId64)}
      <div class="shortcutDndCell" role="listitem">
        <button
          type="button"
          aria-label={account.displayName}
          use:tooltip={account.displayName}
          disabled={isActionBusy}
          on:click={() => pick(account.steamId64)}
        >
          <img src={account.avatarUrl} alt="" draggable="false" />
        </button>
      </div>
    {/each}
  </div>

  {#if overflow.length > 0}
    <div class="shortcutDropdownWrap">
      <button
        type="button"
        class="square steamGamesBar__more"
        class:flip={ddOpen}
        aria-expanded={ddOpen}
        aria-label={$t("Steam_Games_MoreAccounts")}
        use:tooltip={$t("Steam_Games_MoreAccounts")}
        on:click={() => (ddOpen = !ddOpen)}
      >
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 448 512" aria-hidden="true"
          ><path
            d="M201.4 137.4c12.5-12.5 32.8-12.5 45.3 0l160 160c12.5 12.5 12.5 32.8 0 45.3s-32.8 12.5-45.3 0L224 218.7 86.6 342.6c-12.5 12.5-32.8 12.5-45.3 0s-12.5-32.8 0-45.3l160-160z"
          /></svg
        >
      </button>

      {#if ddOpen}
        <div class="shortcutDropdown steamGamesBar__dropdown open">
          <div class="shortcutDropdownItems shortcutDndGrid" role="list" aria-label={$t("Steam_Games_MoreAccounts")}>
            {#each overflow as account (account.steamId64)}
              <div class="shortcutDndCell" role="listitem">
                <button
                  type="button"
                  aria-label={account.displayName}
                  use:tooltip={account.displayName}
                  disabled={isActionBusy}
                  on:click={() => pick(account.steamId64)}
                >
                  <img src={account.avatarUrl} alt="" draggable="false" />
                </button>
              </div>
            {/each}
          </div>
        </div>
      {/if}
    </div>
  {/if}
</div>

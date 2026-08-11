<script lang="ts">
  import { t } from "../stores/i18n";
  import { tooltip } from "../lib/actions/tooltip";
  import { platformActionBusy } from "../stores/platformPage";
  import {
    pickSteamGamesAccount,
    steamGamesBar,
    steamGamesLaunchOnSwitch,
    type SteamGamesBarAccount,
  } from "../stores/steamGamesBar";
  import { offlineMode } from "../stores/offlineMode";
  import { avatarSalt, censoredName, streamerMode } from "../stores/streamerMode";
  import { accountAvatarSrc } from "../lib/accountAvatarSrc";
  import "../styles/gameshortcutbar.scss";
  import "../styles/steamGames.scss";

  /**
   * Tiles the strip shows before the rest fall into the dropdown. Beyond four, the
   * 258px `.shortcuts` box wraps to a second row, which both doubles the action bar's
   * height and leaves a gap where the last row is right-aligned against the first.
   */
  const MAX_PINNED_TILES = 4;

  const PROFILE_PLACEHOLDER = "/img/BasicDefault.webp";
  const LAUNCH_CHECKBOX_ID = "steamGamesLaunchOnSwitch";

  let ddOpen = false;

  $: pinned = $steamGamesBar.accounts.slice(0, MAX_PINNED_TILES);
  $: overflow = $steamGamesBar.accounts.slice(MAX_PINNED_TILES);
  $: ownersUnknown = $steamGamesBar.reason === "owners-unknown";
  $: isActionBusy = $platformActionBusy.busy;
  $: if (overflow.length === 0) ddOpen = false;

  // The store carries the raw avatar URL, so the offline guard has to be applied
  // on render: accounts are loaded once and offline mode can be switched on long
  // after, which would otherwise leave these tiles fetching remote images.
  function avatarSrc(account: SteamGamesBarAccount, offline: boolean, streamer: boolean, salt: string): string {
    return accountAvatarSrc({
      streamer,
      salt,
      platformKey: "Steam",
      accountKey: account.steamId64,
      imageUrl: account.avatarUrl,
      pending: false,
      epoch: 0,
      offline,
      fallback: PROFILE_PLACEHOLDER,
    });
  }

  function pick(steamId64: string): void {
    if (isActionBusy) return;
    ddOpen = false;
    pickSteamGamesAccount(steamId64);
  }
</script>

<div class="steamGamesBar" role="group" aria-label={$t("Steam_Games_AccountStrip")}>
  <div class="form-check steamGamesBar__launch">
    <input
      class="form-check-input"
      type="checkbox"
      id={LAUNCH_CHECKBOX_ID}
      bind:checked={$steamGamesLaunchOnSwitch}
    />
    <!-- Empty label: the app's shared checkbox style draws the tickbox on it. -->
    <label class="form-check-label" for={LAUNCH_CHECKBOX_ID}></label>
    <label for={LAUNCH_CHECKBOX_ID} use:tooltip={$t("Steam_Games_LaunchOnSwitch_Tooltip")}
      >{$t("Steam_Games_LaunchOnSwitch")}</label
    >
  </div>

  {#if pinned.length > 0}
    <div
      class="shortcuts shortcutDndGrid"
      class:steamGamesBar__tiles--noDropdown={overflow.length === 0}
      role="list"
      aria-label={$t("Steam_Games_AccountStrip")}
    >
      {#each pinned as account (account.steamId64)}
        <div class="shortcutDndCell" role="listitem">
          <button
            type="button"
            aria-label={$censoredName(account.displayName)}
            use:tooltip={$censoredName(account.displayName)}
            disabled={isActionBusy}
            on:click={() => pick(account.steamId64)}
          >
            <img src={avatarSrc(account, $offlineMode, $streamerMode, $avatarSalt)} alt="" draggable="false" />
          </button>
        </div>
      {/each}
    </div>
  {:else}
    <span
      class="steamGamesBar__hint"
      use:tooltip={ownersUnknown ? $t("Steam_Games_OwnerUnknown") : $t("Steam_Games_PickGameFirst")}
      >{ownersUnknown
        ? $t("Steam_Games_StripOwnersUnknown")
        : $t("Steam_Games_StripPickGame")}</span
    >
  {/if}

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
                  aria-label={$censoredName(account.displayName)}
                  use:tooltip={$censoredName(account.displayName)}
                  disabled={isActionBusy}
                  on:click={() => pick(account.steamId64)}
                >
                  <img src={avatarSrc(account, $offlineMode, $streamerMode, $avatarSalt)} alt="" draggable="false" />
                </button>
              </div>
            {/each}
          </div>
        </div>
      {/if}
    </div>
  {/if}
</div>

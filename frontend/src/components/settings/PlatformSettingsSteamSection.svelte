<script lang="ts">
  import { createEventDispatcher } from "svelte";
  import { get } from "svelte/store";
  import { viewportDropdown } from "../../lib/actions/viewportDropdown";
  import { dropdownDismiss } from "../../lib/actions/dropdownDismiss";
  import { t } from "../../stores/i18n";
  import { route } from "../../stores/nav";
  import { offlineMode } from "../../stores/offlineMode";
  import { openAlert } from "../../stores/modal";
  import {
    closingValues,
    startingValues,
    closingLabel,
    startingLabel,
    overrideStates,
    withLaunchArgFlag,
  } from "../../lib/platformSettingsShared";
  import SettingsGroup from "./SettingsGroup.svelte";
  import SettingsToggle from "./SettingsToggle.svelte";
  import SettingsField from "./SettingsField.svelte";
  import ProcessMethodDropdown from "./ProcessMethodDropdown.svelte";
  import SteamGuardSettingsSection from "./SteamGuardSettingsSection.svelte";
  import type { Settings } from "../../../bindings/TcNo-Acc-Switcher/internal/steam/models";

  export let name: string;
  export let steamSettings: Settings;
  export let hasDesktopShortcut: boolean = false;
  export let silentOn: boolean = false;
  export let oldUiOn: boolean = false;
  export let closingMethodUiLocked: boolean = false;

  const dispatch = createEventDispatcher();
  const ARG_SILENT = "-silent";
  const ARG_VGUI = "-vgui";
  let stateOpen = false;

  function overrideLabel(v: number): string {
    const row = overrideStates.find((x) => x.v === v);
    return row ? $t(row.key) : $t("NoDefault");
  }

  // The picker needs Steam's relay list from the network, so offline mode has
  // nothing to show it. Say so rather than opening an empty page.
  async function openServerPicker(): Promise<void> {
    if (get(offlineMode)) {
      await openAlert({
        title: $t("ServerPicker_Offline_Title"),
        body: $t("ServerPicker_Offline_Body"),
      });
      return;
    }
    route.set({ page: "steam-server-picker" });
  }
</script>


<SettingsGroup title={$t("Settings_Header_GeneralSettings")}>
  <div class="settings-grid">
    <SettingsToggle
      id="ps-desktop-shortcut"
      checked={hasDesktopShortcut}
      label={$t("Settings_Shortcut", { platform: name })}
      on:change={() => dispatch("toggleDesktopShortcut")}
    />
    <SettingsToggle
      id="ps-run-admin"
      checked={steamSettings.RunAsAdmin}
      label={$t("Settings_Admin", { platform: name })}
      on:change={() => {
        steamSettings.RunAsAdmin = !steamSettings.RunAsAdmin;
        dispatch("save");
      }}
    />
    <SettingsToggle
      id="ps-autostart"
      checked={steamSettings.AutoStart}
      label={$t("Settings_AutoStart", { platform: name })}
      on:change={() => {
        steamSettings.AutoStart = !steamSettings.AutoStart;
        dispatch("save");
      }}
    />
    <SettingsToggle
      id="ps-forget"
      checked={steamSettings.ForgetAccountEnabled}
      label={$t("Settings_ForgetAccountEnabled")}
      on:change={() => {
        steamSettings.ForgetAccountEnabled = !steamSettings.ForgetAccountEnabled;
        dispatch("save");
      }}
    />
    <SettingsToggle
      id="ps-shortnotes"
      checked={steamSettings.ShowShortNotes}
      label={$t("Settings_ShowShortNotes")}
      on:change={() => {
        steamSettings.ShowShortNotes = !steamSettings.ShowShortNotes;
        dispatch("save");
      }}
    />
  </div>
  <div class="settings-actions">
    <button type="button" on:click={() => void openServerPicker()}>
      {$t("ServerPicker_Button")}
    </button>
  </div>
</SettingsGroup>

<SettingsGroup title={$t("Settings_Header_AccountDisplay")}>
  <div class="settings-grid">
    <SettingsToggle
      id="ps-show-user"
      checked={steamSettings.Steam_ShowAccUsername}
      label={$t("Steam_ShowAccUsername")}
      on:change={() => {
        steamSettings.Steam_ShowAccUsername = !steamSettings.Steam_ShowAccUsername;
        dispatch("save");
      }}
    />
    <SettingsToggle
      id="ps-show-sid"
      checked={steamSettings.Steam_ShowSteamID}
      label={$t("Steam_ShowSteamID")}
      on:change={() => {
        steamSettings.Steam_ShowSteamID = !steamSettings.Steam_ShowSteamID;
        dispatch("save");
      }}
    />
    <SettingsToggle
      id="ps-show-ll"
      checked={steamSettings.Steam_ShowLastLogin}
      label={$t("Steam_ShowLastLogin")}
      on:change={() => {
        steamSettings.Steam_ShowLastLogin = !steamSettings.Steam_ShowLastLogin;
        dispatch("save");
      }}
    />
    <SettingsToggle
      id="ps-show-vac"
      checked={steamSettings.Steam_ShowVAC}
      label={$t("Steam_ShowVac")}
      on:change={() => {
        steamSettings.Steam_ShowVAC = !steamSettings.Steam_ShowVAC;
        dispatch("save");
      }}
    />
    <SettingsToggle
      id="ps-show-ltd"
      checked={steamSettings.Steam_ShowLimited}
      label={$t("Steam_ShowLimited")}
      on:change={() => {
        steamSettings.Steam_ShowLimited = !steamSettings.Steam_ShowLimited;
        dispatch("save");
      }}
    />
    <SettingsToggle
      id="ps-show-miniprofile"
      checked={steamSettings.Steam_ShowMiniProfile}
      label={$t("Steam_ShowMiniProfile")}
      tooltip={$t("Tooltip_SteamShowMiniProfile")}
      on:change={() => {
        steamSettings.Steam_ShowMiniProfile = !steamSettings.Steam_ShowMiniProfile;
        dispatch("save");
      }}
    />
    <SettingsToggle
      id="ps-show-avatar-frame"
      checked={steamSettings.Steam_ShowAvatarFrame}
      label={$t("Steam_ShowAvatarFrame")}
      tooltip={$t("Tooltip_SteamShowAvatarFrame")}
      on:change={() => {
        steamSettings.Steam_ShowAvatarFrame = !steamSettings.Steam_ShowAvatarFrame;
        dispatch("save");
      }}
    />
    <SettingsToggle
      id="ps-show-steam-guard-lock"
      checked={steamSettings.Steam_ShowSteamGuardLock}
      label={$t("Steam_ShowSteamGuardLock")}
      tooltip={$t("Tooltip_SteamShowSteamGuardLock")}
      on:change={() => {
        steamSettings.Steam_ShowSteamGuardLock = !steamSettings.Steam_ShowSteamGuardLock;
        dispatch("save");
      }}
    />
  </div>

  <!-- Lives with the Limited indicator it changes the meaning of, not with the
       network settings it superficially resembles. -->
  <SettingsField
    label={$t("Settings_SteamAPIKey")}
    forId="ps-api-key"
    note={$t("Settings_SteamAPIKey_Note")}
    wide
  >
    <input
      id="ps-api-key"
      type="text"
      spellcheck="false"
      autocomplete="off"
      bind:value={steamSettings.SteamWebApiKey}
      on:change={() => dispatch("save")}
    />
  </SettingsField>
</SettingsGroup>

<!-- Everything here needs an authenticated Steam session, so it is grouped apart
     from the display toggles that work on any account. -->
<SettingsGroup title={$t("Settings_Header_SteamGuardAuth")}>
  <!-- One row: the three are alternatives of the same kind, and stacked they
       read as three unrelated features. Each is independent - any one being on
       is enough for the account check to run. -->
  <p class="cs2-states-label">{$t("Settings_ShowStates")}</p>
  <div class="settings-grid cs2-states">
    <SettingsToggle
      id="ps-collect-cs2-cooldown"
      checked={steamSettings.Steam_CollectCS2Cooldowns}
      label={$t("Steam_State_Cs2Cooldown")}
      tooltip={$t("Tooltip_SteamShowCs2Cooldown")}
      on:change={() => {
        steamSettings.Steam_CollectCS2Cooldowns = !steamSettings.Steam_CollectCS2Cooldowns;
        dispatch("save");
      }}
    />
    <SettingsToggle
      id="ps-show-cs2-rank"
      checked={steamSettings.Steam_ShowCS2Rank}
      label={$t("Steam_State_Cs2Rank")}
      tooltip={$t("Tooltip_SteamShowCs2Rank")}
      on:change={() => {
        steamSettings.Steam_ShowCS2Rank = !steamSettings.Steam_ShowCS2Rank;
        dispatch("save");
      }}
    />
    <SettingsToggle
      id="ps-show-cs2-prime"
      checked={steamSettings.Steam_ShowCS2PrimeTag}
      label={$t("Steam_State_Cs2Prime")}
      tooltip={$t("Tooltip_SteamShowCs2PrimeTag")}
      on:change={() => {
        steamSettings.Steam_ShowCS2PrimeTag = !steamSettings.Steam_ShowCS2PrimeTag;
        dispatch("save");
      }}
    />
  </div>
</SettingsGroup>

<SettingsGroup title={$t("Settings_Header_LaunchOptions")}>
  <div class="settings-grid">
    <SettingsToggle
      id="ps-silent"
      checked={silentOn}
      disabled={!steamSettings.AutoStart}
      label={$t("Steam_StartSilent")}
      on:change={() => {
        steamSettings.LaunchArguments = withLaunchArgFlag(steamSettings.LaunchArguments ?? "", ARG_SILENT, !silentOn);
        dispatch("save");
      }}
    />
    <SettingsToggle
      id="ps-oldui"
      checked={oldUiOn}
      disabled={!steamSettings.AutoStart}
      label={$t("Steam_OldUi")}
      on:change={() => {
        steamSettings.LaunchArguments = withLaunchArgFlag(steamSettings.LaunchArguments ?? "", ARG_VGUI, !oldUiOn);
        dispatch("save");
      }}
    />
    <SettingsToggle
      id="ps-steam-switcher"
      checked={steamSettings.ShowSteamSwitcher}
      label={$t("Settings_ShowSteamSwitcher")}
      span
      on:change={() => {
        steamSettings.ShowSteamSwitcher = !steamSettings.ShowSteamSwitcher;
        dispatch("save");
      }}
    />
  </div>

  <SettingsField
    label={$t("Settings_LaunchArgumentsForPlatform", { platform: name })}
    forId="ps-launch-args"
    note={$t("Settings_LaunchArguments_Hint")}
    disabled={!steamSettings.AutoStart}
    wide
  >
    <input
      id="ps-launch-args"
      type="text"
      spellcheck="false"
      autocomplete="off"
      disabled={!steamSettings.AutoStart}
      bind:value={steamSettings.LaunchArguments}
      on:input={() => dispatch("save")}
    />
  </SettingsField>

  <SettingsField label={$t("Steam_OverrideDefaultState")}>
    <div
      class="dropdown"
      class:show={stateOpen}
      use:dropdownDismiss={stateOpen}
      on:dismiss={() => (stateOpen = false)}
    >
      <button type="button" class="dropdown-toggle" on:click={() => (stateOpen = !stateOpen)}>
        {overrideLabel(steamSettings.Steam_OverrideState)}
        <span class="caret" aria-hidden="true"></span>
      </button>
      {#if stateOpen}
        <ul class="custom-dropdown-menu dropdown-menu" use:viewportDropdown>
          {#each overrideStates as o}
            <li>
              <button
                type="button"
                class="dropdown-item"
                on:click={() => {
                  steamSettings.Steam_OverrideState = o.v;
                  stateOpen = false;
                  dispatch("save");
                }}
              >
                {$t(o.key)}
              </button>
            </li>
          {/each}
        </ul>
      {/if}
    </div>
  </SettingsField>
</SettingsGroup>

<SteamGuardSettingsSection />

<SettingsGroup title={$t("Settings_Header_TraySettings")}>
  <SettingsField label={$t("Settings_TrayMax")} forId="ps-tray-max">
    <input
      id="ps-tray-max"
      type="number"
      min="0"
      max="365"
      bind:value={steamSettings.TrayAccNumber}
      on:change={() => dispatch("save")}
    />
    <SettingsToggle
      id="ps-tray-name"
      checked={steamSettings.Steam_TrayAccountName}
      label={$t("Steam_Tray_AccountName")}
      on:change={() => {
        steamSettings.Steam_TrayAccountName = !steamSettings.Steam_TrayAccountName;
        dispatch("save");
      }}
    />
  </SettingsField>
</SettingsGroup>

<SettingsGroup title={$t("Settings_Header_ProfileImages")}>
  <div class="settings-grid">
    <SettingsToggle
      id="ps-collect"
      checked={steamSettings.CollectInfo}
      label={$t("Settings_SteamCollectInfo")}
      span
      on:change={() => {
        steamSettings.CollectInfo = !steamSettings.CollectInfo;
        dispatch("save");
      }}
    />
  </div>
  <SettingsField label={$t("Settings_ImageExpiry")} forId="ps-image-expiry">
    <input
      id="ps-image-expiry"
      type="number"
      min="0"
      max="365"
      bind:value={steamSettings.Steam_ImageExpiryTime}
      on:change={() => dispatch("save")}
    />
  </SettingsField>
</SettingsGroup>

<SettingsGroup title={$t("Settings_Header_ProcessManagement")}>
  {#if !closingMethodUiLocked}
    <ProcessMethodDropdown
      values={closingValues}
      current={steamSettings.ClosingMethod}
      label={$t("Settings_Header_ClosingMethod", { platform: name })}
      labelFn={closingLabel}
      tooltip={$t("Tooltip_ClosingMethod")}
      on:select={(e) => {
        steamSettings.ClosingMethod = e.detail.value;
        dispatch("save");
      }}
    />
  {/if}
  <ProcessMethodDropdown
    values={startingValues}
    current={steamSettings.StartingMethod}
    label={$t("Settings_Header_StartingMethod", { platform: name })}
    labelFn={startingLabel}
    tooltip={$t("Tooltip_StartingMethod")}
    on:select={(e) => {
      steamSettings.StartingMethod = e.detail.value;
      dispatch("save");
    }}
  />
</SettingsGroup>

<style lang="scss">
  .dropdown-toggle {
    position: relative;
    height: 38px;
  }

  /* Lines up with the toggles below it, which the grid pads by the same amount. */
  .cs2-states-label {
    margin: 0 0 0.35rem;
    padding-inline: var(--settings-toggle-pad-x, 0.4rem);
    color: var(--whiteSecondary);
  }

  /* Sized to the labels rather than the grid's fixed columns, so the three sit
     together instead of spreading across the panel. */
  .cs2-states {
    display: flex;
    flex-wrap: wrap;
    gap: 0.35rem 1.25rem;
  }
</style>

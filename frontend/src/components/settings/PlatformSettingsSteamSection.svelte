<script lang="ts">
  import { createEventDispatcher } from "svelte";
  import { t } from "../../stores/i18n";
  import {
    closingValues,
    startingValues,
    closingLabel,
    startingLabel,
    overrideStates,
    withLaunchArgFlag,
  } from "../../lib/platformSettingsShared";
  import SharedSettingCheckbox from "./SharedSettingCheckbox.svelte";
  import ProcessMethodDropdown from "./ProcessMethodDropdown.svelte";
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
</script>

<h2 class="SettingsHeader">{$t("Settings_Header_GeneralSettings")}</h2>
<div class="rowSetting">
  <div class="form-check">
    <input
      id="ps-desktop-shortcut"
      type="checkbox"
      checked={hasDesktopShortcut}
      on:change={() => dispatch("toggleDesktopShortcut")}
    />
    <label class="form-check-label" for="ps-desktop-shortcut"></label>
  </div>
  <label for="ps-desktop-shortcut">{$t("Settings_Shortcut", { platform: name })}</label>
</div>
<SharedSettingCheckbox
  id="ps-run-admin"
  checked={steamSettings.RunAsAdmin}
  label={$t("Settings_Admin", { platform: name })}
  on:change={() => {
    steamSettings.RunAsAdmin = !steamSettings.RunAsAdmin;
    dispatch("save");
  }}
/>
<SharedSettingCheckbox
  id="ps-autostart"
  checked={steamSettings.AutoStart}
  label={$t("Settings_AutoStart", { platform: name })}
  on:change={() => {
    steamSettings.AutoStart = !steamSettings.AutoStart;
    dispatch("save");
  }}
/>
<SharedSettingCheckbox
  id="ps-forget"
  checked={steamSettings.ForgetAccountEnabled}
  label={$t("Settings_ForgetAccountEnabled")}
  on:change={() => {
    steamSettings.ForgetAccountEnabled = !steamSettings.ForgetAccountEnabled;
    dispatch("save");
  }}
/>
<SharedSettingCheckbox
  id="ps-shortnotes"
  checked={steamSettings.ShowShortNotes}
  label={$t("Settings_ShowShortNotes")}
  on:change={() => {
    steamSettings.ShowShortNotes = !steamSettings.ShowShortNotes;
    dispatch("save");
  }}
/>

<h2 class="SettingsHeader">{$t("Settings_Header_AccountDisplay")}</h2>
<SharedSettingCheckbox
  id="ps-show-user"
  checked={steamSettings.Steam_ShowAccUsername}
  label={$t("Steam_ShowAccUsername")}
  on:change={() => {
    steamSettings.Steam_ShowAccUsername = !steamSettings.Steam_ShowAccUsername;
    dispatch("save");
  }}
/>
<SharedSettingCheckbox
  id="ps-show-sid"
  checked={steamSettings.Steam_ShowSteamID}
  label={$t("Steam_ShowSteamID")}
  on:change={() => {
    steamSettings.Steam_ShowSteamID = !steamSettings.Steam_ShowSteamID;
    dispatch("save");
  }}
/>
<SharedSettingCheckbox
  id="ps-show-ll"
  checked={steamSettings.Steam_ShowLastLogin}
  label={$t("Steam_ShowLastLogin")}
  on:change={() => {
    steamSettings.Steam_ShowLastLogin = !steamSettings.Steam_ShowLastLogin;
    dispatch("save");
  }}
/>
<SharedSettingCheckbox
  id="ps-show-vac"
  checked={steamSettings.Steam_ShowVAC}
  label={$t("Steam_ShowVac")}
  on:change={() => {
    steamSettings.Steam_ShowVAC = !steamSettings.Steam_ShowVAC;
    dispatch("save");
  }}
/>
<SharedSettingCheckbox
  id="ps-show-ltd"
  checked={steamSettings.Steam_ShowLimited}
  label={$t("Steam_ShowLimited")}
  on:change={() => {
    steamSettings.Steam_ShowLimited = !steamSettings.Steam_ShowLimited;
    dispatch("save");
  }}
/>
<SharedSettingCheckbox
  id="ps-show-miniprofile"
  checked={steamSettings.Steam_ShowMiniProfile}
  label={$t("Steam_ShowMiniProfile")}
  tooltip={$t("Tooltip_SteamShowMiniProfile")}
  on:change={() => {
    steamSettings.Steam_ShowMiniProfile = !steamSettings.Steam_ShowMiniProfile;
    dispatch("save");
  }}
/>
<SharedSettingCheckbox
  id="ps-show-avatar-frame"
  checked={steamSettings.Steam_ShowAvatarFrame}
  label={$t("Steam_ShowAvatarFrame")}
  tooltip={$t("Tooltip_SteamShowAvatarFrame")}
  on:change={() => {
    steamSettings.Steam_ShowAvatarFrame = !steamSettings.Steam_ShowAvatarFrame;
    dispatch("save");
  }}
/>

<h2 class="SettingsHeader">{$t("Settings_Header_TraySettings")}</h2>
<SharedSettingCheckbox
  id="ps-tray-name"
  checked={steamSettings.Steam_TrayAccountName}
  label={$t("Steam_Tray_AccountName")}
  on:change={() => {
    steamSettings.Steam_TrayAccountName = !steamSettings.Steam_TrayAccountName;
    dispatch("save");
  }}
/>
<div class="form-text tray-max-row">
  <span>{$t("Settings_TrayMax")}</span>
  <input
    type="number"
    min="0"
    max="365"
    bind:value={steamSettings.TrayAccNumber}
    on:change={() => dispatch("save")}
  />
</div>

<h2 class="SettingsHeader">{$t("Settings_Header_LaunchOptions")}</h2>
<div class="rowSetting">
  <div class="form-check">
    <input
      id="ps-silent"
      type="checkbox"
      disabled={!steamSettings.AutoStart}
      checked={silentOn}
      on:change={() => {
        steamSettings.LaunchArguments = withLaunchArgFlag(steamSettings.LaunchArguments ?? "", ARG_SILENT, !silentOn);
        dispatch("save");
      }}
    />
    <label class="form-check-label" for="ps-silent"></label>
  </div>
  <label for="ps-silent">{$t("Steam_StartSilent")}</label>
</div>
<SharedSettingCheckbox
  id="ps-steam-switcher"
  checked={steamSettings.ShowSteamSwitcher}
  label={$t("Settings_ShowSteamSwitcher")}
  on:change={() => {
    steamSettings.ShowSteamSwitcher = !steamSettings.ShowSteamSwitcher;
    dispatch("save");
  }}
/>
<SharedSettingCheckbox
  id="ps-collect"
  checked={steamSettings.CollectInfo}
  label={$t("Settings_SteamCollectInfo")}
  on:change={() => {
    steamSettings.CollectInfo = !steamSettings.CollectInfo;
    dispatch("save");
  }}
/>
<div class="rowSetting">
  <div class="form-check">
    <input
      id="ps-oldui"
      type="checkbox"
      disabled={!steamSettings.AutoStart}
      checked={oldUiOn}
      on:change={() => {
        steamSettings.LaunchArguments = withLaunchArgFlag(steamSettings.LaunchArguments ?? "", ARG_VGUI, !oldUiOn);
        dispatch("save");
      }}
    />
    <label class="form-check-label" for="ps-oldui"></label>
  </div>
  <label for="ps-oldui">{$t("Steam_OldUi")}</label>
</div>
<div class="rowSetting form-text launch-args-row">
  <label for="ps-launch-args">{$t("Settings_LaunchArgumentsForPlatform", { platform: name })}</label>
  <input
    id="ps-launch-args"
    type="text"
    spellcheck="false"
    autocomplete="off"
    disabled={!steamSettings.AutoStart}
    bind:value={steamSettings.LaunchArguments}
    on:input={() => dispatch("save")}
  />
  <p class="subtext">{$t("Settings_LaunchArguments_Hint")}</p>
</div>
<div class="rowSetting rowDropdown">
  <span>{$t("Steam_OverrideDefaultState")}</span>
  <div class="dropdown" class:show={stateOpen}>
    <button type="button" class="dropdown-toggle" on:click={() => (stateOpen = !stateOpen)}>
      {overrideLabel(steamSettings.Steam_OverrideState)}
      <span class="caret" aria-hidden="true"></span>
    </button>
    {#if stateOpen}
      <ul class="custom-dropdown-menu dropdown-menu">
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
</div>
<div class="form-text">
  <span>{$t("Settings_ImageExpiry")}</span>
  <input
    type="number"
    min="0"
    max="365"
    bind:value={steamSettings.Steam_ImageExpiryTime}
    on:change={() => dispatch("save")}
  />
</div>
<div class="form-text">
  <span>{$t("Settings_SteamAPIKey")}</span>
  <input
    type="text"
    spellcheck="false"
    bind:value={steamSettings.SteamWebApiKey}
    on:change={() => dispatch("save")}
  />
  <p class="subtext">{$t("Settings_SteamAPIKey_Note")}</p>
</div>
<h2 class="SettingsHeader">{$t("Settings_Header_ProcessManagement")}</h2>
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

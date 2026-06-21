<script lang="ts">
  import { createEventDispatcher } from "svelte";
  import { t } from "../../stores/i18n";
  import {
    closingValues,
    startingValues,
    closingLabel,
    startingLabel,
  } from "../../lib/platformSettingsShared";
  import SharedSettingCheckbox from "./SharedSettingCheckbox.svelte";
  import ProcessMethodDropdown from "./ProcessMethodDropdown.svelte";
  import type { PlatformSettings } from "../../../bindings/TcNo-Acc-Switcher/internal/platform/models";

  export let name: string;
  export let genericPS: PlatformSettings;
  export let hasDesktopShortcut: boolean = false;
  export let closingMethodUiLocked: boolean = false;
  export let hasRemoteProfileImages: boolean = false;

  const dispatch = createEventDispatcher();

  function pullAccountImagesOnSwitch(): boolean {
    const g = genericPS as unknown as Record<string, unknown>;
    return g.PullAccountImagesOnSwitch !== false;
  }

  function handlePullAccountImagesChange(): void {
    const g = genericPS as unknown as Record<string, unknown>;
    g.PullAccountImagesOnSwitch = !pullAccountImagesOnSwitch();
    dispatch("save");
  }
</script>

<h2 class="SettingsHeader">{$t("Settings_Header_GeneralSettings")}</h2>
<div class="rowSetting">
  <div class="form-check">
    <input
      id="gp-desktop-shortcut"
      type="checkbox"
      checked={hasDesktopShortcut}
      on:change={() => dispatch("toggleDesktopShortcut")}
    />
    <label class="form-check-label" for="gp-desktop-shortcut"></label>
  </div>
  <label for="gp-desktop-shortcut">{$t("Settings_Shortcut", { platform: name })}</label>
</div>
<SharedSettingCheckbox
  id="gp-run-admin"
  checked={genericPS.RunAsAdmin}
  label={$t("Settings_Admin", { platform: name })}
  on:change={() => {
    genericPS.RunAsAdmin = !genericPS.RunAsAdmin;
    dispatch("save");
  }}
/>
<SharedSettingCheckbox
  id="gp-autostart"
  checked={genericPS.AutoStart}
  label={$t("Settings_AutoStart", { platform: name })}
  on:change={() => {
    genericPS.AutoStart = !genericPS.AutoStart;
    dispatch("save");
  }}
/>
<SharedSettingCheckbox
  id="gp-forget"
  checked={genericPS.ForgetAccountEnabled}
  label={$t("Settings_ForgetAccountEnabled")}
  on:change={() => {
    genericPS.ForgetAccountEnabled = !genericPS.ForgetAccountEnabled;
    dispatch("save");
  }}
/>
<SharedSettingCheckbox
  id="gp-shortnotes"
  checked={genericPS.ShowShortNotes}
  label={$t("Settings_ShowShortNotes")}
  on:change={() => {
    genericPS.ShowShortNotes = !genericPS.ShowShortNotes;
    dispatch("save");
  }}
/>
<SharedSettingCheckbox
  id="gp-show-lastused"
  checked={genericPS.ShowLastUsed}
  label={$t("Settings_ShowLastUsed")}
  on:change={() => {
    genericPS.ShowLastUsed = !genericPS.ShowLastUsed;
    dispatch("save");
  }}
/>
<h2 class="SettingsHeader">{$t("Settings_Header_LaunchOptions")}</h2>
<div class="rowSetting form-text launch-args-row">
  <label for="gp-launch-args">{$t("Settings_LaunchArgumentsForPlatform", { platform: name })}</label>
  <input
    id="gp-launch-args"
    type="text"
    spellcheck="false"
    autocomplete="off"
    disabled={!genericPS.AutoStart}
    bind:value={genericPS.LaunchArguments}
    on:input={() => dispatch("save")}
  />
  <p class="subtext">{$t("Settings_LaunchArguments_Hint")}</p>
</div>
<h2 class="SettingsHeader">{$t("Settings_Header_ProcessManagement")}</h2>
{#if !closingMethodUiLocked}
  <ProcessMethodDropdown
    values={closingValues}
    current={genericPS.ClosingMethod}
    label={$t("Settings_Header_ClosingMethod", { platform: name })}
    labelFn={closingLabel}
    tooltip={$t("Tooltip_ClosingMethod")}
    on:select={(e) => {
      genericPS.ClosingMethod = e.detail.value;
      dispatch("save");
    }}
  />
{/if}
<ProcessMethodDropdown
  values={startingValues}
  current={genericPS.StartingMethod}
  label={$t("Settings_Header_StartingMethod", { platform: name })}
  labelFn={startingLabel}
  tooltip={$t("Tooltip_StartingMethod")}
  on:select={(e) => {
    genericPS.StartingMethod = e.detail.value;
    dispatch("save");
  }}
/>

<h2 class="SettingsHeader">{$t("Settings_Header_TraySettings")}</h2>
<div class="form-text tray-max-row">
  <span>{$t("Settings_TrayMax")}</span>
  <input
    type="number"
    min="0"
    max="365"
    bind:value={genericPS.TrayAccNumber}
    on:change={() => dispatch("save")}
  />
</div>
{#if hasRemoteProfileImages}
  <h2 class="SettingsHeader">{$t("Settings_Header_ProfileImages")}</h2>
  <div class="rowSetting">
    <div class="form-check">
      <input
        id="gp-pull-account-images"
        type="checkbox"
        checked={pullAccountImagesOnSwitch()}
        on:change={handlePullAccountImagesChange}
      />
      <label class="form-check-label" for="gp-pull-account-images"></label>
    </div>
    <label for="gp-pull-account-images">{$t("Settings_PullAccountImages")}</label>
  </div>
  <div class="form-text tray-max-row">
    <span>{$t("Settings_ProfileImageExpiryDays")}</span>
    <input
      type="number"
      min="1"
      max="365"
      bind:value={genericPS.ProfileImageExpiryDays}
      on:change={() => dispatch("save")}
    />
  </div>
  <div class="buttoncol">
    <button type="button" on:click={() => dispatch("refreshBasicProfileImages")}>
      {$t("Button_RefreshImages")}
    </button>
    <button type="button" on:click={() => dispatch("clearBasicProfileImages")}>
      {$t("Button_ClearCachedProfileImages")}
    </button>
  </div>
{/if}

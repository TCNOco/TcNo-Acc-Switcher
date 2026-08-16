<script lang="ts">
  import { createEventDispatcher } from "svelte";
  import { t } from "../../stores/i18n";
  import {
    closingValues,
    startingValues,
    closingLabel,
    startingLabel,
  } from "../../lib/platformSettingsShared";
  import SettingsGroup from "./SettingsGroup.svelte";
  import SettingsToggle from "./SettingsToggle.svelte";
  import SettingsField from "./SettingsField.svelte";
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


<SettingsGroup title={$t("Settings_Header_GeneralSettings")}>
  <div class="settings-grid">
    <SettingsToggle
      id="gp-desktop-shortcut"
      checked={hasDesktopShortcut}
      label={$t("Settings_Shortcut", { platform: name })}
      on:change={() => dispatch("toggleDesktopShortcut")}
    />
    <SettingsToggle
      id="gp-run-admin"
      checked={genericPS.RunAsAdmin}
      label={$t("Settings_Admin", { platform: name })}
      on:change={() => {
        genericPS.RunAsAdmin = !genericPS.RunAsAdmin;
        dispatch("save");
      }}
    />
    <SettingsToggle
      id="gp-autostart"
      checked={genericPS.AutoStart}
      label={$t("Settings_AutoStart", { platform: name })}
      on:change={() => {
        genericPS.AutoStart = !genericPS.AutoStart;
        dispatch("save");
      }}
    />
    <SettingsToggle
      id="gp-forget"
      checked={genericPS.ForgetAccountEnabled}
      label={$t("Settings_ForgetAccountEnabled")}
      on:change={() => {
        genericPS.ForgetAccountEnabled = !genericPS.ForgetAccountEnabled;
        dispatch("save");
      }}
    />
    <SettingsToggle
      id="gp-shortnotes"
      checked={genericPS.ShowShortNotes}
      label={$t("Settings_ShowShortNotes")}
      on:change={() => {
        genericPS.ShowShortNotes = !genericPS.ShowShortNotes;
        dispatch("save");
      }}
    />
    <SettingsToggle
      id="gp-show-lastused"
      checked={genericPS.ShowLastUsed}
      label={$t("Settings_ShowLastUsed")}
      on:change={() => {
        genericPS.ShowLastUsed = !genericPS.ShowLastUsed;
        dispatch("save");
      }}
    />
  </div>
</SettingsGroup>

<SettingsGroup title={$t("Settings_Header_LaunchOptions")}>
  <SettingsField
    label={$t("Settings_LaunchArgumentsForPlatform", { platform: name })}
    forId="gp-launch-args"
    note={$t("Settings_LaunchArguments_Hint")}
    disabled={!genericPS.AutoStart}
    wide
  >
    <input
      id="gp-launch-args"
      type="text"
      spellcheck="false"
      autocomplete="off"
      disabled={!genericPS.AutoStart}
      bind:value={genericPS.LaunchArguments}
      on:input={() => dispatch("save")}
    />
  </SettingsField>
</SettingsGroup>

<SettingsGroup title={$t("Settings_Header_TraySettings")}>
  <SettingsField label={$t("Settings_TrayMax")} forId="gp-tray-max">
    <input
      id="gp-tray-max"
      type="number"
      min="0"
      max="365"
      bind:value={genericPS.TrayAccNumber}
      on:change={() => dispatch("save")}
    />
  </SettingsField>
</SettingsGroup>
{#if hasRemoteProfileImages}
<SettingsGroup title={$t("Settings_Header_ProfileImages")}>
  <div class="settings-grid">
    <SettingsToggle
      id="gp-pull-account-images"
      checked={pullAccountImagesOnSwitch()}
      label={$t("Settings_PullAccountImages")}
      on:change={handlePullAccountImagesChange}
    />
  </div>
  <SettingsField label={$t("Settings_ProfileImageExpiryDays")} forId="gp-image-expiry">
    <input
      id="gp-image-expiry"
      type="number"
      min="1"
      max="365"
      bind:value={genericPS.ProfileImageExpiryDays}
      on:change={() => dispatch("save")}
    />
  </SettingsField>
  <div class="settings-actions">
    <button type="button" class="btnicontext" on:click={() => dispatch("refreshBasicProfileImages")}>
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" aria-hidden="true"><path d="M463.5 224H472c13.3 0 24-10.7 24-24V72c0-9.7-5.8-18.5-14.8-22.2s-19.3-1.7-26.2 5.2L413.4 96.6c-87.6-86.5-228.7-86.2-315.8 1c-87.5 87.5-87.5 229.3 0 316.8s229.3 87.5 316.8 0c12.5-12.5 12.5-32.8 0-45.3s-32.8-12.5-45.3 0c-62.5 62.5-163.8 62.5-226.3 0s-62.5-163.8 0-226.3c62.2-62.2 162.7-62.5 225.3-1l-30.4 30.4c-6.9 6.9-8.9 17.2-5.2 26.2s12.5 14.8 22.2 14.8H463.5z"/></svg>
      {$t("Button_RefreshImages")}
    </button>
    <button type="button" class="btnicontext" on:click={() => dispatch("clearBasicProfileImages")}>
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path fill-rule="evenodd" d="M9.5 2h5a1.5 1.5 0 0 1 1.5 1.5V4h4.5a1.2 1.2 0 0 1 1.2 1.2v1.6A1.2 1.2 0 0 1 20.5 8h-17A1.2 1.2 0 0 1 2.3 6.8V5.2A1.2 1.2 0 0 1 3.5 4H8v-.5A1.5 1.5 0 0 1 9.5 2ZM4.9 9.4H19.1L18.1 20.2A2 2 0 0 1 16.11 22H7.89A2 2 0 0 1 5.9 20.2L4.9 9.4ZM9.3 12.2h1.9v7.2H9.3zM12.8 12.2h1.9v7.2h-1.9z"/></svg>
      {$t("Button_ClearCachedProfileImages")}
    </button>
  </div>
</SettingsGroup>
{/if}

<SettingsGroup title={$t("Settings_Header_ProcessManagement")}>
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
</SettingsGroup>

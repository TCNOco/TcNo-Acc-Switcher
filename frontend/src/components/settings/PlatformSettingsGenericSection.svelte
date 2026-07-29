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
    <button type="button" on:click={() => dispatch("refreshBasicProfileImages")}>
      {$t("Button_RefreshImages")}
    </button>
    <button type="button" on:click={() => dispatch("clearBasicProfileImages")}>
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

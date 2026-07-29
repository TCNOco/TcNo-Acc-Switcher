<script lang="ts">
  import { createEventDispatcher } from "svelte";
  import { t } from "../../stores/i18n";
  import { tooltip as tooltipAction } from "../../lib/actions/tooltip";
  import { route } from "../../stores/nav";
  import SettingsGroup from "./SettingsGroup.svelte";

  export let name: string;
  export let isSteam: boolean = false;
  export let installFolder: string = "";
  export let hasCachePaths: boolean = false;
  export let hasBackupFolders: boolean = false;
  export let hasSavedProfileImageSources: boolean = false;
  export let clearingCache: boolean = false;
  export let backingUp: boolean = false;
  export let restoringBackup: boolean = false;

  const dispatch = createEventDispatcher();
</script>

<SettingsGroup title={$t("Settings_Header_GeneralTools")}>
  <p class="settings-note settings-path">
    {$t("Settings_CurrentLocation", { path: installFolder || "" })}
  </p>
  <div class="settings-actions">
    <button type="button" on:click={() => dispatch("pickFolder")}>{$t("Settings_PickFolder", { platform: name })}</button>
    <button type="button" on:click={() => dispatch("reset")}>{$t("Button_ResetSettings")}</button>
    {#if hasCachePaths}
      {#if hasSavedProfileImageSources}
        <button type="button" on:click={() => dispatch("refreshSavedBasicProfileImages")}>
          {$t("Button_RefreshProfileImages")}
        </button>
      {/if}
      <button type="button" disabled={clearingCache} on:click={() => dispatch("clearCache")}>
        {$t("Platform_ClearCache")}
      </button>
    {/if}
    {#if isSteam}
      <button type="button" on:click={() => dispatch("refreshVac")}>{$t("Steam_CheckVac")}</button>
      <button type="button" on:click={() => dispatch("refreshImages")}>{$t("Button_RefreshImages")}</button>
    {/if}
  </div>

  {#if hasBackupFolders}
    <h3 class="settings-subheader">{$t("Settings_Header_BackupRestore")}</h3>
    <div class="settings-actions">
      {#if isSteam}
        <button type="button" disabled={backingUp} on:click={() => dispatch("backup", { everything: false })} use:tooltipAction={$t("Tooltip_Backup")}>
          {$t("Button_Backup")}
        </button>
        <button type="button" disabled={backingUp} on:click={() => dispatch("backup", { everything: true })} use:tooltipAction={$t("Tooltip_BackupAll")}>
          {$t("Button_BackupAll")}
        </button>
      {:else}
        <button type="button" disabled={backingUp} on:click={() => dispatch("backup", { everything: true })}>
          {$t("Button_BackupAll")}
        </button>
      {/if}
      <button type="button" on:click={() => dispatch("openBackupFolder")}>{$t("Button_OpenBackup")}</button>
      <button type="button" disabled={restoringBackup} on:click={() => dispatch("restoreLatestBackup")}>
        {$t("Button_Restore")}
      </button>
    </div>
  {/if}

  <h3 class="settings-subheader">{$t("Settings_Header_OtherTools")}</h3>
  <div class="settings-actions">
    <button type="button" on:click={() => dispatch("openFolder")}>{$t("Settings_OpenFolder", { platform: name })}</button>
    <button type="button" on:click={() => route.set({ page: "steam-advanced-clearing" })}>
      {$t("Button_AdvancedCleaning")}
    </button>
  </div>
</SettingsGroup>

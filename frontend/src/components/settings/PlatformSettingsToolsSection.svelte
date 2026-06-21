<script lang="ts">
  import { createEventDispatcher } from "svelte";
  import { t } from "../../stores/i18n";
  import { tooltip as tooltipAction } from "../../lib/actions/tooltip";
  import { route } from "../../stores/nav";

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

<h2 class="SettingsHeader">{$t("Settings_Header_GeneralTools")}</h2>
<p class="install-loc">
  {$t("Settings_CurrentLocation", { path: installFolder || "" })}
</p>
<div class="buttoncol">
  <button type="button" on:click={() => dispatch("pickFolder")}>{$t("Settings_PickFolder", { platform: name })}</button>
  <button type="button" on:click={() => dispatch("reset")}>{$t("Button_ResetSettings")}</button>
</div>
{#if hasCachePaths}
  <div class="buttoncol">
    {#if hasSavedProfileImageSources}
      <button type="button" on:click={() => dispatch("refreshSavedBasicProfileImages")}>
        {$t("Button_RefreshProfileImages")}
      </button>
      <button type="button" disabled={clearingCache} on:click={() => dispatch("clearCache")}>
        {$t("Platform_ClearCache")}
      </button>
    {:else}
      <button type="button" disabled={clearingCache} on:click={() => dispatch("clearCache")}>
        {$t("Platform_ClearCache")}
      </button>
    {/if}
  </div>
{/if}
{#if isSteam}
  <div class="buttoncol">
    <button type="button" on:click={() => dispatch("refreshVac")}>{$t("Steam_CheckVac")}</button>
    <button type="button" on:click={() => dispatch("refreshImages")}>{$t("Button_RefreshImages")}</button>
  </div>
{/if}

{#if hasBackupFolders}
  <h2 class="SettingsHeader">{$t("Settings_Header_BackupRestore")}</h2>
  {#if isSteam}
    <div class="buttoncol">
      <button type="button" disabled={backingUp} on:click={() => dispatch("backup", { everything: false })} use:tooltipAction={$t("Tooltip_Backup")}>
        {$t("Button_Backup")}
      </button>
      <button type="button" disabled={backingUp} on:click={() => dispatch("backup", { everything: true })} use:tooltipAction={$t("Tooltip_BackupAll")}>
        {$t("Button_BackupAll")}
      </button>
    </div>
  {:else}
    <div class="buttoncol">
      <button type="button" disabled={backingUp} on:click={() => dispatch("backup", { everything: true })}>
        {$t("Button_BackupAll")}
      </button>
    </div>
  {/if}
  <div class="buttoncol">
    <button type="button" on:click={() => dispatch("openBackupFolder")}>{$t("Button_OpenBackup")}</button>
    <button type="button" disabled={restoringBackup} on:click={() => dispatch("restoreLatestBackup")}>
      {$t("Button_Restore")}
    </button>
  </div>
{/if}

<h2 class="SettingsHeader">{$t("Settings_Header_OtherTools")}</h2>
<div class="buttoncol">
  <button type="button" on:click={() => dispatch("openFolder")}>{$t("Settings_OpenFolder", { platform: name })}</button>
  <button type="button" on:click={() => route.set({ page: "steam-advanced-clearing" })}>
    {$t("Button_AdvancedCleaning")}
  </button>
</div>

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
    <button type="button" class="btnicontext" on:click={() => dispatch("pickFolder")}><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M2 5.5A1.5 1.5 0 0 1 3.5 4h5.2a1.5 1.5 0 0 1 1.06.44L12 6.5h5.5A1.5 1.5 0 0 1 19 8v2.6H6.4L4.3 19.9A1.5 1.5 0 0 1 2 18.6ZM8 12.2h14l-3.1 9.6H5.9Z"/></svg>{$t("Settings_PickFolder", { platform: name })}</button>
    <button type="button" class="btnicontext" on:click={() => dispatch("reset")}><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M11.29 4.53A8.2 8.2 0 1 1 4.57 9.24L7.47 10.59A5 5 0 1 0 11.56 7.72ZM12.38 1.91 12.12 9.3 5.05 5.35Z"/></svg>{$t("Button_ResetSettings")}</button>
    {#if hasCachePaths}
      {#if hasSavedProfileImageSources}
        <button type="button" class="btnicontext" on:click={() => dispatch("refreshSavedBasicProfileImages")}>
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" aria-hidden="true"><path d="M463.5 224H472c13.3 0 24-10.7 24-24V72c0-9.7-5.8-18.5-14.8-22.2s-19.3-1.7-26.2 5.2L413.4 96.6c-87.6-86.5-228.7-86.2-315.8 1c-87.5 87.5-87.5 229.3 0 316.8s229.3 87.5 316.8 0c12.5-12.5 12.5-32.8 0-45.3s-32.8-12.5-45.3 0c-62.5 62.5-163.8 62.5-226.3 0s-62.5-163.8 0-226.3c62.2-62.2 162.7-62.5 225.3-1l-30.4 30.4c-6.9 6.9-8.9 17.2-5.2 26.2s12.5 14.8 22.2 14.8H463.5z"/></svg>{$t("Button_RefreshProfileImages")}
        </button>
      {/if}
      <button type="button" class="btnicontext" disabled={clearingCache} on:click={() => dispatch("clearCache")}>
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M19.05 2.27 21.73 4.95 12.34 15.34 9.66 12.66ZM7 12.6 14 12.6 17.2 21.4 14.75 21.4 13.35 19.2 11.95 21.4 10.9 21.4 9.5 19.2 8.1 21.4 7.05 21.4 5.65 19.2 4.25 21.4 1.8 21.4Z"/></svg>{$t("Platform_ClearCache")}
      </button>
    {/if}
    {#if isSteam}
      <button type="button" class="btnicontext" on:click={() => dispatch("refreshVac")}><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path fill-rule="evenodd" d="M12 1.6 21.5 5.2V11.4C21.5 16.6 17.6 20.6 12 22.4 6.4 20.6 2.5 16.6 2.5 11.4V5.2ZM8.7 10.1 6.9 11.9 10.6 15.6 17.2 9 15.4 7.2 10.6 12Z"/></svg>{$t("Steam_CheckVac")}</button>
      <button type="button" class="btnicontext" on:click={() => dispatch("refreshImages")}><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path fill-rule="evenodd" d="M1.5 3L22.5 3L22.5 21L1.5 21ZM4 5.5L4 18.5L20 18.5L20 5.5ZM7.3 6.5A2.1 2.1 0 1 0 7.3 10.7A2.1 2.1 0 1 0 7.3 6.5ZM5.5 17.6L10.5 10.4L13.2 14.3L15.2 11.6L19 17.6Z"/></svg>{$t("Button_RefreshImages")}</button>
    {/if}
  </div>

  {#if hasBackupFolders}
    <h3 class="settings-subheader">{$t("Settings_Header_BackupRestore")}</h3>
    <div class="settings-actions">
      {#if isSteam}
        <button type="button" class="btnicontext" disabled={backingUp} on:click={() => dispatch("backup", { everything: false })} use:tooltipAction={$t("Tooltip_Backup")}>
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path fill-rule="evenodd" d="M2 3.5A1.5 1.5 0 0 1 3.5 2h17A1.5 1.5 0 0 1 22 3.5v3A1.5 1.5 0 0 1 20.5 8h-17A1.5 1.5 0 0 1 2 6.5Zm1.5 6.1h17v10A2.4 2.4 0 0 1 18.1 22H5.9a2.4 2.4 0 0 1-2.4-2.4Zm5.7 2.8h5.6v2.4H9.2Z"/></svg>{$t("Button_Backup")}
        </button>
        <button type="button" class="btnicontext" disabled={backingUp} on:click={() => dispatch("backup", { everything: true })} use:tooltipAction={$t("Tooltip_BackupAll")}>
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path fill-rule="evenodd" d="M2 3.5A1.5 1.5 0 0 1 3.5 2h17A1.5 1.5 0 0 1 22 3.5v3A1.5 1.5 0 0 1 20.5 8h-17A1.5 1.5 0 0 1 2 6.5Zm1.5 6.1h17v10A2.4 2.4 0 0 1 18.1 22H5.9a2.4 2.4 0 0 1-2.4-2.4Zm5.7 2.8h5.6v2.4H9.2Z"/></svg>{$t("Button_BackupAll")}
        </button>
      {:else}
        <button type="button" class="btnicontext" disabled={backingUp} on:click={() => dispatch("backup", { everything: true })}>
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path fill-rule="evenodd" d="M2 3.5A1.5 1.5 0 0 1 3.5 2h17A1.5 1.5 0 0 1 22 3.5v3A1.5 1.5 0 0 1 20.5 8h-17A1.5 1.5 0 0 1 2 6.5Zm1.5 6.1h17v10A2.4 2.4 0 0 1 18.1 22H5.9a2.4 2.4 0 0 1-2.4-2.4Zm5.7 2.8h5.6v2.4H9.2Z"/></svg>{$t("Button_BackupAll")}
        </button>
      {/if}
      <button type="button" class="btnicontext" on:click={() => dispatch("openBackupFolder")}><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M2 5.5A1.5 1.5 0 0 1 3.5 4h5.2a1.5 1.5 0 0 1 1.06.44L12 6.5h5.5A1.5 1.5 0 0 1 19 8v2.6H6.4L4.3 19.9A1.5 1.5 0 0 1 2 18.6ZM8 12.2h14l-3.1 9.6H5.9Z"/></svg>{$t("Button_OpenBackup")}</button>
      <button type="button" class="btnicontext" disabled={restoringBackup} on:click={() => dispatch("restoreLatestBackup")}>
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M8.4 15.2A6.6 6.6 0 1 1 21.6 15.2 6.6 6.6 0 1 1 8.4 15.2zM13.9 16.3h5.8v-2.2h-3.6V10.4h-2.2zM16.69 3.69A10 10 0 0 0 3.61 12.85L5.7 12.92A7.9 7.9 0 0 1 16.04 5.69zM2.16 12.8L4.49 17.89 7.16 12.98z"/></svg>{$t("Button_Restore")}
      </button>
    </div>
  {/if}

  <h3 class="settings-subheader">{$t("Settings_Header_OtherTools")}</h3>
  <div class="settings-actions">
    <button type="button" class="btnicontext" on:click={() => dispatch("openFolder")}><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M2 5.5A1.5 1.5 0 0 1 3.5 4h5.2a1.5 1.5 0 0 1 1.06.44L12 6.5h5.5A1.5 1.5 0 0 1 19 8v2.6H6.4L4.3 19.9A1.5 1.5 0 0 1 2 18.6ZM8 12.2h14l-3.1 9.6H5.9Z"/></svg>{$t("Settings_OpenFolder", { platform: name })}</button>
    <button type="button" class="btnicontext" on:click={() => route.set({ page: "steam-advanced-clearing" })}>
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M19.05 2.27 21.73 4.95 12.34 15.34 9.66 12.66ZM7 12.6 14 12.6 17.2 21.4 14.75 21.4 13.35 19.2 11.95 21.4 10.9 21.4 9.5 19.2 8.1 21.4 7.05 21.4 5.65 19.2 4.25 21.4 1.8 21.4Z"/></svg>{$t("Button_AdvancedCleaning")}
    </button>
  </div>
</SettingsGroup>

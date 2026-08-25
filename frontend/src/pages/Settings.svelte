<script lang="ts">
  import { onMount } from "svelte";
  import { previousPage, appBarTitle, navigateBackLikeButton } from "../stores/nav";
  import { t } from "../stores/i18n";
  import { controllerSpatialNavigation } from "../lib/actions/controllerSpatialNavigation";
  import { settingsFilter } from "../lib/settingsFilter";
  import { DUR, fadeUp } from "../lib/animation";
  import SettingsSearch from "../components/settings/SettingsSearch.svelte";
  import GeneralSettingsBlock from "../components/GeneralSettingsBlock.svelte";
  import "../styles/Settings.scss";

  let query = "";
  let visibleGroups = -1;

  $: appBarTitle.set($t("Title_Settings"));

  onMount(() => {
    previousPage.set({ page: "home" });
  });
</script>

<div class="main-content settings-page" use:controllerSpatialNavigation>
  <SettingsSearch bind:value={query} on:escape={() => navigateBackLikeButton()} />

  <div
    class="settings-sections"
    use:settingsFilter={query}
    on:filtered={(e) => (visibleGroups = e.detail)}
  >
    <h1 class="SettingsHeader">{$t("Settings_Header_AppWide")}</h1>
    <GeneralSettingsBlock />
  </div>

  {#if visibleGroups === 0}
    <p class="settings-empty" in:fadeUp={{ y: 6, duration: DUR.normal }}>
      {$t("Settings_SearchNoResults", { query })}
    </p>
  {/if}
</div>

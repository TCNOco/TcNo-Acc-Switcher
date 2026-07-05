<script lang="ts">
  import { get } from "svelte/store";
  import { onMount } from "svelte";
  import { previousPage, appBarTitle, navigateBackLikeButton } from "../stores/nav";
  import { activeModal } from "../stores/modal";
  import { t } from "../stores/i18n";
  import { controllerSpatialNavigation } from "../lib/actions/controllerSpatialNavigation";
  import "../styles/Settings.scss";

  $: appBarTitle.set($t("Title_Settings"));
  onMount(() => {
    previousPage.set({ page: "home" });
  });

  function onWindowKeyDown(e: KeyboardEvent): void {
    if (e.key !== "Escape") {
      return;
    }
    if (get(activeModal)) {
      return;
    }
    e.preventDefault();
    navigateBackLikeButton();
  }
</script>

<div class="main-content main-spacing" use:controllerSpatialNavigation>
  <h1 class="SettingsHeader">{$t("Settings_Header_AppWide")}</h1>

  {#await import("../components/GeneralSettingsBlock.svelte") then { default: GeneralSettingsBlock }}
    <GeneralSettingsBlock />
  {/await}
</div>
<svelte:window on:keydown={onWindowKeyDown} />

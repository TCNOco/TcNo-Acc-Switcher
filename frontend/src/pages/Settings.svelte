<script lang="ts">
  import { get } from "svelte/store";
  import { route, previousPage, appBarTitle } from "../stores/nav";
  import { activeModal } from "../stores/modal";
  import { t } from "../stores/i18n";
  import "../styles/Settings.scss";
  import GeneralSettingsBlock from "../components/GeneralSettingsBlock.svelte";

  $: appBarTitle.set($t("Title_Settings"));
  previousPage.set({ page: "home" });
  route.set({ page: "settings" });

  function onWindowKeyDown(e: KeyboardEvent): void {
    if (e.key !== "Escape") {
      return;
    }
    if (get(activeModal)) {
      return;
    }
    const prev = get(previousPage);
    route.set(prev ?? { page: "home" });
  }
</script>

<div class="main-content main-spacing">
  <h1 class="SettingsHeader">{$t("Settings_Header_AppWide")}</h1>

  <GeneralSettingsBlock />
</div>
<svelte:window on:keydown={onWindowKeyDown} />

<style lang="scss">
  .settings-page-subtitle {
    margin: 0;
    color: rgba(255, 255, 255, 0.75);
    font-size: 0.95rem;
    line-height: 1.45;
  }
</style>

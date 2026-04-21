<script lang="ts">
  import { route, previousPage, appBarTitle } from "../stores/nav";
  import { t } from "../stores/i18n";
  import "../styles/Settings.scss";
  import GeneralSettingsBlock from "../components/GeneralSettingsBlock.svelte";

  $: appBarTitle.set("TcNo Account Switcher - Settings");
  previousPage.set({ page: "home" });
  route.set({ page: "settings" });
</script>

<div class="main-content main-spacing">
  <h1 class="SettingsHeader">{$t("Settings_Header_AppWide")}</h1>
  <GeneralSettingsBlock />
</div>

<script lang="ts">
  import { get } from "svelte/store";
  import { route } from "../stores/nav";
  import { t } from "../stores/i18n";
  import { pushToast } from "../stores/toast";
  import { formatToastWithError } from "../lib/formatWailsError";
  import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
  import { appBgInfo, platformBgInfo, userOverriddenAppBg, setUserOverride } from "../stores/backgroundImage";
  import { currentThemeBgUrl } from "../lib/themes";
  import ThemePickerControls from "./ThemePickerControls.svelte";
  import BackgroundSettings from "./BackgroundSettings.svelte";

  $: showResetToThemeBg = !!$currentThemeBgUrl && ($appBgInfo.hasImage || $userOverriddenAppBg);

  async function resetToThemeBg(): Promise<void> {
    try {
      if ($appBgInfo.hasImage) {
        await PlatformService.ClearAppBackground();
      }
      await setUserOverride(false);
      const info = await PlatformService.GetAppBackground();
      appBgInfo.set(info);
    } catch (e) {
      pushToast({
        type: "error",
        message: formatToastWithError(get(t)("Toast_SaveFailed"), e),
        duration: 8000,
      });
    }
  }
</script>

<h2 class="SettingsHeader">{$t("Settings_Header_Theme")}</h2>
<div>
  <ThemePickerControls>
    <button
      slot="after-controls"
      type="button"
      class="btnicontext"
      on:click={() => route.set({ page: "preview-css" })}
    >
      {$t("PreviewCss")}
    </button>
  </ThemePickerControls>
</div>

{#if $appBgInfo.hasImage || showResetToThemeBg}
  <div class="bg-settings-row">
    {#if showResetToThemeBg}
      <button type="button" class="btnicontext" on:click={() => void resetToThemeBg()}>
        {$t("Settings_ResetToThemeBackground")}
      </button>
    {/if}
    {#if $appBgInfo.hasImage}
      <BackgroundSettings target="app" />
    {/if}
  </div>
{/if}

{#if $platformBgInfo.hasImage}
  <div class="bg-settings-row">
    <BackgroundSettings target="platform" />
  </div>
{/if}

<style lang="scss">
  button {
    position: relative;
    height: 38px;
  }
</style>

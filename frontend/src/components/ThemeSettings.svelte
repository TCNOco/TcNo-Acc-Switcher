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
  import SettingsGroup from "./settings/SettingsGroup.svelte";

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

<SettingsGroup title={$t("Settings_Header_Theme")}>
  <div class="theme-controls">
    <ThemePickerControls>
      <button
        slot="after-controls"
        type="button"
        class="btnicontext"
        on:click={() => route.set({ page: "preview-css" })}
      >
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path fill-rule="evenodd" d="M12 5C6 5 2 9.6 1 12 2 14.4 6 19 12 19 18 19 22 14.4 23 12 22 9.6 18 5 12 5ZM12 8.6A3.4 3.4 0 1 1 12 15.4 3.4 3.4 0 1 1 12 8.6Z"/></svg>
        {$t("PreviewCss")}
      </button>
    </ThemePickerControls>
  </div>

  {#if $appBgInfo.hasImage || showResetToThemeBg}
    <div class="bg-settings-row theme-bg-row">
      {#if showResetToThemeBg}
        <button type="button" class="btnicontext" on:click={() => void resetToThemeBg()}>
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M11.29 4.53A8.2 8.2 0 1 1 4.57 9.24L7.47 10.59A5 5 0 1 0 11.56 7.72ZM12.38 1.91 12.12 9.3 5.05 5.35Z"/></svg>
          {$t("Settings_ResetToThemeBackground")}
        </button>
      {/if}
      {#if $appBgInfo.hasImage}
        <BackgroundSettings target="app" />
      {/if}
    </div>
  {/if}

  {#if $platformBgInfo.hasImage}
    <div class="bg-settings-row theme-bg-row">
      <BackgroundSettings target="platform" />
    </div>
  {/if}
</SettingsGroup>

<style lang="scss">
  button {
    position: relative;
    height: 38px;
  }

  .theme-controls,
  .theme-bg-row {
    padding-inline: 0.4rem;
  }
</style>

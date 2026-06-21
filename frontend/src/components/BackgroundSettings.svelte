<script lang="ts">
  import { get } from "svelte/store";
  import { route } from "../stores/nav";
  import { t } from "../stores/i18n";
  import { pushToast } from "../stores/toast";
  import { formatToastWithError } from "../lib/formatWailsError";
  import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
  import { appBgInfo, platformBgInfo } from "../stores/backgroundImage";

  export let target: "app" | "platform" = "app";

  let opacity = 0.6;
  let blur = 6.0;
  let opacityDebounce: ReturnType<typeof setTimeout>;
  let blurDebounce: ReturnType<typeof setTimeout>;

  $: bgInfo = target === "app" ? $appBgInfo : $platformBgInfo;
  $: { opacity = bgInfo.opacity; blur = bgInfo.blur; }

  async function handleClear(): Promise<void> {
    try {
      if (target === "app") {
        await PlatformService.ClearAppBackground();
        const info = await PlatformService.GetAppBackground();
        appBgInfo.set(info);
      } else {
        const routeName = ($route as { platformName?: string }).platformName ?? "";
        await PlatformService.ClearPlatformBackground(routeName);
        const info = await PlatformService.GetPlatformBackground(routeName);
        platformBgInfo.set(info);
      }
    } catch (e) {
      pushToast({
        type: "error",
        message: formatToastWithError(get(t)("Toast_SaveFailed"), e),
        duration: 8000,
      });
    }
  }

  function onOpacityInput(e: Event): void {
    const val = parseFloat((e.target as HTMLInputElement).value);
    const store = target === "app" ? appBgInfo : platformBgInfo;
    store.update((s) => ({ ...s, opacity: val }));
    clearTimeout(opacityDebounce);
    opacityDebounce = setTimeout(async () => {
      try {
        if (target === "app") {
          await PlatformService.SetAppBackgroundOpacity(val);
        } else {
          const routeName = ($route as { platformName?: string }).platformName ?? "";
          await PlatformService.SetPlatformBackgroundOpacity(routeName, val);
        }
      } catch { /* ignore */ }
    }, 300);
  }

  function onBlurInput(e: Event): void {
    const val = parseFloat((e.target as HTMLInputElement).value);
    const store = target === "app" ? appBgInfo : platformBgInfo;
    store.update((s) => ({ ...s, blur: val }));
    clearTimeout(blurDebounce);
    blurDebounce = setTimeout(async () => {
      try {
        if (target === "app") {
          await PlatformService.SetAppBackgroundBlur(val);
        } else {
          const routeName = ($route as { platformName?: string }).platformName ?? "";
          await PlatformService.SetPlatformBackgroundBlur(routeName, val);
        }
      } catch { /* ignore */ }
    }, 300);
  }

  $: clearLabel = target === "app" ? $t("Settings_ClearBackground") : $t("Settings_ClearPlatformBackground");
</script>

<button type="button" class="btnicontext" on:click={() => void handleClear()}>
  {clearLabel}
</button>
<div class="bg-slider-group">
  <label class="bg-settings-row__label" for="bg-opacity-{target}">{$t("Settings_BgOpacity")}</label>
  <input
    id="bg-opacity-{target}"
    type="range"
    min="0"
    max="1"
    step="0.01"
    value={opacity}
    on:input={onOpacityInput}
  />
  <span class="bg-slider-value">{Math.round(opacity * 100)}%</span>
</div>
<div class="bg-slider-group">
  <label class="bg-settings-row__label" for="bg-blur-{target}">{$t("Settings_BgBlur")}</label>
  <input
    id="bg-blur-{target}"
    type="range"
    min="0"
    max="40"
    step="0.5"
    value={blur}
    on:input={onBlurInput}
  />
  <span class="bg-slider-value">{blur.toFixed(1)}px</span>
</div>

<style lang="scss">
  button {
    position: relative;
    height: 38px;
  }
</style>

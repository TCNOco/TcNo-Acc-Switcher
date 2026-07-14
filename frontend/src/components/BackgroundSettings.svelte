<script lang="ts">
  import { get } from "svelte/store";
  import { route } from "../stores/nav";
  import { t } from "../stores/i18n";
  import { pushToast } from "../stores/toast";
  import { formatToastWithError } from "../lib/formatWailsError";
  import {
    normalizeBackgroundAlignment,
    normalizeBackgroundFit,
    type BackgroundAlignment,
    type BackgroundFit,
  } from "../lib/backgroundDisplay";
  import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
  import { appBgInfo, platformBgInfo } from "../stores/backgroundImage";

  export let target: "app" | "platform" = "app";

  let opacity = 0.6;
  let blur = 6.0;
  let alignment: BackgroundAlignment = "center";
  let fit: BackgroundFit = "cover";
  let opacityDebounce: ReturnType<typeof setTimeout>;
  let blurDebounce: ReturnType<typeof setTimeout>;

  $: bgInfo = target === "app" ? $appBgInfo : $platformBgInfo;
  $: {
    opacity = bgInfo.opacity;
    blur = bgInfo.blur;
    alignment = normalizeBackgroundAlignment(bgInfo.alignment);
    fit = normalizeBackgroundFit(bgInfo.fit);
  }

  const alignmentOptions: ReadonlyArray<{ value: BackgroundAlignment; labelKey: string }> = [
    { value: "center", labelKey: "Settings_BgAlign_Center" },
    { value: "left", labelKey: "Settings_BgAlign_Left" },
    { value: "right", labelKey: "Settings_BgAlign_Right" },
    { value: "top", labelKey: "Settings_BgAlign_Top" },
    { value: "bottom", labelKey: "Settings_BgAlign_Bottom" },
  ];

  const fitOptions: ReadonlyArray<{ value: BackgroundFit; labelKey: string }> = [
    { value: "cover", labelKey: "Settings_BgFit_Cover" },
    { value: "contain", labelKey: "Settings_BgFit_Contain" },
    { value: "fill", labelKey: "Settings_BgFit_Fill" },
    { value: "none", labelKey: "Settings_BgFit_None" },
    { value: "scale-down", labelKey: "Settings_BgFit_ScaleDown" },
  ];

  function platformName(): string {
    return ($route as { platformName?: string }).platformName ?? "";
  }

  function showSaveError(error: unknown): void {
    pushToast({
      type: "error",
      message: formatToastWithError(get(t)("Toast_SaveFailed"), error),
      duration: 8000,
    });
  }

  async function handleClear(): Promise<void> {
    try {
      if (target === "app") {
        await PlatformService.ClearAppBackground();
        const info = await PlatformService.GetAppBackground();
        appBgInfo.set(info);
      } else {
        const routeName = platformName();
        await PlatformService.ClearPlatformBackground(routeName);
        const info = await PlatformService.GetPlatformBackground(routeName);
        platformBgInfo.set(info);
      }
    } catch (e) {
      showSaveError(e);
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
          await PlatformService.SetPlatformBackgroundOpacity(platformName(), val);
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
          await PlatformService.SetPlatformBackgroundBlur(platformName(), val);
        }
      } catch { /* ignore */ }
    }, 300);
  }

  async function onAlignmentChange(e: Event): Promise<void> {
    const value = normalizeBackgroundAlignment((e.target as HTMLSelectElement).value);
    const previous = alignment;
    const store = target === "app" ? appBgInfo : platformBgInfo;
    store.update((state) => ({ ...state, alignment: value }));
    try {
      if (target === "app") {
        await PlatformService.SetAppBackgroundAlignment(value);
      } else {
        await PlatformService.SetPlatformBackgroundAlignment(platformName(), value);
      }
    } catch (error) {
      store.update((state) => ({ ...state, alignment: previous }));
      showSaveError(error);
    }
  }

  async function onFitChange(e: Event): Promise<void> {
    const value = normalizeBackgroundFit((e.target as HTMLSelectElement).value);
    const previous = fit;
    const store = target === "app" ? appBgInfo : platformBgInfo;
    store.update((state) => ({ ...state, fit: value }));
    try {
      if (target === "app") {
        await PlatformService.SetAppBackgroundFit(value);
      } else {
        await PlatformService.SetPlatformBackgroundFit(platformName(), value);
      }
    } catch (error) {
      store.update((state) => ({ ...state, fit: previous }));
      showSaveError(error);
    }
  }

  $: clearLabel = target === "app" ? $t("Settings_ClearBackground") : $t("Settings_ClearPlatformBackground");
</script>

<button type="button" class="btnicontext" on:click={() => void handleClear()}>
  {clearLabel}
</button>
<div class="bg-option-group">
  <select
    class="bg-option-select"
    aria-label={$t("Settings_BgAlignAria")}
    value={alignment}
    on:change={(e) => void onAlignmentChange(e)}
  >
    {#each alignmentOptions as option}
      <option value={option.value}>{$t(option.labelKey)}</option>
    {/each}
  </select>
  <select
    class="bg-option-select"
    aria-label={$t("Settings_BgFitAria")}
    value={fit}
    on:change={(e) => void onFitChange(e)}
  >
    {#each fitOptions as option}
      <option value={option.value}>{$t(option.labelKey)}</option>
    {/each}
  </select>
</div>
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

  .bg-option-group {
    display: flex;
    gap: 0.5rem;
  }

  .bg-option-select {
    height: 38px;
    min-width: 7.25rem;
    padding: 0 2rem 0 0.65rem;
    border: 1px solid var(--button-bg);
    border-radius: 0.25rem;
    background: var(--button-bg);
    color: var(--whiteSecondary);
    cursor: pointer;
  }
</style>

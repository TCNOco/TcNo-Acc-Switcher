<script lang="ts">
  import "../styles/Settings.scss";
  import { onMount } from "svelte";
  import { route } from "../stores/nav";
  import { t, availableLocales, locale, setUserLanguage } from "../stores/i18n";
  import { pushToast } from "../stores/toast";
  import { formatToastWithError } from "../lib/formatWailsError";
  import { tooltip } from "../lib/actions/tooltip";
  import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
  import { get } from "svelte/store";
  import { offlineMode, setUserOfflineMode } from "../stores/offlineMode";
  import { openAlertNoButton } from "../stores/modal";
  import StatsReportModalBody from "./modals/StatsReportModalBody.svelte";
  import ThemePickerControls from "./ThemePickerControls.svelte";
  import { appBgInfo, platformBgInfo, userOverriddenAppBg, setUserOverride } from "../stores/backgroundImage";
  import { currentThemeBgUrl } from "../lib/themes";
  import { animationsEnabled, loadAnimationsEnabled, setAnimationsEnabled } from "../stores/animationSettings";

  let open = false;

  let bgOpacity = 0.6;
  let bgBlur = 6.0;
  let bgOpacityDebounce: ReturnType<typeof setTimeout>;
  let bgBlurDebounce: ReturnType<typeof setTimeout>;

  $: showResetToThemeBg = !!$currentThemeBgUrl && ($appBgInfo.hasImage || $userOverriddenAppBg);

  $: {
    bgOpacity = $appBgInfo.opacity;
    bgBlur = $appBgInfo.blur;
  }

  async function clearAppBackground(): Promise<void> {
    try {
      await PlatformService.ClearAppBackground();
      const info = await PlatformService.GetAppBackground();
      appBgInfo.set(info); // info.themeBgOverride is true — backend auto-sets it
    } catch (e) {
      pushToast({
        type: "error",
        message: formatToastWithError(get(t)("Toast_SaveFailed"), e),
        duration: 8000,
      });
    }
  }

  function onBgOpacityInput(e: Event): void {
    const val = parseFloat((e.target as HTMLInputElement).value);
    appBgInfo.update((s) => ({ ...s, opacity: val }));
    clearTimeout(bgOpacityDebounce);
    bgOpacityDebounce = setTimeout(async () => {
      try {
        await PlatformService.SetAppBackgroundOpacity(val);
      } catch { /* ignore */ }
    }, 300);
  }

  function onBgBlurInput(e: Event): void {
    const val = parseFloat((e.target as HTMLInputElement).value);
    appBgInfo.update((s) => ({ ...s, blur: val }));
    clearTimeout(bgBlurDebounce);
    bgBlurDebounce = setTimeout(async () => {
      try {
        await PlatformService.SetAppBackgroundBlur(val);
      } catch { /* ignore */ }
    }, 300);
  }

  let platBgOpacity = 0.6;
  let platBgBlur = 6.0;
  let platBgOpacityDebounce: ReturnType<typeof setTimeout>;
  let platBgBlurDebounce: ReturnType<typeof setTimeout>;

  $: {
    platBgOpacity = $platformBgInfo.opacity;
    platBgBlur = $platformBgInfo.blur;
  }

  async function clearPlatformBackground(): Promise<void> {
    const routeName = ($route as { platformName?: string }).platformName ?? "";
    try {
      await PlatformService.ClearPlatformBackground(routeName);
      const info = await PlatformService.GetPlatformBackground(routeName);
      platformBgInfo.set(info);
    } catch (e) {
      pushToast({
        type: "error",
        message: formatToastWithError(get(t)("Toast_SaveFailed"), e),
        duration: 8000,
      });
    }
  }

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

  function onPlatBgOpacityInput(e: Event): void {
    const routeName = ($route as { platformName?: string }).platformName ?? "";
    const val = parseFloat((e.target as HTMLInputElement).value);
    platformBgInfo.update((s) => ({ ...s, opacity: val }));
    clearTimeout(platBgOpacityDebounce);
    platBgOpacityDebounce = setTimeout(async () => {
      try {
        await PlatformService.SetPlatformBackgroundOpacity(routeName, val);
      } catch { /* ignore */ }
    }, 300);
  }

  function onPlatBgBlurInput(e: Event): void {
    const routeName = ($route as { platformName?: string }).platformName ?? "";
    const val = parseFloat((e.target as HTMLInputElement).value);
    platformBgInfo.update((s) => ({ ...s, blur: val }));
    clearTimeout(platBgBlurDebounce);
    platBgBlurDebounce = setTimeout(async () => {
      try {
        await PlatformService.SetPlatformBackgroundBlur(routeName, val);
      } catch { /* ignore */ }
    }, 300);
  }

  let isWindows = false;
  let protocolEnabled = false;
  let protocolLoading = false;
  let offlineLoading = false;
  let statsEnabled = true;
  let statsShare = true;
  let statsEnabledLoading = false;
  let statsShareLoading = false;
  let discordRpc = true;
  let discordRpcShare = false;
  let discordRpcLoading = false;
  let discordRpcShareLoading = false;

  let exitToTray = false;
  let minimizeOnSwitch = false;
  let exitToTrayLoading = false;
  let minimizeOnSwitchLoading = false;

  let startTrayWithWindows = false;
  let startProgramCentered = false;
  let desktopHomeShortcut = false;
  let startTrayLoading = false;
  let startCenteredLoading = false;
  let desktopShortcutLoading = false;

  let animationsEnabledLocal = true;
  let animationsLoading = false;

  $: animationsEnabledLocal = $animationsEnabled;

  onMount(() => {
    isWindows = /windows/i.test(navigator.userAgent) || /win32/i.test(navigator.userAgent);
    if (isWindows) {
      void PlatformService.GetProtocolEnabled()
        .then((v) => {
          protocolEnabled = v;
        })
        .catch(() => {
          protocolEnabled = false;
        });
    }
    void PlatformService.GetExitToTray()
      .then((v) => {
        exitToTray = v;
      })
      .catch(() => {
        exitToTray = false;
      });
    void PlatformService.GetStatsEnabled()
      .then((v) => {
        statsEnabled = v;
      })
      .catch(() => {
        statsEnabled = true;
      });
    void PlatformService.GetStatsShare()
      .then((v) => {
        statsShare = v;
      })
      .catch(() => {
        statsShare = true;
      });
    void PlatformService.GetDiscordRpc()
      .then((v) => {
        discordRpc = v;
      })
      .catch(() => {
        discordRpc = true;
      });
    void PlatformService.GetDiscordRpcShare()
      .then((v) => {
        discordRpcShare = v;
      })
      .catch(() => {
        discordRpcShare = false;
      });
    void PlatformService.GetMinimizeOnSwitch()
      .then((v) => {
        minimizeOnSwitch = v;
      })
      .catch(() => {
        minimizeOnSwitch = false;
      });
    void PlatformService.GetStartProgramCentered()
      .then((v) => {
        startProgramCentered = v;
      })
      .catch(() => {
        startProgramCentered = false;
      });
    void loadAnimationsEnabled();
    if (isWindows) {
      void PlatformService.GetStartTrayWithWindows()
        .then((v) => {
          startTrayWithWindows = v;
        })
        .catch(() => {
          startTrayWithWindows = false;
        });
      void PlatformService.GetDesktopHomeShortcutExists()
        .then((v) => {
          desktopHomeShortcut = v;
        })
        .catch(() => {
          desktopHomeShortcut = false;
        });
    }
  });

  function nameFor(code: string): string {
    const dn = new Intl.DisplayNames([$locale.replace(/_/g, "-")], { type: "language" });
    return dn.of(code.replace(/_/g, "-")) ?? code;
  }

  $: currentLabel = nameFor($locale);

  async function pick(code: string): Promise<void> {
    await setUserLanguage(code);
    open = false;
  }

  async function toggleProtocol(): Promise<void> {
    if (!isWindows || protocolLoading) {
      return;
    }
    const next = !protocolEnabled;
    protocolLoading = true;
    try {
      await PlatformService.SetProtocolEnabled(next);
      protocolEnabled = next;
      pushToast({
        type: "success",
        message: next ? $t("Toast_ProtocolEnabled") : $t("Toast_ProtocolDisabled"),
        duration: 6000,
      });
    } catch (e) {
      pushToast({
        type: "error",
        message: formatToastWithError($t("Toast_SaveFailed"), e),
        duration: 8000,
      });
    } finally {
      protocolLoading = false;
    }
  }

  async function toggleExitToTray(): Promise<void> {
    if (exitToTrayLoading) {
      return;
    }
    const next = !exitToTray;
    exitToTrayLoading = true;
    try {
      await PlatformService.SetExitToTray(next);
      exitToTray = next;
      pushToast({
        type: "success",
        message: get(t)("Toast_SavedItem", { item: get(t)("Settings_ExitToTray") }),
        duration: 4000,
      });
    } catch (e) {
      pushToast({
        type: "error",
        message: formatToastWithError($t("Toast_SaveFailed"), e),
        duration: 8000,
      });
    } finally {
      exitToTrayLoading = false;
    }
  }

  async function toggleMinimizeOnSwitch(): Promise<void> {
    if (minimizeOnSwitchLoading) {
      return;
    }
    const next = !minimizeOnSwitch;
    minimizeOnSwitchLoading = true;
    try {
      await PlatformService.SetMinimizeOnSwitch(next);
      minimizeOnSwitch = next;
      pushToast({
        type: "success",
        message: get(t)("Toast_SavedItem", { item: get(t)("Settings_MinimizeOnSwitch") }),
        duration: 4000,
      });
    } catch (e) {
      pushToast({
        type: "error",
        message: formatToastWithError($t("Toast_SaveFailed"), e),
        duration: 8000,
      });
    } finally {
      minimizeOnSwitchLoading = false;
    }
  }

  async function refreshDesktopShortcutState(): Promise<void> {
    if (!isWindows) {
      return;
    }
    try {
      desktopHomeShortcut = await PlatformService.GetDesktopHomeShortcutExists();
    } catch {
      desktopHomeShortcut = false;
    }
  }

  async function toggleStartTrayWithWindows(): Promise<void> {
    if (!isWindows || startTrayLoading) {
      return;
    }
    const next = !startTrayWithWindows;
    startTrayLoading = true;
    try {
      await PlatformService.SetStartTrayWithWindows(next);
      startTrayWithWindows = next;
      pushToast({
        type: "success",
        message: get(t)("Toast_SavedItem", { item: get(t)("Settings_Tray_StartWindows") }),
        duration: 4000,
      });
    } catch (e) {
      pushToast({
        type: "error",
        message: formatToastWithError($t("Toast_SaveFailed"), e),
        duration: 8000,
      });
    } finally {
      startTrayLoading = false;
    }
  }

  async function toggleStartProgramCentered(): Promise<void> {
    if (startCenteredLoading) {
      return;
    }
    const next = !startProgramCentered;
    startCenteredLoading = true;
    try {
      await PlatformService.SetStartProgramCentered(next);
      startProgramCentered = next;
      pushToast({
        type: "success",
        message: get(t)("Toast_SavedItem", { item: get(t)("Settings_StartCentered") }),
        duration: 4000,
      });
    } catch (e) {
      pushToast({
        type: "error",
        message: formatToastWithError($t("Toast_SaveFailed"), e),
        duration: 8000,
      });
    } finally {
      startCenteredLoading = false;
    }
  }

  async function toggleAnimations(): Promise<void> {
    if (animationsLoading) {
      return;
    }
    const next = !animationsEnabledLocal;
    animationsEnabledLocal = next;
    animationsLoading = true;
    try {
      await setAnimationsEnabled(next);
      pushToast({
        type: "success",
        message: get(t)("Toast_SavedItem", { item: get(t)("Settings_AnimationsEnabled") }),
        duration: 3000,
      });
    } catch (e) {
      animationsEnabledLocal = !next;
      pushToast({
        type: "error",
        message: formatToastWithError($t("Toast_SaveFailed"), e),
        duration: 8000,
      });
    } finally {
      animationsLoading = false;
    }
  }

  async function toggleDesktopHomeShortcut(): Promise<void> {
    if (!isWindows || desktopShortcutLoading) {
      return;
    }
    const next = !desktopHomeShortcut;
    desktopShortcutLoading = true;
    try {
      await PlatformService.SetDesktopHomeShortcut(next);
      await refreshDesktopShortcutState();
      pushToast({
        type: "success",
        message: get(t)("Toast_SavedItem", { item: get(t)("Settings_DesktopShortcut") }),
        duration: 4000,
      });
    } catch (e) {
      pushToast({
        type: "error",
        message: formatToastWithError($t("Toast_SaveFailed"), e),
        duration: 8000,
      });
    } finally {
      desktopShortcutLoading = false;
    }
  }

  async function toggleOfflineMode(): Promise<void> {
    if (offlineLoading) {
      return;
    }
    const next = !get(offlineMode);
    offlineLoading = true;
    try {
      await setUserOfflineMode(next);
      if (next) {
        discordRpc = false;
        discordRpcShare = false;
      }
      pushToast({
        type: "success",
        message: next ? $t("Toast_OfflineModeEnabled") : $t("Toast_OfflineModeDisabled"),
        duration: 6000,
      });
    } catch (e) {
      pushToast({
        type: "error",
        message: formatToastWithError($t("Toast_SaveFailed"), e),
        duration: 8000,
      });
    } finally {
      offlineLoading = false;
    }
  }

  async function toggleStatsEnabled(): Promise<void> {
    if (statsEnabledLoading) {
      return;
    }
    const next = !statsEnabled;
    statsEnabledLoading = true;
    try {
      await PlatformService.SetStatsEnabled(next);
      statsEnabled = next;
      pushToast({
        type: "success",
        message: get(t)("Toast_SavedItem", { item: get(t)("Settings_CollectStats") }),
        duration: 4000,
      });
    } catch (e) {
      pushToast({
        type: "error",
        message: formatToastWithError($t("Toast_SaveFailed"), e),
        duration: 8000,
      });
    } finally {
      statsEnabledLoading = false;
    }
  }

  async function openStatsModal(): Promise<void> {
    try {
      const report = await PlatformService.GetStatsReport();
      void openAlertNoButton({
        title: $t("Settings_ViewStats"),
        bodyComponent: StatsReportModalBody,
        bodyProps: { initialReport: report },
      });
    } catch (e) {
      pushToast({
        type: "error",
        message: formatToastWithError($t("Toast_StatsReportFailed"), e),
        duration: 8000,
      });
    }
  }

  async function toggleStatsShare(): Promise<void> {
    if (statsShareLoading || !statsEnabled) {
      return;
    }
    const next = !statsShare;
    statsShareLoading = true;
    try {
      await PlatformService.SetStatsShare(next);
      statsShare = next;
      pushToast({
        type: "success",
        message: get(t)("Toast_SavedItem", { item: get(t)("Settings_ShareStats") }),
        duration: 4000,
      });
    } catch (e) {
      pushToast({
        type: "error",
        message: formatToastWithError($t("Toast_SaveFailed"), e),
        duration: 8000,
      });
    } finally {
      statsShareLoading = false;
    }
  }

  async function toggleDiscordRpc(): Promise<void> {
    if (discordRpcLoading || get(offlineMode)) {
      return;
    }
    const next = !discordRpc;
    discordRpcLoading = true;
    try {
      await PlatformService.SetDiscordRpc(next);
      discordRpc = next;
      if (!next) {
        discordRpcShare = false;
      }
      pushToast({
        type: "success",
        message: get(t)("Toast_SavedItem", { item: get(t)("Settings_DiscordRpc") }),
        duration: 4000,
      });
    } catch (e) {
      pushToast({
        type: "error",
        message: formatToastWithError($t("Toast_SaveFailed"), e),
        duration: 8000,
      });
    } finally {
      discordRpcLoading = false;
    }
  }

  async function toggleDiscordRpcShare(): Promise<void> {
    if (discordRpcShareLoading || get(offlineMode) || !discordRpc) {
      return;
    }
    const next = !discordRpcShare;
    discordRpcShareLoading = true;
    try {
      await PlatformService.SetDiscordRpcShare(next);
      discordRpcShare = next;
      pushToast({
        type: "success",
        message: get(t)("Toast_SavedItem", { item: get(t)("Settings_DiscordRpcShare") }),
        duration: 4000,
      });
    } catch (e) {
      pushToast({
        type: "error",
        message: formatToastWithError($t("Toast_SaveFailed"), e),
        duration: 8000,
      });
    } finally {
      discordRpcShareLoading = false;
    }
  }
</script>

<h2 class="SettingsHeader">{$t("Settings_Header_Language")}</h2>
<div class="rowDropdown">
  <span>{$t("Header_ChooseLanguage")}</span>
  <div class="dropdown" class:show={open}>
    <button type="button" class="dropdown-toggle" on:click={() => (open = !open)}>
      {currentLabel}
      <span class="caret" aria-hidden="true"></span>
    </button>
    {#if open}
      <ul
        class="custom-dropdown-menu dropdown-menu"
      >
        {#each availableLocales as code}
          <li role="none">
            <button type="button" class="dropdown-item" on:click={() => void pick(code)}>
              {code} - {nameFor(code)}
            </button>
          </li>
        {/each}
      </ul>
    {/if}
  </div>
</div>

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
    {#if $appBgInfo.hasImage}
      <button type="button" class="btnicontext" on:click={() => void clearAppBackground()}>
        {$t("Settings_ClearBackground")}
      </button>
    {/if}
    {#if showResetToThemeBg}
      <button type="button" class="btnicontext" on:click={() => void resetToThemeBg()}>
        {$t("Settings_ResetToThemeBackground")}
      </button>
    {/if}
    {#if $appBgInfo.hasImage}
      <div class="bg-slider-group">
        <label class="bg-settings-row__label" for="gs-bg-opacity">{$t("Settings_BgOpacity")}</label>
        <input
          id="gs-bg-opacity"
          type="range"
          min="0"
          max="1"
          step="0.01"
          value={bgOpacity}
          on:input={onBgOpacityInput}
        />
        <span class="bg-slider-value">{Math.round(bgOpacity * 100)}%</span>
      </div>
      <div class="bg-slider-group">
        <label class="bg-settings-row__label" for="gs-bg-blur">{$t("Settings_BgBlur")}</label>
        <input
          id="gs-bg-blur"
          type="range"
          min="0"
          max="40"
          step="0.5"
          value={bgBlur}
          on:input={onBgBlurInput}
        />
        <span class="bg-slider-value">{bgBlur.toFixed(1)}px</span>
      </div>
    {/if}
  </div>
{/if}

{#if $platformBgInfo.hasImage}
  <div class="bg-settings-row">
    <button type="button" class="btnicontext" on:click={() => void clearPlatformBackground()}>
      {$t("Settings_ClearPlatformBackground")}
    </button>
    <div class="bg-slider-group">
      <label class="bg-settings-row__label" for="gen-plat-bg-opacity">{$t("Settings_BgOpacity")}</label>
      <input
        id="gen-plat-bg-opacity"
        type="range"
        min="0"
        max="1"
        step="0.01"
        value={platBgOpacity}
        on:input={onPlatBgOpacityInput}
      />
      <span class="bg-slider-value">{Math.round(platBgOpacity * 100)}%</span>
    </div>
    <div class="bg-slider-group">
      <label class="bg-settings-row__label" for="gen-plat-bg-blur">{$t("Settings_BgBlur")}</label>
      <input
        id="gen-plat-bg-blur"
        type="range"
        min="0"
        max="40"
        step="0.5"
        value={platBgBlur}
        on:input={onPlatBgBlurInput}
      />
      <span class="bg-slider-value">{platBgBlur.toFixed(1)}px</span>
    </div>
  </div>
{/if}

<h2 class="SettingsHeader">{$t("Settings_Header_System")}</h2>

<div class="rowSetting">
  <div class="form-check">
    <input
      id="gs-offline"
      type="checkbox"
      checked={$offlineMode}
      disabled={offlineLoading}
      on:change={() => void toggleOfflineMode()}
    />
    <label class="form-check-label" for="gs-offline"></label>
  </div>
  <label for="gs-offline" use:tooltip={$t("Settings_OfflineMode")}>{$t("Settings_OfflineMode")}</label>
</div>

<div class="rowSetting">
  <div class="form-check">
    <input
      id="gs-min-switch"
      type="checkbox"
      checked={minimizeOnSwitch}
      disabled={minimizeOnSwitchLoading}
      on:change={() => void toggleMinimizeOnSwitch()}
    />
    <label class="form-check-label" for="gs-min-switch"></label>
  </div>
  <label for="gs-min-switch" use:tooltip={$t("Settings_MinimizeOnSwitch")}
    >{$t("Settings_MinimizeOnSwitch")}</label
  >
</div>

{#if isWindows}
  <div class="rowSetting">
    <div class="form-check">
      <input
        id="gs-start-tray-win"
        type="checkbox"
        checked={startTrayWithWindows}
        disabled={startTrayLoading}
        on:change={() => void toggleStartTrayWithWindows()}
      />
      <label class="form-check-label" for="gs-start-tray-win"></label>
    </div>
    <label for="gs-start-tray-win" use:tooltip={$t("Settings_Tray_StartWindows")}
      >{$t("Settings_Tray_StartWindows")}</label
    >

    <div class="form-check">
      <input
        id="gs-exit-tray"
        type="checkbox"
        checked={exitToTray}
        disabled={exitToTrayLoading}
        on:change={() => void toggleExitToTray()}
      />
      <label class="form-check-label" for="gs-exit-tray"></label>
    </div>
    <label for="gs-exit-tray" use:tooltip={$t("Settings_ExitToTray")}>{$t("Settings_ExitToTray")}</label>
  </div>

  <div class="rowSetting">
    <div class="form-check">
      <input
        id="gs-protocol"
        type="checkbox"
        checked={protocolEnabled}
        disabled={protocolLoading}
        on:change={() => void toggleProtocol()}
      />
      <label class="form-check-label" for="gs-protocol"></label>
    </div>
    <label for="gs-protocol" use:tooltip={$t("Settings_Protocol")}
      >{$t("Settings_Protocol")}</label
    >
  </div>
{/if}

<div class="rowSetting">
  <div class="form-check">
    <input
      id="gs-start-centered"
      type="checkbox"
      checked={startProgramCentered}
      disabled={startCenteredLoading}
      on:change={() => void toggleStartProgramCentered()}
    />
    <label class="form-check-label" for="gs-start-centered"></label>
  </div>
  <label for="gs-start-centered" use:tooltip={$t("Settings_StartCentered")}
    >{$t("Settings_StartCentered")}</label
  >
</div>

<div class="rowSetting">
  <div class="form-check">
    <input
      type="checkbox"
      id="settings-animations"
      checked={animationsEnabledLocal}
      disabled={animationsLoading}
      on:change={() => void toggleAnimations()}
    />
    <label class="form-check-label" for="settings-animations"></label>
  </div>
  <label for="settings-animations">{$t("Settings_AnimationsEnabled")}</label>
</div>

{#if isWindows}
  <div class="rowSetting">
    <div class="form-check">
      <input
        id="gs-desktop-home"
        type="checkbox"
        checked={desktopHomeShortcut}
        disabled={desktopShortcutLoading}
        on:change={() => void toggleDesktopHomeShortcut()}
      />
      <label class="form-check-label" for="gs-desktop-home"></label>
    </div>
    <label for="gs-desktop-home">{$t("Settings_DesktopShortcut")}</label>
  </div>
{/if}

<h2 class="SettingsHeader">{$t("Settings_Header_StatsSharing")}</h2>

<div class="rowSetting">
  <div class="form-check">
    <input
      id="gs-stats-enabled"
      type="checkbox"
      checked={statsEnabled}
      disabled={statsEnabledLoading}
      on:change={() => void toggleStatsEnabled()}
    />
    <label class="form-check-label" for="gs-stats-enabled"></label>
  </div>
  <label for="gs-stats-enabled">{$t("Settings_CollectStats")}</label>
  <div class="form-check">
    <input
      id="gs-stats-share"
      type="checkbox"
      checked={statsShare}
      disabled={statsShareLoading || !statsEnabled}
      on:change={() => void toggleStatsShare()}
    />
    <label class="form-check-label" for="gs-stats-share"></label>
  </div>
  <label for="gs-stats-share">{$t("Settings_ShareStats")}</label>
  <button type="button" class="btnicontext" on:click={() => void openStatsModal()}>
    {$t("Settings_ViewStats")}
  </button>
</div>

<div class="rowSetting">
  <div class="form-check">
    <input
      id="gs-discord-rpc"
      type="checkbox"
      checked={discordRpc}
      disabled={discordRpcLoading || $offlineMode}
      on:change={() => void toggleDiscordRpc()}
    />
    <label class="form-check-label" for="gs-discord-rpc"></label>
  </div>
  <label for="gs-discord-rpc">{$t("Settings_DiscordRpc")}</label>
  <div class="form-check">
    <input
      id="gs-discord-rpc-share"
      type="checkbox"
      checked={discordRpcShare}
      disabled={discordRpcShareLoading || $offlineMode || !discordRpc}
      on:change={() => void toggleDiscordRpcShare()}
    />
    <label class="form-check-label" for="gs-discord-rpc-share"></label>
  </div>
  <label for="gs-discord-rpc-share">{$t("Settings_DiscordRpcShare")}</label>
</div>

<style lang="scss">
  button {
    position: relative;
    height: 38px;
  }
</style>

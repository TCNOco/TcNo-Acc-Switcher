<script lang="ts">
  import "../styles/Settings.scss";
  import { onMount } from "svelte";
  import { route } from "../stores/nav";
  import { t, availableLocales, locale, setUserLanguage } from "../stores/i18n";
  import { pushToast } from "../stores/toast";
  import { formatToastWithError } from "../lib/formatWailsError";
  import { tooltip } from "../lib/actions/tooltip";
  import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
  import { listThemes, setUserTheme, currentThemeId, type ThemeOption } from "../lib/themes";
  import { get } from "svelte/store";
  import { offlineMode, setUserOfflineMode } from "../stores/offlineMode";
  import { openAlertNoButton } from "../stores/modal";
  import StatsReportModalBody from "./modals/StatsReportModalBody.svelte";

  let open = false;
  let themeOpen = false;
  let themes: ThemeOption[] = [];

  let isWindows = false;
  let protocolEnabled = false;
  let protocolLoading = false;
  let offlineLoading = false;
  let statsEnabled = true;
  let statsShare = true;
  let statsEnabledLoading = false;
  let statsShareLoading = false;

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

  onMount(() => {
    themes = listThemes();
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
  $: currentThemeLabel =
    themes.find((th) => th.id === $currentThemeId)?.label ?? themes[0]?.label ?? "";

  async function pickTheme(id: string): Promise<void> {
    await setUserTheme(id);
    themeOpen = false;
  }

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
  <div class="rowDropdown">
    <span>{$t("Settings_CurrentStyle")}</span>
    <div class="dropdown" class:show={themeOpen}>
      <button type="button" class="dropdown-toggle" on:click={() => (themeOpen = !themeOpen)}>
        {currentThemeLabel}
        <span class="caret" aria-hidden="true"></span>
      </button>
      {#if themeOpen}
        <ul
          class="custom-dropdown-menu dropdown-menu"
        >
          {#each themes as th}
            <li role="none">
              <button type="button" class="dropdown-item" on:click={() => void pickTheme(th.id)}>
                {th.label}
              </button>
            </li>
          {/each}
        </ul>
      {/if}
    </div>
    <button type="button" class="btnicontext" on:click={() => route.set({ page: "preview-css" })}>
      {$t("PreviewCss")}
    </button>
  </div>
</div>

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

<style lang="scss">
  button {
    position: relative;
    height: 38px;
  }
</style>

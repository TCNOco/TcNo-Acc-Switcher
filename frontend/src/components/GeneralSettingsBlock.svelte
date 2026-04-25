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

  let open = false;
  let themeOpen = false;
  let themes: ThemeOption[] = [];

  let isWindows = false;
  let protocolEnabled = false;
  let protocolLoading = false;
  let offlineLoading = false;

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
</script>

<h2 class="SettingsHeader">{$t("Settings_Header_Theme")}</h2>
<div class="themeRow">
  <div class="rowDropdown themeRow__dropdown">
    <span>{$t("Settings_CurrentStyle")}</span>
    <div class="dropdown" class:show={themeOpen}>
      <button type="button" class="dropdown-toggle" on:click={() => (themeOpen = !themeOpen)}>
        {currentThemeLabel}
        <span class="caret" aria-hidden="true"></span>
      </button>
      {#if themeOpen}
        <ul
          class="custom-dropdown-menu dropdown-menu"
          style="position:absolute; top:100%; left:0; z-index:1000; margin:0;"
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
  </div>
  <button type="button" class="btnicontext themeRow__preview" on:click={() => route.set({ page: "preview-css" })}>
    {$t("PreviewCss")}
  </button>
</div>

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

<h2 class="SettingsHeader">{$t("Settings_Header_System")}</h2>

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
        style="position:absolute; top:100%; left:0; z-index:1000; margin:0;"
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

<style lang="scss">
  .themeRow {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-end;
    gap: 0.75rem 1rem;
    margin-bottom: 1.25rem;
  }

  .themeRow__dropdown {
    flex: 1 1 12rem;
    min-width: 0;
  }

  .themeRow__preview {
    flex: 0 0 auto;
    align-self: flex-end;
  }
</style>

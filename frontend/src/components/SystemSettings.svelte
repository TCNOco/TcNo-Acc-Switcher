<script lang="ts">
  import { onMount } from "svelte";
  import { get } from "svelte/store";
  import { t } from "../stores/i18n";
  import { showUserDataMoveOverlay, hideUserDataMoveOverlay } from "../stores/userDataMove";
  import { pushToast } from "../stores/toast";
  import { formatToastWithError } from "../lib/formatWailsError";
  import { tooltip } from "../lib/actions/tooltip";
  import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
  import { offlineMode, setUserOfflineMode } from "../stores/offlineMode";
  import { FOLDER_PICKER_APPDATA, FOLDER_PICKER_PORTABLE, openAlertNoButton, openFeedbackModal, openFolderPicker } from "../stores/modal";
  import { animationsEnabled, loadAnimationsEnabled, setAnimationsEnabled } from "../stores/animationSettings";
  import { checkForUpdatesManually, formatAppVersion } from "../lib/checkForUpdates";
  import { parentDisplayPath } from "../lib/fsPaths";
  import StatsReportModalBody from "./modals/StatsReportModalBody.svelte";

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
  let currentVersion = "";
  let updateCheckLoading = false;
  let userDataPath = "";
  let userDataMoveLoading = false;

  $: animationsEnabledLocal = $animationsEnabled;

  onMount(() => {
    isWindows = /windows/i.test(navigator.userAgent) || /win32/i.test(navigator.userAgent);
    if (isWindows) {
      void PlatformService.GetProtocolEnabled()
        .then((v) => { protocolEnabled = v; })
        .catch(() => { protocolEnabled = false; });
    }
    void PlatformService.GetExitToTray()
      .then((v) => { exitToTray = v; })
      .catch(() => { exitToTray = false; });
    void PlatformService.GetStatsEnabled()
      .then((v) => { statsEnabled = v; })
      .catch(() => { statsEnabled = true; });
    void PlatformService.GetStatsShare()
      .then((v) => { statsShare = v; })
      .catch(() => { statsShare = true; });
    void PlatformService.GetDiscordRpc()
      .then((v) => { discordRpc = v; })
      .catch(() => { discordRpc = true; });
    void PlatformService.GetDiscordRpcShare()
      .then((v) => { discordRpcShare = v; })
      .catch(() => { discordRpcShare = false; });
    void PlatformService.GetMinimizeOnSwitch()
      .then((v) => { minimizeOnSwitch = v; })
      .catch(() => { minimizeOnSwitch = false; });
    void PlatformService.GetStartProgramCentered()
      .then((v) => { startProgramCentered = v; })
      .catch(() => { startProgramCentered = false; });
    void loadAnimationsEnabled();
    void PlatformService.GetAppVersion()
      .then((v) => { currentVersion = v || ""; })
      .catch(() => { currentVersion = ""; });
    void PlatformService.GetUserDataLocation()
      .then((v) => { userDataPath = v || ""; })
      .catch(() => { userDataPath = ""; });
    if (isWindows) {
      void PlatformService.GetStartTrayWithWindows()
        .then((v) => { startTrayWithWindows = v; })
        .catch(() => { startTrayWithWindows = false; });
      void PlatformService.GetDesktopHomeShortcutExists()
        .then((v) => { desktopHomeShortcut = v; })
        .catch(() => { desktopHomeShortcut = false; });
    }
  });

  async function toggleProtocol(): Promise<void> {
    if (!isWindows || protocolLoading) return;
    const next = !protocolEnabled;
    protocolLoading = true;
    try {
      await PlatformService.SetProtocolEnabled(next);
      protocolEnabled = next;
      pushToast({ type: "success", message: next ? $t("Toast_ProtocolEnabled") : $t("Toast_ProtocolDisabled"), duration: 6000 });
    } catch (e) {
      pushToast({ type: "error", message: formatToastWithError($t("Toast_SaveFailed"), e), duration: 8000 });
    } finally {
      protocolLoading = false;
    }
  }

  async function toggleExitToTray(): Promise<void> {
    if (exitToTrayLoading) return;
    const next = !exitToTray;
    exitToTrayLoading = true;
    try {
      await PlatformService.SetExitToTray(next);
      exitToTray = next;
      pushToast({ type: "success", message: get(t)("Toast_SavedItem", { item: get(t)("Settings_ExitToTray") }), duration: 4000 });
    } catch (e) {
      pushToast({ type: "error", message: formatToastWithError($t("Toast_SaveFailed"), e), duration: 8000 });
    } finally {
      exitToTrayLoading = false;
    }
  }

  async function toggleMinimizeOnSwitch(): Promise<void> {
    if (minimizeOnSwitchLoading) return;
    const next = !minimizeOnSwitch;
    minimizeOnSwitchLoading = true;
    try {
      await PlatformService.SetMinimizeOnSwitch(next);
      minimizeOnSwitch = next;
      pushToast({ type: "success", message: get(t)("Toast_SavedItem", { item: get(t)("Settings_MinimizeOnSwitch") }), duration: 4000 });
    } catch (e) {
      pushToast({ type: "error", message: formatToastWithError($t("Toast_SaveFailed"), e), duration: 8000 });
    } finally {
      minimizeOnSwitchLoading = false;
    }
  }

  async function refreshDesktopShortcutState(): Promise<void> {
    if (!isWindows) return;
    try {
      desktopHomeShortcut = await PlatformService.GetDesktopHomeShortcutExists();
    } catch {
      desktopHomeShortcut = false;
    }
  }

  async function toggleStartTrayWithWindows(): Promise<void> {
    if (!isWindows || startTrayLoading) return;
    const next = !startTrayWithWindows;
    startTrayLoading = true;
    try {
      await PlatformService.SetStartTrayWithWindows(next);
      startTrayWithWindows = next;
      pushToast({ type: "success", message: get(t)("Toast_SavedItem", { item: get(t)("Settings_Tray_StartWindows") }), duration: 4000 });
    } catch (e) {
      pushToast({ type: "error", message: formatToastWithError($t("Toast_SaveFailed"), e), duration: 8000 });
    } finally {
      startTrayLoading = false;
    }
  }

  async function toggleStartProgramCentered(): Promise<void> {
    if (startCenteredLoading) return;
    const next = !startProgramCentered;
    startCenteredLoading = true;
    try {
      await PlatformService.SetStartProgramCentered(next);
      startProgramCentered = next;
      pushToast({ type: "success", message: get(t)("Toast_SavedItem", { item: get(t)("Settings_StartCentered") }), duration: 4000 });
    } catch (e) {
      pushToast({ type: "error", message: formatToastWithError($t("Toast_SaveFailed"), e), duration: 8000 });
    } finally {
      startCenteredLoading = false;
    }
  }

  async function toggleAnimations(): Promise<void> {
    if (animationsLoading) return;
    const next = !animationsEnabledLocal;
    animationsEnabledLocal = next;
    animationsLoading = true;
    try {
      await setAnimationsEnabled(next);
      pushToast({ type: "success", message: get(t)("Toast_SavedItem", { item: get(t)("Settings_AnimationsEnabled") }), duration: 3000 });
    } catch (e) {
      animationsEnabledLocal = !next;
      pushToast({ type: "error", message: formatToastWithError($t("Toast_SaveFailed"), e), duration: 8000 });
    } finally {
      animationsLoading = false;
    }
  }

  async function toggleDesktopHomeShortcut(): Promise<void> {
    if (!isWindows || desktopShortcutLoading) return;
    const next = !desktopHomeShortcut;
    desktopShortcutLoading = true;
    try {
      await PlatformService.SetDesktopHomeShortcut(next);
      await refreshDesktopShortcutState();
      pushToast({ type: "success", message: get(t)("Toast_SavedItem", { item: get(t)("Settings_DesktopShortcut") }), duration: 4000 });
    } catch (e) {
      pushToast({ type: "error", message: formatToastWithError($t("Toast_SaveFailed"), e), duration: 8000 });
    } finally {
      desktopShortcutLoading = false;
    }
  }

  async function toggleOfflineMode(): Promise<void> {
    if (offlineLoading) return;
    const next = !get(offlineMode);
    offlineLoading = true;
    try {
      await setUserOfflineMode(next);
      if (next) {
        discordRpc = false;
        discordRpcShare = false;
      }
      pushToast({ type: "success", message: next ? $t("Toast_OfflineModeEnabled") : $t("Toast_OfflineModeDisabled"), duration: 6000 });
    } catch (e) {
      pushToast({ type: "error", message: formatToastWithError($t("Toast_SaveFailed"), e), duration: 8000 });
    } finally {
      offlineLoading = false;
    }
  }

  async function toggleStatsEnabled(): Promise<void> {
    if (statsEnabledLoading) return;
    const next = !statsEnabled;
    statsEnabledLoading = true;
    try {
      await PlatformService.SetStatsEnabled(next);
      statsEnabled = next;
      pushToast({ type: "success", message: get(t)("Toast_SavedItem", { item: get(t)("Settings_CollectStats") }), duration: 4000 });
    } catch (e) {
      pushToast({ type: "error", message: formatToastWithError($t("Toast_SaveFailed"), e), duration: 8000 });
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
      pushToast({ type: "error", message: formatToastWithError($t("Toast_StatsReportFailed"), e), duration: 8000 });
    }
  }

  async function toggleStatsShare(): Promise<void> {
    if (statsShareLoading || !statsEnabled) return;
    const next = !statsShare;
    statsShareLoading = true;
    try {
      await PlatformService.SetStatsShare(next);
      statsShare = next;
      pushToast({ type: "success", message: get(t)("Toast_SavedItem", { item: get(t)("Settings_ShareStats") }), duration: 4000 });
    } catch (e) {
      pushToast({ type: "error", message: formatToastWithError($t("Toast_SaveFailed"), e), duration: 8000 });
    } finally {
      statsShareLoading = false;
    }
  }

  async function toggleDiscordRpc(): Promise<void> {
    if (discordRpcLoading || get(offlineMode)) return;
    const next = !discordRpc;
    discordRpcLoading = true;
    try {
      await PlatformService.SetDiscordRpc(next);
      discordRpc = next;
      if (!next) discordRpcShare = false;
      pushToast({ type: "success", message: get(t)("Toast_SavedItem", { item: get(t)("Settings_DiscordRpc") }), duration: 4000 });
    } catch (e) {
      pushToast({ type: "error", message: formatToastWithError($t("Toast_SaveFailed"), e), duration: 8000 });
    } finally {
      discordRpcLoading = false;
    }
  }

  async function toggleDiscordRpcShare(): Promise<void> {
    if (discordRpcShareLoading || get(offlineMode) || !discordRpc) return;
    const next = !discordRpcShare;
    discordRpcShareLoading = true;
    try {
      await PlatformService.SetDiscordRpcShare(next);
      discordRpcShare = next;
      pushToast({ type: "success", message: get(t)("Toast_SavedItem", { item: get(t)("Settings_DiscordRpcShare") }), duration: 4000 });
    } catch (e) {
      pushToast({ type: "error", message: formatToastWithError($t("Toast_SaveFailed"), e), duration: 8000 });
    } finally {
      discordRpcShareLoading = false;
    }
  }

  async function runUserDataMove(action: () => Promise<void>): Promise<void> {
    if (userDataMoveLoading) return;
    userDataMoveLoading = true;
    showUserDataMoveOverlay();
    try {
      await action();
    } catch (e) {
      hideUserDataMoveOverlay();
      pushToast({ type: "error", message: formatToastWithError($t("Toast_SaveFailed"), e), duration: 8000 });
      userDataMoveLoading = false;
    }
  }

  async function openMoveUserDataModal(): Promise<void> {
    if (userDataMoveLoading) return;
    const picked = await openFolderPicker({
      title: $t("Modal_Title_MoveUserdata"),
      body: $t("Modal_SetUserdata"),
      initialPath: parentDisplayPath(userDataPath),
      dirsOnly: true,
      showPortableButton: true,
      positiveLabel: $t("Modal_SetUserdata_Button"),
    });
    if (!picked) return;
    if (picked === FOLDER_PICKER_PORTABLE) {
      await runUserDataMove(() => PlatformService.MoveUserDataPortable());
      return;
    }
    if (picked === FOLDER_PICKER_APPDATA) {
      await runUserDataMove(() => PlatformService.MoveUserDataAppData());
      return;
    }
    await runUserDataMove(() => PlatformService.MoveUserDataTo(picked));
  }

  async function openUserDataFolder(): Promise<void> {
    try {
      await PlatformService.OpenUserDataFolder();
    } catch (e) {
      pushToast({ type: "error", message: formatToastWithError($t("Toast_SaveFailed"), e), duration: 8000 });
    }
  }

  async function onCheckForUpdates(): Promise<void> {
    if (updateCheckLoading) {
      return;
    }
    updateCheckLoading = true;
    try {
      await checkForUpdatesManually();
    } finally {
      updateCheckLoading = false;
    }
  }
</script>

<h2 class="SettingsHeader">{$t("Settings_Header_System")}</h2>

<div class="multilineSetting">
  <span>{$t("Settings_CurrentDataLocation", { path: userDataPath || "…" })}</span>
  <span>
    <button
      type="button"
      class="fancyLink"
      disabled={userDataMoveLoading}
      on:click={() => void openMoveUserDataModal()}
    >{$t("Settings_SetDataLocation")}</button>
    <button
      type="button"
      class="fancyLink"
      on:click={() => void openUserDataFolder()}
    >{$t("Settings_OpenUserDataFolder")}</button>
    </span>
</div>

<div class="rowSetting">
  <div class="form-check">
    <input id="gs-offline" type="checkbox" checked={$offlineMode} disabled={offlineLoading} on:change={() => void toggleOfflineMode()} />
    <label class="form-check-label" for="gs-offline"></label>
  </div>
  <label for="gs-offline" use:tooltip={$t("Settings_OfflineMode")}>{$t("Settings_OfflineMode")}</label>
</div>

<div class="rowSetting">
  <div class="form-check">
    <input id="gs-min-switch" type="checkbox" checked={minimizeOnSwitch} disabled={minimizeOnSwitchLoading} on:change={() => void toggleMinimizeOnSwitch()} />
    <label class="form-check-label" for="gs-min-switch"></label>
  </div>
  <label for="gs-min-switch" use:tooltip={$t("Settings_MinimizeOnSwitch")}>{$t("Settings_MinimizeOnSwitch")}</label>
</div>

{#if isWindows}
  <div class="rowSetting">
    <div class="form-check">
      <input id="gs-start-tray-win" type="checkbox" checked={startTrayWithWindows} disabled={startTrayLoading} on:change={() => void toggleStartTrayWithWindows()} />
      <label class="form-check-label" for="gs-start-tray-win"></label>
    </div>
    <label for="gs-start-tray-win" use:tooltip={$t("Settings_Tray_StartWindows")}>{$t("Settings_Tray_StartWindows")}</label>

    <div class="form-check">
      <input id="gs-exit-tray" type="checkbox" checked={exitToTray} disabled={exitToTrayLoading} on:change={() => void toggleExitToTray()} />
      <label class="form-check-label" for="gs-exit-tray"></label>
    </div>
    <label for="gs-exit-tray" use:tooltip={$t("Settings_ExitToTray")}>{$t("Settings_ExitToTray")}</label>
  </div>

  <div class="rowSetting">
    <div class="form-check">
      <input id="gs-protocol" type="checkbox" checked={protocolEnabled} disabled={protocolLoading} on:change={() => void toggleProtocol()} />
      <label class="form-check-label" for="gs-protocol"></label>
    </div>
    <label for="gs-protocol" use:tooltip={$t("Settings_Protocol")}>{$t("Settings_Protocol")}</label>
  </div>
{/if}

<div class="rowSetting">
  <div class="form-check">
    <input id="gs-start-centered" type="checkbox" checked={startProgramCentered} disabled={startCenteredLoading} on:change={() => void toggleStartProgramCentered()} />
    <label class="form-check-label" for="gs-start-centered"></label>
  </div>
  <label for="gs-start-centered" use:tooltip={$t("Settings_StartCentered")}>{$t("Settings_StartCentered")}</label>
</div>

<div class="rowSetting">
  <div class="form-check">
    <input type="checkbox" id="settings-animations" checked={animationsEnabledLocal} disabled={animationsLoading} on:change={() => void toggleAnimations()} />
    <label class="form-check-label" for="settings-animations"></label>
  </div>
  <label for="settings-animations">{$t("Settings_AnimationsEnabled")}</label>
</div>

{#if isWindows}
  <div class="rowSetting">
    <div class="form-check">
      <input id="gs-desktop-home" type="checkbox" checked={desktopHomeShortcut} disabled={desktopShortcutLoading} on:change={() => void toggleDesktopHomeShortcut()} />
      <label class="form-check-label" for="gs-desktop-home"></label>
    </div>
    <label for="gs-desktop-home">{$t("Settings_DesktopShortcut")}</label>
  </div>
{/if}

<div class="rowDropdown version-row">
  <span>{formatAppVersion(currentVersion || "0.0.0")}</span>
  <button
    type="button"
    class="btnicontext"
    disabled={updateCheckLoading}
    on:click={() => void onCheckForUpdates()}
  >
    {$t("Button_CheckForUpdates")}
  </button>
</div>

<h2 class="SettingsHeader">{$t("Settings_Header_StatsSharing")}</h2>

<div class="rowSetting">
  <div class="form-check">
    <input id="gs-stats-enabled" type="checkbox" checked={statsEnabled} disabled={statsEnabledLoading} on:change={() => void toggleStatsEnabled()} />
    <label class="form-check-label" for="gs-stats-enabled"></label>
  </div>
  <label for="gs-stats-enabled">{$t("Settings_CollectStats")}</label>
  <div class="form-check">
    <input id="gs-stats-share" type="checkbox" checked={statsShare} disabled={statsShareLoading || !statsEnabled} on:change={() => void toggleStatsShare()} />
    <label class="form-check-label" for="gs-stats-share"></label>
  </div>
  <label for="gs-stats-share">{$t("Settings_ShareStats")}</label>
  <button type="button" class="btnicontext" on:click={() => void openStatsModal()}>
    {$t("Settings_ViewStats")}
  </button>
</div>

<div class="rowSetting">
  <div class="form-check">
    <input id="gs-discord-rpc" type="checkbox" checked={discordRpc} disabled={discordRpcLoading || $offlineMode} on:change={() => void toggleDiscordRpc()} />
    <label class="form-check-label" for="gs-discord-rpc"></label>
  </div>
  <label for="gs-discord-rpc">{$t("Settings_DiscordRpc")}</label>
  <div class="form-check">
    <input id="gs-discord-rpc-share" type="checkbox" checked={discordRpcShare} disabled={discordRpcShareLoading || $offlineMode || !discordRpc} on:change={() => void toggleDiscordRpcShare()} />
    <label class="form-check-label" for="gs-discord-rpc-share"></label>
  </div>
  <label for="gs-discord-rpc-share">{$t("Settings_DiscordRpcShare")}</label>
</div>

<div class="rowSetting">
  <button type="button" class="btnicontext" on:click={() => void openFeedbackModal({ mode: "suggestion" })}>
    {$t("Settings_SuggestFeature")}
  </button>
</div>

<style lang="scss">
  button:not(.fancyLink) {
    position: relative;
    height: 38px;
  }

  .version-row {
    margin-top: 0.25rem;
  }
</style>

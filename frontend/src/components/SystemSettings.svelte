<script lang="ts">
  import { onMount } from "svelte";
  import { get, writable } from "svelte/store";
  import { t } from "../stores/i18n";
  import { pushToast } from "../stores/toast";
  import { formatToastWithError } from "../lib/formatWailsError";
  import { tooltip } from "../lib/actions/tooltip";
  import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
  import { offlineMode, setUserOfflineMode } from "../stores/offlineMode";
  import { openFeedbackModal } from "../stores/modal";
  import { animationsEnabled, loadAnimationsEnabled, setAnimationsEnabled } from "../stores/animationSettings";
  import { formatAppVersion } from "../lib/checkForUpdates";
  import { createToggle } from "../lib/useToggleSetting";
  import { openMoveUserDataModal, openUserDataFolder, onCheckForUpdates, openStatsModal } from "../lib/settingsOperations";

  let isWindows = false;
  let currentVersion = "";
  let userDataPath = "";

  const userDataMoveLoading = writable(false);
  const updateCheckLoading = writable(false);
  const offlineLoading = writable(false);

  const exitToTray = createToggle(
    () => PlatformService.GetExitToTray(),
    (v) => PlatformService.SetExitToTray(v),
    get(t)("Settings_ExitToTray"),
  );

  const minimizeOnSwitch = createToggle(
    () => PlatformService.GetMinimizeOnSwitch(),
    (v) => PlatformService.SetMinimizeOnSwitch(v),
    get(t)("Settings_MinimizeOnSwitch"),
  );

  const startTrayWithWindows = createToggle(
    () => PlatformService.GetStartTrayWithWindows(),
    (v) => PlatformService.SetStartTrayWithWindows(v),
    get(t)("Settings_Tray_StartWindows"),
  );

  const startProgramCentered = createToggle(
    () => PlatformService.GetStartProgramCentered(),
    (v) => PlatformService.SetStartProgramCentered(v),
    get(t)("Settings_StartCentered"),
  );

  const statsEnabled = createToggle(
    () => PlatformService.GetStatsEnabled(),
    (v) => PlatformService.SetStatsEnabled(v),
    get(t)("Settings_CollectStats"),
  );

  const crashReportAutoSubmit = createToggle(
    () => PlatformService.GetCrashReportAutoSubmit(),
    (v) => PlatformService.SetCrashReportAutoSubmit(v),
    get(t)("Settings_CrashReportAutoSubmit"),
    () => !get(offlineMode),
  );

  const protocol = createToggle(
    () => PlatformService.GetProtocolEnabled(),
    (v) => PlatformService.SetProtocolEnabled(v),
    get(t)("Settings_Protocol"),
  );
  protocol.toggle = async () => {
    if (get(protocol.loading)) return;
    const next = !get(protocol.value);
    protocol.loading.set(true);
    try {
      await PlatformService.SetProtocolEnabled(next);
      protocol.value.set(next);
      pushToast({ type: "success", message: next ? $t("Toast_ProtocolEnabled") : $t("Toast_ProtocolDisabled"), duration: 6000 });
    } catch (e) {
      pushToast({ type: "error", message: formatToastWithError($t("Toast_SaveFailed"), e), duration: 8000 });
    } finally {
      protocol.loading.set(false);
    }
  };

  const animations = createToggle(
    async () => { await loadAnimationsEnabled(); return get(animationsEnabled); },
    (v) => setAnimationsEnabled(v),
    get(t)("Settings_AnimationsEnabled"),
  );
  animations.toggle = async () => {
    if (get(animations.loading)) return;
    const next = !get(animations.value);
    animations.value.set(next);
    animations.loading.set(true);
    try {
      await setAnimationsEnabled(next);
      pushToast({ type: "success", message: get(t)("Toast_SavedItem", { item: get(t)("Settings_AnimationsEnabled") }), duration: 3000 });
    } catch (e) {
      animations.value.set(!next);
      pushToast({ type: "error", message: formatToastWithError($t("Toast_SaveFailed"), e), duration: 8000 });
    } finally {
      animations.loading.set(false);
    }
  };

  const desktopHomeShortcut = createToggle(
    () => PlatformService.GetDesktopHomeShortcutExists(),
    (v) => PlatformService.SetDesktopHomeShortcut(v),
    get(t)("Settings_DesktopShortcut"),
  );
  async function refreshDesktopShortcutState(): Promise<void> {
    try {
      desktopHomeShortcut.value.set(await PlatformService.GetDesktopHomeShortcutExists());
    } catch {
      desktopHomeShortcut.value.set(false);
    }
  }
  desktopHomeShortcut.toggle = async () => {
    if (get(desktopHomeShortcut.loading)) return;
    const next = !get(desktopHomeShortcut.value);
    desktopHomeShortcut.loading.set(true);
    try {
      await PlatformService.SetDesktopHomeShortcut(next);
      await refreshDesktopShortcutState();
      pushToast({ type: "success", message: get(t)("Toast_SavedItem", { item: get(t)("Settings_DesktopShortcut") }), duration: 4000 });
    } catch (e) {
      pushToast({ type: "error", message: formatToastWithError($t("Toast_SaveFailed"), e), duration: 8000 });
    } finally {
      desktopHomeShortcut.loading.set(false);
    }
  };

  const discordRpc = createToggle(
    () => PlatformService.GetDiscordRpc(),
    (v) => PlatformService.SetDiscordRpc(v),
    get(t)("Settings_DiscordRpc"),
    () => !get(offlineMode),
  );
  discordRpc.toggle = async () => {
    if (get(discordRpc.loading) || get(offlineMode)) return;
    const next = !get(discordRpc.value);
    discordRpc.loading.set(true);
    try {
      await PlatformService.SetDiscordRpc(next);
      discordRpc.value.set(next);
      if (!next) discordRpcShare.value.set(false);
      pushToast({ type: "success", message: get(t)("Toast_SavedItem", { item: get(t)("Settings_DiscordRpc") }), duration: 4000 });
    } catch (e) {
      pushToast({ type: "error", message: formatToastWithError($t("Toast_SaveFailed"), e), duration: 8000 });
    } finally {
      discordRpc.loading.set(false);
    }
  };

  const statsShare = createToggle(
    () => PlatformService.GetStatsShare(),
    (v) => PlatformService.SetStatsShare(v),
    get(t)("Settings_ShareStats"),
    () => get(statsEnabled.value),
  );

  const discordRpcShare = createToggle(
    () => PlatformService.GetDiscordRpcShare(),
    (v) => PlatformService.SetDiscordRpcShare(v),
    get(t)("Settings_DiscordRpcShare"),
    () => !get(offlineMode) && get(discordRpc.value),
  );

  async function toggleOfflineMode(): Promise<void> {
    if (get(offlineLoading)) return;
    const next = !get(offlineMode);
    offlineLoading.set(true);
    try {
      await setUserOfflineMode(next);
      if (next) {
        discordRpc.value.set(false);
        discordRpcShare.value.set(false);
      }
      pushToast({ type: "success", message: next ? $t("Toast_OfflineModeEnabled") : $t("Toast_OfflineModeDisabled"), duration: 6000 });
    } catch (e) {
      pushToast({ type: "error", message: formatToastWithError($t("Toast_SaveFailed"), e), duration: 8000 });
    } finally {
      offlineLoading.set(false);
    }
  }

  onMount(() => {
    isWindows = /windows/i.test(navigator.userAgent) || /win32/i.test(navigator.userAgent);
    void protocol.init();
    void exitToTray.init();
    void statsEnabled.init();
    void statsShare.init();
    void crashReportAutoSubmit.init();
    void discordRpc.init();
    void discordRpcShare.init();
    void minimizeOnSwitch.init();
    void startProgramCentered.init();
    void animations.init();
    void PlatformService.GetAppVersion()
      .then((v) => { currentVersion = v || ""; })
      .catch(() => { currentVersion = ""; });
    void PlatformService.GetUserDataLocation()
      .then((v) => { userDataPath = v || ""; })
      .catch(() => { userDataPath = ""; });
    if (isWindows) {
      void startTrayWithWindows.init();
      void desktopHomeShortcut.init();
    }
  });
</script>

<h2 class="SettingsHeader">{$t("Settings_Header_System")}</h2>

<div class="multilineSetting">
  <span>{$t("Settings_CurrentDataLocation", { path: userDataPath || "…" })}</span>
  <span>
    <button
      type="button"
      class="fancyLink"
      disabled={$userDataMoveLoading}
      on:click={() => void openMoveUserDataModal(userDataMoveLoading, userDataPath)}
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
    <input id="gs-offline" type="checkbox" checked={$offlineMode} disabled={$offlineLoading} on:change={() => void toggleOfflineMode()} />
    <label class="form-check-label" for="gs-offline"></label>
  </div>
  <label for="gs-offline" use:tooltip={$t("Settings_OfflineMode")}>{$t("Settings_OfflineMode")}</label>
</div>

<div class="rowSetting">
  <div class="form-check">
    <input id="gs-min-switch" type="checkbox" checked={$minimizeOnSwitch.value} disabled={$minimizeOnSwitch.loading} on:change={() => void minimizeOnSwitch.toggle()} />
    <label class="form-check-label" for="gs-min-switch"></label>
  </div>
  <label for="gs-min-switch" use:tooltip={$t("Settings_MinimizeOnSwitch")}>{$t("Settings_MinimizeOnSwitch")}</label>
</div>

{#if isWindows}
  <div class="rowSetting">
    <div class="form-check">
      <input id="gs-start-tray-win" type="checkbox" checked={$startTrayWithWindows.value} disabled={$startTrayWithWindows.loading} on:change={() => void startTrayWithWindows.toggle()} />
      <label class="form-check-label" for="gs-start-tray-win"></label>
    </div>
    <label for="gs-start-tray-win" use:tooltip={$t("Settings_Tray_StartWindows")}>{$t("Settings_Tray_StartWindows")}</label>

    <div class="form-check">
      <input id="gs-exit-tray" type="checkbox" checked={$exitToTray.value} disabled={$exitToTray.loading} on:change={() => void exitToTray.toggle()} />
      <label class="form-check-label" for="gs-exit-tray"></label>
    </div>
    <label for="gs-exit-tray" use:tooltip={$t("Settings_ExitToTray")}>{$t("Settings_ExitToTray")}</label>
  </div>

  <div class="rowSetting">
    <div class="form-check">
      <input id="gs-protocol" type="checkbox" checked={$protocol.value} disabled={$protocol.loading} on:change={() => void protocol.toggle()} />
      <label class="form-check-label" for="gs-protocol"></label>
    </div>
    <label for="gs-protocol" use:tooltip={$t("Settings_Protocol")}>{$t("Settings_Protocol")}</label>
  </div>
{/if}

<div class="rowSetting">
  <div class="form-check">
    <input id="gs-start-centered" type="checkbox" checked={$startProgramCentered.value} disabled={$startProgramCentered.loading} on:change={() => void startProgramCentered.toggle()} />
    <label class="form-check-label" for="gs-start-centered"></label>
  </div>
  <label for="gs-start-centered" use:tooltip={$t("Settings_StartCentered")}>{$t("Settings_StartCentered")}</label>
</div>

<div class="rowSetting">
  <div class="form-check">
    <input type="checkbox" id="settings-animations" checked={$animations.value} disabled={$animations.loading} on:change={() => void animations.toggle()} />
    <label class="form-check-label" for="settings-animations"></label>
  </div>
  <label for="settings-animations">{$t("Settings_AnimationsEnabled")}</label>
</div>

{#if isWindows}
  <div class="rowSetting">
    <div class="form-check">
      <input id="gs-desktop-home" type="checkbox" checked={$desktopHomeShortcut.value} disabled={$desktopHomeShortcut.loading} on:change={() => void desktopHomeShortcut.toggle()} />
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
    disabled={$updateCheckLoading}
    on:click={() => void onCheckForUpdates(updateCheckLoading)}
  >
    {$t("Button_CheckForUpdates")}
  </button>
</div>

<h2 class="SettingsHeader">{$t("Settings_Header_StatsSharing")}</h2>

<div class="rowSetting">
  <div class="form-check">
    <input
      id="gs-crash-report-auto-submit"
      type="checkbox"
      checked={$crashReportAutoSubmit.value}
      disabled={$crashReportAutoSubmit.loading || $offlineMode}
      on:change={() => void crashReportAutoSubmit.toggle()}
    />
    <label class="form-check-label" for="gs-crash-report-auto-submit"></label>
  </div>
  <label for="gs-crash-report-auto-submit">{$t("Settings_CrashReportAutoSubmit")}</label>
</div>

<div class="rowSetting">
  <div class="form-check">
    <input id="gs-stats-enabled" type="checkbox" checked={$statsEnabled.value} disabled={$statsEnabled.loading} on:change={() => void statsEnabled.toggle()} />
    <label class="form-check-label" for="gs-stats-enabled"></label>
  </div>
  <label for="gs-stats-enabled">{$t("Settings_CollectStats")}</label>
  <div class="form-check">
    <input id="gs-stats-share" type="checkbox" checked={$statsShare.value} disabled={$statsShare.loading || !$statsEnabled.value} on:change={() => void statsShare.toggle()} />
    <label class="form-check-label" for="gs-stats-share"></label>
  </div>
  <label for="gs-stats-share">{$t("Settings_ShareStats")}</label>
  <button type="button" class="btnicontext" on:click={() => void openStatsModal()}>
    {$t("Settings_ViewStats")}
  </button>
</div>

<div class="rowSetting">
  <div class="form-check">
    <input id="gs-discord-rpc" type="checkbox" checked={$discordRpc.value} disabled={$discordRpc.loading || $offlineMode} on:change={() => void discordRpc.toggle()} />
    <label class="form-check-label" for="gs-discord-rpc"></label>
  </div>
  <label for="gs-discord-rpc">{$t("Settings_DiscordRpc")}</label>
  <div class="form-check">
    <input id="gs-discord-rpc-share" type="checkbox" checked={$discordRpcShare.value} disabled={$discordRpcShare.loading || $offlineMode || !$discordRpc.value} on:change={() => void discordRpcShare.toggle()} />
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

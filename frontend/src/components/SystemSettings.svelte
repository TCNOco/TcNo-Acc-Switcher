<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import { get, writable } from "svelte/store";
  import { t } from "../stores/i18n";
  import { pushToast } from "../stores/toast";
  import { formatToastWithError } from "../lib/formatWailsError";
  import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
  import { offlineMode, setUserOfflineMode } from "../stores/offlineMode";
  import {
    setAutoStreamerMode,
    setStreamerMode,
    streamerState,
  } from "../stores/streamerMode";
  import { openConfirm, openFeedbackModal, openPasswordSetupModal, openPrompt } from "../stores/modal";
  import { animationsEnabled, loadAnimationsEnabled, setAnimationsEnabled } from "../stores/animationSettings";
  import SettingsGroup from "./settings/SettingsGroup.svelte";
  import SettingsToggle from "./settings/SettingsToggle.svelte";
  import SettingsField from "./settings/SettingsField.svelte";
  import {
    deleteQuarantine,
    disableSavedAccountEncryption,
    enableSavedAccountEncryption,
    isWeakPassword,
    listQuarantines,
    loadSecurityStatus,
    repairInterruptedRestore,
    removeAppPassword,
    retryQuarantineImport,
    securityStatus,
    setAppPassword,
    type SecurityQuarantineInfo,
  } from "../stores/security";
  import {
    commandPaletteHotkey,
    formatCommandPaletteHotkeyEvent,
    loadCommandPaletteHotkey,
    normalizeCommandPaletteHotkey,
    setCommandPaletteHotkey,
  } from "../stores/commandPalette";
  import {
    applyControllerSupportEnabled,
    loadControllerSupportEnabled,
    setControllerSupportEnabled,
  } from "../stores/controllerSupport";
  import { formatAppVersion } from "../lib/checkForUpdates";
  import { createToggle } from "../lib/useToggleSetting";
  import { openMoveUserDataModal, openUserDataFolder, onCheckForUpdates, openStatsModal } from "../lib/settingsOperations";

  let isWindows = false;
  let currentVersion = "";
  let userDataPath = "";

  const userDataMoveLoading = writable(false);
  const updateCheckLoading = writable(false);
  const offlineLoading = writable(false);
  const securityLoading = writable(false);
  let quarantines: SecurityQuarantineInfo[] = [];
  let commandPaletteHotkeyCaptureActive = false;

  function stopCommandPaletteHotkeyCapture(): void {
    if (!commandPaletteHotkeyCaptureActive) return;
    commandPaletteHotkeyCaptureActive = false;
    window.removeEventListener("keydown", onCommandPaletteHotkeyCaptureKeydown, true);
  }

  function startCommandPaletteHotkeyCapture(): void {
    if (commandPaletteHotkeyCaptureActive) return;
    commandPaletteHotkeyCaptureActive = true;
    window.addEventListener("keydown", onCommandPaletteHotkeyCaptureKeydown, true);
  }

  function toggleCommandPaletteHotkeyCapture(): void {
    if (commandPaletteHotkeyCaptureActive) {
      stopCommandPaletteHotkeyCapture();
      return;
    }
    startCommandPaletteHotkeyCapture();
  }

  function onCommandPaletteHotkeyCaptureKeydown(e: KeyboardEvent): void {
    if (!commandPaletteHotkeyCaptureActive) return;
    e.preventDefault();
    e.stopPropagation();
    if (e.key === "Escape") {
      stopCommandPaletteHotkeyCapture();
      return;
    }
    const next = formatCommandPaletteHotkeyEvent(e);
    if (!next) return;
    stopCommandPaletteHotkeyCapture();
    void setCommandPaletteHotkey(next);
  }

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

  const streamerMode = createToggle(
    () => PlatformService.GetStreamerMode(),
    (v) => setStreamerMode(v),
    get(t)("Settings_StreamerMode"),
  );

  const autoStreamerMode = createToggle(
    () => PlatformService.GetAutoStreamerMode(),
    (v) => setAutoStreamerMode(v),
    get(t)("Settings_AutoStreamerMode"),
  );

  const hideFromScreenshots = createToggle(
    () => PlatformService.GetHideFromScreenshots(),
    (v) => PlatformService.SetHideFromScreenshots(v),
    get(t)("Settings_HideFromScreenshots"),
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
    const prev = get(protocol.value);
    const next = !prev;
    protocol.loading.set(true);
    protocol.value.set(next);
    try {
      await PlatformService.SetProtocolEnabled(next);
      pushToast({ type: "success", message: next ? $t("Toast_ProtocolEnabled") : $t("Toast_ProtocolDisabled"), duration: 6000 });
    } catch (e) {
      protocol.value.set(prev);
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
      animations.value.set(get(animationsEnabled));
      pushToast({ type: "error", message: formatToastWithError($t("Toast_SaveFailed"), e), duration: 8000 });
    } finally {
      animations.loading.set(false);
    }
  };

  const controllerSupport = createToggle(
    () => loadControllerSupportEnabled(),
    (v) => setControllerSupportEnabled(v),
    get(t)("Settings_ControllerSupport"),
  );

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
    desktopHomeShortcut.value.set(next);
    try {
      await PlatformService.SetDesktopHomeShortcut(next);
      await refreshDesktopShortcutState();
      pushToast({ type: "success", message: get(t)("Toast_SavedItem", { item: get(t)("Settings_DesktopShortcut") }), duration: 4000 });
    } catch (e) {
      /* The shortcut either exists on disk or it doesn't — re-read rather than
         assume the failed call left the previous state intact. */
      await refreshDesktopShortcutState();
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
    const prev = get(discordRpc.value);
    const prevShare = get(discordRpcShare.value);
    const next = !prev;
    discordRpc.loading.set(true);
    discordRpc.value.set(next);
    if (!next) discordRpcShare.value.set(false);
    try {
      await PlatformService.SetDiscordRpc(next);
      pushToast({ type: "success", message: get(t)("Toast_SavedItem", { item: get(t)("Settings_DiscordRpc") }), duration: 4000 });
    } catch (e) {
      discordRpc.value.set(prev);
      discordRpcShare.value.set(prevShare);
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

  const prereleaseUpdates = createToggle(
    () => PlatformService.GetPrereleaseUpdates(),
    (v) => PlatformService.SetPrereleaseUpdates(v),
    get(t)("Settings_PrereleaseUpdates"),
  );

  const discordRpcShare = createToggle(
    () => PlatformService.GetDiscordRpcShare(),
    (v) => PlatformService.SetDiscordRpcShare(v),
    get(t)("Settings_DiscordRpcShare"),
    () => !get(offlineMode) && get(discordRpc.value),
  );

  type SettingsSnapshot = Awaited<ReturnType<typeof PlatformService.ReadSettings>>;

  function applySettingsSnapshot(settings: SettingsSnapshot): void {
    offlineMode.set(settings.offlineMode);
    protocol.value.set(settings.protocolEnabled);
    exitToTray.value.set(settings.exitToTray);
    statsEnabled.value.set(settings.statsEnabled);
    statsShare.value.set(settings.statsShare);
    prereleaseUpdates.value.set(settings.prereleaseUpdates);
    crashReportAutoSubmit.value.set(settings.crashReportAutoSubmit);
    discordRpc.value.set(settings.discordRpc);
    discordRpcShare.value.set(settings.discordRpcShare);
    minimizeOnSwitch.value.set(settings.minimizeOnSwitch);
    startTrayWithWindows.value.set(settings.startTrayWithWindows);
    startProgramCentered.value.set(settings.startProgramCentered);
    streamerMode.value.set(settings.streamerMode);
    autoStreamerMode.value.set(settings.autoStreamerMode);
    hideFromScreenshots.value.set(settings.hideFromScreenshots);
    animationsEnabled.set(settings.animationsEnabled);
    animations.value.set(settings.animationsEnabled);
    controllerSupport.value.set(applyControllerSupportEnabled(settings.controllerSupportEnabled));
    commandPaletteHotkey.set(normalizeCommandPaletteHotkey(settings.commandPaletteHotkey));
    currentVersion = settings.appVersion || "";
  }

  async function initSettingsIndividually(): Promise<void> {
    void protocol.init();
    void exitToTray.init();
    void statsEnabled.init();
    void statsShare.init();
    void prereleaseUpdates.init();
    void crashReportAutoSubmit.init();
    void discordRpc.init();
    void discordRpcShare.init();
    void minimizeOnSwitch.init();
    void startProgramCentered.init();
    void streamerMode.init();
    void autoStreamerMode.init();
    if (isWindows) {
      void hideFromScreenshots.init();
    }
    void animations.init();
    void controllerSupport.init();
    void loadCommandPaletteHotkey();
    void PlatformService.GetAppVersion()
      .then((v) => { currentVersion = v || ""; })
      .catch(() => { currentVersion = ""; });
    if (isWindows) {
      void startTrayWithWindows.init();
    }
  }

  async function hydrateSettings(): Promise<void> {
    try {
      applySettingsSnapshot(await PlatformService.ReadSettings());
    } catch {
      await initSettingsIndividually();
    }
  }

  /* `setUserOfflineMode` only writes the store once the backend call succeeds, so
     this has the same stranded-checkbox problem as the toggles above. */
  async function toggleOfflineMode(): Promise<void> {
    if (get(offlineLoading)) return;
    const prev = get(offlineMode);
    const prevRpc = get(discordRpc.value);
    const prevRpcShare = get(discordRpcShare.value);
    const next = !prev;
    offlineLoading.set(true);
    offlineMode.set(next);
    if (next) {
      discordRpc.value.set(false);
      discordRpcShare.value.set(false);
    }
    try {
      await setUserOfflineMode(next);
      pushToast({ type: "success", message: next ? $t("Toast_OfflineModeEnabled") : $t("Toast_OfflineModeDisabled"), duration: 6000 });
    } catch (e) {
      offlineMode.set(prev);
      discordRpc.value.set(prevRpc);
      discordRpcShare.value.set(prevRpcShare);
      pushToast({ type: "error", message: formatToastWithError($t("Toast_SaveFailed"), e), duration: 8000 });
    } finally {
      offlineLoading.set(false);
    }
  }

  async function refreshSecurity(): Promise<void> {
    try {
      const status = await loadSecurityStatus();
      quarantines = status.quarantineCount > 0 ? await listQuarantines() : [];
    } catch {
      quarantines = [];
    }
  }

  async function promptSecurityPassword(title: string, body: string): Promise<string | null> {
    return openPrompt({
      title,
      body,
      inputType: "password",
      positiveLabel: $t("Ok"),
      negativeLabel: $t("Button_Cancel"),
    });
  }

  async function onSetAppPassword(): Promise<void> {
    if (get(securityLoading)) return;
    const result = await openPasswordSetupModal({
      title: $t("Security_SetAppPassword"),
      positiveLabel: $t("Security_SetAppPassword"),
      negativeLabel: $t("Button_Cancel"),
    });
    if (!result) return;
    securityLoading.set(true);
    try {
      await setAppPassword(result.password);
      await refreshSecurity();
      pushToast({ type: "success", message: $t("Security_AppPasswordSet"), duration: 4000 });
    } catch (e) {
      pushToast({ type: "error", message: formatToastWithError($t("Toast_SaveFailed"), e), duration: 8000 });
    } finally {
      securityLoading.set(false);
    }
  }

  async function onRemoveAppPassword(): Promise<void> {
    if (get(securityLoading)) return;
    const password = await promptSecurityPassword(
      $t("Security_RemoveAppPassword"),
      $t($securityStatus.savedAccountDataEncrypted ? "Security_RemovePasswordEncryptedBody" : "Security_CurrentPasswordBody"),
    );
    if (password === null) return;
    securityLoading.set(true);
    try {
      await removeAppPassword(password);
      await refreshSecurity();
      pushToast({ type: "success", message: $t("Security_AppPasswordRemoved"), duration: 5000 });
    } catch (e) {
      pushToast({ type: "error", message: formatToastWithError($t("Security_PasswordActionFailed"), e), duration: 8000 });
    } finally {
      securityLoading.set(false);
    }
  }

  async function onToggleSavedDataEncryption(next: boolean): Promise<void> {
    if (get(securityLoading)) return;
    const password = await promptSecurityPassword(
      next ? $t("Security_EnableEncryption") : $t("Security_DisableEncryption"),
      next ? $t("Security_EnableEncryptionBody") : $t("Security_DisableEncryptionBody"),
    );
    if (password === null) {
      await refreshSecurity();
      return;
    }
    if (next && isWeakPassword(password)) {
      const ok = await openConfirm({
        title: $t("Security_WeakPasswordTitle"),
        body: $t("Security_WeakPasswordBody"),
        positiveLabel: $t("Security_WeakPasswordContinue"),
        negativeLabel: $t("Button_Cancel"),
        style: "okcancel",
      });
      if (!ok) {
        await refreshSecurity();
        return;
      }
    }
    securityLoading.set(true);
    try {
      if (next) {
        await enableSavedAccountEncryption(password);
      } else {
        await disableSavedAccountEncryption(password);
      }
      await refreshSecurity();
      pushToast({ type: "success", message: next ? $t("Security_EncryptionEnabled") : $t("Security_EncryptionDisabled"), duration: 5000 });
    } catch (e) {
      await refreshSecurity();
      pushToast({ type: "error", message: formatToastWithError($t("Security_PasswordActionFailed"), e), duration: 8000 });
    } finally {
      securityLoading.set(false);
    }
  }

  function onSavedDataEncryptionChange(next: boolean): void {
    if (get(securityLoading) || $securityStatus.operationBusy) return;
    void onToggleSavedDataEncryption(next);
  }

  async function onRetryQuarantine(id: string): Promise<void> {
    const password = await promptSecurityPassword($t("Security_QuarantineRetry"), $t("Security_QuarantineRetryBody"));
    if (password === null) return;
    securityLoading.set(true);
    try {
      await retryQuarantineImport(id, password);
      await refreshSecurity();
      pushToast({ type: "success", message: $t("Security_QuarantineRetryDone"), duration: 5000 });
    } catch (e) {
      pushToast({ type: "error", message: formatToastWithError($t("Security_PasswordActionFailed"), e), duration: 8000 });
    } finally {
      securityLoading.set(false);
    }
  }

  async function onDeleteQuarantine(id: string): Promise<void> {
    const ok = await openConfirm({
      title: $t("Security_QuarantineDelete"),
      body: $t("Security_QuarantineDeleteBody"),
      positiveLabel: $t("Security_QuarantineDelete"),
      negativeLabel: $t("Button_Cancel"),
      style: "okcancel",
    });
    if (!ok) return;
    securityLoading.set(true);
    try {
      await deleteQuarantine(id);
      await refreshSecurity();
    } finally {
      securityLoading.set(false);
    }
  }

  async function onRepairInterruptedRestore(): Promise<void> {
    const ok = await openConfirm({
      title: $t("Security_InterruptedRestore_Title"),
      body: $t("Security_InterruptedRestore_Body"),
      positiveLabel: $t("Security_InterruptedRestore_Repair"),
      negativeLabel: $t("Security_InterruptedRestore_Later"),
      style: "yesno",
    });
    if (!ok) return;
    securityLoading.set(true);
    try {
      await repairInterruptedRestore();
      await refreshSecurity();
      pushToast({ type: "success", message: $t("Security_InterruptedRestore_Repaired"), duration: 5000 });
    } catch (e) {
      pushToast({ type: "error", message: formatToastWithError($t("Security_InterruptedRestore_RepairFailed"), e), duration: 8000 });
    } finally {
      securityLoading.set(false);
    }
  }

  onMount(() => {
    isWindows = /windows/i.test(navigator.userAgent) || /win32/i.test(navigator.userAgent);
    void hydrateSettings();
    void PlatformService.GetUserDataLocation()
      .then((v) => { userDataPath = v || ""; })
      .catch(() => { userDataPath = ""; });
    void refreshSecurity();
    if (isWindows) {
      void desktopHomeShortcut.init();
    }
  });

  onDestroy(() => {
    stopCommandPaletteHotkeyCapture();
  });
</script>

<SettingsGroup title={$t("Settings_Header_GeneralSettings")}>
  <div class="settings-grid">
    <SettingsToggle
      id="gs-offline"
      checked={$offlineMode}
      disabled={$offlineLoading}
      label={$t("Settings_OfflineMode")}
      span
      on:change={() => void toggleOfflineMode()}
    />
    <SettingsToggle
      id="gs-min-switch"
      checked={$minimizeOnSwitch.value}
      disabled={$minimizeOnSwitch.loading}
      label={$t("Settings_MinimizeOnSwitch")}
      on:change={() => void minimizeOnSwitch.toggle()}
    />
    <SettingsToggle
      id="gs-start-centered"
      checked={$startProgramCentered.value}
      disabled={$startProgramCentered.loading}
      label={$t("Settings_StartCentered")}
      on:change={() => void startProgramCentered.toggle()}
    />
    <SettingsToggle
      id="settings-animations"
      checked={$animations.value}
      disabled={$animations.loading}
      label={$t("Settings_AnimationsEnabled")}
      on:change={() => void animations.toggle()}
    />
    <SettingsToggle
      id="settings-controller-support"
      checked={$controllerSupport.value}
      disabled={$controllerSupport.loading}
      label={$t("Settings_ControllerSupport")}
      on:change={() => void controllerSupport.toggle()}
    />
    {#if isWindows}
      <SettingsToggle
        id="gs-desktop-home"
        checked={$desktopHomeShortcut.value}
        disabled={$desktopHomeShortcut.loading}
        label={$t("Settings_DesktopShortcut")}
        on:change={() => void desktopHomeShortcut.toggle()}
      />
    {/if}
  </div>

  <SettingsField label={$t("Settings_CommandPaletteHotkey")}>
    <button
      type="button"
      class="btnicontext hotkey-button"
      class:capturing={commandPaletteHotkeyCaptureActive}
      aria-pressed={commandPaletteHotkeyCaptureActive}
      on:click={toggleCommandPaletteHotkeyCapture}
    >
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M4 4.5h16a3 3 0 0 1 3 3v9a3 3 0 0 1-3 3H4a3 3 0 0 1-3-3v-9a3 3 0 0 1 3-3zM4 7h4v2H4zm6 0h4v2h-4zm6 0h4v2h-4zM4 11h4v2H4zm6 0h4v2h-4zm6 0h4v2h-4zM7 15h10v2H7z" fill-rule="evenodd"/></svg>
      {commandPaletteHotkeyCaptureActive ? $t("Settings_CommandPaletteHotkey_Prompt") : $commandPaletteHotkey}
    </button>
  </SettingsField>
</SettingsGroup>

<SettingsGroup title={$t("Settings_Header_Privacy")}>
  <div class="settings-grid">
    {#if isWindows}
      <SettingsToggle
        id="gs-hide-from-screenshots"
        checked={$hideFromScreenshots.value}
        disabled={$hideFromScreenshots.loading}
        label={$t("Settings_HideFromScreenshots")}
        tooltip={$t("Settings_HideFromScreenshots_Tooltip")}
        on:change={() => void hideFromScreenshots.toggle()}
      />
    {/if}
    <div class="settings-stack">
      <SettingsToggle
        id="gs-streamer-mode"
        checked={$streamerMode.value}
        disabled={$streamerMode.loading}
        label={$t("Settings_StreamerMode")}
        tooltip={$t("Settings_StreamerMode_Tooltip")}
        on:change={() => void streamerMode.toggle()}
      />
      <div class="settings-sub">
        <SettingsToggle
          id="gs-auto-streamer-mode"
          checked={$autoStreamerMode.value}
          disabled={$autoStreamerMode.loading}
          label={$t("Settings_AutoStreamerMode")}
          tooltip={$t("Settings_AutoStreamerMode_Tooltip")}
          on:change={() => void autoStreamerMode.toggle()}
        />
      </div>
      {#if $streamerState.autoEnabled && $streamerState.autoActive}
        <p class="settings-note">
          {$t("Settings_AutoStreamerMode_Active", { app: $streamerState.detectedExe })}
        </p>
      {/if}
    </div>
  </div>
</SettingsGroup>

<SettingsGroup title={$t("Settings_Header_System")}>
  <p class="settings-note settings-path">{$t("Settings_CurrentDataLocation", { path: userDataPath || "…" })}</p>
  <p class="settings-links">
    <button
      type="button"
      class="fancyLink"
      disabled={$userDataMoveLoading}
      on:click={() => void openMoveUserDataModal(userDataMoveLoading, userDataPath)}
    >{$t("Settings_SetDataLocation")}</button>
    <button type="button" class="fancyLink" on:click={() => void openUserDataFolder()}
      >{$t("Settings_OpenUserDataFolder")}</button>
  </p>

  {#if isWindows}
    <div class="settings-grid">
      <SettingsToggle
        id="gs-start-tray-win"
        checked={$startTrayWithWindows.value}
        disabled={$startTrayWithWindows.loading}
        label={$t("Settings_Tray_StartWindows")}
        on:change={() => void startTrayWithWindows.toggle()}
      />
      <SettingsToggle
        id="gs-exit-tray"
        checked={$exitToTray.value}
        disabled={$exitToTray.loading}
        label={$t("Settings_ExitToTray")}
        on:change={() => void exitToTray.toggle()}
      />
      <SettingsToggle
        id="gs-protocol"
        checked={$protocol.value}
        disabled={$protocol.loading}
        label={$t("Settings_Protocol")}
        span
        on:change={() => void protocol.toggle()}
      />
    </div>
  {/if}
</SettingsGroup>

<SettingsGroup title={$t("Settings_Header_Security")}>
  <div class="settings-actions">
    {#if $securityStatus.appPasswordSet}
      <button type="button" class="btnicontext" disabled={$securityLoading} on:click={() => void onRemoveAppPassword()}>
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M2.5 10V8A5.5 5.5 0 0 1 13.5 8V13H10.5V8A2.5 2.5 0 0 0 5.5 8V10ZM9.8 11H20.2A1.8 1.8 0 0 1 22 12.8V20.7A1.8 1.8 0 0 1 20.2 22.5H9.8A1.8 1.8 0 0 1 8 20.7V12.8A1.8 1.8 0 0 1 9.8 11Z"/></svg>
        {$t("Security_RemoveAppPassword")}
      </button>
    {:else}
      <button type="button" class="btnicontext" disabled={$securityLoading} on:click={() => void onSetAppPassword()}>
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M7 6A6 6 0 1 1 7 18 6 6 0 1 1 7 6ZM7 9.4A2.6 2.6 0 1 0 7 14.6 2.6 2.6 0 1 0 7 9.4ZM11 10.2H22.5V17.5H19.8V13.8H17.3V18.5H14.8V13.8H11Z"/></svg>
        {$t("Security_SetAppPassword")}
      </button>
    {/if}
  </div>

  {#if $securityStatus.appPasswordSet}
    <div class="settings-grid">
      <SettingsToggle
        id="security-encrypt-cache"
        checked={$securityStatus.savedAccountDataEncrypted}
        disabled={$securityLoading || $securityStatus.operationBusy}
        label={$t("Security_EncryptSavedAccountData")}
        span
        manual
        on:change={(e) => onSavedDataEncryptionChange(e.detail)}
      />
    </div>
  {/if}

  {#if $securityStatus.interruptedRestorePending}
    <div class="settings-card settings-card--warn">
      <span>{$t("Security_InterruptedRestorePending")}</span>
      <div class="settings-card__row">
        <button type="button" class="btnicontext" disabled={$securityLoading} on:click={() => void onRepairInterruptedRestore()}>
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M5.41 2.59A6 6 0 1 1 2.59 5.41L5.67 8.49A2 2 0 0 0 8.49 5.67zM12.3 8.56L21.5 18.1A2.4 2.4 0 0 1 18.1 21.5L8.9 11.96z"/></svg>
          {$t("Security_InterruptedRestore_Repair")}
        </button>
      </div>
    </div>
  {/if}

  {#if quarantines.length > 0}
    <div class="settings-card settings-card--warn">
      <span>{$t("Security_QuarantineStatus", { count: quarantines.length })}</span>
      {#each quarantines as q}
        <div class="settings-card__row">
          <span class="settings-path">{q.accounts.join(", ")}</span>
          <button type="button" class="btnicontext" disabled={$securityLoading} on:click={() => void onRetryQuarantine(q.id)}>
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" aria-hidden="true"><path d="M463.5 224H472c13.3 0 24-10.7 24-24V72c0-9.7-5.8-18.5-14.8-22.2s-19.3-1.7-26.2 5.2L413.4 96.6c-87.6-86.5-228.7-86.2-315.8 1c-87.5 87.5-87.5 229.3 0 316.8s229.3 87.5 316.8 0c12.5-12.5 12.5-32.8 0-45.3s-32.8-12.5-45.3 0c-62.5 62.5-163.8 62.5-226.3 0s-62.5-163.8 0-226.3c62.2-62.2 162.7-62.5 225.3-1l-30.4 30.4c-6.9 6.9-8.9 17.2-5.2 26.2s12.5 14.8 22.2 14.8H463.5z"/></svg>
            {$t("Security_QuarantineRetry")}
          </button>
          <button type="button" class="btnicontext" disabled={$securityLoading} on:click={() => void onDeleteQuarantine(q.id)}>
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path fill-rule="evenodd" d="M9.5 2h5a1.5 1.5 0 0 1 1.5 1.5V4h4.5a1.2 1.2 0 0 1 1.2 1.2v1.6A1.2 1.2 0 0 1 20.5 8h-17A1.2 1.2 0 0 1 2.3 6.8V5.2A1.2 1.2 0 0 1 3.5 4H8v-.5A1.5 1.5 0 0 1 9.5 2ZM4.9 9.4H19.1L18.1 20.2A2 2 0 0 1 16.11 22H7.89A2 2 0 0 1 5.9 20.2L4.9 9.4ZM9.3 12.2h1.9v7.2H9.3zM12.8 12.2h1.9v7.2h-1.9z"/></svg>
            {$t("Security_QuarantineDelete")}
          </button>
        </div>
      {/each}
    </div>
  {/if}
</SettingsGroup>

<SettingsGroup title={$t("Settings_Header_StatsSharing")}>
  <div class="settings-grid">
    <div class="settings-stack">
      <SettingsToggle
        id="gs-stats-enabled"
        checked={$statsEnabled.value}
        disabled={$statsEnabled.loading}
        label={$t("Settings_CollectStats")}
        on:change={() => void statsEnabled.toggle()}
      />
      <div class="settings-sub">
        <SettingsToggle
          id="gs-stats-share"
          checked={$statsShare.value}
          disabled={$statsShare.loading || !$statsEnabled.value}
          label={$t("Settings_ShareStats")}
          on:change={() => void statsShare.toggle()}
        />
      </div>
    </div>

    <div class="settings-stack">
      <SettingsToggle
        id="gs-discord-rpc"
        checked={$discordRpc.value}
        disabled={$discordRpc.loading || $offlineMode}
        label={$t("Settings_DiscordRpc")}
        on:change={() => void discordRpc.toggle()}
      />
      <div class="settings-sub">
        <SettingsToggle
          id="gs-discord-rpc-share"
          checked={$discordRpcShare.value}
          disabled={$discordRpcShare.loading || $offlineMode || !$discordRpc.value}
          label={$t("Settings_DiscordRpcShare")}
          on:change={() => void discordRpcShare.toggle()}
        />
      </div>
    </div>

    <SettingsToggle
      id="gs-crash-report-auto-submit"
      checked={$crashReportAutoSubmit.value}
      disabled={$crashReportAutoSubmit.loading || $offlineMode}
      label={$t("Settings_CrashReportAutoSubmit")}
      on:change={() => void crashReportAutoSubmit.toggle()}
    />
  </div>

  <div class="settings-actions">
    <button type="button" class="btnicontext" on:click={() => void openStatsModal()}>
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M3.5 13h3a1 1 0 0 1 1 1v6.5a1 1 0 0 1-1 1h-3a1 1 0 0 1-1-1V14a1 1 0 0 1 1-1zM10.5 5.5h3a1 1 0 0 1 1 1v14a1 1 0 0 1-1 1h-3a1 1 0 0 1-1-1v-14a1 1 0 0 1 1-1zM17.5 9.5h3a1 1 0 0 1 1 1v10a1 1 0 0 1-1 1h-3a1 1 0 0 1-1-1v-10a1 1 0 0 1 1-1z"/></svg>
      {$t("Settings_ViewStats")}
    </button>
  </div>
</SettingsGroup>

<SettingsGroup title={$t("Settings_Header_Program")}>
  <SettingsField label={formatAppVersion(currentVersion || "0.0.0")} spread>
    <button
      type="button"
      class="btnicontext"
      disabled={$updateCheckLoading}
      on:click={() => void onCheckForUpdates(updateCheckLoading)}
    >
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" aria-hidden="true"><path d="M463.5 224H472c13.3 0 24-10.7 24-24V72c0-9.7-5.8-18.5-14.8-22.2s-19.3-1.7-26.2 5.2L413.4 96.6c-87.6-86.5-228.7-86.2-315.8 1c-87.5 87.5-87.5 229.3 0 316.8s229.3 87.5 316.8 0c12.5-12.5 12.5-32.8 0-45.3s-32.8-12.5-45.3 0c-62.5 62.5-163.8 62.5-226.3 0s-62.5-163.8 0-226.3c62.2-62.2 162.7-62.5 225.3-1l-30.4 30.4c-6.9 6.9-8.9 17.2-5.2 26.2s12.5 14.8 22.2 14.8H463.5z"/></svg>
      {$t("Button_CheckForUpdates")}
    </button>
    <SettingsToggle
      id="settings-prerelease-updates"
      checked={$prereleaseUpdates.value}
      disabled={$prereleaseUpdates.loading}
      label={$t("Settings_PrereleaseUpdates")}
      on:change={() => void prereleaseUpdates.toggle()}
    />
    <button type="button" class="btnicontext" on:click={() => void openFeedbackModal({ mode: "suggestion" })}>
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M12 1.5c-4.2 0-7.5 3.3-7.5 7.3 0 2.5 1.3 4.3 2.4 5.6.6.7 1 1.3 1.2 1.9h7.8c.2-.6.6-1.2 1.2-1.9 1.1-1.3 2.4-3.1 2.4-5.6 0-4-3.3-7.3-7.5-7.3zM8.2 18.2h7.6v2.4H8.2v-2.4zm1 3.6h5.6c-.6 1.3-1.6 2.2-2.8 2.2s-2.2-.9-2.8-2.2z"/></svg>
      {$t("Settings_SuggestFeature")}
    </button>
  </SettingsField>
</SettingsGroup>

<style lang="scss">
  .hotkey-button {
    position: relative;
    min-width: 7rem;
    height: 38px;
  }

  .hotkey-button.capturing {
    background: var(--accent);
    color: var(--text-on-bright-bg);
  }
</style>

<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import { get, writable } from "svelte/store";
  import { t } from "../stores/i18n";
  import { pushToast } from "../stores/toast";
  import { formatToastWithError } from "../lib/formatWailsError";
  import { tooltip } from "../lib/actions/tooltip";
  import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
  import { offlineMode, setUserOfflineMode } from "../stores/offlineMode";
  import { openConfirm, openFeedbackModal, openPasswordSetupModal, openPrompt } from "../stores/modal";
  import { animationsEnabled, loadAnimationsEnabled, setAnimationsEnabled } from "../stores/animationSettings";
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
    setCommandPaletteHotkey,
  } from "../stores/commandPalette";
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

  function onSavedDataEncryptionClick(e: MouseEvent): void {
    e.preventDefault();
    if (get(securityLoading) || $securityStatus.operationBusy) return;
    void onToggleSavedDataEncryption(!$securityStatus.savedAccountDataEncrypted);
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
    void loadCommandPaletteHotkey();
    void PlatformService.GetAppVersion()
      .then((v) => { currentVersion = v || ""; })
      .catch(() => { currentVersion = ""; });
    void PlatformService.GetUserDataLocation()
      .then((v) => { userDataPath = v || ""; })
      .catch(() => { userDataPath = ""; });
    void refreshSecurity();
    if (isWindows) {
      void startTrayWithWindows.init();
      void desktopHomeShortcut.init();
    }
  });

  onDestroy(() => {
    stopCommandPaletteHotkeyCapture();
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

<div class="security-settings">
  <div class="rowDropdown security-password-row">
    <span>{$t("Settings_Header_Security")}</span>
    {#if $securityStatus.appPasswordSet}
      <span class="security-password-controls">
        <button
          type="button"
          class="btnicontext"
          disabled={$securityLoading}
          on:click={() => void onRemoveAppPassword()}
        >
          {$t("Security_RemoveAppPassword")}
        </button>
        <span class="security-encryption-inline">
          <span class="form-check">
            <input
              id="security-encrypt-cache"
              type="checkbox"
              checked={$securityStatus.savedAccountDataEncrypted}
              disabled={$securityLoading || $securityStatus.operationBusy}
              on:click={onSavedDataEncryptionClick}
            />
            <label class="form-check-label" for="security-encrypt-cache"></label>
          </span>
          <label for="security-encrypt-cache">{$t("Security_EncryptSavedAccountData")}</label>
        </span>
      </span>
    {:else}
      <button
        type="button"
        class="btnicontext"
        disabled={$securityLoading}
        on:click={() => void onSetAppPassword()}
      >
        {$t("Security_SetAppPassword")}
      </button>
    {/if}
  </div>

  {#if $securityStatus.interruptedRestorePending}
    <div class="multilineSetting security-warning">
      <span>{$t("Security_InterruptedRestorePending")}</span>
      <button type="button" class="btnicontext" disabled={$securityLoading} on:click={() => void onRepairInterruptedRestore()}>
        {$t("Security_InterruptedRestore_Repair")}
      </button>
    </div>
  {/if}

  {#if quarantines.length > 0}
    <div class="multilineSetting security-warning">
      <span>{$t("Security_QuarantineStatus", { count: quarantines.length })}</span>
      {#each quarantines as q}
        <span class="security-quarantine-row">
          <span>{q.accounts.join(", ")}</span>
          <button type="button" class="btnicontext" disabled={$securityLoading} on:click={() => void onRetryQuarantine(q.id)}>
            {$t("Security_QuarantineRetry")}
          </button>
          <button type="button" class="btnicontext" disabled={$securityLoading} on:click={() => void onDeleteQuarantine(q.id)}>
            {$t("Security_QuarantineDelete")}
          </button>
        </span>
      {/each}
    </div>
  {/if}
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

<div class="rowDropdown hotkey-row">
  <span>{$t("Settings_CommandPaletteHotkey")}</span>
  <button
    type="button"
    class="btnicontext hotkey-button"
    class:capturing={commandPaletteHotkeyCaptureActive}
    aria-pressed={commandPaletteHotkeyCaptureActive}
    on:click={toggleCommandPaletteHotkeyCapture}
  >
    {commandPaletteHotkeyCaptureActive ? $t("Settings_CommandPaletteHotkey_Prompt") : $commandPaletteHotkey}
  </button>
</div>

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
  <button type="button" class="btnicontext" on:click={() => void openFeedbackModal({ mode: "suggestion" })}>
    {$t("Settings_SuggestFeature")}
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

<style lang="scss">
  button:not(.fancyLink) {
    position: relative;
    height: 38px;
  }

  .version-row {
    margin-top: 0.25rem;
  }

  .hotkey-row {
    gap: 0.75rem;
  }

  .security-settings {
    display: grid;
    gap: 0.25rem;
    margin-bottom: 0.35rem;
  }

  .security-password-row {
    align-items: center;
  }

  .security-password-controls,
  .security-encryption-inline {
    display: inline-flex;
    align-items: center;
    gap: 0.65rem;
    flex-wrap: wrap;
    justify-content: flex-end;
  }

  .security-encryption-inline {
    gap: 0.4rem;
  }

  .security-warning {
    color: var(--whiteSecondary);
  }

  .security-quarantine-row {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    flex-wrap: wrap;
  }

  .hotkey-button {
    min-width: 7rem;
  }

  .hotkey-button.capturing {
    background: var(--accent);
    color: var(--text-on-bright-bg);
  }
</style>

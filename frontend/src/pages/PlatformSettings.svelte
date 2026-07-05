<script lang="ts">
  import { get } from "svelte/store";
  import { onDestroy, onMount } from "svelte";
  import { route, previousPage, appBarTitle, navigateBackLikeButton } from "../stores/nav";
  import { t } from "../stores/i18n";
  import { activeModal, openConfirm, openFolderPicker } from "../stores/modal";
  import { pushToast } from "../stores/toast";
  import GeneralSettingsBlock from "../components/GeneralSettingsBlock.svelte";
  import PlatformSettingsSteamSection from "../components/settings/PlatformSettingsSteamSection.svelte";
  import PlatformSettingsGenericSection from "../components/settings/PlatformSettingsGenericSection.svelte";
  import PlatformSettingsToolsSection from "../components/settings/PlatformSettingsToolsSection.svelte";
  import * as Wails from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";
  import * as BasicService from "../../bindings/TcNo-Acc-Switcher/internal/basic/basicservice.js";
  import * as Shortcuts from "wails-shortcuts-service";
  import { SaveSteamSettings, GetSteamSettings, RefreshVACStatus, RefreshAllSteamImages } from "../../bindings/TcNo-Acc-Switcher/internal/steam/steamservice.js";
  import { requestPlatformAccountsRefresh } from "../stores/platformPage";
  import { controllerSpatialNavigation } from "../lib/actions/controllerSpatialNavigation";
  import { PlatformSettings } from "../../bindings/TcNo-Acc-Switcher/internal/platform/models.js";
  import { Settings } from "../../bindings/TcNo-Acc-Switcher/internal/steam/models.js";
  import {
    ARG_SILENT,
    ARG_VGUI,
    hasLaunchArgFlag,
    sanitizeSettingsPayload,
    isClosingMethodForcedPayload,
  } from "../lib/platformSettingsShared";
  import "../styles/Settings.scss";

  export let name: string;

  $: appBarTitle.set($t("Title_Template_Settings", { platformName: name }));
  $: if (name) {
    route.set({ page: "platform-settings", platformName: name });
    previousPage.set({ page: "platform", platformName: name });
  }

  $: isSteam = name === "Steam";

  let steamSettings: Settings | null = null;
  let genericPS: PlatformSettings | null = null;
  let installFolder = "";
  let loadError = "";

  let hasDesktopShortcut = false;
  let hasCachePaths = false;
  let hasBackupFolders = false;
  let hasRemoteProfileImages = false;
  let hasSavedProfileImageSources = false;
  let clearingCache = false;
  let backingUp = false;
  let restoringBackup = false;
  let closingMethodUiLocked = false;

  let saveTimer: ReturnType<typeof setTimeout> | undefined;

  $: silentOn =
    isSteam && steamSettings
      ? hasLaunchArgFlag(steamSettings.LaunchArguments ?? "", ARG_SILENT)
      : false;
  $: oldUiOn =
    isSteam && steamSettings
      ? hasLaunchArgFlag(steamSettings.LaunchArguments ?? "", ARG_VGUI)
      : false;

  function debouncedSaveSteam(): void {
    if (!isSteam || !steamSettings) return;
    clearTimeout(saveTimer);
    saveTimer = setTimeout(() => {
      void SaveSteamSettings(steamSettings!).catch(() => {});
    }, 450);
  }

  function bumpSteamSettings(): void {
    steamSettings = steamSettings;
  }

  function bumpGenericPlatformSettings(): void {
    genericPS = genericPS;
  }

  function debouncedSaveGeneric(): void {
    if (isSteam || !genericPS) return;
    clearTimeout(saveTimer);
    saveTimer = setTimeout(() => {
      void Wails.SavePlatformSettings(name, genericPS!).catch(() => {});
    }, 450);
  }

  function onSteamSave(): void {
    bumpSteamSettings();
    debouncedSaveSteam();
  }

  function onGenericSave(): void {
    bumpGenericPlatformSettings();
    debouncedSaveGeneric();
  }

  async function refreshDesktopShortcutState(): Promise<void> {
    try {
      hasDesktopShortcut = await Shortcuts.PlatformShortcutExists(name);
    } catch {
      hasDesktopShortcut = false;
    }
  }

  async function refreshCachePathState(): Promise<void> {
    try {
      hasCachePaths = await Wails.HasPlatformCachePaths(name);
    } catch {
      hasCachePaths = false;
    }
  }

  async function refreshBackupPathState(): Promise<void> {
    try {
      hasBackupFolders = await Wails.HasPlatformBackupFolders(name);
    } catch {
      hasBackupFolders = false;
    }
  }

  async function onToggleDesktopShortcut(): Promise<void> {
    const was = hasDesktopShortcut;
    hasDesktopShortcut = !was;
    try {
      if (hasDesktopShortcut) {
        await Shortcuts.CreatePlatformShortcut(name);
        pushToast({
          type: "success",
          title: "",
          message: $t("Toast_ShortcutCreated"),
          duration: 5000,
        });
      } else {
        await Shortcuts.DeletePlatformShortcut(name);
        pushToast({ type: "success", title: "", message: $t("Done"), duration: 3000 });
      }
    } catch (err) {
      hasDesktopShortcut = was;
      pushToast({
        type: "error",
        title: "",
        message: err instanceof Error ? err.message : String(err),
        duration: 8000,
      });
    }
  }

  async function refreshInstallFolder(): Promise<void> {
    try {
      installFolder = (await Wails.GetPlatformInstallFolder(name)) ?? "";
    } catch {
      installFolder = "";
    }
  }

  onMount(() => {
    void (async () => {
      loadError = "";
      try {
        await refreshInstallFolder();
        if (isSteam) {
          await loadSteamSettings();
        } else {
          await loadGenericPlatformSettings();
        }
        await refreshCachePathState();
        await refreshBackupPathState();
        await refreshDesktopShortcutState();
      } catch (e) {
        loadError = e instanceof Error ? e.message : String(e);
      }
    })();
  });

  onDestroy(() => {
    clearTimeout(saveTimer);
  });

  async function onPickFolder(): Promise<void> {
    try {
      const r = await Wails.ResolvePlatformLaunch(name);
      const picked = await openFolderPicker({
        title: $t("Modal_Title_LocatePlatform", { platform: name }),
        body: `<p>${$t("Modal_LocatePlatform", { platformExe: r.soughtExeName })}</p>`,
        initialPath: r.initialPath ?? installFolder ?? "",
        dirsOnly: false,
        soughtFilename: r.soughtExeName,
        positiveLabel: $t("Modal_Button_Select"),
      });
      if (picked) {
        await Wails.ConfirmPlatformExePath(name, picked);
        await refreshInstallFolder();
        if (isSteam) {
          const raw = await GetSteamSettings();
          steamSettings = Settings.createFrom(sanitizeSettingsPayload(raw) as Partial<Settings>);
        }
        pushToast({
          type: "success",
          title: "",
          message: $t("Toast_PlatformPathUpdated"),
          duration: 5000,
        });
      }
    } catch (e) {
      pushToast({
        type: "error",
        title: "",
        message: e instanceof Error ? e.message : String(e),
        duration: 8000,
      });
    }
  }

  async function loadSteamSettings(): Promise<void> {
    const raw = await GetSteamSettings();
    closingMethodUiLocked = isClosingMethodForcedPayload(raw);
    steamSettings = Settings.createFrom(sanitizeSettingsPayload(raw) as Partial<Settings>);
    if (steamSettings.AlwaysSwapOnShortcut === undefined) steamSettings.AlwaysSwapOnShortcut = true;
    if (steamSettings.LaunchArguments === undefined) steamSettings.LaunchArguments = "";
  }

  async function loadGenericPlatformSettings(): Promise<void> {
    const raw = await Wails.GetPlatformSettings(name);
    closingMethodUiLocked = isClosingMethodForcedPayload(raw);
    const payload = sanitizeSettingsPayload(raw) as Record<string, unknown>;
    genericPS = PlatformSettings.createFrom(payload as Partial<PlatformSettings>);
    if (genericPS.AlwaysSwapOnShortcut === undefined) genericPS.AlwaysSwapOnShortcut = true;
    if (genericPS.LaunchArguments === undefined) genericPS.LaunchArguments = "";
    if (!("ShowLastUsed" in payload)) genericPS.ShowLastUsed = true;
    if (
      genericPS.ProfileImageExpiryDays === undefined ||
      genericPS.ProfileImageExpiryDays === null ||
      typeof genericPS.ProfileImageExpiryDays !== "number" ||
      genericPS.ProfileImageExpiryDays < 1
    ) {
      genericPS.ProfileImageExpiryDays = 7;
    }
    if (!("PullAccountImagesOnSwitch" in payload)) {
      (genericPS as unknown as Record<string, unknown>).PullAccountImagesOnSwitch = true;
    }
    try {
      hasRemoteProfileImages = await BasicService.PlatformUsesRemoteProfileImages(name);
    } catch {
      hasRemoteProfileImages = false;
    }
    try {
      hasSavedProfileImageSources = (
        BasicService as unknown as { PlatformProfileImagesSavedPerAccount: (platformKey: string) => Promise<boolean> }
      ).PlatformProfileImagesSavedPerAccount
        ? await (
            BasicService as unknown as {
              PlatformProfileImagesSavedPerAccount: (platformKey: string) => Promise<boolean>;
            }
          ).PlatformProfileImagesSavedPerAccount(name)
        : false;
    } catch {
      hasSavedProfileImageSources = false;
    }
  }

  async function onReset(): Promise<void> {
    const ok = await openConfirm({
      title: $t("Button_ResetSettings"),
      body: `<p>${$t("Settings_ResetConfirm", { platform: name })}</p>`,
      style: "yesno",
      positiveLabel: $t("Yes"),
      negativeLabel: $t("No"),
    });
    if (!ok) return;

    await runWithToast(async () => {
      await Wails.ResetPlatformSettings(name);
      await refreshInstallFolder();
      if (isSteam) await loadSteamSettings();
      else await loadGenericPlatformSettings();
      await refreshDesktopShortcutState();
    }, $t("Toast_SettingsReset"));
  }

  async function onOpenFolder(): Promise<void> {
    try {
      await Wails.OpenPlatformFolder(name);
    } catch (e) {
      pushToast({
        type: "error",
        title: "",
        message: e instanceof Error ? e.message : String(e),
        duration: 8000,
      });
    }
  }

  async function onClearCache(): Promise<void> {
    if (clearingCache) return;
    clearingCache = true;
    try {
      await Wails.ClearPlatformCache(name);
      pushToast({ type: "success", title: "", message: $t("Done"), duration: 4000 });
    } catch (e) {
      pushToast({
        type: "error",
        title: "",
        message: e instanceof Error ? e.message : String(e),
        duration: 8000,
      });
    } finally {
      clearingCache = false;
    }
  }

  async function onBackup(everything: boolean): Promise<void> {
    if (backingUp) return;
    backingUp = true;
    try {
      await Wails.BackupPlatform(name, everything);
      pushToast({ type: "success", title: "", message: $t("Done"), duration: 4000 });
    } catch (e) {
      pushToast({
        type: "error",
        title: "",
        message: e instanceof Error ? e.message : String(e),
        duration: 8000,
      });
    } finally {
      backingUp = false;
    }
  }

  async function onOpenBackupFolder(): Promise<void> {
    try {
      await Wails.OpenPlatformBackupFolder(name);
    } catch (e) {
      pushToast({
        type: "error",
        title: "",
        message: e instanceof Error ? e.message : String(e),
        duration: 8000,
      });
    }
  }

  async function onRestoreLatestBackup(): Promise<void> {
    if (restoringBackup) return;
    restoringBackup = true;
    try {
      await Wails.RestoreLatestPlatformBackup(name);
      pushToast({ type: "success", title: "", message: $t("Toast_RestoreComplete"), duration: 5000 });
    } catch (e) {
      pushToast({
        type: "error",
        title: "",
        message: e instanceof Error ? e.message : String(e),
        duration: 8000,
      });
    } finally {
      restoringBackup = false;
    }
  }

  async function runWithToast(op: () => Promise<void>, successMsg: string): Promise<void> {
    try {
      await op();
      pushToast({ type: "success", title: "", message: successMsg, duration: 5000 });
    } catch (e) {
      pushToast({
        type: "error",
        title: "",
        message: e instanceof Error ? e.message : String(e),
        duration: 8000,
      });
    }
  }

  async function onRefreshVac(): Promise<void> {
    await runWithToast(() => RefreshVACStatus(), $t("Toast_Steam_VacCleared"));
  }

  async function onRefreshBasicProfileImages(): Promise<void> {
    await runWithToast(async () => {
      await BasicService.RefreshAllBasicProfileImages(name);
      requestPlatformAccountsRefresh(name);
    }, $t("Toast_ImagesRefreshing"));
  }

  async function onRefreshSavedBasicProfileImages(): Promise<void> {
    await runWithToast(async () => {
      await (
        BasicService as unknown as { RefreshSavedBasicProfileImages: (platformKey: string) => Promise<void> }
      ).RefreshSavedBasicProfileImages(name);
      requestPlatformAccountsRefresh(name);
    }, $t("Toast_ImagesRefreshing"));
  }

  async function onClearBasicProfileImages(): Promise<void> {
    await runWithToast(async () => {
      await (BasicService as unknown as { ClearAllBasicProfileImages: (platformKey: string) => Promise<void> })
        .ClearAllBasicProfileImages(name);
      requestPlatformAccountsRefresh(name);
    }, $t("Done"));
  }

  async function onRefreshImages(): Promise<void> {
    await runWithToast(() => RefreshAllSteamImages(), $t("Toast_ImagesRefreshing"));
  }

  function onWindowKeyDown(e: KeyboardEvent): void {
    if (e.key !== "Escape") {
      return;
    }
    if (get(activeModal)) {
      return;
    }
    e.preventDefault();
    navigateBackLikeButton();
  }
</script>

<div class="main-content main-spacing platform-settings-scroll" use:controllerSpatialNavigation>
  {#if loadError}
    <p class="platform-settings-err">{loadError}</p>
  {/if}

  <h1 class="SettingsHeader">{$t("Settings_Header_Platform", { platformName: name })}</h1>

  {#if isSteam && steamSettings}
    <PlatformSettingsSteamSection
      {name}
      {steamSettings}
      {hasDesktopShortcut}
      {silentOn}
      {oldUiOn}
      {closingMethodUiLocked}
      on:save={onSteamSave}
      on:toggleDesktopShortcut={onToggleDesktopShortcut}
    />
  {:else if !isSteam && genericPS}
    <PlatformSettingsGenericSection
      {name}
      {genericPS}
      {hasDesktopShortcut}
      {closingMethodUiLocked}
      {hasRemoteProfileImages}
      on:save={onGenericSave}
      on:toggleDesktopShortcut={onToggleDesktopShortcut}
      on:refreshBasicProfileImages={onRefreshBasicProfileImages}
      on:clearBasicProfileImages={onClearBasicProfileImages}
    />
  {/if}

  {#if (isSteam && steamSettings) || (!isSteam && genericPS)}
    <PlatformSettingsToolsSection
      {name}
      {isSteam}
      {installFolder}
      {hasCachePaths}
      {hasBackupFolders}
      {hasSavedProfileImageSources}
      {clearingCache}
      {backingUp}
      {restoringBackup}
      on:pickFolder={onPickFolder}
      on:reset={onReset}
      on:clearCache={onClearCache}
      on:backup={(e) => onBackup(e.detail.everything)}
      on:openBackupFolder={onOpenBackupFolder}
      on:restoreLatestBackup={onRestoreLatestBackup}
      on:refreshVac={onRefreshVac}
      on:refreshImages={onRefreshImages}
      on:refreshSavedBasicProfileImages={onRefreshSavedBasicProfileImages}
      on:openFolder={onOpenFolder}
    />

    <hr class="settings-divider" />

    <h2 class="SettingsHeader">{$t("Settings_Header_AppWide")}</h2>
    <GeneralSettingsBlock />
  {/if}
</div>
<svelte:window on:keydown={onWindowKeyDown} />

<style lang="scss">
  .platform-settings-scroll {
    overflow-y: auto;
    flex: 1;
    min-height: 0;
    padding-bottom: 1rem;
  }

  .platform-settings-err {
    color: var(--red);
    padding: 0.5rem 1rem;
  }

  .settings-divider {
    border: 0;
    border-top: 1px solid var(--accent);
    margin: 2rem 0 1.5rem;
  }
</style>

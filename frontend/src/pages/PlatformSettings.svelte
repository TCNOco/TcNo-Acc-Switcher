<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import { route, previousPage, appBarTitle } from "../stores/nav";
  import { t } from "../stores/i18n";
  import { openConfirm, openFolderPicker } from "../stores/modal";
  import { pushToast } from "../stores/toast";
  import { tooltip } from "../lib/actions/tooltip";
  import GeneralSettingsBlock from "../components/GeneralSettingsBlock.svelte";
  import * as Wails from "../lib/platformBindings";
  import * as BasicService from "../../bindings/TcNo-Acc-Switcher/internal/basic/basicservice.js";
  import * as Shortcuts from "wails-shortcuts-service";
  import { requestPlatformAccountsRefresh } from "../stores/platformPage";
  import { PlatformSettings } from "../../bindings/TcNo-Acc-Switcher/internal/platform/models.js";
  import { Settings } from "../../bindings/TcNo-Acc-Switcher/internal/steam/models.js";
  import "../styles/Settings.scss";

  export let name: string;

  $: appBarTitle.set(`${name} - Settings`);
  $: if (name) {
    route.set({ page: "platform-settings", platformName: name });
    previousPage.set({ page: "platform", platformName: name });
  }

  $: isSteam = name === "Steam";

  let steamSettings: Settings | null = null;
  let genericPS: PlatformSettings | null = null;
  let installFolder = "";
  let loadError = "";
  let closingOpen = false;
  let startingOpen = false;
  let stateOpen = false;

  let saveTimer: ReturnType<typeof setTimeout> | undefined;

  let hasDesktopShortcut = false;
  let hasCachePaths = false;
  let hasBackupFolders = false;
  let hasRemoteProfileImages = false;
  let hasSavedProfileImageSources = false;
  let clearingCache = false;
  let backingUp = false;
  let restoringBackup = false;

  const ARG_SILENT = "-silent";
  const ARG_VGUI = "-vgui";

  function launchArgTokens(line: string): string[] {
    return line.trim().split(/\s+/).filter((x) => x.length > 0);
  }

  function hasLaunchArgFlag(line: string, flag: string): boolean {
    const f = flag.trim().toLowerCase();
    return launchArgTokens(line).some((t) => t.toLowerCase() === f);
  }

  function withLaunchArgFlag(line: string, flag: string, on: boolean): string {
    const f = flag.trim();
    const lower = f.toLowerCase();
    const parts = launchArgTokens(line).filter((t) => t.toLowerCase() !== lower);
    if (on) parts.push(f);
    return parts.join(" ");
  }

  function sanitizeSettingsPayload(raw: unknown): Record<string, unknown> {
    const source = raw && typeof raw === "object" ? (raw as Record<string, unknown>) : {};
    const next: Record<string, unknown> = { ...source };
    if (!Array.isArray(next.Shortcuts)) {
      delete next.Shortcuts;
    }
    if (!next.AccountNotes || typeof next.AccountNotes !== "object" || Array.isArray(next.AccountNotes)) {
      delete next.AccountNotes;
    }
    return next;
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

  function onSteamSilentChange(e: Event): void {
    const el = e.currentTarget as HTMLInputElement;
    const s = steamSettings;
    if (!s) return;
    s.LaunchArguments = withLaunchArgFlag(s.LaunchArguments ?? "", ARG_SILENT, el.checked);
    debouncedSaveSteam();
  }

  function onSteamOldUiChange(e: Event): void {
    const el = e.currentTarget as HTMLInputElement;
    const s = steamSettings;
    if (!s) return;
    s.LaunchArguments = withLaunchArgFlag(s.LaunchArguments ?? "", ARG_VGUI, el.checked);
    debouncedSaveSteam();
  }

  async function onToggleDesktopShortcut(e: Event): Promise<void> {
    const el = e.currentTarget as HTMLInputElement;
    const want = el.checked;
    try {
      if (want) {
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
      hasDesktopShortcut = want;
    } catch (err) {
      el.checked = !want;
      pushToast({
        type: "error",
        title: "",
        message: err instanceof Error ? err.message : String(err),
        duration: 8000,
      });
    }
  }

  const closingValues = ["Combined", "Close", "TaskKill"] as const;
  const startingValues = ["Default", "Direct"] as const;

  function closingLabel(v: string): string {
    if (v === "Combined") return "Combined (Best)";
    if (v === "Close") return "Close";
    if (v === "TaskKill") return "TaskKill (Old)";
    return v;
  }

  function startingLabel(v: string): string {
    if (v === "Default") return "Default (Best)";
    if (v === "Direct") return "Direct";
    return v;
  }

  const overrideStates: { v: number; key: string }[] = [
    { v: -1, key: "NoDefault" },
    { v: 1, key: "Online" },
    { v: 7, key: "Invisible" },
    { v: 0, key: "Offline" },
    { v: 2, key: "Busy" },
    { v: 3, key: "Away" },
    { v: 4, key: "Snooze" },
    { v: 5, key: "LookingToTrade" },
    { v: 6, key: "LookingToPlay" },
  ];

  function overrideLabel(v: number): string {
    const row = overrideStates.find((x) => x.v === v);
    return row ? $t(row.key) : $t("NoDefault");
  }

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
      void Wails.SaveSteamSettings(steamSettings!).catch(() => {});
    }, 450);
  }

  /** Svelte 4 only invalidates on top-level assignment; bump after mutating nested fields. */
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

  function getPullAccountImagesOnSwitch(): boolean {
    const g = genericPS as unknown as Record<string, unknown> | null;
    if (!g) return true;
    return g.PullAccountImagesOnSwitch !== false;
  }

  function onPullAccountImagesOnSwitchChange(e: Event): void {
    const g = genericPS as unknown as Record<string, unknown> | null;
    if (!g) return;
    g.PullAccountImagesOnSwitch = (e.currentTarget as HTMLInputElement).checked;
    debouncedSaveGeneric();
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
          const raw = await Wails.GetSteamSettings();
          steamSettings = Settings.createFrom(sanitizeSettingsPayload(raw) as Partial<Settings>);
          if (steamSettings.AlwaysSwapOnShortcut === undefined) {
            steamSettings.AlwaysSwapOnShortcut = true;
          }
          if (steamSettings.LaunchArguments === undefined) {
            steamSettings.LaunchArguments = "";
          }
        } else {
          const raw = await Wails.GetPlatformSettings(name);
          const payload = sanitizeSettingsPayload(raw) as Record<string, unknown>;
          genericPS = PlatformSettings.createFrom(payload as Partial<PlatformSettings>);
          if (genericPS.AlwaysSwapOnShortcut === undefined) {
            genericPS.AlwaysSwapOnShortcut = true;
          }
          if (genericPS.LaunchArguments === undefined) {
            genericPS.LaunchArguments = "";
          }
          if (!("ShowLastUsed" in payload)) {
            genericPS.ShowLastUsed = true;
          }
          if (
            genericPS.ProfileImageExpiryDays === undefined ||
            genericPS.ProfileImageExpiryDays === null ||
            !(typeof genericPS.ProfileImageExpiryDays === "number") ||
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
          const raw = await Wails.GetSteamSettings();
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

  async function onReset(): Promise<void> {
    const ok = await openConfirm({
      title: $t("Button_ResetSettings"),
      body: `<p>${$t("Settings_ResetConfirm", { platform: name })}</p>`,
      style: "yesno",
      positiveLabel: $t("Yes"),
      negativeLabel: $t("No"),
    });
    if (!ok) return;
    try {
      await Wails.ResetPlatformSettings(name);
      await refreshInstallFolder();
      if (isSteam) {
        const raw = await Wails.GetSteamSettings();
        steamSettings = Settings.createFrom(sanitizeSettingsPayload(raw) as Partial<Settings>);
      } else {
        const raw = await Wails.GetPlatformSettings(name);
        const payload = sanitizeSettingsPayload(raw) as Record<string, unknown>;
        genericPS = PlatformSettings.createFrom(payload as Partial<PlatformSettings>);
        if (!("ShowLastUsed" in payload)) {
          genericPS.ShowLastUsed = true;
        }
        if (
          genericPS.ProfileImageExpiryDays === undefined ||
          genericPS.ProfileImageExpiryDays === null ||
          !(typeof genericPS.ProfileImageExpiryDays === "number") ||
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
      await refreshDesktopShortcutState();
      pushToast({
        type: "success",
        title: "",
        message: $t("Toast_SettingsReset"),
        duration: 4000,
      });
    } catch (e) {
      pushToast({
        type: "error",
        title: "",
        message: e instanceof Error ? e.message : String(e),
        duration: 8000,
      });
    }
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

  async function onRefreshVac(): Promise<void> {
    try {
      await Wails.RefreshVACStatus();
      pushToast({ type: "success", title: "", message: $t("Toast_Steam_VacCleared"), duration: 5000 });
    } catch (e) {
      pushToast({
        type: "error",
        title: "",
        message: e instanceof Error ? e.message : String(e),
        duration: 8000,
      });
    }
  }

  async function onRefreshBasicProfileImages(): Promise<void> {
    try {
      await BasicService.RefreshAllBasicProfileImages(name);
      requestPlatformAccountsRefresh(name);
      pushToast({ type: "success", title: "", message: $t("Toast_ImagesRefreshing"), duration: 5000 });
    } catch (e) {
      pushToast({
        type: "error",
        title: "",
        message: e instanceof Error ? e.message : String(e),
        duration: 8000,
      });
    }
  }

  async function onRefreshSavedBasicProfileImages(): Promise<void> {
    try {
      await (
        BasicService as unknown as { RefreshSavedBasicProfileImages: (platformKey: string) => Promise<void> }
      ).RefreshSavedBasicProfileImages(name);
      requestPlatformAccountsRefresh(name);
      pushToast({ type: "success", title: "", message: $t("Toast_ImagesRefreshing"), duration: 5000 });
    } catch (e) {
      pushToast({
        type: "error",
        title: "",
        message: e instanceof Error ? e.message : String(e),
        duration: 8000,
      });
    }
  }

  async function onClearBasicProfileImages(): Promise<void> {
    try {
      await (BasicService as unknown as { ClearAllBasicProfileImages: (platformKey: string) => Promise<void> })
        .ClearAllBasicProfileImages(name);
      requestPlatformAccountsRefresh(name);
      pushToast({ type: "success", title: "", message: $t("Done"), duration: 5000 });
    } catch (e) {
      pushToast({
        type: "error",
        title: "",
        message: e instanceof Error ? e.message : String(e),
        duration: 8000,
      });
    }
  }

  async function onRefreshImages(): Promise<void> {
    try {
      await Wails.RefreshAllSteamImages();
      pushToast({ type: "success", title: "", message: $t("Toast_ImagesRefreshing"), duration: 5000 });
    } catch (e) {
      pushToast({
        type: "error",
        title: "",
        message: e instanceof Error ? e.message : String(e),
        duration: 8000,
      });
    }
  }
</script>

<div class="main-content main-spacing platform-settings-scroll">
  {#if loadError}
    <p class="platform-settings-err">{loadError}</p>
  {/if}

  {#if isSteam && steamSettings}
    <h1 class="SettingsHeader">{$t("Settings_Header_Platform", { platformName: name })}</h1>

    <h2 class="SettingsHeader">{$t("Settings_Header_GeneralSettings")}</h2>
    <div class="rowSetting">
      <div class="form-check">
        <input
          id="ps-desktop-shortcut"
          type="checkbox"
          checked={hasDesktopShortcut}
          on:change={onToggleDesktopShortcut}
        />
        <label class="form-check-label" for="ps-desktop-shortcut"></label>
      </div>
      <label for="ps-desktop-shortcut">{$t("Settings_Shortcut", { platform: name })}</label>
    </div>
    <div class="rowSetting">
      <div class="form-check">
        <input
          id="ps-run-admin"
          type="checkbox"
          bind:checked={steamSettings.RunAsAdmin}
          on:change={debouncedSaveSteam}
        />
        <label class="form-check-label" for="ps-run-admin"></label>
      </div>
      <label for="ps-run-admin">{$t("Settings_Admin", { platform: name })}</label>
    </div>
    <div class="rowSetting rowDropdown">
      <span use:tooltip={{ text: $t("Tooltip_ClosingMethod"), placement: "right" }}
        >{$t("Settings_Header_ClosingMethod", { platform: name })}</span
      >
      <div class="dropdown" class:show={closingOpen}>
        <button type="button" class="dropdown-toggle" on:click={() => (closingOpen = !closingOpen)}>
          {closingLabel(steamSettings.ClosingMethod)}
          <span class="caret" aria-hidden="true"></span>
        </button>
        {#if closingOpen}
          <ul class="custom-dropdown-menu dropdown-menu" style="position:absolute;z-index:20;margin:0;">
            {#each closingValues as v}
              <li>
                <button
                  type="button"
                  class="dropdown-item"
                  on:click={() => {
                    const s = steamSettings;
                    if (!s) return;
                    s.ClosingMethod = v;
                    bumpSteamSettings();
                    closingOpen = false;
                    debouncedSaveSteam();
                  }}
                >
                  {closingLabel(v)}
                </button>
              </li>
            {/each}
          </ul>
        {/if}
      </div>
    </div>
    <div class="rowSetting rowDropdown">
      <span use:tooltip={{ text: $t("Tooltip_StartingMethod"), placement: "right" }}
        >{$t("Settings_Header_StartingMethod", { platform: name })}</span
      >
      <div class="dropdown" class:show={startingOpen}>
        <button type="button" class="dropdown-toggle" on:click={() => (startingOpen = !startingOpen)}>
          {startingLabel(steamSettings.StartingMethod)}
          <span class="caret" aria-hidden="true"></span>
        </button>
        {#if startingOpen}
          <ul class="custom-dropdown-menu dropdown-menu" style="position:absolute;z-index:20;margin:0;">
            {#each startingValues as v}
              <li>
                <button
                  type="button"
                  class="dropdown-item"
                  on:click={() => {
                    const s = steamSettings;
                    if (!s) return;
                    s.StartingMethod = v;
                    bumpSteamSettings();
                    startingOpen = false;
                    debouncedSaveSteam();
                  }}
                >
                  {startingLabel(v)}
                </button>
              </li>
            {/each}
          </ul>
        {/if}
      </div>
    </div>
    <div class="rowSetting">
      <div class="form-check">
        <input
          id="ps-autostart"
          type="checkbox"
          bind:checked={steamSettings.AutoStart}
          on:change={debouncedSaveSteam}
        />
        <label class="form-check-label" for="ps-autostart"></label>
      </div>
      <label for="ps-autostart">{$t("Settings_AutoStart", { platform: name })}</label>
    </div>
    <div class="rowSetting">
      <div class="form-check">
        <input
          id="ps-forget"
          type="checkbox"
          bind:checked={steamSettings.ForgetAccountEnabled}
          on:change={debouncedSaveSteam}
        />
        <label class="form-check-label" for="ps-forget"></label>
      </div>
      <label for="ps-forget">{$t("Settings_ForgetAccountEnabled")}</label>
    </div>
    <div class="rowSetting">
      <div class="form-check">
        <input
          id="ps-shortnotes"
          type="checkbox"
          bind:checked={steamSettings.ShowShortNotes}
          on:change={debouncedSaveSteam}
        />
        <label class="form-check-label" for="ps-shortnotes"></label>
      </div>
      <label for="ps-shortnotes">{$t("Settings_ShowShortNotes")}</label>
    </div>
    <h2 class="SettingsHeader">{$t("Settings_Header_AccountDisplay")}</h2>
    <div class="rowSetting">
      <div class="form-check">
        <input
          id="ps-show-user"
          type="checkbox"
          bind:checked={steamSettings.Steam_ShowAccUsername}
          on:change={debouncedSaveSteam}
        />
        <label class="form-check-label" for="ps-show-user"></label>
      </div>
      <label for="ps-show-user">{$t("Steam_ShowAccUsername")}</label>
    </div>
    <div class="rowSetting">
      <div class="form-check">
        <input
          id="ps-show-sid"
          type="checkbox"
          bind:checked={steamSettings.Steam_ShowSteamID}
          on:change={debouncedSaveSteam}
        />
        <label class="form-check-label" for="ps-show-sid"></label>
      </div>
      <label for="ps-show-sid">{$t("Steam_ShowSteamID")}</label>
    </div>
    <div class="rowSetting">
      <div class="form-check">
        <input
          id="ps-show-ll"
          type="checkbox"
          bind:checked={steamSettings.Steam_ShowLastLogin}
          on:change={debouncedSaveSteam}
        />
        <label class="form-check-label" for="ps-show-ll"></label>
      </div>
      <label for="ps-show-ll">{$t("Steam_ShowLastLogin")}</label>
    </div>
    <div class="rowSetting">
      <div class="form-check">
        <input
          id="ps-show-vac"
          type="checkbox"
          bind:checked={steamSettings.Steam_ShowVAC}
          on:change={debouncedSaveSteam}
        />
        <label class="form-check-label" for="ps-show-vac"></label>
      </div>
      <label for="ps-show-vac">{$t("Steam_ShowVac")}</label>
    </div>
    <div class="rowSetting">
      <div class="form-check">
        <input
          id="ps-show-ltd"
          type="checkbox"
          bind:checked={steamSettings.Steam_ShowLimited}
          on:change={debouncedSaveSteam}
        />
        <label class="form-check-label" for="ps-show-ltd"></label>
      </div>
      <label for="ps-show-ltd">{$t("Steam_ShowLimited")}</label>
    </div>
    <div class="rowSetting">
      <div class="form-check">
        <input
          id="ps-show-miniprofile"
          type="checkbox"
          bind:checked={steamSettings.Steam_ShowMiniProfile}
          on:change={debouncedSaveSteam}
        />
        <label class="form-check-label" for="ps-show-miniprofile"></label>
      </div>
      <label for="ps-show-miniprofile" use:tooltip={{ text: $t("Tooltip_SteamShowMiniProfile"), placement: "right" }}
        >{$t("Steam_ShowMiniProfile")}</label
      >
    </div>
    <div class="rowSetting">
      <div class="form-check">
        <input
          id="ps-show-avatar-frame"
          type="checkbox"
          bind:checked={steamSettings.Steam_ShowAvatarFrame}
          on:change={debouncedSaveSteam}
        />
        <label class="form-check-label" for="ps-show-avatar-frame"></label>
      </div>
      <label for="ps-show-avatar-frame" use:tooltip={{ text: $t("Tooltip_SteamShowAvatarFrame"), placement: "right" }}
        >{$t("Steam_ShowAvatarFrame")}</label
      >
    </div>

    <h2 class="SettingsHeader">{$t("Settings_Header_TraySettings")}</h2>
    <div class="rowSetting">
      <div class="form-check">
        <input
          id="ps-tray-name"
          type="checkbox"
          bind:checked={steamSettings.Steam_TrayAccountName}
          on:change={debouncedSaveSteam}
        />
        <label class="form-check-label" for="ps-tray-name"></label>
      </div>
      <label for="ps-tray-name">{$t("Steam_Tray_AccountName")}</label>
    </div>
    <div class="form-text tray-max-row">
      <span>{$t("Settings_TrayMax")}</span>
      <input
        type="number"
        min="0"
        max="365"
        bind:value={steamSettings.TrayAccNumber}
        on:change={debouncedSaveSteam}
      />
    </div>

    <h2 class="SettingsHeader">{$t("Settings_Header_LaunchOptions")}</h2>
    <div class="rowSetting">
      <div class="form-check">
        <input
          id="ps-silent"
          type="checkbox"
          disabled={!steamSettings.AutoStart}
          checked={silentOn}
          on:change={onSteamSilentChange}
        />
        <label class="form-check-label" for="ps-silent"></label>
      </div>
      <label for="ps-silent">{$t("Steam_StartSilent")}</label>
    </div>
    <div class="rowSetting">
      <div class="form-check">
        <input
          id="ps-oldui"
          type="checkbox"
          disabled={!steamSettings.AutoStart}
          checked={oldUiOn}
          on:change={onSteamOldUiChange}
        />
        <label class="form-check-label" for="ps-oldui"></label>
      </div>
      <label for="ps-oldui">{$t("Steam_OldUi")}</label>
    </div>
    <div class="rowSetting form-text launch-args-row">
      <label for="ps-launch-args">{$t("Settings_LaunchArgumentsForPlatform", { platform: name })}</label>
      <input
        id="ps-launch-args"
        type="text"
        spellcheck="false"
        autocomplete="off"
        disabled={!steamSettings.AutoStart}
        bind:value={steamSettings.LaunchArguments}
        on:input={debouncedSaveSteam}
      />
      <p class="subtext">{$t("Settings_LaunchArguments_Hint")}</p>
    </div>
    <div class="rowSetting">
      <div class="form-check">
        <input
          id="ps-steam-switcher"
          type="checkbox"
          bind:checked={steamSettings.ShowSteamSwitcher}
          on:change={debouncedSaveSteam}
        />
        <label class="form-check-label" for="ps-steam-switcher"></label>
      </div>
      <label for="ps-steam-switcher">{$t("Settings_ShowSteamSwitcher")}</label>
    </div>
    <div class="rowSetting">
      <div class="form-check">
        <input
          id="ps-collect"
          type="checkbox"
          bind:checked={steamSettings.CollectInfo}
          on:change={debouncedSaveSteam}
        />
        <label class="form-check-label" for="ps-collect"></label>
      </div>
      <label for="ps-collect">{$t("Settings_SteamCollectInfo")}</label>
    </div>
    <div class="rowSetting rowDropdown">
      <span>{$t("Steam_OverrideDefaultState")}</span>
      <div class="dropdown" class:show={stateOpen}>
        <button type="button" class="dropdown-toggle" on:click={() => (stateOpen = !stateOpen)}>
          {overrideLabel(steamSettings.Steam_OverrideState)}
          <span class="caret" aria-hidden="true"></span>
        </button>
        {#if stateOpen}
          <ul class="custom-dropdown-menu dropdown-menu" style="position:absolute;z-index:20;margin:0;">
            {#each overrideStates as o}
              <li>
                <button
                  type="button"
                  class="dropdown-item"
                  on:click={() => {
                    const s = steamSettings;
                    if (!s) return;
                    s.Steam_OverrideState = o.v;
                    bumpSteamSettings();
                    stateOpen = false;
                    debouncedSaveSteam();
                  }}
                >
                  {$t(o.key)}
                </button>
              </li>
            {/each}
          </ul>
        {/if}
      </div>
    </div>
    <div class="form-text">
      <span>{$t("Settings_ImageExpiry")}</span>
      <input
        type="number"
        min="0"
        max="365"
        bind:value={steamSettings.Steam_ImageExpiryTime}
        on:change={debouncedSaveSteam}
      />
    </div>
    <div class="form-text">
      <span>{$t("Settings_SteamAPIKey")}</span>
      <input
        type="text"
        spellcheck="false"
        bind:value={steamSettings.SteamWebApiKey}
        on:change={debouncedSaveSteam}
      />
      <p class="subtext">{$t("Settings_SteamAPIKey_Note")}</p>
    </div>
  {:else if !isSteam && genericPS}
    <h1 class="SettingsHeader">{$t("Settings_Header_Platform", { platformName: name })}</h1>
    <h2 class="SettingsHeader">{$t("Settings_Header_GeneralSettings")}</h2>
    <div class="rowSetting">
      <div class="form-check">
        <input
          id="gp-desktop-shortcut"
          type="checkbox"
          checked={hasDesktopShortcut}
          on:change={onToggleDesktopShortcut}
        />
        <label class="form-check-label" for="gp-desktop-shortcut"></label>
      </div>
      <label for="gp-desktop-shortcut">{$t("Settings_Shortcut", { platform: name })}</label>
    </div>
    <div class="rowSetting">
      <div class="form-check">
        <input
          id="gp-run-admin"
          type="checkbox"
          bind:checked={genericPS.RunAsAdmin}
          on:change={debouncedSaveGeneric}
        />
        <label class="form-check-label" for="gp-run-admin"></label>
      </div>
      <label for="gp-run-admin">{$t("Settings_Admin", { platform: name })}</label>
    </div>
    <div class="rowSetting rowDropdown">
      <span use:tooltip={{ text: $t("Tooltip_ClosingMethod"), placement: "right" }}
        >{$t("Settings_Header_ClosingMethod", { platform: name })}</span
      >
      <div class="dropdown" class:show={closingOpen}>
        <button type="button" class="dropdown-toggle" on:click={() => (closingOpen = !closingOpen)}>
          {closingLabel(genericPS.ClosingMethod)}
          <span class="caret" aria-hidden="true"></span>
        </button>
        {#if closingOpen}
          <ul class="custom-dropdown-menu dropdown-menu" style="position:absolute;z-index:20;margin:0;">
            {#each closingValues as v}
              <li>
                <button
                  type="button"
                  class="dropdown-item"
                  on:click={() => {
                    const g = genericPS;
                    if (!g) return;
                    g.ClosingMethod = v;
                    bumpGenericPlatformSettings();
                    closingOpen = false;
                    debouncedSaveGeneric();
                  }}
                >
                  {closingLabel(v)}
                </button>
              </li>
            {/each}
          </ul>
        {/if}
      </div>
    </div>
    <div class="rowSetting rowDropdown">
      <span use:tooltip={{ text: $t("Tooltip_StartingMethod"), placement: "right" }}
        >{$t("Settings_Header_StartingMethod", { platform: name })}</span
      >
      <div class="dropdown" class:show={startingOpen}>
        <button type="button" class="dropdown-toggle" on:click={() => (startingOpen = !startingOpen)}>
          {startingLabel(genericPS.StartingMethod)}
          <span class="caret" aria-hidden="true"></span>
        </button>
        {#if startingOpen}
          <ul class="custom-dropdown-menu dropdown-menu" style="position:absolute;z-index:20;margin:0;">
            {#each startingValues as v}
              <li>
                <button
                  type="button"
                  class="dropdown-item"
                  on:click={() => {
                    const g = genericPS;
                    if (!g) return;
                    g.StartingMethod = v;
                    bumpGenericPlatformSettings();
                    startingOpen = false;
                    debouncedSaveGeneric();
                  }}
                >
                  {startingLabel(v)}
                </button>
              </li>
            {/each}
          </ul>
        {/if}
      </div>
    </div>
    <div class="rowSetting">
      <div class="form-check">
        <input
          id="gp-autostart"
          type="checkbox"
          bind:checked={genericPS.AutoStart}
          on:change={debouncedSaveGeneric}
        />
        <label class="form-check-label" for="gp-autostart"></label>
      </div>
      <label for="gp-autostart">{$t("Settings_AutoStart", { platform: name })}</label>
    </div>
    <div class="rowSetting">
      <div class="form-check">
        <input
          id="gp-forget"
          type="checkbox"
          bind:checked={genericPS.ForgetAccountEnabled}
          on:change={debouncedSaveGeneric}
        />
        <label class="form-check-label" for="gp-forget"></label>
      </div>
      <label for="gp-forget">{$t("Settings_ForgetAccountEnabled")}</label>
    </div>
    <div class="rowSetting">
      <div class="form-check">
        <input
          id="gp-shortnotes"
          type="checkbox"
          bind:checked={genericPS.ShowShortNotes}
          on:change={debouncedSaveGeneric}
        />
        <label class="form-check-label" for="gp-shortnotes"></label>
      </div>
      <label for="gp-shortnotes">{$t("Settings_ShowShortNotes")}</label>
    </div>
    <div class="rowSetting">
      <div class="form-check">
        <input
          id="gp-show-lastused"
          type="checkbox"
          bind:checked={genericPS.ShowLastUsed}
          on:change={debouncedSaveGeneric}
        />
        <label class="form-check-label" for="gp-show-lastused"></label>
      </div>
      <label for="gp-show-lastused">{$t("Settings_ShowLastUsed")}</label>
    </div>
    <h2 class="SettingsHeader">{$t("Settings_Header_LaunchOptions")}</h2>
    <div class="rowSetting form-text launch-args-row">
      <label for="gp-launch-args">{$t("Settings_LaunchArgumentsForPlatform", { platform: name })}</label>
      <input
        id="gp-launch-args"
        type="text"
        spellcheck="false"
        autocomplete="off"
        disabled={!genericPS.AutoStart}
        bind:value={genericPS.LaunchArguments}
        on:input={debouncedSaveGeneric}
      />
      <p class="subtext">{$t("Settings_LaunchArguments_Hint")}</p>
    </div>

    <h2 class="SettingsHeader">{$t("Settings_Header_TraySettings")}</h2>
    <div class="form-text tray-max-row">
      <span>{$t("Settings_TrayMax")}</span>
      <input
        type="number"
        min="0"
        max="365"
        bind:value={genericPS.TrayAccNumber}
        on:change={debouncedSaveGeneric}
      />
    </div>
    {#if hasRemoteProfileImages}
      <h2 class="SettingsHeader">{$t("Settings_Header_ProfileImages")}</h2>
      <div class="rowSetting">
        <div class="form-check">
          <input
            id="gp-pull-account-images"
            type="checkbox"
            checked={getPullAccountImagesOnSwitch()}
            on:change={onPullAccountImagesOnSwitchChange}
          />
          <label class="form-check-label" for="gp-pull-account-images"></label>
        </div>
        <label for="gp-pull-account-images">{$t("Settings_PullAccountImages")}</label>
      </div>
      <div class="form-text tray-max-row">
        <span>{$t("Settings_ProfileImageExpiryDays")}</span>
        <input
          type="number"
          min="1"
          max="365"
          bind:value={genericPS.ProfileImageExpiryDays}
          on:change={debouncedSaveGeneric}
        />
      </div>
      <div class="buttoncol settings-tools-row">
        <button type="button" on:click={() => void onRefreshBasicProfileImages()}>
          {$t("Button_RefreshImages")}
        </button>
        <button type="button" on:click={() => void onClearBasicProfileImages()}>
          {$t("Button_ClearCachedProfileImages")}
        </button>
      </div>
    {/if}
  {/if}

  {#if (isSteam && steamSettings) || (!isSteam && genericPS)}
    <h2 class="SettingsHeader">{$t("Settings_Header_GeneralTools")}</h2>
    <p class="install-loc">
      {$t("Settings_CurrentLocation", { path: installFolder || "" })}
    </p>
    <div class="buttoncol settings-tools-row">
      <button type="button" on:click={onPickFolder}>{$t("Settings_PickFolder", { platform: name })}</button>
      <button type="button" on:click={onReset}>{$t("Button_ResetSettings")}</button>
    </div>
    {#if hasCachePaths}
      <div class="buttoncol settings-tools-row settings-tools-row--single">
        {#if hasSavedProfileImageSources}
          <button type="button" on:click={() => void onRefreshSavedBasicProfileImages()}>
            {$t("Button_RefreshProfileImages")}
          </button>
          <button type="button" disabled={clearingCache} on:click={onClearCache}>
            {$t("Platform_ClearCache")}
          </button>
        {:else}
          <button type="button" disabled={clearingCache} on:click={onClearCache}>
            {$t("Platform_ClearCache")}
          </button>
        {/if}
      </div>
    {/if}
    {#if isSteam}
      <div class="buttoncol settings-tools-row">
        <button type="button" on:click={onRefreshVac}>{$t("Steam_CheckVac")}</button>
        <button type="button" on:click={onRefreshImages}>{$t("Button_RefreshImages")}</button>
      </div>
    {/if}

    {#if hasBackupFolders}
      <h2 class="SettingsHeader">{$t("Settings_Header_BackupRestore")}</h2>
      {#if isSteam}
        <div class="buttoncol settings-tools-row">
          <button type="button" disabled={backingUp} on:click={() => void onBackup(false)}>
            {$t("Button_Backup")}
          </button>
          <button type="button" disabled={backingUp} on:click={() => void onBackup(true)}>
            {$t("Button_BackupAll")}
          </button>
        </div>
      {:else}
        <div class="buttoncol settings-tools-row settings-tools-row--single">
          <button type="button" disabled={backingUp} on:click={() => void onBackup(true)}>
            {$t("Button_BackupAll")}
          </button>
        </div>
      {/if}
      <div class="buttoncol settings-tools-row">
        <button type="button" on:click={onOpenBackupFolder}>{$t("Button_OpenBackup")}</button>
        <button type="button" disabled={restoringBackup} on:click={onRestoreLatestBackup}>
          {$t("Button_Restore")}
        </button>
      </div>
    {/if}

    <h2 class="SettingsHeader">{$t("Settings_Header_OtherTools")}</h2>
    <div class="buttoncol settings-tools-row settings-tools-row--single">
      <button type="button" on:click={onOpenFolder}>{$t("Settings_OpenFolder", { platform: name })}</button>
    </div>

    <hr class="settings-divider" />

    <h2 class="SettingsHeader">{$t("Settings_Header_AppWide")}</h2>
    <GeneralSettingsBlock />
  {/if}
</div>

<style lang="scss">
  .platform-settings-scroll {
    overflow-y: auto;
    flex: 1;
    min-height: 0;
    padding-bottom: 1rem;
  }

  :global(.platform-settings-scroll .rowSetting) {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-start;
    gap: 0.25rem 0.5rem;
    margin: 0.4rem 0;
  }

  .platform-settings-err {
    color: #f88;
    padding: 0.5rem 1rem;
  }

  .install-loc {
    color: rgba(255, 255, 255, 0.9);
    font-size: 0.9rem;
    margin: 0.5rem 0 1rem;
    word-break: break-all;
  }

  .settings-divider {
    border: 0;
    border-top: 1px solid var(--accent, #6272a4);
    margin: 2rem 0 1.5rem;
  }

  .settings-tools-row {
    position: relative;
    height: auto;
    min-height: 3.2em;
    margin-bottom: 0.5rem;

    button {
      position: relative;
      width: 49%;
      margin: 0;
    }

    &--single button {
      width: 100%;
    }
  }

  .tray-max-row {
    margin: 0.5rem 0 1rem;
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.35rem;
  }

  .subtext {
    font-size: 0.8rem;
    opacity: 0.85;
    margin-top: 0.25rem;
  }

  .launch-args-row {
    flex-direction: column;
    align-items: stretch;
    gap: 0.35rem;
  }

  .launch-args-row input[type="text"] {
    width: 100%;
    max-width: 42rem;
  }
</style>

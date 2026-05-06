/** Re-exports generated Wails platform + Steam service calls via `src/` for stable TS resolution. */
export {
  BackupPlatform,
  CheckAdminForPlatform,
  ClearPlatformCache,
  ConfirmPlatformExePath,
  GetPlatformExeIcon,
  GetPlatformInstallFolder,
  GetPlatformSettings,
  HasPlatformBackupFolders,
  HasPlatformCachePaths,
  HasShortcutMainExe,
  LaunchPlatform,
  LaunchPlatformAs,
  NotifyLaunchUpdateCheck,
  OpenPlatformBackupFolder,
  OpenPlatformFolder,
  ResetPlatformSettings,
  ResolvePlatformLaunch,
  RestoreLatestPlatformBackup,
  RestartAsAdmin,
  SavePlatformSettings,
} from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";

export {
  AdvancedClearingItems,
  AdvancedClearingRegistrySupported,
  GetSteamSettings,
  RefreshAllSteamImages,
  RefreshVACStatus,
  RunAdvancedClearingAction,
  SaveSteamSettings,
} from "../../bindings/TcNo-Acc-Switcher/internal/steam/steamservice.js";

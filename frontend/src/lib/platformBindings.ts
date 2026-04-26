/** Re-exports generated Wails platform + Steam service calls via `src/` for stable TS resolution. */
export {
  CheckAdminForPlatform,
  ClearPlatformCache,
  ConfirmPlatformExePath,
  GetPlatformExeIcon,
  GetPlatformInstallFolder,
  GetPlatformSettings,
  HasPlatformCachePaths,
  HasShortcutMainExe,
  LaunchPlatform,
  LaunchPlatformAs,
  OpenPlatformFolder,
  ResetPlatformSettings,
  ResolvePlatformLaunch,
  RestartAsAdmin,
  SavePlatformSettings,
} from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";

export {
  GetSteamSettings,
  RefreshAllSteamImages,
  RefreshVACStatus,
  SaveSteamSettings,
} from "../../bindings/TcNo-Acc-Switcher/internal/steam/steamservice.js";

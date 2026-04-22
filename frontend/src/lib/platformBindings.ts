/** Re-exports generated Wails platform + Steam service calls via `src/` for stable TS resolution. */
export {
  ConfirmPlatformExePath,
  GetPlatformExeIcon,
  GetPlatformInstallFolder,
  GetPlatformSettings,
  HasShortcutMainExe,
  LaunchPlatform,
  LaunchPlatformAs,
  OpenPlatformFolder,
  ResetPlatformSettings,
  ResolvePlatformLaunch,
  SavePlatformSettings,
} from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";

export {
  GetSteamSettings,
  RefreshAllSteamImages,
  RefreshVACStatus,
  SaveSteamSettings,
} from "../../bindings/TcNo-Acc-Switcher/internal/steam/steamservice.js";

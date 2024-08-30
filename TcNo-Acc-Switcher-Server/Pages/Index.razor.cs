// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2024 TroubleChute (Wesley Pyburn)
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

using System;
using System.IO;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.Basic;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Pages.Steam;
using static SteamKit2.GC.Dota.Internal.CMsgPracticeLobbyCreate;
using BasicSettings = TcNo_Acc_Switcher_Server.Data.Settings.Basic;
using SteamSettings = TcNo_Acc_Switcher_Server.Data.Settings.Steam;

namespace TcNo_Acc_Switcher_Server.Pages
{
    public partial class Index
    {
        private static readonly Lang Lang = Lang.Instance;

        public void Check(string platform)
        {
            Globals.DebugWriteLine($@"[Func:Index.Check] platform={platform}");
            switch (platform)
            {
                case "Steam":
                    if (!GeneralFuncs.CanKillProcess(SteamSettings.Processes, SteamSettings.ClosingMethod)) return;
                    if (!Directory.Exists(SteamSettings.FolderPath) || !File.Exists(SteamSettings.Exe()))
                    {
                        // Check Start Menu locations for shortcut with this name, and check that location.
                        if (OperatingSystem.IsWindows())
                        {
                            var foundPath = "";
                            // Handle multiple possible inputs:
                            var searchFor = "Steam";

                            // Foreach in searchFor, until we find a valid path
                            var path1 = Globals.ExpandEnvironmentVariables("%StartMenuAppData%");
                            var path2 = Globals.ExpandEnvironmentVariables("%StartMenuProgramData%");
                            var path3 = Globals.ExpandEnvironmentVariables("%Desktop%");


                            var startMenuFiles = Directory.GetFiles(Globals.ExpandEnvironmentVariables("%StartMenuAppData%"), searchFor + ".lnk", SearchOption.AllDirectories);
                            var commonStartMenuFiles = Directory.GetFiles(Globals.ExpandEnvironmentVariables("%StartMenuProgramData%"), searchFor + ".lnk", SearchOption.AllDirectories);
                            // Also check Desktop icons. Non-recursive to not waste too much time if desktop is cluttered.
                            var desktopFiles = Directory.GetFiles(Globals.ExpandEnvironmentVariables("%Desktop%"), searchFor + ".lnk", SearchOption.TopDirectoryOnly);

                            if (startMenuFiles.Length > 0)
                                foreach (var file in startMenuFiles)
                                {
                                    foundPath = Globals.GetShortcutTarget(file);
                                    if (!string.IsNullOrEmpty(foundPath) && TryGetPathSteam(foundPath)) return;
                                }

                            if (commonStartMenuFiles.Length > 0)
                                foreach (var file in commonStartMenuFiles)
                                {
                                    foundPath = Globals.GetShortcutTarget(file);
                                    if (!string.IsNullOrEmpty(foundPath) && TryGetPathSteam(foundPath)) return;
                                }

                            if (desktopFiles.Length > 0)
                                foreach (var file in desktopFiles)
                                {
                                    foundPath = Globals.GetShortcutTarget(file);
                                    if (!string.IsNullOrEmpty(foundPath) && TryGetPathSteam(foundPath)) return;
                                }
                        }

                        _ = GeneralInvocableFuncs.ShowModal("find:Steam:Steam.exe:SteamSettings");
                        return;
                    }
                    if (SteamSwitcherFuncs.SteamSettingsValid()) AppData.ActiveNavMan.NavigateTo("/Steam/");
                    else _ = GeneralInvocableFuncs.ShowModal(Lang["Toast_Steam_CantLocateLoginusers"]);
                    break;

                default:
                    if (BasicPlatforms.PlatformExistsFromShort(platform)) // Is a basic platform!
                    {
                        BasicPlatforms.SetCurrentPlatformFromShort(platform);
                        if (!GeneralFuncs.CanKillProcess(CurrentPlatform.ExesToEnd, BasicSettings.ClosingMethod)) return;

                        // Check if program exists and is still in the same location, or default location
                        if (Directory.Exists(BasicSettings.FolderPath) && File.Exists(BasicSettings.Exe()))
                        {
                            AppData.ActiveNavMan.NavigateTo("/Basic/");
                            break;
                        }

                        // Directory and EXE not found. However, shortuct name is found. Check Start Menu locations for shortcut with this name, and check that location.
                        if (!string.IsNullOrEmpty(CurrentPlatform.GetPathFromShortcutNamed) && OperatingSystem.IsWindows())
                        {
                            var foundPath = "";
                            // Handle multiple possible inputs:
                            var possibleTitles = CurrentPlatform.GetPathFromShortcutNamed.Split("|");

                            // Foreach in searchFor, until we find a valid path
                            foreach (var searchFor in possibleTitles)
                            {
                                var startMenuFiles = Directory.GetFiles(BasicSwitcherFuncs.ExpandEnvironmentVariables("%StartMenuAppData%"), searchFor + ".lnk", SearchOption.AllDirectories);
                                var commonStartMenuFiles = Directory.GetFiles(BasicSwitcherFuncs.ExpandEnvironmentVariables("%StartMenuProgramData%"), searchFor + ".lnk", SearchOption.AllDirectories);
                                // Also check Desktop icons. Non-recursive to not waste too much time if desktop is cluttered.
                                var desktopFiles = Directory.GetFiles(BasicSwitcherFuncs.ExpandEnvironmentVariables("%Desktop%"), searchFor + ".lnk", SearchOption.TopDirectoryOnly);
                                if (startMenuFiles.Length > 0)
                                    foreach (var file in startMenuFiles)
                                    {
                                        foundPath = Globals.GetShortcutTarget(file);
                                        if (!string.IsNullOrEmpty(foundPath) && TryGetPath(foundPath)) return;
                                    }

                                if (commonStartMenuFiles.Length > 0)
                                    foreach (var file in commonStartMenuFiles)
                                    {
                                        foundPath = Globals.GetShortcutTarget(file);
                                        if (!string.IsNullOrEmpty(foundPath) && TryGetPath(foundPath)) return;
                                    }

                                if (desktopFiles.Length > 0)
                                    foreach (var file in desktopFiles)
                                    {
                                        foundPath = Globals.GetShortcutTarget(file);
                                        if (!string.IsNullOrEmpty(foundPath) && TryGetPath(foundPath)) return;
                                    }
                            }
                        }

                        // Otherwise ask the user to locate it.
                        _ = GeneralInvocableFuncs.ShowModal($"find:{CurrentPlatform.SafeName}:{CurrentPlatform.ExeName}:{CurrentPlatform.SafeName}");
                    }
                    break;
            }
        }

        private static bool TryGetPath(string path)
        {
            // Check if the required EXE is in this path (not just any old exe it's pointing to)
            var shortcutTargetFolder = Path.GetDirectoryName(path);
            var shortcutTargetFolderTargetExe = Path.Join(shortcutTargetFolder, CurrentPlatform.ExeName);
            if (File.Exists(shortcutTargetFolderTargetExe))
            {
                // Found platform exe we're looking for! Save this location and continue.
                BasicSettings.FolderPath = shortcutTargetFolder;
                BasicSettings.SaveSettings();
                AppData.ActiveNavMan.NavigateTo("/Basic/");
                _ = GeneralInvocableFuncs.ShowToast("success", Lang["Toast_FoundExeViaShortcut"], renderTo: "toastarea", duration: 30000);
                return true;
            }

            return false;
        }

        private static bool TryGetPathSteam(string path)
        {
            // Check if the required EXE is in this path (not just any old exe it's pointing to)
            var shortcutTargetFolder = Path.GetDirectoryName(path);
            var shortcutTargetFolderTargetExe = Path.Join(shortcutTargetFolder, "steam.exe");
            if (File.Exists(shortcutTargetFolderTargetExe))
            {
                // Found platform exe we're looking for! Save this location and continue.
                SteamSettings.FolderPath = shortcutTargetFolder;
                SteamSettings.SaveSettings();
                AppData.ActiveNavMan.NavigateTo("/Steam/");
                _ = GeneralInvocableFuncs.ShowToast("success", Lang["Toast_FoundExeViaShortcut"], renderTo: "toastarea", duration: 30000);
                return true;
            }

            return false;
        }
    }
}

// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2022 TechNobo (Wesley Pyburn)
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
using System.Threading.Tasks;
using System.Web;
using Microsoft.AspNetCore.Components;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.Basic;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Pages.Steam;
using BasicSettings = TcNo_Acc_Switcher_Server.Data.Settings.Basic;
using SteamSettings = TcNo_Acc_Switcher_Server.Data.Settings.Steam;

namespace TcNo_Acc_Switcher_Server.Shared.Modal
{
    public class ModalFuncs
    {
        #region PATH PICKER MODALS
        public static void ShowUpdatePlatformFolderModal()
        {
            ModalData.PathPicker =
                new ModalData.PathPickerRequest(ModalData.PathPickerRequest.PathPickerGoal.FindPlatformExe);
            ModalData.ShowModal("find");
        }

        /// <summary>
        /// Change the selected platform's EXE folder to another and save changes
        /// </summary>
        public static void UpdatePlatformFolder()
        {
            var path = ModalData.PathPicker.LastPath;

            Globals.DebugWriteLine($@"[ModalFuncs.UpdatePlatformFolder] file={AppData.CurrentSwitcher}, path={path}");
            var settingsFile = AppData.CurrentSwitcher == "Steam"
                ? SteamSettings.SettingsFile
                : CurrentPlatform.SettingsFile;

            var settings = GeneralFuncs.LoadSettings(settingsFile);
            settings["FolderPath"] = path;
            GeneralFuncs.SaveSettings(settingsFile, settings);
            if (!Globals.IsFolder(path))
                path = Path.GetDirectoryName(path); // Remove .exe
            if (!string.IsNullOrWhiteSpace(path) && path.EndsWith(".exe"))
                path = Path.GetDirectoryName(path) ?? string.Join("\\", path.Split("\\")[..^1]);

            if (AppData.CurrentSwitcher == "Steam")
                SteamSettings.FolderPath = path;
            else
                BasicSettings.FolderPath = path;
        }

        /// <summary>
        /// Show the PathPicker modal to find an image to import and use as the app background
        /// </summary>
        public static void ShowSetBackgroundModal()
        {
            ModalData.PathPicker =
                new ModalData.PathPickerRequest(ModalData.PathPickerRequest.PathPickerGoal.SetBackground);
            ModalData.ShowModal("find");
        }

        /// <summary>
        /// Import image to use as the app background
        /// </summary>
        public static void SetBackground()
        {
            var path = ModalData.PathPicker.LastPath;

            AppSettings.Background = $"{path}";

            if (File.Exists(path) && path != "")
            {
                Directory.CreateDirectory(Path.Join(Globals.UserDataFolder, "wwwroot\\img\\custom\\"));
                Globals.CopyFile(path, Path.Join(Globals.UserDataFolder, "wwwroot\\img\\custom\\background" + Path.GetExtension(path)));
                AppSettings.Background = $"img/custom/background{Path.GetExtension(path)}";
                AppSettings.SaveSettings();
            }
            AppData.CacheReloadPage();
        }

        /// <summary>
        /// Show the PathPicker modal to move all Userdata files to a new folder, and set it as default.
        /// </summary>
        public static void ShowChangeUserdataFolderModal()
        {
            ModalData.PathPicker =
                new ModalData.PathPickerRequest(ModalData.PathPickerRequest.PathPickerGoal.SetUserdata);
            ModalData.ShowModal("find");
        }

        /// <summary>
        /// Moves all Userdata files to a new folder, and set it as default.
        /// </summary>
        public static async Task ChangeUserdataFolder()
        {
            var path = ModalData.PathPicker.LastPath;
            // Verify this is different.
            var diOriginal = new DirectoryInfo(Globals.UserDataFolder);
            var diNew = new DirectoryInfo(path);
            if (diOriginal.FullName == diNew.FullName) return;

            if (Directory.Exists(path) && path != "")
            {
                await File.WriteAllTextAsync(Path.Join(Globals.AppDataFolder, "userdata_path.txt"), path);
            }

            bool folderEmpty;
            if (Directory.Exists(path))
                folderEmpty = Globals.IsDirectoryEmpty(path);
            else
            {
                folderEmpty = true;
                Directory.CreateDirectory(path);
            }


            if (folderEmpty)
            {
                await GeneralInvocableFuncs.ShowToast("info", Lang.Instance["Toast_DataLocationCopying"], renderTo: "toastarea");
                if (!Globals.CopyFilesRecursive(Globals.UserDataFolder, path))
                    await GeneralInvocableFuncs.ShowToast("error", Lang.Instance["Toast_FileCopyFail"], renderTo: "toastarea");
            }
            else
                await GeneralInvocableFuncs.ShowToast("info", Lang.Instance["Toast_DataLocationNotCopying"], renderTo: "toastarea");

            await GeneralInvocableFuncs.ShowToast("info", Lang.Instance["Toast_DataLocationSet"], renderTo: "toastarea");
        }

        /// <summary>
        /// Show the Username change modal
        /// </summary>
        public static void ShowChangeAccImageModal()
        {
            ModalData.PathPicker =
                new ModalData.PathPickerRequest(ModalData.PathPickerRequest.PathPickerGoal.SetAccountImage);
            ModalData.ShowModal("find");
        }

        /// <summary>
        /// Update an accounts username
        /// </summary>
        public static async Task ChangeAccImage()
        {
            // Verify path exists and copy image in.
            if (!File.Exists(ModalData.PathPicker.LastPath)) return;
            var imageDest = Path.Join(Globals.UserDataFolder, "wwwroot\\img\\profiles\\", AppData.CurrentSwitcherSafe);
            Globals.CopyFile(ModalData.PathPicker.LastPath, Path.Join(imageDest, AppData.SelectedAccountId + ".jpg"));

            // Update file last write time, so it's not deleted and updated.
            File.SetLastWriteTime(Path.Join(imageDest, AppData.SelectedAccountId + ".jpg"), DateTime.Now);

            // Reload page.
            AppData.CacheReloadPage();
            await GeneralInvocableFuncs.ShowToast("success", Lang.Instance["Toast_UpdatedImage"], renderTo: "toastarea");
        }

        #endregion



        #region TEXT INPUT MODALS

        /// <summary>
        /// Show the Set App Password modal
        /// </summary>
        public static void ShowSetAppPasswordModal()
        {
            ModalData.TextInput =
                new ModalData.TextInputRequest(ModalData.TextInputRequest.TextInputGoal.AppPassword);
            ModalData.ShowModal("getText");
        }

        /// <summary>
        /// Sets the App Password, to stop simple eyes from snooping
        /// </summary>
        public static async Task SetAppPassword()
        {
            AppSettings.PasswordHash = Globals.GetSha256HashString(ModalData.TextInput.LastString);
            AppSettings.SaveSettings();
            await GeneralInvocableFuncs.ShowToast("success", Lang.Instance["Toast_PasswordChanged"], renderTo: "toastarea");
        }

        /// <summary>
        /// Show the Set App Password modal
        /// </summary>
        public static void ShowSetAccountStringModal()
        {
            ModalData.TextInput =
                new ModalData.TextInputRequest(ModalData.TextInputRequest.TextInputGoal.AccString);
            ModalData.ShowModal("getText");
        }

        /// <summary>
        /// Show the Set App Password modal
        /// </summary>
        public static void ShowChangeUsernameModal()
        {
            ModalData.TextInput =
                new ModalData.TextInputRequest(ModalData.TextInputRequest.TextInputGoal.ChangeUsername);
            ModalData.ShowModal("getText");
        }

        /// <summary>
        /// Sets the App Password, to stop simple eyes from snooping
        /// </summary>
        public static async Task SetAccountString()
        {
            ModalData.IsShown = false;

            if (AppData.CurrentSwitcher == "Steam")
            {
                if (ModalData.TextInput.Goal is ModalData.TextInputRequest.TextInputGoal.ChangeUsername)
                    await SteamSwitcherFuncs.ChangeUsername();
            }
            else
            {
                if (ModalData.TextInput.Goal is ModalData.TextInputRequest.TextInputGoal.ChangeUsername)
                    await BasicSwitcherFuncs.ChangeUsername(AppData.SelectedAccountId, ModalData.TextInput.LastString);
                else
                    await BasicSwitcherFuncs.BasicAddCurrent(ModalData.TextInput.LastString);
            }
        }

        #endregion
    }
}

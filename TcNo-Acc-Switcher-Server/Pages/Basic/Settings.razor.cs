﻿// TcNo Account Switcher - A Super fast account switcher
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
using System.Diagnostics;
using System.IO;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Components;
using Microsoft.AspNetCore.Components.Forms;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General;

namespace TcNo_Acc_Switcher_Server.Pages.Basic
{
    public partial class Settings
    {
        [Inject]
        public AppData AppData { get; set; }
        [Inject] private IGeneralFuncs GeneralFuncs { get; }
        protected override void OnInitialized()
        {
            AppData.WindowTitle = Lang["Title_Template_Settings", new { platformName = CurrentPlatform.FullName }];
            Globals.DebugWriteLine(@"[Auto:Basic\Settings.razor.cs.OnInitializedAsync]");
        }

        #region SETTINGS_GENERAL
        // BUTTON: Pick folder
        public async Task PickFolder()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Basic\Settings.razor.cs.PickFolder]");
            await GeneralFuncs.ShowModal("find:Basic:BasicGamesLauncher.exe:BasicSettings");
        }

        // BUTTON: Reset settings
        public void ClearSettings()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Basic\Settings.razor.cs.ClearSettings]");
            Basic.ResetSettings();
            AppData.NavigateToWithToast("/Basic/", "success", Lang["Success"], Lang["Toast_ClearedPlatformSettings", new { platform = "Basic" }]);
        }
        #endregion

        private static bool _currentlyBackingUp;
        private static bool _currentlyRestoring;
        private async void RestoreFile(InputFileChangeEventArgs e)
        {
            if (!_currentlyRestoring)
                _currentlyRestoring = true;
            else
            {
                await GeneralFuncs.ShowToast("error", Lang["Toast_RestoreBusy"], renderTo: "toastarea");
                return;
            }

            foreach (var file in e.GetMultipleFiles(1))
            {
                try
                {
                    if (!file.Name.EndsWith("7z")) continue;

                    await GeneralFuncs.ShowToast("info", Lang["Toast_RestoreExt"], renderTo: "toastarea");

                    var outputFolder = Path.Join("Restore", Path.GetFileNameWithoutExtension(file.Name));
                    Directory.CreateDirectory(outputFolder);
                    var tempFile = Path.Join("Restore", file.Name);

                    // Import 7z
                    var s = file.OpenReadStream(4294967296); // 4 GB as bytes
                    var fs = File.Create(tempFile);
                    await s.CopyToAsync(fs);
                    fs.Close();

                    // Decompress and remove temp file
                    Globals.DecompressZip(tempFile, outputFolder);
                    File.Delete(tempFile);

                    await GeneralFuncs.ShowToast("info", Lang["Toast_RestoreCopying"], renderTo: "toastarea");

                    // Move files and folders back
                    foreach (var (toPath, fromPath) in CurrentPlatform.BackupPaths)
                    {
                        var fullFromPath = Path.Join(outputFolder, fromPath);
                        if (Globals.IsFile(fullFromPath))
                            Globals.CopyFile(fullFromPath, toPath);
                        else if (Globals.IsFolder(fullFromPath))
                        {
                            if (!Globals.CopyFilesRecursive(fullFromPath, toPath))
                                await GeneralFuncs.ShowToast("error", Lang["Toast_FileCopyFail"], renderTo: "toastarea");
                        }
                    }

                    await GeneralFuncs.ShowToast("info", Lang["Toast_RestoreDeleting"], renderTo: "toastarea");

                    // Remove temp files
                    Globals.RecursiveDelete(outputFolder, false);

                    await GeneralFuncs.ShowToast("success", Lang["Toast_RestoreComplete"], renderTo: "toastarea");
                }
                catch (Exception ex)
                {
                    Globals.WriteToLog("Failed to restore from file: " + file.Name, ex);
                    await GeneralFuncs.ShowToast("error", Lang["Status_FailedLog"], renderTo: "toastarea");

                }
                _currentlyRestoring = false;
            }
        }

        /// <summary>
        /// Backs up platform folders, settings, etc - as defined in the platform settings json
        /// </summary>
        private async Task BackupButton(bool everything = false)
        {
            if (!_currentlyBackingUp)
                _currentlyBackingUp = true;
            else
            {
                await GeneralFuncs.ShowToast("error", Lang["Toast_BackupBusy"], renderTo: "toastarea");
                return;
            }
            // Let user know it's copying files to a temp location
            await GeneralFuncs.ShowToast("info", Lang["Toast_BackupCopy"], renderTo: "toastarea");

            // Generate temporary folder:
            var tempFolder = $"BackupTemp\\Backup_{CurrentPlatform.FullName}_{DateTime.Now:dd-MM-yyyy_hh-mm-ss}";
            Directory.CreateDirectory("Backups\\BackupTemp");

            if (!everything)
                foreach (var (f, t) in CurrentPlatform.BackupPaths)
                {
                    var fExpanded = Basic.ExpandEnvironmentVariables(f);
                    var dest = Path.Join(tempFolder, t);

                    // Handle file entry
                    if (Globals.IsFile(f))
                    {
                        Globals.CopyFile(f, dest);
                        continue;
                    }
                    if (!Directory.Exists(fExpanded)) continue;

                    // Handle folder entry
                    if (CurrentPlatform.BackupFileTypesInclude.Count > 0)
                        Globals.CopyFilesRecursive(fExpanded, dest, true, CurrentPlatform.BackupFileTypesInclude, true);
                    else if (CurrentPlatform.BackupFileTypesIgnore.Count > 0)
                        Globals.CopyFilesRecursive(fExpanded, dest, true, CurrentPlatform.BackupFileTypesIgnore, false);
                }
            else
                foreach (var (f, t) in CurrentPlatform.BackupPaths)
                {
                    var fExpanded = Basic.ExpandEnvironmentVariables(f);
                    var dest = Path.Join(tempFolder, t);

                    // Handle file entry
                    if (Globals.IsFile(f))
                    {
                        Globals.CopyFile(f, dest);
                        continue;
                    }
                    if (!Directory.Exists(fExpanded)) continue;

                    // Handle folder entry
                    if (!Globals.CopyFilesRecursive(fExpanded, dest))
                        await GeneralFuncs.ShowToast("error", Lang["Toast_FileCopyFail"], renderTo: "toastarea");
                }

            var backupThread = new Thread(FinishBackup(tempFolder).RunSynchronously);
            backupThread.Start();
        }

        /// <summary>
        /// Runs async so the previous function can return, and an error isn't thrown with the Blazor function timeout
        /// </summary>
        private async Task FinishBackup(string tempFolder)
        {

            var folderSize = Globals.FolderSizeString(tempFolder);
            await GeneralFuncs.ShowToast("info", Lang["Toast_BackupCompress", new { size = folderSize }], duration: 3000, renderTo: "toastarea");

            var zipFile = Path.Join("Backups", (tempFolder.Contains("\\") ? tempFolder.Split("\\")[1] : tempFolder) + ".7z");

            var backupWatcher = new Thread(CompressionUpdater(zipFile).RunSynchronously);
            backupWatcher.Start();
            try
            {
                Globals.CompressFolder(tempFolder, zipFile);
            }
            catch (Exception e)
            {
                if (e is FileNotFoundException && e.ToString().Contains("7z.dll"))
                {
                    await GeneralFuncs.ShowToast("error", Lang["Error_RequiredFileVerify"],
                        "Stylesheet error", "toastarea");
                }
            }

            Globals.RecursiveDelete(tempFolder, false);
            await GeneralFuncs.ShowToast("success", Lang["Toast_BackupComplete", new { size = folderSize, compressedSize = Globals.FileSizeString(zipFile) }], renderTo: "toastarea");

            _currentlyBackingUp = false;
        }

        /// <summary>
        /// Keeps the user updated with compression progress
        /// </summary>
        private async Task CompressionUpdater(string zipFile)
        {
            Thread.Sleep(3500);
            while (_currentlyBackingUp)
            {
                await GeneralFuncs.ShowToast("info", Lang["Toast_BackupProgress", new { compressedSize = Globals.FileSizeString(zipFile) }], duration: 1000, renderTo: "toastarea");
                Thread.Sleep(2000);
            }
        }

        private void OpenBackupFolder()
        {
            if (Directory.Exists("Backups"))
                Process.Start("explorer.exe", Path.GetFullPath("Backups"));
        }
    }
}

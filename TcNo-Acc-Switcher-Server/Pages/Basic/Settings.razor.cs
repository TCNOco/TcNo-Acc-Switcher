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
using System.Diagnostics;
using System.IO;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Components;
using Microsoft.AspNetCore.Components.Forms;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.State;
using TcNo_Acc_Switcher_Server.State.DataTypes;
using TcNo_Acc_Switcher_Server.State.Interfaces;

namespace TcNo_Acc_Switcher_Server.Pages.Basic;

public partial class Settings
{
    [Inject] private IModals Modals { get; set; }
    [Inject] private ITemplatedPlatformSettings TemplatedPlatformSettings { get; set; }

    protected override void OnInitialized()
    {
        AppState.WindowState.WindowTitle = Lang["Title_Template_Settings", new { platformName = TemplatedPlatformState.CurrentPlatform.Name }];
        Globals.DebugWriteLine(@"[Auto:Basic\Settings.razor.cs.OnInitializedAsync]");
    }

    #region SETTINGS_GENERAL
    // BUTTON: Pick folder
    public void PickFolder()
    {
        Globals.DebugWriteLine(@"[ButtonClicked:Basic\Settings.razor.cs.PickFolder]");
        Modals.ShowUpdatePlatformFolderModal();
    }

    // BUTTON: Reset settings
    public void ClearSettings()
    {
        Globals.DebugWriteLine(@"[ButtonClicked:Basic\Settings.razor.cs.ClearSettings]");
        TemplatedPlatformSettings.Reset();
        AppState.Navigation.NavigateToWithToast(NavigationManager, "/Basic/", "Success", Lang["Success"], Lang["Toast_ClearedPlatformSettings", new { platform = "Basic" }]);
    }
    #endregion

    private static bool _currentlyBackingUp;
    private static bool _currentlyRestoring;
    private async Task RestoreFile(InputFileChangeEventArgs e)
    {
        if (!_currentlyRestoring)
            _currentlyRestoring = true;
        else
        {
            Toasts.ShowToastLang(ToastType.Error, "Toast_RestoreBusy");
            return;
        }

        foreach (var file in e.GetMultipleFiles(1))
        {
            try
            {
                if (!file.Name.EndsWith("7z")) continue;

                Toasts.ShowToastLang(ToastType.Info, "Toast_RestoreExt");

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

                Toasts.ShowToastLang(ToastType.Info, "Toast_RestoreCopying");

                // Move files and folders back
                foreach (var (toPath, fromPath) in TemplatedPlatformState.CurrentPlatform.Extras.BackupPaths)
                {
                    var fullFromPath = Path.Join(outputFolder, fromPath);
                    if (Globals.IsFile(fullFromPath))
                        Globals.CopyFile(fullFromPath, toPath);
                    else if (Globals.IsFolder(fullFromPath))
                    {
                        if (!Globals.CopyFilesRecursive(fullFromPath, toPath))
                            Toasts.ShowToastLang(ToastType.Error, "Toast_FileCopyFail");
                    }
                }

                Toasts.ShowToastLang(ToastType.Info, "Toast_RestoreDeleting");

                // Remove temp files
                Globals.RecursiveDelete(outputFolder, false);

                Toasts.ShowToastLang(ToastType.Success, "Toast_RestoreComplete");
            }
            catch (Exception ex)
            {
                Globals.WriteToLog("Failed to restore from file: " + file.Name, ex);
                Toasts.ShowToastLang(ToastType.Error, "Status_FailedLog");

            }
            _currentlyRestoring = false;
        }
    }

    /// <summary>
    /// Backs up platform folders, settings, etc - as defined in the platform settings json
    /// </summary>
    private void BackupButton(bool everything = false)
    {
        if (!_currentlyBackingUp)
            _currentlyBackingUp = true;
        else
        {
            Toasts.ShowToastLang(ToastType.Error, "Toast_BackupBusy");
            return;
        }
        // Let user know it's copying files to a temp location
        Toasts.ShowToastLang(ToastType.Info, "Toast_BackupCopy");

        // Generate temporary folder:
        var tempFolder = $"BackupTemp\\Backup_{TemplatedPlatformState.CurrentPlatform.Name}_{DateTime.Now:dd-MM-yyyy_hh-mm-ss}";
        Directory.CreateDirectory("Backups\\BackupTemp");

        if (!everything)
            foreach (var (f, t) in TemplatedPlatformState.CurrentPlatform.Extras.BackupPaths)
            {
                var fExpanded = TemplatedPlatformFuncs.ExpandEnvironmentVariables(f);
                var dest = Path.Join(tempFolder, t);

                // Handle file entry
                if (Globals.IsFile(f))
                {
                    Globals.CopyFile(f, dest);
                    continue;
                }
                if (!Directory.Exists(fExpanded)) continue;

                // Handle folder entry
                if (TemplatedPlatformState.CurrentPlatform.Extras.BackupFileTypesInclude.Count > 0)
                    Globals.CopyFilesRecursive(fExpanded, dest, true, TemplatedPlatformState.CurrentPlatform.Extras.BackupFileTypesInclude, true);
                else if (TemplatedPlatformState.CurrentPlatform.Extras.BackupFileTypesIgnore.Count > 0)
                    Globals.CopyFilesRecursive(fExpanded, dest, true, TemplatedPlatformState.CurrentPlatform.Extras.BackupFileTypesIgnore, false);
            }
        else
            foreach (var (f, t) in TemplatedPlatformState.CurrentPlatform.Extras.BackupPaths)
            {
                var fExpanded = TemplatedPlatformFuncs.ExpandEnvironmentVariables(f);
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
                    Toasts.ShowToastLang(ToastType.Error,"Toast_FileCopyFail");
            }

        var backupThread = new Thread(() => FinishBackup(tempFolder));
        backupThread.Start();
    }

    /// <summary>
    /// Runs async so the previous function can return, and an error isn't thrown with the Blazor function timeout
    /// </summary>
    private void FinishBackup(string tempFolder)
    {

        var folderSize = Globals.FolderSizeString(tempFolder);
        Toasts.ShowToastLang(ToastType.Info, new LangSub("Toast_BackupCompress", new { size = folderSize }), 3000);

        var zipFile = Path.Join("Backups", (tempFolder.Contains("\\") ? tempFolder.Split("\\")[1] : tempFolder) + ".7z");

        var backupWatcher = new Thread(() => CompressionUpdater(zipFile));
        backupWatcher.Start();
        try
        {
            Globals.CompressFolder(tempFolder, zipFile);
        }
        catch (Exception e)
        {
            if (e is FileNotFoundException && e.ToString().Contains("7z.dll"))
            {
                Toasts.ShowToastLang(ToastType.Error, "Stylesheet error", "Error_RequiredFileVerify");
            }
        }

        Globals.RecursiveDelete(tempFolder, false);
        Toasts.ShowToastLang(ToastType.Success, new LangSub("Toast_BackupComplete", new { size = folderSize, compressedSize = Globals.FileSizeString(zipFile) }));

        _currentlyBackingUp = false;
    }

    /// <summary>
    /// Keeps the user updated with compression progress
    /// </summary>
    private void CompressionUpdater(string zipFile)
    {
        Thread.Sleep(3500);
        while (_currentlyBackingUp)
        {
            Toasts.ShowToastLang(ToastType.Info, new LangSub("Toast_BackupProgress", new { compressedSize = Globals.FileSizeString(zipFile) }), 1000);
            Thread.Sleep(2000);
        }
    }

    private void OpenBackupFolder()
    {
        if (Directory.Exists("Backups"))
            Process.Start("explorer.exe", Path.GetFullPath("Backups"));
    }
}
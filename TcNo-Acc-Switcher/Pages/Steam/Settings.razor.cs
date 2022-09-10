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
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Components;
using Microsoft.AspNetCore.Components.Forms;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher.State.DataTypes;
using TcNo_Acc_Switcher.State.Interfaces;

namespace TcNo_Acc_Switcher.Pages.Steam;

public partial class Settings
{
    [Inject] private IToasts Toasts { get; set; }
    [Inject] private IModals Modals { get; set; }
    [Inject] private ITemplatedPlatformState TemplatedPlatformState { get; set; }

    protected override void OnInitialized()
    {
        AppState.WindowState.WindowTitle = Lang["Title_Steam_Settings"];
    }

    public void SaveAndClose()
    {
        WindowSettings.Save();
        SteamSettings.Save();

        NavigationManager.NavigateTo("/Steam");
    }

    private string _selectedState;
    private Task DefaultState(int state)
    {
        SteamSettings.OverrideState = state;
        _selectedState = StateToString(state);
        return Task.CompletedTask;
    }

    private void OpenBackupFolder()
    {
        if (Directory.Exists("Backups"))
            Process.Start("explorer.exe", Path.GetFullPath("Backups"));
    }

    private readonly Dictionary<string, string> _backupPaths = new()
    {
        { "%Platform_Folder%\\config", "config" },
        { "%Platform_Folder%\\userdata", "userdata" }
    };

    private readonly List<string> _backupFileTypesInclude = new()
    {
        ".cfg", ".ini", ".dat", ".db", ".json", ".ProfileData", ".sav", ".save", ".nfo",".txt", ".vcfg", ".vdf", ".vdf_last", ".vrmanifest", ".xml"
    };

    private static bool _currentlyBackingUp;

    /// <summary>
    /// Backs up platform folders, settings, etc - as defined in the platform settings json
    /// </summary>
    private void BackupButton(bool everything = false)
    {
        if (!_currentlyBackingUp)
            _currentlyBackingUp = true;
        else
            Toasts.ShowToastLang(ToastType.Error, "Toast_BackupBusy");

        // Let user know it's copying files to a temp location
        Toasts.ShowToastLang(ToastType.Info, "Toast_BackupCopy");

        // Generate temporary folder:
        var tempFolder = $"BackupTemp\\Backup_Steam_{DateTime.Now:dd-MM-yyyy_hh-mm-ss}";
        Directory.CreateDirectory("BackupTemp");
        Directory.CreateDirectory("Backups");
        Directory.CreateDirectory(tempFolder);

        if (!everything)
            foreach (var (f, t) in _backupPaths)
            {
                var fExpanded = f.Replace("%Platform_Folder%", SteamSettings.FolderPath);
                if (!Directory.Exists(fExpanded)) continue;
                Globals.CopyFilesRecursive(fExpanded, Path.Join(tempFolder, t), true, _backupFileTypesInclude, true);
            }
        else
            foreach (var (f, t) in _backupPaths)
            {
                var fExpanded = f.Replace("%Platform_Folder%", SteamSettings.FolderPath);
                if (!Directory.Exists(fExpanded)) continue;
                if (!Globals.CopyFilesRecursive(fExpanded, Path.Join(tempFolder, t)))
                    Toasts.ShowToastLang(ToastType.Error, "Toast_FileCopyFail");
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

        var zipFile = Path.Join("Backups", tempFolder.Split(Path.DirectorySeparatorChar).Last() + ".7z");
        Directory.CreateDirectory("Backups");

        var backupWatcher = new Thread(() => CompressionUpdater(zipFile));
        backupWatcher.Start();

        Globals.CompressFolder(tempFolder, zipFile);

        Globals.RecursiveDelete(tempFolder, false);
        if (File.Exists(zipFile))
            Toasts.ShowToastLang(ToastType.Success, new LangSub("Toast_BackupComplete", new { size = folderSize, compressedSize = Globals.FileSizeString(zipFile) }));
        else
        {
            Globals.WriteToLog($"ERROR: Could not find compressed backup file! Expected path: {zipFile}");
            Toasts.ShowToastLang(ToastType.Error, "Toast_BackupFail");
        }

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
            Toasts.ShowToastLang(ToastType.Info, new LangSub("Toast_BackupProgress", new { compressedSize = Globals.FileSizeString(zipFile) }), 2000);
            Thread.Sleep(2000);
        }
    }

    public string StateToString(int state)
    {
        return state switch
        {
            -1 => Lang["NoDefault"],
            0 => Lang["Offline"],
            1 => Lang["Online"],
            2 => Lang["Busy"],
            3 => Lang["Away"],
            4 => Lang["Snooze"],
            5 => Lang["LookingToTrade"],
            6 => Lang["LookingToPlay"],
            7 => Lang["Invisible"],
            _ => ""
        };
    }

    private static bool _currentlyRestoring;
    private void RestoreFile(InputFileChangeEventArgs e)
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
                s.CopyTo(fs);
                fs.Close();

                // Decompress and remove temp file
                Globals.DecompressZip(tempFile, outputFolder);
                File.Delete(tempFile);

                Toasts.ShowToastLang(ToastType.Info, "Toast_RestoreCopying");

                // Move files and folders back
                foreach (var (toPath, fromPath) in _backupPaths)
                {
                    var fullFromPath = Path.Join(outputFolder, fromPath);
                    if (Globals.IsFile(fullFromPath))
                        Globals.CopyFile(fullFromPath, toPath);
                    else if (Globals.IsFolder(fullFromPath) && !Globals.CopyFilesRecursive(fullFromPath, toPath))
                        Toasts.ShowToastLang(ToastType.Error, "Toast_FileCopyFail");
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

    #region SETTINGS_GENERAL
    // BUTTON: Pick folder
    public void PickFolder()
    {
        Globals.DebugWriteLine(@"[ButtonClicked:Steam\Settings.razor.cs.PickFolder]");
        Modals.ShowUpdatePlatformFolderModal();
    }

    // BUTTON: Check account VAC status
    public void ClearVacStatus()
    {
        Globals.DebugWriteLine(@"[ButtonClicked:Steam\Settings.razor.cs.ClearVacStatus]");
        if (Globals.DeleteFile(SteamSettings.VacCacheFile))
            Toasts.ShowToastLang(ToastType.Success, "Toast_Steam_VacCleared");
        else
            Toasts.ShowToastLang(ToastType.Error, "Error", "Toast_Steam_CantDeleteVacCache");
    }

    // BUTTON: Reset settings
    public void ClearSettings()
    {
        Globals.DebugWriteLine(@"[ButtonClicked:Steam\Settings.razor.cs.ClearSettings]");
        SteamSettings.Reset();
        AppState.Navigation.NavigateToWithToast(NavigationManager, "/Steam", "Success", Lang["Success"], Lang["Toast_ClearedPlatformSettings", new { platform = "Steam" }]);
    }

    // BUTTON: Reset images
    /// <summary>
    /// Clears images folder of contents, to re-download them on next load.
    /// </summary>
    public void ClearImages()
    {
        Globals.DebugWriteLine(@"[ButtonClicked:Steam\Settings.razor.cs.ClearImages]");
        if (!Directory.Exists(SteamSettings.SteamImagePath))
        {
            Toasts.ShowToastLang(ToastType.Error, "Error", "Toast_CantClearImages");
        }
        Globals.DeleteFiles(SteamSettings.SteamImagePath);

        // Reload page, then display notification using a new thread.
        AppState.Navigation.ReloadWithToast(NavigationManager, "success", Uri.EscapeDataString(Lang["Success"]), Uri.EscapeDataString(Lang["Toast_ClearedImages"]));
    }
    #endregion
}
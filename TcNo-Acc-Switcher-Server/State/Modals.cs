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
using Microsoft.AspNetCore.Components;
using Microsoft.JSInterop;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.State.DataTypes;
using TcNo_Acc_Switcher_Server.State.Interfaces;

namespace TcNo_Acc_Switcher_Server.State;

public class Modals
{
    [Inject] IAppState AppState { get; set; }
    [Inject] Toasts Toasts { get; set; }
    [Inject] IWindowSettings WindowSettings { get; set; }
    [Inject] JSRuntime JsRuntime { get; set; }
    [Inject] NavigationManager NavigationManager { get; set; }

    public event Action OnChange;
    public void NotifyDataChanged() => OnChange?.Invoke();

    private bool _isShown;
    public bool IsShown
    {
        get => _isShown;
        set
        {
            _isShown = value;
            JsRuntime.InvokeVoidAsync(value ? "showModal" : "hideModal");
            NotifyDataChanged();
        }
    }
    private string _type;
    public string Type
    {
        get => _type;
        set
        {
            _type = value;
            NotifyDataChanged();
        }
    }

    private string _title;
    public string Title
    {
        get => _title;
        set
        {
            _title = value;
            NotifyDataChanged();
        }
    }

    public ExtraArg ExtraArgs { get; set; }
    public StatsSelectorState CurrentStatsSelectorState { get; set; }


    #region PathPicker
    public PathPickerRequest PathPicker { get; set; }

    // These MUST be separate from the class above to refresh the element properly.
    public event Action PathPickerOnChange;
    public void PathPickerNotifyDataChanged() => PathPickerOnChange?.Invoke();
    #endregion

    #region TextInputModal
    public TextInputRequest TextInput { get; set; }

    // These MUST be separate from the class above to refresh the element properly.
    public event Action TextInputOnChange;
    public void TextInputNotifyDataChanged() => TextInputOnChange?.Invoke();
    #endregion

    #region GameStatsModal
    public event Action GameStatsModalOnChange;
    public void GameStatsModalOnChangeChanged() => GameStatsModalOnChange?.Invoke();
    #endregion

    public void ShowModal(string type, ExtraArg arg = ExtraArg.None)
    {
        Type = type;
        ExtraArgs = arg;
        IsShown = true;
    }

    public void ShowGameStatsSelectorModal()
    {
        CurrentStatsSelectorState = StatsSelectorState.GamesList;
        ShowModal("gameStatsSelector");
        GameStatsModalOnChangeChanged();
    }


    #region PATH PICKER MODALS
    public void ShowUpdatePlatformFolderModal()
    {
        PathPicker =
            new PathPickerRequest(PathPickerRequest.PathPickerGoal.FindPlatformExe);
        ShowModal("find");
    }

    /// <summary>
    /// Change the selected platform's EXE folder to another and save changes
    /// </summary>
    public void UpdatePlatformFolder()
    {
        var path = PathPicker.LastPath;
        Globals.DebugWriteLine($@"[Modals.UpdatePlatformFolder] file={AppState.Switcher.CurrentSwitcher}, path={path}");
        var settingsFile = AppState.Switcher.CurrentSwitcher == "Steam"
            ? SteamSettings.SettingsFile
            : CurrentPlatform.SettingsFile;

        var settings = GeneralFuncs.LoadSettings(settingsFile);
        settings["FolderPath"] = path;
        GeneralFuncs.SaveSettings(settingsFile, settings);
        if (!Globals.IsFolder(path))
            path = Path.GetDirectoryName(path); // Remove .exe
        if (!string.IsNullOrWhiteSpace(path) && path.EndsWith(".exe"))
            path = Path.GetDirectoryName(path) ?? string.Join("\\", path.Split("\\")[..^1]);

        if (AppState.Switcher.CurrentSwitcher == "Steam")
            SteamSettings.FolderPath = path;
        else
            BasicSettings.FolderPath = path;
    }

    /// <summary>
    /// Show the PathPicker modal to find an image to import and use as the app background
    /// </summary>
    public void ShowSetBackgroundModal()
    {
        PathPicker =
            new PathPickerRequest(PathPickerRequest.PathPickerGoal.SetBackground);
        ShowModal("find");
    }

    /// <summary>
    /// Import image to use as the app background
    /// </summary>
    public void SetBackground()
    {
        var path = PathPicker.LastPath;

        WindowSettings.Background = $"{path}";

        if (File.Exists(path) && path != "")
        {
            Directory.CreateDirectory(Path.Join(Globals.UserDataFolder, "wwwroot\\img\\custom\\"));
            Globals.CopyFile(path, Path.Join(Globals.UserDataFolder, "wwwroot\\img\\custom\\background" + Path.GetExtension(path)));
            WindowSettings.Background = $"img/custom/background{Path.GetExtension(path)}";
            WindowSettings.Save();
        }

        NavigationManager.NavigateTo(NavigationManager.Uri, forceLoad: true);
    }

    /// <summary>
    /// Show the PathPicker modal to move all Userdata files to a new folder, and set it as default.
    /// </summary>
    public void ShowChangeUserdataFolderModal()
    {
        PathPicker =
            new PathPickerRequest(PathPickerRequest.PathPickerGoal.SetUserdata);
        ShowModal("find");
    }

    /// <summary>
    /// Moves all Userdata files to a new folder, and set it as default.
    /// </summary>
    public async Task ChangeUserdataFolder()
    {
        var path = PathPicker.LastPath;
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
            Toasts.ShowToastLang(ToastType.Info, "Toast_DataLocationCopying");
            if (!Globals.CopyFilesRecursive(Globals.UserDataFolder, path))
                Toasts.ShowToastLang(ToastType.Error, "Toast_FileCopyFail");
        }
        else
            Toasts.ShowToastLang(ToastType.Info, "Toast_DataLocationNotCopying");

        Toasts.ShowToastLang(ToastType.Info, "Toast_DataLocationSet");
    }

    /// <summary>
    /// Show the Username change modal
    /// </summary>
    public void ShowChangeAccImageModal()
    {
        PathPicker =
            new PathPickerRequest(PathPickerRequest.PathPickerGoal.SetAccountImage);
        ShowModal("find");
    }

    /// <summary>
    /// Update an accounts username
    /// </summary>
    public async Task ChangeAccImage()
    {
        // Verify path exists and copy image in.
        if (!File.Exists(PathPicker.LastPath)) return;
        var imageDest = Path.Join(Globals.UserDataFolder, "wwwroot\\img\\profiles\\", AppState.Switcher.CurrentSwitcherSafe);
        Globals.CopyFile(PathPicker.LastPath, Path.Join(imageDest, AppState.Switcher.SelectedAccountId + ".jpg"));

        // Update file last write time, so it's not deleted and updated.
        File.SetLastWriteTime(Path.Join(imageDest, AppState.Switcher.SelectedAccountId + ".jpg"), DateTime.Now);

        // Reload page.
        NavigationManager.NavigateTo(NavigationManager.Uri, forceLoad: true);
        Toasts.ShowToastLang(ToastType.Success, "Toast_UpdatedImage");
    }

    #endregion



    #region TEXT INPUT MODALS

    /// <summary>
    /// Show the Set App Password modal
    /// </summary>
    public void ShowSetAppPasswordModal()
    {
        TextInput =
            new TextInputRequest(TextInputRequest.TextInputGoal.AppPassword);
        ShowModal("getText");
    }

    /// <summary>
    /// Sets the App Password, to stop simple eyes from snooping
    /// </summary>
    public async Task SetAppPassword()
    {
        WindowSettings.PasswordHash = Globals.GetSha256HashString(TextInput.LastString);
        WindowSettings.Save();
        Toasts.ShowToastLang(ToastType.Success, "Toast_PasswordChanged");
    }

    /// <summary>
    /// Show the Set App Password modal
    /// </summary>
    public void ShowSetAccountStringModal()
    {
        TextInput =
            new TextInputRequest(TextInputRequest.TextInputGoal.AccString);
        ShowModal("getText");
    }

    /// <summary>
    /// Show the Set App Password modal
    /// </summary>
    public void ShowChangeUsernameModal()
    {
        TextInput =
            new TextInputRequest(TextInputRequest.TextInputGoal.ChangeUsername);
        ShowModal("getText");
    }

    public async Task SetAccountString()
    {
        IsShown = false;

        if (AppState.Switcher.CurrentSwitcher == "Steam")
        {
            if (TextInput.Goal is TextInputRequest.TextInputGoal.ChangeUsername)
            {
                SteamSettings.CustomAccountNames[AppState.Switcher.SelectedAccountId] = TextInput.LastString;
                AppState.Switcher.SelectedAccount.DisplayName = TextInput.LastString;
                SteamSettings.Save();

                TextInputNotifyDataChanged();
                Toasts.ShowToastLang(ToastType.Success, "Toast_ChangedUsername");
            }
        }
        else
        {
            if (TextInput.Goal is TextInputRequest.TextInputGoal.ChangeUsername)
                await TemplatedChangeUsername(AppState.Switcher.SelectedAccountId, TextInput.LastString);
            else
                await BasicSwitcherFuncs.BasicAddCurrent(TextInput.LastString);
        }
    }

    public static async Task TemplatedChangeUsername(string accId, string newName, bool reload = true)
    {
        LoadAccountIds();
        var oldName = GetNameFromId(accId);

        try
        {
            // No need to rename image as accId. That step is skipped here.
            Directory.Move($"LoginCache\\{CurrentPlatform.SafeName}\\{oldName}\\", $"LoginCache\\{CurrentPlatform.SafeName}\\{newName}\\"); // Rename login cache folder
        }
        catch (IOException e)
        {
            Globals.WriteToLog("Failed to write to file: " + e);
            await GeneralInvocableFuncs.ShowToast("error", Lang["Error_FileAccessDenied", new { logPath = Globals.GetLogPath() }], Lang["Error"], renderTo: "toastarea");
            return;
        }

        try
        {
            AccountIds[accId] = newName;
            SaveAccountIds();
        }
        catch (Exception e)
        {
            Globals.WriteToLog("Failed to change username: " + e);
            await GeneralInvocableFuncs.ShowToast("error", Lang["Toast_CantChangeUsername"], Lang["Error"], renderTo: "toastarea");
            return;
        }

        if (AppState.Switcher.SelectedAccount is not null)
        {
            AppState.Switcher.SelectedAccount.DisplayName = Modals.TextInput.LastString;
            AppState.Switcher.SelectedAccount.NotifyDataChanged();
        }

        await GeneralInvocableFuncs.ShowToast("success", Lang["Toast_ChangedUsername"], renderTo: "toastarea");
    }
    #endregion
}

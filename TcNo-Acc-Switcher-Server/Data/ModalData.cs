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
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data.Settings;

namespace TcNo_Acc_Switcher_Server.Data
{
    public interface IModalData
    {
        event Action OnChange;
        bool IsShown { get; set; }
        string Type { get; set; }
        ExtraArg ExtraArgs { get; set; }
        string Title { get; set; }
        PathPickerRequest PathPicker { get; set; }
        TextInputRequest TextInput { get; set; }
        void NotifyDataChanged();
        void ShowModal(string type, ExtraArg arg = ExtraArg.None);
        event Action PathPickerOnChange;
        void PathPickerNotifyDataChanged();
        event Action TextInputOnChange;
        void TextInputNotifyDataChanged();
        void ShowUpdatePlatformFolderModal();

        /// <summary>
        /// Change the selected platform's EXE folder to another and save changes
        /// </summary>
        void UpdatePlatformFolder();

        /// <summary>
        /// Show the PathPicker modal to find an image to import and use as the app background
        /// </summary>
        void ShowSetBackgroundModal();

        /// <summary>
        /// Import image to use as the app background
        /// </summary>
        void SetBackground();

        /// <summary>
        /// Show the PathPicker modal to move all Userdata files to a new folder, and set it as default.
        /// </summary>
        void ShowChangeUserdataFolderModal();

        /// <summary>
        /// Moves all Userdata files to a new folder, and set it as default.
        /// </summary>
        Task ChangeUserdataFolder();

        /// <summary>
        /// Show the Username change modal
        /// </summary>
        void ShowChangeAccImageModal();

        /// <summary>
        /// Update an accounts username
        /// </summary>
        Task ChangeAccImage();

        /// <summary>
        /// Show the Set App Password modal
        /// </summary>
        void ShowSetAppPasswordModal();

        /// <summary>
        /// Sets the App Password, to stop simple eyes from snooping
        /// </summary>
        Task SetAppPassword();

        /// <summary>
        /// Show the Set App Password modal
        /// </summary>
        void ShowSetAccountStringModal();

        /// <summary>
        /// Show the Set App Password modal
        /// </summary>
        void ShowChangeUsernameModal();

        /// <summary>
        /// Sets the App Password, to stop simple eyes from snooping
        /// </summary>
        Task SetAccountString();
    }

    public class ModalData : IModalData
    {
        private readonly ILang _lang;
        private readonly IAppSettings _appSettings;
        private readonly IAppData _appData;
        private readonly Lazy<ISteam> _lSteam;
        private ISteam Steam => _lSteam.Value;
        private readonly Lazy<IBasic> _lBasic;
        private IBasic Basic => _lBasic.Value;
        private readonly ICurrentPlatform _currentPlatform;
        private readonly IGeneralFuncs _generalFuncs;

        public ModalData(ILang lang, IAppSettings appSettings, IAppData appData, Lazy<ISteam> lSteam, Lazy<IBasic> lBasic, ICurrentPlatform currentPlatform, IGeneralFuncs generalFuncs)
        {
            _lang = lang;
            _appSettings = appSettings;
            _appData = appData;
            _lSteam = lSteam;
            _lBasic = lBasic;
            _currentPlatform = currentPlatform;
            _generalFuncs = generalFuncs;
        }

        public event Action OnChange;
        public void NotifyDataChanged() => OnChange?.Invoke();

        private bool _isShown;
        public bool IsShown
        {
            get => _isShown;
            set
            {
                _isShown = value;
                _ = _appData.InvokeVoidAsync(value ? "showModal" : "hideModal");
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

        public ExtraArg ExtraArgs { get; set; }

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

        public void ShowModal(string type, ExtraArg arg = ExtraArg.None)
        {
            Type = type;
            ExtraArgs = arg;
            IsShown = true;
        }


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


        #region PATH PICKER MODALS
        public void ShowUpdatePlatformFolderModal()
        {
            PathPicker = new PathPickerRequest(PathPickerRequest.PathPickerGoal.FindPlatformExe);
            ShowModal("find");
        }

        /// <summary>
        /// Change the selected platform's EXE folder to another and save changes
        /// </summary>
        public void UpdatePlatformFolder()
        {
            var path = PathPicker.LastPath;

            Globals.DebugWriteLine($@"[ModalData.UpdatePlatformFolder] file={_appData.CurrentSwitcher}, path={path}");
            var settingsFile = _appData.CurrentSwitcher == "Steam"
                ? Steam.SettingsFile
                : _currentPlatform.SettingsFile;

            var settings = _generalFuncs.LoadSettings(settingsFile);
            settings["FolderPath"] = path;
            Data.GeneralFuncs.SaveSettings(settingsFile, settings);
            if (!Globals.IsFolder(path))
                path = Path.GetDirectoryName(path); // Remove .exe
            if (!string.IsNullOrWhiteSpace(path) && path.EndsWith(".exe"))
                path = Path.GetDirectoryName(path) ?? string.Join("\\", path.Split("\\")[..^1]);

            if (_appData.CurrentSwitcher == "Steam")
                Steam.FolderPath = path;
            else
                Basic.FolderPath = path;
        }

        /// <summary>
        /// Show the PathPicker modal to find an image to import and use as the app background
        /// </summary>
        public void ShowSetBackgroundModal()
        {
            PathPicker = new PathPickerRequest(PathPickerRequest.PathPickerGoal.SetBackground);
            ShowModal("find");
        }

        /// <summary>
        /// Import image to use as the app background
        /// </summary>
        public void SetBackground()
        {
            var path = PathPicker.LastPath;

            _appSettings.Background = $"{path}";

            if (File.Exists(path) && path != "")
            {
                Directory.CreateDirectory(Path.Join(Globals.UserDataFolder, "wwwroot\\img\\custom\\"));
                Globals.CopyFile(path, Path.Join(Globals.UserDataFolder, "wwwroot\\img\\custom\\background" + Path.GetExtension(path)));
                _appSettings.Background = $"img/custom/background{Path.GetExtension(path)}";
                _appSettings.SaveSettings();
            }
            _appData.CacheReloadPage();
        }

        /// <summary>
        /// Show the PathPicker modal to move all Userdata files to a new folder, and set it as default.
        /// </summary>
        public void ShowChangeUserdataFolderModal()
        {
            PathPicker = new PathPickerRequest(PathPickerRequest.PathPickerGoal.SetUserdata);
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
                await _generalFuncs.ShowToast("info", _lang["Toast_DataLocationCopying"], renderTo: "toastarea");
                if (!Globals.CopyFilesRecursive(Globals.UserDataFolder, path))
                    await _generalFuncs.ShowToast("error", _lang["Toast_FileCopyFail"], renderTo: "toastarea");
            }
            else
                await _generalFuncs.ShowToast("info", _lang["Toast_DataLocationNotCopying"], renderTo: "toastarea");

            await _generalFuncs.ShowToast("info", _lang["Toast_DataLocationSet"], renderTo: "toastarea");
        }

        /// <summary>
        /// Show the Username change modal
        /// </summary>
        public void ShowChangeAccImageModal()
        {
            PathPicker = new PathPickerRequest(PathPickerRequest.PathPickerGoal.SetAccountImage);
            ShowModal("find");
        }

        /// <summary>
        /// Update an accounts username
        /// </summary>
        public async Task ChangeAccImage()
        {
            // Verify path exists and copy image in.
            if (!File.Exists(PathPicker.LastPath)) return;
            var imageDest = Path.Join(Globals.UserDataFolder, "wwwroot\\img\\profiles\\", _appData.CurrentSwitcherSafe);
            Globals.CopyFile(PathPicker.LastPath, Path.Join(imageDest, _appData.SelectedAccountId + ".jpg"));

            // Update file last write time, so it's not deleted and updated.
            File.SetLastWriteTime(Path.Join(imageDest, _appData.SelectedAccountId + ".jpg"), DateTime.Now);

            // Reload page.
            _appData.CacheReloadPage();
            await _generalFuncs.ShowToast("success", _lang["Toast_UpdatedImage"], renderTo: "toastarea");
        }

        #endregion



        #region TEXT INPUT MODALS

        /// <summary>
        /// Show the Set App Password modal
        /// </summary>
        public void ShowSetAppPasswordModal()
        {
            TextInput = new TextInputRequest(TextInputRequest.TextInputGoal.AppPassword);
            ShowModal("getText");
        }

        /// <summary>
        /// Sets the App Password, to stop simple eyes from snooping
        /// </summary>
        public async Task SetAppPassword()
        {
            _appSettings.PasswordHash = Globals.GetSha256HashString(TextInput.LastString);
            _appSettings.SaveSettings();
            await _generalFuncs.ShowToast("success", _lang["Toast_PasswordChanged"], renderTo: "toastarea");
        }

        /// <summary>
        /// Show the Set App Password modal
        /// </summary>
        public void ShowSetAccountStringModal()
        {
            TextInput = new TextInputRequest(TextInputRequest.TextInputGoal.AccString);
            ShowModal("getText");
        }

        /// <summary>
        /// Show the Set App Password modal
        /// </summary>
        public void ShowChangeUsernameModal()
        {
            TextInput = new TextInputRequest(TextInputRequest.TextInputGoal.ChangeUsername);
            ShowModal("getText");
        }

        /// <summary>
        /// Sets the App Password, to stop simple eyes from snooping
        /// </summary>
        public async Task SetAccountString()
        {
            IsShown = false;

            if (_appData.CurrentSwitcher == "Steam")
            {
                if (TextInput.Goal is TextInputRequest.TextInputGoal.ChangeUsername)
                    await Steam.ChangeUsername();
            }
            else
            {
                if (TextInput.Goal is TextInputRequest.TextInputGoal.ChangeUsername)
                    await Basic.ChangeUsername(_appData.SelectedAccountId, TextInput.LastString);
                else
                    await Basic.BasicAddCurrent(TextInput.LastString);
            }
        }

        #endregion
    }
    public class PathPickerRequest
    {
        [Inject] private ILang Lang { get; }
        [Inject] private IAppData AppData { get; }
        [Inject] private IModalData ModalData { get; }
        [Inject] private ICurrentPlatform CurrentPlatform { get; }

        public enum PathPickerElement
        {
            None,
            File,
            Folder
        }

        public enum PathPickerGoal
        {
            FindPlatformExe,
            SetBackground,
            SetUserdata,
            SetAccountImage
        }

        /// <summary>
        /// This can be a filename, a folder name, or: AnyFile, AnyFolder
        /// </summary>
        public string RequestedFile;
        public string LastPath = "";
        public bool ShowFiles;
        public PathPickerElement LastElement = PathPickerElement.None;
        public PathPickerGoal Goal;

        public string ModalHeader;
        public MarkupString ModalText;
        public string ModalButtonText;

        public PathPickerRequest() { }
        public PathPickerRequest(PathPickerGoal goal, string requestedFile = "", bool showFiles = true)
        {
            // Clear existing data, if any.
            RequestedFile = requestedFile;
            ShowFiles = showFiles;
            Goal = goal;
            LastPath = "";

            if (Goal == PathPickerGoal.FindPlatformExe)
            {
                string platformName = AppData.CurrentSwitcher;
                RequestedFile = AppData.CurrentSwitcher == "Steam" ? "steam.exe" : CurrentPlatform.ExeName;

                ModalHeader = Lang["Modal_Title_LocatePlatform", new { platform = platformName }];
                ModalText = new MarkupString(Lang["Modal_EnterDirectory", new { platform = platformName }]);
                ModalButtonText = Lang["Modal_LocatePlatformFolder", new { platform = platformName }];
            }
            else if (Goal == PathPickerGoal.SetBackground)
            {
                ModalHeader = Lang["Modal_Title_Background"];
                ModalText = new MarkupString(Lang["Modal_SetBackground"]);
                ModalButtonText = Lang["Modal_SetBackground_Button"];
                RequestedFile = "AnyFile";
            }
            else if (Goal == PathPickerGoal.SetUserdata)
            {
                ModalHeader = Lang["Modal_Title_Userdata"];
                ModalText = new MarkupString(Lang["Modal_SetUserdata"]);
                ModalButtonText = Lang["Modal_SetUserdata_Button"];
                RequestedFile = "AnyFolder";
            }
            else if (Goal == PathPickerGoal.SetAccountImage)
            {
                ModalHeader = Lang["Modal_Title_Userdata"];
                ModalText = new MarkupString(Lang["Modal_SetImageHeader"]);
                ModalButtonText = Lang["Modal_SetImage"];
                RequestedFile = "AnyFile";
            }

            ModalData.PathPickerNotifyDataChanged();
        }
    }
    public class TextInputRequest
    {
        [Inject] private ILang Lang { get; }
        [Inject] private IAppData AppData { get; }
        [Inject] private IModalData ModalData { get; }
        [Inject] private ICurrentPlatform CurrentPlatform { get; }
        public enum TextInputGoal
        {
            AppPassword,
            AccString,
            ChangeUsername
        }

        public string LastString = "";
        public TextInputGoal Goal;

        public string ModalHeader;
        public MarkupString ModalText;
        public MarkupString ModalSubheading;
        public string ModalButtonText;
        public MarkupString ExtraButtons;

        public TextInputRequest() { }
        public TextInputRequest(TextInputGoal goal)
        {
            // Clear existing data, if any.
            LastString = "";
            Goal = goal;

            if (Goal == TextInputGoal.AccString)
            {
                // Adding a new account, but need a DisplayName before.
                ModalSubheading = new MarkupString();

                ModalHeader = Lang["Modal_Title_AddNew", new { platform = AppData.CurrentSwitcher }];
                ModalText = new MarkupString(Lang["Modal_AddNew", new { platform = AppData.CurrentSwitcher }]);
                ModalButtonText = Lang["Modal_AddCurrentAccount", new { platform = AppData.CurrentSwitcher }];
                ExtraButtons = AppData.CurrentSwitcher == "Steam" ? new MarkupString() : CurrentPlatform.GetUserModalExtraButtons;
            }
            else if (Goal == TextInputGoal.AppPassword)
            {
                ExtraButtons = new MarkupString();

                ModalHeader = Lang["Modal_Title_SetPassword"];
                ModalSubheading = new MarkupString(Lang["Modal_SetPassword"]);
                ModalText = new MarkupString(Lang["Modal_SetPassword_Info", new { link = "https://github.com/TcNobo/TcNo-Acc-Switcher/wiki/FAQ---More-Info#can-i-put-this-program-on-a-usb-portable" }]);
                ModalButtonText = Lang["Modal_SetPassword_Button"];
            }
            else if (Goal == TextInputGoal.ChangeUsername)
            {
                ModalSubheading = new MarkupString();

                ModalHeader = Lang["Modal_Title_ChangeUsername"];
                ModalText = new MarkupString(Lang["Modal_ChangeUsername", new { link = AppData.CurrentSwitcher }]);
                ModalButtonText = Lang["Toast_SetUsername"];
                ExtraButtons = AppData.CurrentSwitcher == "Steam" ? new MarkupString() : CurrentPlatform.GetUserModalExtraButtons;
            }

            ModalData.TextInputNotifyDataChanged();
        }
    }

    public enum ExtraArg
    {
        None,
        RestartAsAdmin,
        ClearStats,
        ForgetAccount
    }
}

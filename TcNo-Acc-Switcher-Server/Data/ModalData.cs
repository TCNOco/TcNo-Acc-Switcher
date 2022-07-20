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
using Microsoft.AspNetCore.Components;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Data.Settings;

namespace TcNo_Acc_Switcher_Server.Data
{
    public class ModalData
    {
        private static readonly Lang Lang = Lang;
        private static ModalData _instance = new();

        private static readonly object LockObj = new();

        public static ModalData Instance
        {
            get
            {
                lock (LockObj)
                {
                    return _instance ??= new ModalData();
                }
            }
            set
            {
                lock (LockObj)
                {
                    _instance = value;
                }
            }
        }
        public event Action OnChange;
        public void NotifyDataChanged() => OnChange?.Invoke();

        private bool _isShown;
        public static bool IsShown
        {
            get => Instance._isShown;
            set
            {
                Instance._isShown = value;
                _ = AppData.InvokeVoidAsync(value ? "showModal" : "hideModal");
                Instance.NotifyDataChanged();
            }
        }

        private string _type;

        public static string Type
        {
            get => Instance._type;
            set
            {
                Instance._type = value;
                Instance.NotifyDataChanged();
            }
        }

        public enum ExtraArg
        {
            None,
            RestartAsAdmin,
            ClearStats,
            ForgetAccount
        }

        private ExtraArg _extraArgs;
        public static ExtraArg ExtraArgs { get => Instance._extraArgs; set => Instance._extraArgs = value; }

        private string _title;

        public static string Title
        {
            get => Instance._title;
            set
            {
                Instance._title = value;
                Instance.NotifyDataChanged();
            }
        }

        public static void ShowModal(string type, ExtraArg arg = ExtraArg.None)
        {
            Type = type;
            ExtraArgs = arg;
            IsShown = true;
        }

        public static void ShowGameStatsSelectorModal()
        {
            CurrentStatsSelectorState = StatsSelectorState.GamesList;
            ShowModal("gameStatsSelector");
            GameStatsModalOnChangeChanged();
        }

        private StatsSelectorState _currentStatsSelectorState;
        public static StatsSelectorState CurrentStatsSelectorState {get => Instance._currentStatsSelectorState; set => Instance._currentStatsSelectorState = value; }
        public enum StatsSelectorState
        {
            GamesList,
            VarsList
        }

        #region PathPicker
        public class PathPickerRequest
        {
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

            public PathPickerRequest() {}
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

                PathPickerNotifyDataChanged();
            }
        }

        private PathPickerRequest _pathPicker;
        public static PathPickerRequest PathPicker { get => Instance._pathPicker; set => Instance._pathPicker = value; }

        // These MUST be separate from the class above to refresh the element properly.
        public event Action PathPickerOnChange;
        public static void PathPickerNotifyDataChanged() => Instance.PathPickerOnChange?.Invoke();
        #endregion

        #region TextInputModal
        public class TextInputRequest
        {
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
            public MarkupString ModalSubheading = new();
            public string ModalButtonText;
            public MarkupString ExtraButtons = new();

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

                TextInputNotifyDataChanged();
            }
        }

        private TextInputRequest _textInput;
        public static TextInputRequest TextInput { get => Instance._textInput; set => Instance._textInput = value; }

        // These MUST be separate from the class above to refresh the element properly.
        public event Action TextInputOnChange;
        public static void TextInputNotifyDataChanged() => Instance.TextInputOnChange?.Invoke();
        #endregion


        #region GameStatsModal
        public event Action GameStatsModalOnChange;
        public static void GameStatsModalOnChangeChanged() => Instance.GameStatsModalOnChange?.Invoke();
        #endregion
    }
}

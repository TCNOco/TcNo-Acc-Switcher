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
using TcNo_Acc_Switcher_Server.Data;

namespace TcNo_Acc_Switcher_Server.Data
{
    public class ModalData
    {
        private static readonly Lang Lang = Lang.Instance;
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
            /// This can be a filename, a folder name, or: AnyFile, AnyFolder, *
            /// </summary>
            public string RequestedFile;
            public string LastPath = "";
            public bool ShowFiles;
            public PathPickerElement LastElement = PathPickerElement.None;
            public PathPickerGoal Goal;

            public string ModalHeader;
            public string ModalText;
            public string ModalButtonText;

            public PathPickerRequest() {}
            public PathPickerRequest(PathPickerGoal goal, string requestedFile = "", bool showFiles = true)
            {
                // Clear existing data, if any.
                RequestedFile = requestedFile;
                ShowFiles = showFiles;
                Goal = goal;

                if (Goal == PathPickerGoal.FindPlatformExe)
                {
                    string platformName, platformExe;
                    if (AppData.CurrentSwitcher == "Steam")
                    {
                        platformName = "Steam";
                        platformExe = "steam.exe";
                    }
                    else
                    {
                        platformName = CurrentPlatform.FullName;
                        platformExe = CurrentPlatform.ExeName;
                    }

                    ModalHeader = Lang.Instance["Modal_Title_LocatePlatform", new { platform = platformName }];
                    ModalText = Lang.Instance["Modal_EnterDirectory", new { platform = platformName }];
                    ModalButtonText = Lang.Instance["Modal_LocatePlatformFolder", new { platform = platformName }];
                    RequestedFile = platformExe;
                }
                // TODO: Add remaining actions here make them actually do something in the modal window button click functions.

                PathPickerNotifyDataChanged();
            }
        }

        private PathPickerRequest _pathPicker;
        public static PathPickerRequest PathPicker { get => Instance._pathPicker; set => Instance._pathPicker = value; }

        // These MUST be separate from the class above to refresh the element properly.
        public event Action PathPickerOnChange;
        public static void PathPickerNotifyDataChanged() => Instance.PathPickerOnChange?.Invoke();

        #endregion
    }
}

using Microsoft.AspNetCore.Components;
using TcNo_Acc_Switcher_Server.State.Interfaces;

namespace TcNo_Acc_Switcher_Server.State.DataTypes
{
    public class PathPickerRequest
    {
        [Inject] private NewLang Lang { get; set; }
        [Inject] private IAppState AppState { get; set; }


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
                string platformName = AppState.Switcher.CurrentSwitcher;
                RequestedFile = AppState.Switcher.CurrentSwitcher == "Steam" ? "steam.exe" : CurrentPlatform.ExeName;

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

            AppState.Modal.PathPickerNotifyDataChanged();
        }
    }
}

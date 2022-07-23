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

using Microsoft.AspNetCore.Components;
using TcNo_Acc_Switcher_Server.State.Interfaces;

namespace TcNo_Acc_Switcher_Server.State.DataTypes;

public class PathPickerRequest
{
    [Inject] private Lang Lang { get; set; }
    [Inject] private IAppState AppState { get; set; }


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
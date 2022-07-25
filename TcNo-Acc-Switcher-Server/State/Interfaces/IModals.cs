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
using System.ComponentModel;
using TcNo_Acc_Switcher_Server.State.DataTypes;

namespace TcNo_Acc_Switcher_Server.State.Interfaces;

public interface IModals
{
    event PropertyChangedEventHandler PropertyChanged;
    bool IsShown { get; set; }
    string Type { get; set; }
    string Title { get; set; }
    ExtraArg ExtraArgs { get; set; }
    StatsSelectorState CurrentStatsSelectorState { get; set; }
    PathPickerRequest PathPicker { get; set; }
    TextInputRequest TextInput { get; set; }
    event Action PathPickerOnChange;
    void PathPickerNotifyDataChanged();
    event Action TextInputOnChange;
    void TextInputNotifyDataChanged();
    event Action GameStatsModalOnChange;
    void GameStatsModalOnChangeChanged();
    void ShowModal(string type, ExtraArg arg = ExtraArg.None);
    void ShowGameStatsSelectorModal();
    void ShowUpdatePlatformFolderModal();

    /// <summary>
    /// Show the PathPicker modal to find an image to import and use as the app background
    /// </summary>
    void ShowSetBackgroundModal();

    /// <summary>
    /// Show the Username change modal
    /// </summary>
    void ShowChangeAccImageModal();

    /// <summary>
    /// Show the Set App Password modal
    /// </summary>
    void ShowChangeUsernameModal();
}
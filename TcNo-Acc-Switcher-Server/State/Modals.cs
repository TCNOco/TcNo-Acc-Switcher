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
using System.ComponentModel;
using System.Runtime.CompilerServices;
using TcNo_Acc_Switcher_Server.State.DataTypes;
using TcNo_Acc_Switcher_Server.State.Interfaces;

namespace TcNo_Acc_Switcher_Server.State;

public class Modals : IModals, INotifyPropertyChanged
{
    public event PropertyChangedEventHandler? PropertyChanged;
    protected void OnPropertyChanged(string propertyName) => PropertyChanged?.Invoke(this, new PropertyChangedEventArgs(propertyName));
    protected bool SetField<T>(ref T field, T value, [CallerMemberName] string propertyName = "")
    {
        if (EqualityComparer<T>.Default.Equals(field, value)) return false;
        field = value;
        OnPropertyChanged(propertyName);
        return true;
    }

    public Modals() { }


    private bool _isShown;
    public bool IsShown
    {
        get => _isShown;
        set => SetField(ref _isShown, value);
    }
    private string _type;
    public string Type
    {
        get => _type;
        set => SetField(ref _type, value);
    }

    private string _title;
    public string Title
    {
        get => _title;
        set => SetField(ref _title, value);
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

    /// <summary>
    /// This can be called to close the modal, and unload everything.
    /// </summary>
    public void CloseModal()
    {
        IsShown = false;
        Title = null;
        Type = null;
        ExtraArgs = ExtraArg.None;
        CurrentStatsSelectorState = StatsSelectorState.None;
        PathPicker = null;
        TextInput = null;
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
            new PathPickerRequest(PathPickerGoal.FindPlatformExe);
        ShowModal("find");
    }

    /// <summary>
    /// Show the PathPicker modal to find an image to import and use as the app background
    /// </summary>
    public void ShowSetBackgroundModal()
    {
        PathPicker = new PathPickerRequest(PathPickerGoal.SetBackground);
        ShowModal("find");
    }

    /// <summary>
    /// Show the Username change modal
    /// </summary>
    public void ShowChangeAccImageModal()
    {
        PathPicker =
            new PathPickerRequest(PathPickerGoal.SetAccountImage);
        ShowModal("find");
    }
    #endregion


    #region TEXT INPUT MODALS
    /// <summary>
    /// Show the Set App Password modal
    /// </summary>
    public void ShowChangeUsernameModal()
    {
        TextInput = new TextInputRequest(TextInputGoal.ChangeUsername);
        ShowModal("getText");
    }
    #endregion
}

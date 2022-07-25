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
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.State.DataTypes;
using TcNo_Acc_Switcher_Server.State.Interfaces;

namespace TcNo_Acc_Switcher_Server.State;

public class Modals : IModals
{
    [Inject] private IJSRuntime JsRuntime { get; set; }

    public Modals() { }


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
            new PathPickerRequest(PathPickerGoal.FindPlatformExe);
        ShowModal("find");
    }

    /// <summary>
    /// Show the PathPicker modal to find an image to import and use as the app background
    /// </summary>
    public void ShowSetBackgroundModal()
    {
        PathPicker =
            new PathPickerRequest(PathPickerGoal.SetBackground);
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
        TextInput =
            new TextInputRequest(TextInputGoal.ChangeUsername);
        ShowModal("getText");
    }
    #endregion
}

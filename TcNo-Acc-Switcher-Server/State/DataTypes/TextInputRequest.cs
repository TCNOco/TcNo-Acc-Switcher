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

public class TextInputRequest
{
    [Inject] private Lang Lang { get; set; }
    [Inject] private IAppState AppState { get; set; }

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

            ModalHeader = Lang["Modal_Title_AddNew", new { platform = AppState.Switcher.CurrentSwitcher }];
            ModalText = new MarkupString(Lang["Modal_AddNew", new { platform = AppState.Switcher.CurrentSwitcher }]);
            ModalButtonText = Lang["Modal_AddCurrentAccount", new { platform = AppState.Switcher.CurrentSwitcher }];
            ExtraButtons = AppState.Switcher.CurrentSwitcher == "Steam" ? new MarkupString() : CurrentPlatform.GetUserModalExtraButtons;
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
            ModalText = new MarkupString(Lang["Modal_ChangeUsername", new { link = AppState.Switcher.CurrentSwitcher }]);
            ModalButtonText = Lang["Toast_SetUsername"];
            ExtraButtons = AppState.Switcher.CurrentSwitcher == "Steam" ? new MarkupString() : CurrentPlatform.GetUserModalExtraButtons;
        }

        AppState.Modal.TextInputNotifyDataChanged();
    }
}
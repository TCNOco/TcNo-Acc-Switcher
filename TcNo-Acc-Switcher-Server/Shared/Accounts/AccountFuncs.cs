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

using System.Linq;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Components;
using Microsoft.AspNetCore.Components.Web;
using Microsoft.JSInterop;
using TcNo_Acc_Switcher_Server.State;
using TcNo_Acc_Switcher_Server.State.Classes;
using TcNo_Acc_Switcher_Server.State.Interfaces;

namespace TcNo_Acc_Switcher_Server.Shared.Accounts;

public class AccountFuncs : ComponentBase
{
    [Inject] private JSRuntime JsRuntime { get; set; }
    [Inject] private AppState AppState { get; set; }
    [Inject] private ILang Lang { get; set; }

    #region Selecting accounts
    /// <summary>
    /// Highlights the specified account
    /// </summary>
    public void SetSelectedAccount(Account acc)
    {
        AppState.Switcher.CurrentStatus = Lang["Status_SelectedAccount", new { name = acc.DisplayName }];
        AppState.Switcher.SelectedAccount = acc;
        UnselectAllAccounts();
        acc.IsChecked = true;
    }

    /// <summary>
    /// Removes highlight from all accounts
    /// </summary>
    public void UnselectAllAccounts()
    {
        if (AppState.Switcher.CurrentSwitcher == "Steam")
            foreach (var account in AppState.Switcher.SteamAccounts)
            {
                account.IsChecked = false;
            }
        else
            foreach (var account in AppState.Switcher.TemplatedAccounts)
            {
                account.IsChecked = false;
            }
    }
    #endregion

    #region Context menu
    /// <summary>
    /// Highlights the right-clicked account, and shows the context menu
    /// </summary>
    /// <param name="e"></param>
    /// <param name="acc"></param>
    public async void AccountRightClick(MouseEventArgs e, Account acc)
    {
        if (e.Button != 2) return;
        SetSelectedAccount(acc);
        await JsRuntime.InvokeVoidAsync("positionAndShowMenu", e, "#AccOrPlatList");
    }
    #endregion

    #region Highlighting current account
    /// <summary>
    /// Highlights the specified account
    /// </summary>
    public async Task SetCurrentAccount(Account acc)
    {
        await UnCurrentAllAccounts();
        acc.IsCurrent = true;
        acc.TitleText = $"{Lang["Tooltip_CurrentAccount"]}";

        // getBestOffset
        await JsRuntime.InvokeVoidAsync("setBestOffset", acc.AccountId);
        // then initTooltips
        await JsRuntime.InvokeVoidAsync("initTooltips");
    }
    public async Task SetCurrentAccount(string accId) =>
        await SetCurrentAccount(AppState.Switcher.CurrentSwitcher == "Steam" ? AppState.Switcher.SteamAccounts.First(x => x.AccountId == accId) : AppState.Switcher.TemplatedAccounts.First(x => x.AccountId == accId));

    /// <summary>
    /// Removes "currently logged in" border from all accounts
    /// </summary>
    public async Task UnCurrentAllAccounts()
    {
        if (AppState.Switcher.CurrentSwitcher == "Steam")
            foreach (var account in AppState.Switcher.SteamAccounts)
            {
                account.IsCurrent = false;
            }
        else
            foreach (var account in AppState.Switcher.TemplatedAccounts)
            {
                account.IsCurrent = false;
            }

        // Clear the hover text
        await JsRuntime.InvokeVoidAsync("clearAccountTooltips");
    }
    #endregion
}
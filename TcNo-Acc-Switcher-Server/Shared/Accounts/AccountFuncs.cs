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

using System.Threading.Tasks;
using Microsoft.AspNetCore.Components;
using Microsoft.AspNetCore.Components.Web;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.State.Classes;

namespace TcNo_Acc_Switcher_Server.Shared.Accounts
{
    public class AccountFuncs : ComponentBase
    {
        #region Selecting accounts
        /// <summary>
        /// Highlights the specified account
        /// </summary>
        public static void SetSelectedAccount(Account acc)
        {
            AppData.CurrentStatus = Lang["Status_SelectedAccount", new { name = acc.DisplayName }];
            AppData.SelectedAccount = acc;
            UnselectAllAccounts();
            acc.IsChecked = true;
            AppData.Instance.NotifyDataChanged();
        }

        /// <summary>
        /// Removes highlight from all accounts
        /// </summary>
        public static void UnselectAllAccounts()
        {
            if (AppData.CurrentSwitcher == "Steam")
                foreach (var account in AppData.SteamAccounts)
                {
                    account.IsChecked = false;
                }
            else
                foreach (var account in AppData.BasicAccounts)
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
        public static async void AccountRightClick(MouseEventArgs e, Account acc)
        {
            if (e.Button != 2) return;
            SetSelectedAccount(acc);
            await AppData.InvokeVoidAsync("positionAndShowMenu", e, "#AccOrPlatList");
        }
        #endregion

        #region Highlighting current account
        /// <summary>
        /// Highlights the specified account
        /// </summary>
        public static async Task SetCurrentAccount(Account acc)
        {
            await UnCurrentAllAccounts();
            acc.IsCurrent = true;
            acc.TitleText = $"{Lang["Tooltip_CurrentAccount"]}";
            AppData.Instance.NotifyDataChanged();

            // getBestOffset
            await AppData.InvokeVoidAsync("setBestOffset", acc.AccountId);
            // then initTooltips
            await AppData.InvokeVoidAsync("initTooltips");
        }
        public static async Task SetCurrentAccount(string accId) =>
            await SetCurrentAccount(AppData.CurrentSwitcher == "Steam" ? AppData.SteamAccounts.First(x => x.AccountId == accId) : AppData.BasicAccounts.First(x => x.AccountId == accId));

        /// <summary>
        /// Removes "currently logged in" border from all accounts
        /// </summary>
        public static async Task UnCurrentAllAccounts()
        {
            if (AppData.CurrentSwitcher == "Steam")
                foreach (var account in AppData.SteamAccounts)
                {
                    account.IsCurrent = false;
                }
            else
                foreach (var account in AppData.BasicAccounts)
                {
                    account.IsCurrent = false;
                }

            // Clear the hover text
            await AppData.InvokeVoidAsync("clearAccountTooltips");
        }
        #endregion
    }
}

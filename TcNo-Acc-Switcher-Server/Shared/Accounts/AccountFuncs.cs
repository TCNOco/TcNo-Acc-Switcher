﻿// TcNo Account Switcher - A Super fast account switcher
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
using TcNo_Acc_Switcher_Server.Data;

namespace TcNo_Acc_Switcher_Server.Shared.Accounts
{
    public interface IAccountFuncs
    {
        /// <summary>
        /// Highlights the specified account
        /// </summary>
        void SetSelectedAccount(Account acc);

        /// <summary>
        /// Removes highlight from all accounts
        /// </summary>
        void UnselectAllAccounts();

        /// <summary>
        /// Highlights the right-clicked account, and shows the context menu
        /// </summary>
        /// <param name="e"></param>
        /// <param name="acc"></param>
        void AccountRightClick(MouseEventArgs e, Account acc);

        /// <summary>
        /// Highlights the specified account
        /// </summary>
        Task SetCurrentAccount(Account acc);

        Task SetCurrentAccount(string accId);

        /// <summary>
        /// Removes "currently logged in" border from all accounts
        /// </summary>
        Task UnCurrentAllAccounts();

        Task SetParametersAsync(ParameterView parameters);
    }

    public class AccountFuncs : ComponentBase, IAccountFuncs
    {
        [Inject] private ILang Lang { get; }
        [Inject] private IAppData AppData { get; }

        #region Selecting accounts
        /// <summary>
        /// Highlights the specified account
        /// </summary>
        public void SetSelectedAccount(Account acc)
        {
            AppData.CurrentStatus = Lang["Status_SelectedAccount", new { name = acc.DisplayName }];
            AppData.SelectedAccount = acc;
            UnselectAllAccounts();
            acc.IsChecked = true;
            AppData.NotifyDataChanged();
        }

        /// <summary>
        /// Removes highlight from all accounts
        /// </summary>
        public void UnselectAllAccounts()
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
        public async void AccountRightClick(MouseEventArgs e, Account acc)
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
        public async Task SetCurrentAccount(Account acc)
        {
            await UnCurrentAllAccounts();
            acc.IsCurrent = true;
            acc.TitleText = $"{Lang["Tooltip_CurrentAccount"]}";
            AppData.NotifyDataChanged();

            // getBestOffset
            await AppData.InvokeVoidAsync("setBestOffset", acc.AccountId);
            // then initTooltips
            await AppData.InvokeVoidAsync("initTooltips");
        }
        public async Task SetCurrentAccount(string accId) =>
            await SetCurrentAccount(AppData.CurrentSwitcher == "Steam" ? AppData.SteamAccounts.First(x => x.AccountId == accId) : AppData.BasicAccounts.First(x => x.AccountId == accId));

        /// <summary>
        /// Removes "currently logged in" border from all accounts
        /// </summary>
        public async Task UnCurrentAllAccounts()
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

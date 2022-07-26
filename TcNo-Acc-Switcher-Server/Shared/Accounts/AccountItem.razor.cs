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
using Microsoft.AspNetCore.Components.Web;
using Microsoft.JSInterop;
using TcNo_Acc_Switcher_Server.State;
using TcNo_Acc_Switcher_Server.State.Classes;
using TcNo_Acc_Switcher_Server.State.Interfaces;

namespace TcNo_Acc_Switcher_Server.Shared.Accounts
{
    public partial class AccountItem
    {
        [Inject] private IJSRuntime JsRuntime { get; set; }
        [Inject] private ILang Lang { get; set; }

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
    }
}

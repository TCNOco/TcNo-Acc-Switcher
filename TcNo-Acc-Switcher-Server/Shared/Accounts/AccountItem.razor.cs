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
    }
}

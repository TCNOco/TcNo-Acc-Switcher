using System.Linq;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Components;
using Microsoft.AspNetCore.Components.Web;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Data.Settings;
using TcNo_Acc_Switcher_Server.Pages.General;

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
            AppData.CurrentStatus = Lang.Instance["Status_SelectedAccount", new { name = acc.DisplayName }];
            AppData.SelectedAccount = acc.AccountId;
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
                foreach (var account in Steam.Accounts)
                {
                    account.IsChecked = false;
                }
            else
                foreach (var account in Basic.Accounts)
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
            acc.TitleText = $"{Lang.Instance["Tooltip_CurrentAccount"]}";
            AppData.Instance.NotifyDataChanged();

            // getBestOffset
            await AppData.InvokeVoidAsync("setBestOffset", acc.AccountId);
            // then initTooltips
            await AppData.InvokeVoidAsync("initTooltips");
        }
        public static async Task SetCurrentAccount(string accId) =>
            await SetCurrentAccount(AppData.CurrentSwitcher == "Steam" ? Steam.Accounts.First(x => x.AccountId == accId) : Basic.Accounts.First(x => x.AccountId == accId));

        /// <summary>
        /// Removes "currently logged in" border from all accounts
        /// </summary>
        public static async Task UnCurrentAllAccounts()
        {
            if (AppData.CurrentSwitcher == "Steam")
                foreach (var account in Steam.Accounts)
                {
                    account.IsCurrent = false;
                }
            else
                foreach (var account in Basic.Accounts)
                {
                    account.IsCurrent = false;
                }

            // Clear the hover text
            await AppData.InvokeVoidAsync("clearAccountTooltips");
        }
        #endregion
    }
}

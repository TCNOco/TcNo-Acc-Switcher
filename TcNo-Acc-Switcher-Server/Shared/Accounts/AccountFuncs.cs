using Microsoft.AspNetCore.Components.Web;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Data.Settings;
using TcNo_Acc_Switcher_Server.Pages.General;

namespace TcNo_Acc_Switcher_Server.Shared.Accounts
{
    public partial class AccountItem
    {
        public static void SetSelectedAccount(Account acc)
        {
            AppData.CurrentStatus = Lang.Instance["Status_SelectedAccount", new { name = acc.DisplayName }];
            AppData.SelectedAccount = acc.AccountId;
            UnselectAllAccounts();
            acc.IsChecked = true;
            AppData.Instance.NotifyDataChanged();
        }

        public static async void AccountRightClick(MouseEventArgs e, Account acc)
        {
            if (e.Button != 2) return;
            SetSelectedAccount(acc);
            await AppData.InvokeVoidAsync("positionAndShowMenu", e, "#AccOrPlatList");
        }

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
    }
}

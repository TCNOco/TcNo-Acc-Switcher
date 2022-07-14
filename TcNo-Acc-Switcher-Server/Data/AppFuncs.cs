using System;
using System.Threading.Tasks;
using TcNo_Acc_Switcher_Server.Data.Settings;
using TcNo_Acc_Switcher_Server.Pages.Basic;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Pages.Steam;

namespace TcNo_Acc_Switcher_Server.Data
{
    public class AppFuncs
    {
        /// <summary>
        /// Swap to the current AppData.SelectedAccount.
        /// </summary>
        /// <param name="state">Optional profile state for Steam accounts</param>
        public static async Task SwapToAccount(int state = -1)
        {
            if (!OperatingSystem.IsWindows()) return;

            if (AppData.CurrentSwitcher != "Steam") BasicSwitcherFuncs.SwapBasicAccounts(AppData.SelectedAccount);

            if (state == -1) state = Steam.OverrideState;
            await SteamSwitcherFuncs.SwapSteamAccounts(AppData.SelectedAccount, state);
        }

        /// <summary>
        /// Swaps to an empty account, allowing the user to sign in.
        /// </summary>
        /// <param name="state">Optional profile state for Steam accounts</param>
        public static async Task SwapToNewAccount(int state = -1)
        {
            if (!OperatingSystem.IsWindows()) return;

            if (AppData.CurrentSwitcher == "Steam") BasicSwitcherFuncs.SwapBasicAccounts("");

            if (state == -1) state = Steam.OverrideState;
            await SteamSwitcherFuncs.SwapSteamAccounts("", state);
        }

        public static async Task ShowModal(string modal)
        {
            await AppData.InvokeVoidAsync("showModal", modal);
        }

        public static async Task ForgetAccount()
        {
            var skipConfirm = AppData.CurrentSwitcher == "Steam" ? Steam.ForgetAccountEnabled : Basic.ForgetAccountEnabled;
            if (!skipConfirm)
                await ShowModal($"confirm:AcceptForget{AppData.CurrentSwitcher}Acc:{AppData.SelectedAccount}");
            else
            {
                if (AppData.CurrentSwitcher == "Steam")
                {
                    Steam.SetForgetAcc(true);
                    _ = SteamSwitcherFuncs.ForgetAccount(AppData.SelectedAccount);
                }
                else
                {
                    Basic.SetForgetAcc(true);
                    _ = GeneralFuncs.ForgetAccount_Generic(AppData.SelectedAccount, CurrentPlatform.SafeName, true);
                }

                AppData.Instance.NotifyDataChanged();

                await GeneralInvocableFuncs.ShowToast("success", Lang.Instance["Success"], renderTo: "toastarea");
            }
        }
    }
}

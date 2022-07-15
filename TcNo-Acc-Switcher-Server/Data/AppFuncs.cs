using System;
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using Newtonsoft.Json;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data.Settings;
using TcNo_Acc_Switcher_Server.Pages.Basic;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Pages.Steam;
using TcNo_Acc_Switcher_Server.Shared.Accounts;
using TextCopy;

namespace TcNo_Acc_Switcher_Server.Data
{
    public class AppFuncs
    {
        #region Account Management
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
                var trayAcc = AppData.SelectedAccount;
                if (AppData.CurrentSwitcher == "Steam")
                {
                    Steam.SetForgetAcc(true);

                    // Load and remove account that matches SteamID above.
                    var userAccounts = await SteamSwitcherFuncs.GetSteamUsers(Steam.LoginUsersVdf());
                    _ = userAccounts.RemoveAll(x => x.SteamId == AppData.SelectedAccount);

                    // Save updated loginusers.vdf file
                    await SteamSwitcherFuncs.SaveSteamUsersIntoVdf(userAccounts);
                    trayAcc = "+s:" + AppData.SelectedAccount;

                    // Remove from Steam accounts list
                    Steam.Accounts.Remove(Steam.Accounts.First(x => x.AccountId == AppData.SelectedAccount));
                }
                else
                {
                    Basic.SetForgetAcc(true);

                    // Remove ID from list of ids
                    var idsFile = $"LoginCache\\{AppData.CurrentSwitcher}\\ids.json";
                    if (File.Exists(idsFile))
                    {
                        var allIds = GeneralFuncs.ReadDict(idsFile).Remove(AppData.SelectedAccount);
                        await File.WriteAllTextAsync(idsFile, JsonConvert.SerializeObject(allIds));
                    }

                    // Remove cached files
                    Globals.RecursiveDelete($"LoginCache\\{AppData.CurrentSwitcher}\\{AppData.SelectedAccount}", false);

                    // Remove from Steam accounts list
                    Basic.Accounts.Remove(Basic.Accounts.First(x => x.AccountId == AppData.SelectedAccount));
                }

                // Remove from Tray
                Globals.RemoveTrayUserByArg(AppData.CurrentSwitcher, trayAcc);

                // Remove image
                Globals.DeleteFile(Path.Join(GeneralFuncs.WwwRoot(), $"\\img\\profiles\\{AppData.CurrentSwitcher}\\{Globals.GetCleanFilePath(AppData.SelectedAccount)}.jpg"));

                AppData.Instance.NotifyDataChanged();

                await GeneralInvocableFuncs.ShowToast("success", Lang.Instance["Success"], renderTo: "toastarea");
            }
        }

        public static Account GetCurrentAccount() =>
            AppData.CurrentSwitcher == "Steam"
                ? Steam.Accounts.First(x => x.AccountId == AppData.SelectedAccount)
                : Basic.Accounts.First(x => x.AccountId == AppData.SelectedAccount);
        #endregion

        #region Clipboard
        [JSInvokable]
        public static async Task CopyText(string text) => await ClipboardService.SetTextAsync(text);

        #endregion
    }
}

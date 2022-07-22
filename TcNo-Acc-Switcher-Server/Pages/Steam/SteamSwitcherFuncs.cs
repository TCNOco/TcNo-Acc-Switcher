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

using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Runtime.Versioning;
using System.Text;
using System.Threading.Tasks;
using Gameloop.Vdf.JsonConverter;
using Microsoft.Win32;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Converters;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Shared.Accounts;
using TcNo_Acc_Switcher_Server.State;


namespace TcNo_Acc_Switcher_Server.Pages.Steam
{
    public class SteamSwitcherFuncs
    {
        private static readonly Lang Lang = Lang.Instance;

        #region True Static
        public static bool BackupGameDataFolder(string folder)
        {
            var backupFolder = folder + "_switcher_backup";
            if (!Directory.Exists(folder) || Directory.Exists(backupFolder)) return false;
            Globals.CopyDirectory(folder, backupFolder, true);
            return true;
        }

        /// <summary>
        /// Verify whether input Steam64ID is valid or not
        /// </summary>
        public static bool VerifySteamId(string steamId)
        {
            Globals.DebugWriteLine($@"[Func:Steam\SteamSwitcherFuncs.VerifySteamId] Verifying SteamID: {steamId.Substring(steamId.Length - 4, 4)}");
            const long steamIdMin = 0x0110000100000001;
            const long steamIdMax = 0x01100001FFFFFFFF;
            if (!IsDigitsOnly(steamId) || steamId.Length != 17) return false;
            // Size check: https://stackoverflow.com/questions/33933705/steamid64-minimum-and-maximum-length#40810076
            var steamIdVal = double.Parse(steamId);
            return steamIdVal is > steamIdMin and < steamIdMax;
        }
        private static bool IsDigitsOnly(string str) => str.All(c => c is >= '0' and <= '9');

        /// <summary>
        /// Returns string representation of Steam ePersonaState int
        /// </summary>
        /// <param name="ePersonaState">integer state to return string for</param>
        public static string PersonaStateToString(int ePersonaState)
        {
            return ePersonaState switch
            {
                -1 => "",
                0 => Lang["Offline"],
                1 => Lang["Online"],
                2 => Lang["Busy"],
                3 => Lang["Away"],
                4 => Lang["Snooze"],
                5 => Lang["LookingToTrade"],
                6 => Lang["LookingToPlay"],
                7 => Lang["Invisible"],
                _ => Lang["Unrecognized_EPersonaState"]
            };
        }
        #endregion



        #region STEAM_SWITCHER_MAIN


        /// <summary>
        /// This relies on Steam updating loginusers.vdf. It could go out of sync assuming it's not updated reliably. There is likely a better way to do this.
        /// I am avoiding using the Steam API because it's another DLL to include, but is the next best thing - I assume.
        /// </summary>
        public static string GetCurrentAccountId(bool getNumericId = false)
        {
            Globals.DebugWriteLine($@"[Func:Steam\SteamSwitcherFuncs.GetCurrentAccountId]");
            try
            {
                // Refreshing the list of SteamUsers doesn't help here when switching, as the account list is not updated by Steam just yet.
                Index.Steamuser mostRecent = null;
                foreach (var su in AppData.SteamUsers)
                {
                    int.TryParse(su.LastLogin, out var last);

                    int.TryParse(mostRecent?.LastLogin, out var recent);

                    if (mostRecent == null || last > recent)
                        mostRecent = su;
                }

                int.TryParse(mostRecent?.LastLogin, out var mrTimestamp);

                if (SteamSettings.LastAccTimestamp > mrTimestamp)
                {
                    return SteamSettings.LastAccSteamId;
                }

                if (getNumericId) return mostRecent?.SteamId ?? "";
                return mostRecent?.AccName ?? "";
            }
            catch (Exception)
            {
                //
            }

            return "";
        }


        /// <summary>
        /// Swap to the current AppData.SelectedAccountId.
        /// </summary>
        /// <param name="state">Optional profile state for Steam accounts</param>
        public async Task SwapToAccount(int state = -1)
        {
            if (state == -1) state = SteamSettings.OverrideState;
            await SwapSteamAccounts(AppState.Switcher.SelectedAccountId, state);
        }

        /// <summary>
        /// Swaps to an empty account, allowing the user to sign in.
        /// </summary>
        /// <param name="state">Optional profile state for Steam accounts</param>
        public async Task SwapToNewAccount(int state = -1)
        {
            if (state == -1) state = SteamSettings.OverrideState;
            await SwapSteamAccounts("", state);
        }

        public async Task ForgetAccount()
        {
            if (!SteamSettings.ForgetAccountEnabled)
                Modals.ShowModal("confirm", ExtraArg.ForgetAccount);
            else
            {
                var trayAcc = AppState.Switcher.SelectedAccountId;
                SetForgetAcc(true);

                // Load and remove account that matches SteamID above.
                var userAccounts = GetSteamUsers(SteamSettings.LoginUsersVdf);
                _ = userAccounts.RemoveAll(x => x.SteamId == AppState.Switcher.SelectedAccountId);

                // Save updated loginusers.vdf file
                await SaveSteamUsersIntoVdf(userAccounts);
                trayAcc = "+s:" + AppState.Switcher.SelectedAccountId;

                // Remove from Steam accounts list
                AppState.Switcher.SteamAccounts.Remove(AppState.Switcher.SteamAccounts.First(x => x.AccountId == AppState.Switcher.SelectedAccountId));

                // Remove from Tray
                Globals.RemoveTrayUserByArg(AppState.Switcher.CurrentSwitcher, trayAcc);

                // Remove image
                Globals.DeleteFile(Path.Join(GeneralFuncs.WwwRoot(), $"\\img\\profiles\\{AppState.Switcher.CurrentSwitcher}\\{Globals.GetCleanFilePath(AppState.Switcher.SelectedAccountId)}.jpg"));

                Toasts.ShowToastLang(ToastType.Success, "Success");
            }
        }

        /// <summary>
        /// Restart Steam with a new account selected. Leave args empty to log into a new account.
        /// </summary>
        /// <param name="steamId">(Optional) User's SteamID</param>
        /// <param name="ePersonaState">(Optional) Persona state for user [0: Offline, 1: Online...]</param>
        /// <param name="args">Starting arguments</param>
        public static async Task SwapSteamAccounts(string steamId = "", int ePersonaState = -1, string args = "")
        {
            Globals.DebugWriteLine($@"[Func:Steam\SteamSwitcherFuncs.SwapSteamAccounts] Swapping to: hidden. ePersonaState={ePersonaState}");
            if (steamId != "" && !VerifySteamId(steamId))
            {
                return;
            }

            await AppData.InvokeVoidAsync("updateStatus", Lang["Status_ClosingPlatform", new { platform  = "Steam" }]);
            if (!await GeneralFuncs.CloseProcesses(SteamSettings.Processes, SteamSettings.ClosingMethod))
            {
                if (Globals.IsAdministrator)
                    await AppData.InvokeVoidAsync("updateStatus", Lang["Status_ClosingPlatformFailed", new { platform = "Steam" }]);
                else
                {
                    await GeneralInvocableFuncs.ShowToast("error", Lang["Toast_RestartAsAdmin"], Lang["Failed"], "toastarea");
                    ModalData.ShowModal("confirm", ModalData.ExtraArg.RestartAsAdmin);
                }
                return;
            }

            if (OperatingSystem.IsWindows()) await UpdateLoginUsers(steamId, ePersonaState);

            await AppData.InvokeVoidAsync("updateStatus", Lang["Status_StartingPlatform", new { platform = "Steam" }]);
            if (SteamSettings.AutoStart)
            {
                if (SteamSettings.StartSilent) args += " -silent";

                if (Globals.StartProgram(SteamSettings.Exe(), SteamSettings.Admin, args, SteamSettings.StartingMethod))
                    await GeneralInvocableFuncs.ShowToast("info",
                        Lang["Status_StartingPlatform", new {platform = "Steam"}], renderTo: "toastarea");
                else
                    await GeneralInvocableFuncs.ShowToast("error",
                        Lang["Toast_StartingPlatformFailed", new {platform = "Steam"}], renderTo: "toastarea");
            }

            if (SteamSettings.AutoStart && AppSettings.MinimizeOnSwitch) await AppData.InvokeVoidAsync("hideWindow");

            NativeFuncs.RefreshTrayArea();
            await AppData.InvokeVoidAsync("updateStatus", Lang["Done"]);
            AppStats.IncrementSwitches("Steam");

            try
            {
                SteamSettings.LastAccSteamId = AppData.SteamUsers.Where(x => x.SteamId == steamId).ToList()[0].SteamId;
                SteamSettings.LastAccTimestamp = Globals.GetUnixTimeInt();
                if (SteamSettings.LastAccSteamId != "")
                    await AccountFuncs.SetCurrentAccount(SteamSettings.LastAccSteamId);
            }
            catch (Exception)
            {
                //
            }
        }
        #endregion

        #region STEAM_MANAGEMENT
        /// <summary>
        /// Updates loginusers and registry to select an account as "most recent"
        /// </summary>
        /// <param name="selectedSteamId">Steam ID64 to switch to</param>
        /// <param name="pS">[PersonaState]0-7 custom persona state [0: Offline, 1: Online...]</param>
        [SupportedOSPlatform("windows")]
        public static async Task UpdateLoginUsers(string selectedSteamId, int pS)
        {
            Globals.DebugWriteLine($@"[Func:Steam\SteamSwitcherFuncs.UpdateLoginUsers] Updating loginusers: selectedSteamId={(selectedSteamId.Length > 0 ? selectedSteamId.Substring(selectedSteamId.Length - 4, 4) : "")}, pS={pS}");
            var userAccounts = await GetSteamUsers(SteamSettings.LoginUsersVdf());
            // -----------------------------------
            // ----- Manage "loginusers.vdf" -----
            // -----------------------------------
            await AppData.InvokeVoidAsync("updateStatus", Lang["Status_UpdatingFile", new { file = "loginusers.vdf" }]);
            var tempFile = SteamSettings.LoginUsersVdf() + "_temp";
            Globals.DeleteFile(tempFile);

            // MostRec is "00" by default, just update the one that matches SteamID.
            try
            {
                userAccounts.Where(x => x.SteamId == selectedSteamId).ToList().ForEach(u =>
                {
                    u.MostRec = "1";
                    u.RememberPass = "1";
                    u.OfflineMode = pS == -1 ? u.OfflineMode : pS > 1 ? "0" : pS == 1 ? "0" : "1";
                    // u.OfflineMode: Set ONLY if defined above
                    // If defined & > 1, it's custom, therefor: Online
                    // Otherwise, invert [0 == Offline => Online, 1 == Online => Offline]
                });
            }
            catch (InvalidOperationException)
            {
                await GeneralInvocableFuncs.ShowToast("error", Lang["Toast_MissingUserId"]);
            }
            //userAccounts.Single(x => x.SteamId == selectedSteamId).MostRec = "1";

            // Save updated loginusers.vdf
            await SaveSteamUsersIntoVdf(userAccounts);

            // -----------------------------------
            // - Update localconfig.vdf for user -
            // -----------------------------------
            if (pS != -1) SetPersonaState(selectedSteamId, pS); // Update persona state, if defined above.

            Index.Steamuser user = new() { AccName = "" };
            try
            {
                if (selectedSteamId != "")
                    user = userAccounts.Single(x => x.SteamId == selectedSteamId);
            }
            catch (InvalidOperationException)
            {
                await GeneralInvocableFuncs.ShowToast("error", Lang["Toast_MissingUserId"]);
            }
            // -----------------------------------
            // --------- Manage registry ---------
            // -----------------------------------
            /*
            ------------ Structure ------------
            HKEY_CURRENT_USER\Software\Valve\Steam\
                --> AutoLoginUser = username
                --> RememberPassword = 1
            */
            await AppData.InvokeVoidAsync("updateStatus", Lang["Status_UpdatingRegistry"]);
            using var key = Registry.CurrentUser.CreateSubKey(@"Software\Valve\Steam");
            key?.SetValue("AutoLoginUser", user.AccName); // Account name is not set when changing user accounts from launch arguments (part of the viewmodel). -- Can be "" if no account
            key?.SetValue("RememberPassword", 1);

            // -----------------------------------
            // ------Update Tray users list ------
            // -----------------------------------
            if (selectedSteamId != "")
                Globals.AddTrayUser("Steam", "+s:" + user.SteamId, SteamSettings.TrayAccName ? user.AccName : GetName(user), SteamSettings.TrayAccNumber);
        }

        /// <summary>
        /// Save updated list of Steamuser into loginusers.vdf, in vdf format.
        /// </summary>
        /// <param name="userAccounts">List of Steamuser to save into loginusers.vdf</param>
        public static async Task SaveSteamUsersIntoVdf(List<Index.Steamuser> userAccounts)
        {
            Globals.DebugWriteLine($@"[Func:Steam\SteamSwitcherFuncs.SaveSteamUsersIntoVdf] Saving updated loginusers.vdf. Count: {userAccounts.Count}");
            // Convert list to JObject list, ready to save into vdf.
            var outJObject = new JObject();
            foreach (var ua in userAccounts)
            {
                outJObject[ua.SteamId] = (JObject)JToken.FromObject(ua);
            }

            // Write changes to files.
            var tempFile = SteamSettings.LoginUsersVdf() + "_temp";
            await File.WriteAllTextAsync(tempFile, @"""users""" + Environment.NewLine + outJObject.ToVdf());
            if (!File.Exists(tempFile))
            {
                File.Replace(tempFile, SteamSettings.LoginUsersVdf(), SteamSettings.LoginUsersVdf() + "_last");
                return;
            }
            try
            {
                // Let's try break this down, as some users are having issues with the above function.
                // Step 1: Backup
                if (File.Exists(SteamSettings.LoginUsersVdf()))
                {
                    File.Copy(SteamSettings.LoginUsersVdf(), SteamSettings.LoginUsersVdf() + "_last", true);
                }

                // Step 2: Write new info
                await File.WriteAllTextAsync(SteamSettings.LoginUsersVdf(), @"""users""" + Environment.NewLine + outJObject.ToVdf());
            }
            catch (Exception ex)
            {
                Globals.WriteToLog("Failed to swap Steam users! Could not create temp loginusers.vdf file, and replace original using workaround! Contact TechNobo.", ex);
                await GeneralInvocableFuncs.ShowToast("error", Lang["CouldNotFindX", new { x = tempFile }]);
            }
        }

        /// <summary>
        /// Sets whether the user is invisible or not
        /// </summary>
        /// <param name="steamId">SteamID of user to update</param>
        /// <param name="ePersonaState">Persona state enum for user (0-7)</param>
        public static void SetPersonaState(string steamId, int ePersonaState)
        {
            Globals.DebugWriteLine($@"[Func:Steam\SteamSwitcherFuncs.SetPersonaState] Setting persona state for: {steamId.Substring(steamId.Length - 4, 4)}, To: {ePersonaState}");
            // Values:
            // 0: Offline, 1: Online, 2: Busy, 3: Away, 4: Snooze, 5: Looking to Trade, 6: Looking to Play, 7: Invisible
            var id32 = new SteamIdConvert(steamId).Id32; // Get SteamID
            var localConfigFilePath = Path.Join(SteamSettings.FolderPath, "userdata", id32, "config", "localconfig.vdf");
            if (!File.Exists(localConfigFilePath)) return;
            var localConfigText = Globals.ReadAllText(localConfigFilePath); // Read relevant localconfig.vdf

            // Find index of range needing to be changed.
            var positionOfVar = localConfigText.IndexOf("ePersonaState", StringComparison.Ordinal); // Find where the variable is being set
            if (positionOfVar == -1) return;
            var indexOfBefore = localConfigText.IndexOf(":", positionOfVar, StringComparison.Ordinal) + 1; // Find where the start of the variable's value is
            var indexOfAfter = localConfigText.IndexOf(",", positionOfVar, StringComparison.Ordinal); // Find where the end of the variable's value is

            // The variable is now in-between the above numbers. Remove it and insert something different here.
            var sb = new StringBuilder(localConfigText);
            _ = sb.Remove(indexOfBefore, indexOfAfter - indexOfBefore);
            _ = sb.Insert(indexOfBefore, ePersonaState);
            localConfigText = sb.ToString();

            // Output
            File.WriteAllText(localConfigFilePath, localConfigText);
        }
        #endregion

        #region STEAM_GAME_MANAGEMENT
        /// <summary>
        /// Copy settings from currently logged in account to selected game and account
        /// </summary>
        public static async Task CopySettingsFrom(string gameId)
        {
            var destSteamId = GetCurrentAccountId(true);
            if (!VerifySteamId(AppData.SelectedAccountId) || !VerifySteamId(destSteamId))
            {
                await GeneralInvocableFuncs.ShowToast("error", Lang["Toast_NoValidSteamId"], Lang["Failed"],
                    "toastarea");
                return;
            }
            if (destSteamId == AppData.SelectedAccountId)
            {
                await GeneralInvocableFuncs.ShowToast("info", Lang["Toast_SameAccount"], Lang["Failed"],
                    "toastarea");
                return;
            }

            var sourceSteamId32 = new SteamIdConvert(AppData.SelectedAccountId).Id32;
            var destSteamId32 = new SteamIdConvert(destSteamId).Id32;
            var sourceFolder = Path.Join(SteamSettings.FolderPath, $"userdata\\{sourceSteamId32}\\{gameId}");
            var destFolder = Path.Join(SteamSettings.FolderPath, $"userdata\\{destSteamId32}\\{gameId}");
            if (!Directory.Exists(sourceFolder))
            {
                await GeneralInvocableFuncs.ShowToast("error", Lang["Toast_NoFindSteamUserdata"], Lang["Failed"],
                    "toastarea");
                return;
            }

            if (Directory.Exists(destFolder))
            {
                // Backup the account's data you're copying to
                var toAccountBackup = Path.Join(Globals.UserDataFolder, $"Backups\\Steam\\{destSteamId32}\\{gameId}");
                if (Directory.Exists(toAccountBackup)) Directory.Delete(toAccountBackup, true);
                Globals.CopyDirectory(destFolder, toAccountBackup, true);
                Directory.Delete(destFolder, true);
            }
            Globals.CopyDirectory(sourceFolder, destFolder, true);
            await GeneralInvocableFuncs.ShowToast("success", Lang["Toast_SettingsCopied"], Lang["Success"], "toastarea");
        }


        public static async Task RestoreSettingsTo(string gameId)
        {
            if (!VerifySteamId(AppData.SelectedAccountId)) return;
            var steamId32 = new SteamIdConvert(AppData.SelectedAccountId).Id32;
            var backupFolder = Path.Join(Globals.UserDataFolder, $"Backups\\Steam\\{steamId32}\\{gameId}");

            var folder = Path.Join(SteamSettings.FolderPath, $"userdata\\{steamId32}\\{gameId}");
            if (!Directory.Exists(backupFolder))
            {
                await GeneralInvocableFuncs.ShowToast("error", Lang["Toast_NoFindGameBackup"], Lang["Failed"],
                    "toastarea");
                return;
            }
            if (Directory.Exists(folder)) Directory.Delete(folder, true);
            Globals.CopyDirectory(backupFolder, folder, true);
            await GeneralInvocableFuncs.ShowToast("success", Lang["Toast_GameDataRestored"], Lang["Success"], "toastarea");
        }

        public static async Task BackupGameData(string gameId)
        {
            var steamId32 = new SteamIdConvert(AppData.SelectedAccountId).Id32;
            var sourceFolder = Path.Join(SteamSettings.FolderPath, $"userdata\\{steamId32}\\{gameId}");
            if (!VerifySteamId(AppData.SelectedAccountId) || !Directory.Exists(sourceFolder))
            {
                await GeneralInvocableFuncs.ShowToast("error", Lang["Toast_NoFindGameData"], Lang["Failed"],
                    "toastarea");
                return;
            }

            var destFolder = Path.Join(Globals.UserDataFolder, $"Backups\\Steam\\{steamId32}\\{gameId}");
            if (Directory.Exists(destFolder)) Directory.Delete(destFolder, true);

            Globals.CopyDirectory(sourceFolder, destFolder, true);
            await GeneralInvocableFuncs.ShowToast("success", Lang["Toast_GameBackupDone", new { folderLocation = destFolder }], Lang["Success"], "toastarea");
        }
        #endregion
    }
}

using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text.RegularExpressions;
using System.Threading.Tasks;
using HtmlAgilityPack;
using Microsoft.JSInterop;
using Newtonsoft.Json;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General;

namespace TcNo_Acc_Switcher_Server.Pages.Ubisoft
{
    public class UbisoftSwitcherFuncs
    {
        private static readonly Lang Lang = Lang.Instance;

        private static readonly Data.Settings.Ubisoft Ubisoft = Data.Settings.Ubisoft.Instance;
        private static string _ubisoftAppData = Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "Ubisoft Game Launcher");
        private static readonly string UbisoftAvatarFolder = Path.Join(Ubisoft.FolderPath, "cache", "avatars");
        /// <summary>
        /// Main function for Ubisoft Account Switcher. Run on load.
        /// Collects accounts from Ubisoft Connect's files
        /// Prepares HTML Elements string for insertion into the account switcher GUI.
        /// </summary>
        /// <returns>Whether account loading is successful, or a path reset is needed (invalid dir saved)</returns>
        public static async Task LoadProfiles()
        {
            // Normal:
            Globals.DebugWriteLine(@"[Func:Ubisoft\UbisoftSwitcherFuncs.LoadProfiles] Loading Steam profiles");

            const string localCachePath = "LoginCache\\Ubisoft\\";
            if (!Directory.Exists(localCachePath) || !File.Exists(Path.Join(localCachePath, "ids.json"))) return;
            var allIds = ReadAllIds();

            // Order
            if (File.Exists("LoginCache\\Ubisoft\\order.json"))
            {
                Dictionary<string, string> newIds = new();
                var savedOrder = JsonConvert.DeserializeObject<List<string>>(await File.ReadAllTextAsync("LoginCache\\Ubisoft\\order.json").ConfigureAwait(false));
                if (savedOrder != null)
                {
                    if (savedOrder is { Count: > 0 })
                    {
                        var ids = allIds;
                        foreach (var (key, value) in from i in savedOrder where ids.Any(x => x.Key == i) select ids.Single(x => x.Key == i))
                            newIds.Add(key, value);
                    }

                    allIds = newIds;
                }
            }

            GenericFunctions.InsertAccounts(allIds, "ubisoft");
        }

        public static void UbisoftAddCurrent(string accName = "", bool saveOnlyIfExists = false)
        {
            Globals.DebugWriteLine(@"[Func:Ubisoft\UbisoftSwitcherFuncs.UbisoftAddCurrent]");
            // 1. Get account userID from log file (May also change soon, because the user.dat file changed)?
            var userId = GetLastLoginUserId();
            if (userId == "NOTFOUND")
            {
                _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_Ubisoft_NoUsername"], Lang["Error"], "toastarea", 10000);
                return;
            }

            // For adding new accounts: Skip saving, to avoid creating blank username accounts
            var savedUsername = HasUserSaved();
            if (saveOnlyIfExists && savedUsername == "") return;

            // 2. Copy the profile picture
            ImportAvatar(userId);

            var localCachePath = $"LoginCache\\Ubisoft\\{userId}\\";
            _ = Directory.CreateDirectory(localCachePath);

            if (!File.Exists(Path.Join(_ubisoftAppData, "settings.yaml")) || !File.Exists(Path.Join(_ubisoftAppData, "user.dat")))
            {
                _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_CouldNotLocate"], Lang["Failed"], "toastarea");
                return;
            }
            // Save files
            File.Copy(Path.Join(_ubisoftAppData, "settings.yaml"), Path.Join(localCachePath, "settings.yaml"), true);
            File.Copy(Path.Join(_ubisoftAppData, "user.dat"), Path.Join(localCachePath, "user.dat"), true);

            // Add username
            if (accName != "") SetUsername(userId, accName);
            else accName = savedUsername;

            AppData.ActiveNavMan?.NavigateTo("/Ubisoft/?cacheReload&toast_type=success&toast_title=Success&toast_message=" + Uri.EscapeDataString("Saved: " + accName), true);
        }

        public static string HasUserSaved()
        {
            // 1. Get account userID from log file (May also change soon, because the user.dat file changed)?
            var userId = GetLastLoginUserId();
            if (userId == "NOTFOUND")
            {
                _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_Ubisoft_NoUsername"], Lang["Error"], "toastarea", 10000);
                return "";
            }

            var allIds = ReadAllIds();
            return (allIds.ContainsKey(userId) ? allIds[userId] : "");
        }

        private static string GetLastLoginUserId()
        {
            Globals.DebugWriteLine(@"[Func:Ubisoft\UbisoftSwitcherFuncs.GetLastLoginUserId]");
            File.Copy(Path.Join(Ubisoft.FolderPath, "logs", "launcher_log.txt"), "templog");
            var lastUser = "";
            using (var reader = new StreamReader("templog"))
            {
                string line;

                while ((line = reader.ReadLine()) != null)
                {
                    if (!line.Contains("User: ")) continue;
                    line = line.Split("User: ")[1];
                    while (line.EndsWith(" ")) line = line[..^1];
                    lastUser = line;
                }
            }
            File.Delete("templog");

            return lastUser != "" ? lastUser : "NOTFOUND";
        }

        // Overload for below
        public static void SetUsername(string id, string username) => SetUsername(id, username, false);

        public static void SetUsername(string id, string username, bool reload)
        {
            var allIds = ReadAllIds();
            allIds[id] = username;
            File.WriteAllText("LoginCache\\Ubisoft\\ids.json", JsonConvert.SerializeObject(allIds));
            if (reload) AppData.ActiveNavMan?.NavigateTo("/Ubisoft/?cacheReload&toast_type=success&toast_title=Success&toast_message=" + Uri.EscapeDataString("Set username"), true);
        }


        public static Dictionary<string, string> ReadAllIds()
        {
            Globals.DebugWriteLine(@"[Func:Ubisoft\UbisoftSwitcherFuncs.ReadAllIds]");
            const string localAllIds = "LoginCache\\Ubisoft\\ids.json";
            var s = JsonConvert.SerializeObject(new Dictionary<string, string>());
            if (!File.Exists(localAllIds)) return JsonConvert.DeserializeObject<Dictionary<string, string>>(s);
            try
            {
                s = Globals.ReadAllText(localAllIds);
            }
            catch (Exception)
            {
                //
            }

            return JsonConvert.DeserializeObject<Dictionary<string, string>>(s);
        }

        public static void ImportAvatar(string userId)
        {
            Globals.DebugWriteLine(@"[Func:Ubisoft\UbisoftSwitcherFuncs.ImportAvatar] userId:hidden");
            string i64 = Path.Join(UbisoftAvatarFolder, userId + "_64.png"),
                i128 = Path.Join(UbisoftAvatarFolder, userId + "_128.png"),
                i256 = Path.Join(UbisoftAvatarFolder, userId + "_256.png");

            _ = Directory.CreateDirectory("wwwroot\\img\\profiles\\ubisoft\\");
            var outPath = Path.Join(GeneralFuncs.WwwRoot(), $"\\img\\profiles\\ubisoft\\{userId}.png");
            if (File.Exists(outPath))
            {
                _ = GeneralFuncs.DeletedOutdatedFile(outPath, Ubisoft.ImageExpiryTime);
                _ = GeneralFuncs.DeletedInvalidImage(outPath);
            }

            if (File.Exists(outPath)) return;
            if (File.Exists(i256)) File.Copy(i256, outPath, true);
            else if (File.Exists(i128)) File.Copy(i128, outPath, true);
            else if (File.Exists(i64)) File.Copy(i64, outPath, true);
            else File.Copy(Path.Join(GeneralFuncs.WwwRoot(), "img\\QuestionMark.jpg"), outPath, true);
        }

        //// This seemed very possible: But... As far as I know the settings.yaml file needs to be adjusted, as well as the user.dat...
        //// Might change the way that this works later.
        //private static void FindAllUsers()
        //{
        //    Directory.CreateDirectory("LoginCache\\Ubisoft\\temp\\");
        //    var tempUsersDat = $"LoginCache\\Ubisoft\\temp\\user.dat";
        //    File.Copy(Path.Join(UbisoftAppData, "user.dat"), $"LoginCache\\Ubisoft\\temp\\user.dat", true);


        //    var username = "";
        //    var userId = "";
        //    using (var reader = new StreamReader(File.Open(tempUsersDat, FileMode.Open)))
        //    {
        //        var line = "";
        //        while ((line = reader.ReadLine()) != null)
        //        {
        //            if (!line.Contains("\u001a")) continue;
        //            var temp = line.Split("\u001a$")[1];

        //            userId = temp.Substring(0, 37);
        //            username = temp.Substring(38, temp.IndexOf(":", StringComparison.Ordinal) - 38);


        //            GeneralFuncs.RecursiveDelete(new DirectoryInfo("LoginCache\\Ubisoft\\temp\\"), false);
        //            Directory.CreateDirectory($"LoginCache\\Ubisoft\\{username}\\");
        //            File.Copy(Path.Join(UbisoftAppData, "settings.yaml"), $"LoginCache\\Ubisoft\\{username}\\settings.yaml", true);
        //            File.Copy(Path.Join(UbisoftAppData, "user.dat"), $"LoginCache\\Ubisoft\\{username}\\user.dat", true);

        //            break;
        //        }
        //    }

        //}


        /// <summary>
        /// Used in JS. Gets whether forget account is enabled (Whether to NOT show prompt, or show it).
        /// </summary>
        /// <returns></returns>
        [JSInvokable]
        public static Task<bool> GetUbisoftForgetAcc() => Task.FromResult(Ubisoft.ForgetAccountEnabled);

        /// <summary>
        /// Remove requested account
        /// </summary>
        /// <param name="userId">ID of the user account to remove</param>
        public static bool ForgetAccount(string userId)
        {
            Globals.DebugWriteLine(@"[Func:Ubisoft\UbisoftSwitcherFuncs.ForgetAccount] Forgetting account:hidden");
            GeneralFuncs.RecursiveDelete(new DirectoryInfo($"LoginCache\\Ubisoft\\{userId}"), false);

            var allIds = ReadAllIds();
            _ = allIds.Remove(userId);
            File.WriteAllText("LoginCache\\Ubisoft\\ids.json", JsonConvert.SerializeObject(allIds));


            var img = Path.Join(GeneralFuncs.WwwRoot(), $"\\img\\profiles\\ubisoft\\{userId}.png");
            if (File.Exists(img)) File.Delete(img);
            return true;
        }

        // Overload for below
        public static void SwapUbisoftAccounts() => SwapUbisoftAccounts("", 0);

        /// <summary>
        /// Restart Ubisoft with a new account selected. Leave args empty to log into a new account.
        /// </summary>
        /// <param name="userId">User's UserId</param>
        /// <param name="state">(Optional) State of user. 0 is online, anything else if Offline</param>
        /// <param name="args">Starting arguments</param>
        public static void SwapUbisoftAccounts(string userId, int state, string args = "")
        {
            Globals.DebugWriteLine(@"[Func:Ubisoft\UbisoftSwitcherFuncs.SwapUbisoftAccounts] Swapping to:hidden.");
            _ = AppData.InvokeVoidAsync("updateStatus", "Closing Ubisoft");
            if (!GeneralFuncs.CloseProcesses(Data.Settings.Ubisoft.Processes)) return;
            UbisoftAddCurrent(saveOnlyIfExists: true);

            if (userId != "")
            {
                if (!UbisoftCopyInAccount(userId, state)) return;
                Globals.AddTrayUser("Ubisoft", "+u:" + userId, ReadAllIds()[userId], Ubisoft.TrayAccNumber); // Add to Tray list
            }
            else
                ClearCurrentUser();

            _ = AppData.InvokeVoidAsync("updateStatus", "Starting Ubisoft");

            GeneralFuncs.StartProgram(Ubisoft.Exe(), Ubisoft.Admin, args);

            Globals.RefreshTrayArea();
        }

        private static void ClearCurrentUser()
        {
            Globals.DebugWriteLine(@"[Func:Ubisoft\UbisoftSwitcherFuncs.ClearCurrentUser]");
            File.Delete(Path.Join(_ubisoftAppData, "settings.yml"));
            File.Delete(Path.Join(_ubisoftAppData, "user.dat"));
        }

        /// <summary>
        /// Copies user's account files from LoginCache\\Ubisoft\\{userId} to
        /// </summary>
        /// <param name="userId"></param>
        /// <param name="state"></param>
        private static bool UbisoftCopyInAccount(string userId, int state = 0)
        {
            Globals.DebugWriteLine(@"[Func:Ubisoft\UbisoftSwitcherFuncs.UbisoftCopyInAccount]");
            var localCachePath = $"LoginCache\\Ubisoft\\{userId}\\";
            if (!Directory.Exists(localCachePath))
            {
                _ = GeneralInvocableFuncs.ShowToast("error", Lang["CouldNotFindX", new { x = localCachePath }], Lang["DirectoryNotFound"], "toastarea");
                return false;
            }

            if (state == -1)
            {
                if (File.Exists($"{localCachePath}users.dat")) File.Move($"{localCachePath}users.dat", $"{localCachePath}user.dat"); // 2021-07-24 - Ubisoft file name change.
                File.Copy($"{localCachePath}settings.yaml", Path.Join(_ubisoftAppData, "settings.yaml"), true);
                File.Copy($"{localCachePath}user.dat", Path.Join(_ubisoftAppData, "user.dat"), true);
                return true;
            }
            using var fs = new StreamWriter(Path.Join(_ubisoftAppData, "settings.yaml"));
            foreach (var l in Globals.ReadAllLines($"{localCachePath}settings.yaml"))
            {
                if (l.Contains("forceoffline")) fs.WriteLine("  forceoffline: " + (state != 0 ? "true" : "false"));
                else fs.WriteLine(l);
            }
            fs.Close();

            File.Copy($"{localCachePath}user.dat", Path.Join(_ubisoftAppData, "user.dat"), true);
            File.Copy(Path.Join(_ubisoftAppData, "settings.yaml"), $"{localCachePath}settings.yaml", true);
            return true;
        }
    }
}

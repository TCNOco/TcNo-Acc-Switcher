using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Runtime.CompilerServices;
using System.Threading;
using System.Threading.Tasks;
using System.Xml;
using Microsoft.JSInterop;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General;

namespace TcNo_Acc_Switcher_Server.Pages.Ubisoft
{
    public class UbisoftSwitcherFuncs
    {
        private static readonly Data.Settings.Ubisoft Ubisoft = Data.Settings.Ubisoft.Instance;
        private static string UbisoftAppData;
        private static string UbisoftLogFile;
        private static string UbisoftAvatarFolder;
        /// <summary>
        /// Main function for Ubisoft Account Switcher. Run on load.
        /// Collects accounts from Ubisoft Connect's files
        /// Prepares HTML Elements string for insertion into the account switcher GUI.
        /// </summary>
        /// <returns>Whether account loading is successful, or a path reset is needed (invalid dir saved)</returns>
        public static async void LoadProfiles()
        {
            // Normal:
            Globals.DebugWriteLine($@"[Func:Steam\SteamSwitcherFuncs.LoadProfiles] Loading Steam profiles");
            UbisoftAppData = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "Ubisoft Game Launcher");
            UbisoftLogFile = Path.Combine(Ubisoft.FolderPath, "logs", "launcher_log.txt");
            UbisoftAvatarFolder = Path.Combine(Ubisoft.FolderPath, "cache", "avatars");
            
            var localCachePath = $"LoginCache\\Ubisoft\\";
            if (!Directory.Exists(localCachePath) || !File.Exists(Path.Join(localCachePath, "ids.json"))) return;
            var allIds = ReadAllIds();
            foreach (var (userId, username) in allIds)
            {
                var element =
                    $"<input type=\"radio\" id=\"{userId}\" Username=\"{username}\" class=\"acc\" name=\"accounts\" onchange=\"SelectedItemChanged()\" />\r\n" +
                    $"<label for=\"{userId}\" class=\"acc\">\r\n" +
                    $"<img src=\"" + $"\\img\\profiles\\Ubisoft\\{userId}.png" + "\" draggable=\"false\" />\r\n" +
                    $"<h6>{username}</h6>\r\n";
                //$"<p>{UnixTimeStampToDateTime(ua.LastLogin)}</p>\r\n</label>";  TODO: Add some sort of "Last logged in" json file
                await AppData.ActiveIJsRuntime.InvokeVoidAsync("jQueryAppend", new object[] { "#acc_list", element });
            }
            await AppData.ActiveIJsRuntime.InvokeVoidAsync("initContextMenu");
        }

        public static void UbisoftAddCurrent()
        {

            // To add account section:
            var userId = GetLastLoginUserId();
            if (userId == "NOTFOUND")
            {
                GeneralInvocableFuncs.ShowToast("error", "Could not find last logged in user. Try logging into Ubisoft again first.", "Error", "toastarea");
                return;
            }

            // Find username from users.dat file
            var username = FindUsername(userId);

            // Import profile picture to $"LoginCache\\Ubisoft\\{userId}\\pfp.png"
            ImportAvatar(userId);

            // Notification
            AppData.ActiveNavMan?.NavigateTo("/Ubisoft/?cacheReload&toast_type=success&toast_title=Success&toast_message=" + Uri.EscapeUriString("Saved: " + username), true);
        }

        private static string GetLastLoginUserId()
        {
            File.Copy(UbisoftLogFile, "templog");
            var lastUser = "";
            using (var reader = new StreamReader("templog"))
            {
                var line = "";

                while ((line = reader.ReadLine()) != null)
                {
                    if (!line.Contains("User: ")) continue;
                    line = line.Split("User: ")[1];
                    while (line.EndsWith(" ")) line = line.Substring(0, line.Length - 1);
                    lastUser = line;
                }
            }
            File.Delete("templog");

            return (lastUser != "" ? lastUser : "NOTFOUND");
        }

        private static string FindUsername(string userId)
        {
            Directory.CreateDirectory("LoginCache\\Ubisoft\\temp\\");
            var tempUsersDat = $"LoginCache\\Ubisoft\\temp\\users.dat";
            File.Copy(Path.Join(UbisoftAppData, "users.dat"), $"LoginCache\\Ubisoft\\temp\\users.dat", true);

            var username = "";
            using (var reader = new StreamReader(File.Open(tempUsersDat, FileMode.Open)))
            {
                var line = "";
                while ((line = reader.ReadLine()) != null)
                {
                    if (!line.Contains(userId)) continue;
                    username = line.Split(userId)[1].Split(":")[0];
                    username = username.Substring(2, username.Length - 2);
                    break;
                }
            }

            var allIds = ReadAllIds();
            allIds[userId] = username;
            File.WriteAllText($"LoginCache\\Ubisoft\\ids.json", JsonConvert.SerializeObject(allIds));


            GeneralFuncs.RecursiveDelete(new DirectoryInfo("LoginCache\\Ubisoft\\temp\\"), false);
            Directory.CreateDirectory($"LoginCache\\Ubisoft\\{userId}\\");
            File.Copy(Path.Join(UbisoftAppData, "settings.yml"), $"LoginCache\\Ubisoft\\{userId}\\settings.yml", true);
            File.Copy(Path.Join(UbisoftAppData, "users.dat"), $"LoginCache\\Ubisoft\\{userId}\\users.dat", true);
            return username;
        }
        private static Dictionary<string, string> ReadAllIds()
        {
            var localAllIds = $"LoginCache\\Ubisoft\\ids.json";
            var s = JsonConvert.SerializeObject(new Dictionary<string, string>());
            if (File.Exists(localAllIds))
            {
                try
                {
                    s = File.ReadAllText(localAllIds);
                }
                catch (Exception)
                {
                    //
                }
            }

            return JsonConvert.DeserializeObject<Dictionary<string, string>>(s);
        }

        private static void ImportAvatar(string userId)
        {
            string i64 = Path.Join(UbisoftAvatarFolder, userId + "_64.png"),
                i128 = Path.Join(UbisoftAvatarFolder, userId + "_128.png"),
                i256 = Path.Join(UbisoftAvatarFolder, userId + "_256.png");
            Directory.CreateDirectory($"\\img\\profiles\\Ubisoft\\");
            var outPath = Path.Join(GeneralFuncs.WwwRoot, $"\\img\\profiles\\Ubisoft\\{userId}.png");
            if (File.Exists(i256)) File.Copy(i256, outPath, true);
            else if (File.Exists(i128)) File.Copy(i128, outPath, true);
            else if (File.Exists(i64)) File.Copy(i64, outPath, true);
            else File.Copy(Path.Join(GeneralFuncs.WwwRoot, "img\\QuestionMark.jpg"), outPath, true);
        }

        //// This seemed very possible: But... As far as I know the settings.yml file needs to be adjusted, as well as the users.dat...
        //// Might change the way that this works later.
        //private static void FindAllUsers()
        //{
        //    Directory.CreateDirectory("LoginCache\\Ubisoft\\temp\\");
        //    var tempUsersDat = $"LoginCache\\Ubisoft\\temp\\users.dat";
        //    File.Copy(Path.Join(UbisoftAppData, "users.dat"), $"LoginCache\\Ubisoft\\temp\\users.dat", true);
            

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
        //            File.Copy(Path.Join(UbisoftAppData, "settings.yml"), $"LoginCache\\Ubisoft\\{username}\\settings.yml", true);
        //            File.Copy(Path.Join(UbisoftAppData, "users.dat"), $"LoginCache\\Ubisoft\\{username}\\users.dat", true);

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
            Globals.DebugWriteLine($@"[Func:Ubisoft\UbisoftSwitcherFuncs.ForgetAccount] Forgetting account: {userId}");
            GeneralFuncs.RecursiveDelete(new DirectoryInfo($"LoginCache\\Ubisoft\\{userId}"), false);
            File.Delete(Path.Join(GeneralFuncs.WwwRoot, $"\\img\\profiles\\Ubisoft\\{userId}.png"));
            return true;
        }

        /// <summary>
        /// Restart Ubisoft with a new account selected. Leave args empty to log into a new account.
        /// </summary>
        /// <param name="userId">User's UserId</param>
        /// <param name="state">(Optional) State of user. 0 is online, anything else if Offline</param>
        public static void SwapUbisoftAccounts(string userId = "", int state = 0)
        {
            Globals.DebugWriteLine($@"[Func:Ubisoft\UbisoftSwitcherFuncs.SwapUbisoftAccounts] Swapping to: {userId}.");
            AppData.ActiveIJsRuntime.InvokeVoidAsync("updateStatus", "Closing Ubisoft");
            CloseUbisoft();
            UbisoftAddCurrent();

            if (userId != "")
            {
                UbisoftCopyInAccount(userId, state);
            }
            else
                ClearCurrentUser();

            AppData.ActiveIJsRuntime.InvokeVoidAsync("updateStatus", "Starting Ubisoft");
            if (Ubisoft.Admin)
                Process.Start(Ubisoft.UbisoftExe());
            else
                Process.Start(new ProcessStartInfo("explorer.exe", Ubisoft.UbisoftExe()));
        }

        private static void ClearCurrentUser()
        {
            var settingsYml = File.ReadAllLines(Path.Join(UbisoftAppData, "settings.yml"));
            for (var i = 0; i < settingsYml.Length; i++)
            {
                if (settingsYml[i].Contains("height:"))
                    settingsYml[i] = "  height: 0";
                if (settingsYml[i].Contains("left:"))
                    settingsYml[i] = "  left: 0";
                if (settingsYml[i].Contains("top:"))
                    settingsYml[i] = "  top: 0";
                if (settingsYml[i].Contains("width:"))
                    settingsYml[i] = "  width: 0";
                
                if (settingsYml[i].Contains("password:"))
                    settingsYml[i] = "  password: \"\"";
                if (settingsYml[i].Contains("remember:"))
                    settingsYml[i] = "  remember: false";
                if (settingsYml[i].Contains("remember_ticket:"))
                    settingsYml[i] = "  remember_ticket: \"\"";
                if (settingsYml[i].Contains("username: "))
                    settingsYml[i] = "  username: \"\"";
            }
            File.WriteAllLines(Path.Join(UbisoftAppData, "settings.yml"), settingsYml);
        }

        private static void UbisoftCopyInAccount(string userId, int state = 0)
        {
            var lineToReplace = "LINE HERE";
            foreach (var l in File.ReadAllLines($"LoginCache\\Ubisoft\\{userId}\\settings.yml"))
                if (l.Contains("forceoffline")) lineToReplace = l;

            var settingsFile = File.ReadAllText($"LoginCache\\Ubisoft\\{userId}\\settings.yml");
            settingsFile.Replace(lineToReplace, $"  forceoffline: {(state == 0 ? "false" : "true")}");
            File.WriteAllText(Path.Join(UbisoftAppData, "settings.yml"), settingsFile);
            File.Copy($"LoginCache\\Ubisoft\\{userId}\\users.dat", Path.Join(UbisoftAppData, "users.dat"), true);
            
        }
        

        #region UBISOFT_MANAGEMENT
        /// <summary>
        /// Kills Origin processes when run via cmd.exe
        /// </summary>
        public static void CloseUbisoft()
        {
            Globals.KillProcess("upc");
        }
        /// <summary>
        /// Clears images folder of contents, to re-download them on next load.
        /// </summary>
        /// <returns>Whether files were deleted or not</returns>
        public static async void ClearImages()
        {
            //Globals.DebugWriteLine($@"[Func:Ubisoft\UbisoftSwitcherFuncs.ClearImages] Clearing images.");
            ////if (!Directory.Exists(Steam.SteamImagePath))
            ////{
            ////    await GeneralInvocableFuncs.ShowToast("error", "Could not clear images", "Error", "toastarea");
            ////}
            ////foreach (var file in Directory.GetFiles(Origin.OriginImage))
            ////{
            ////    File.Delete(file);
            ////}
            ////// Reload page, then display notification using a new thread.
            //AppData.ActiveNavMan?.NavigateTo("/Ubisoft/?cacheReload&toast_type=success&toast_title=Success&toast_message=" + Uri.EscapeUriString("Cleared images"), true);
        }

        /// <summary>
        /// Only runs ForgetAccount, but allows Javascript to wait for it's completion before refreshing, instead of just doing it instantly >> Not showing proper results.
        /// </summary>
        /// <param name="userId">Account ID to remove.</param>
        /// <returns>true</returns>
        [JSInvokable]
        public static Task<bool> ForgetUbisoftAccountJs(string userId)
        {
            Globals.DebugWriteLine($@"[JSInvoke:Ubisoft\UbisoftSwitcherFuncs.ForgetUbisoftAccountJs] accName:{userId}");
            var allIds = ReadAllIds();
            allIds.Remove(userId);
            File.WriteAllText("LoginCache\\Ubisoft\\ids.json", JsonConvert.SerializeObject(allIds));
            return Task.FromResult(ForgetAccount(userId));
        }

        #endregion
    }
}

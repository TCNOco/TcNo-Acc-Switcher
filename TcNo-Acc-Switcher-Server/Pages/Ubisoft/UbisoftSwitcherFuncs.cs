using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Runtime.CompilerServices;
using System.Text.RegularExpressions;
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
        private static string _ubisoftAppData = Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "Ubisoft Game Launcher");
        private static string _ubisoftAvatarFolder = Path.Join(Ubisoft.FolderPath, "cache", "avatars");
        /// <summary>
        /// Main function for Ubisoft Account Switcher. Run on load.
        /// Collects accounts from Ubisoft Connect's files
        /// Prepares HTML Elements string for insertion into the account switcher GUI.
        /// </summary>
        /// <returns>Whether account loading is successful, or a path reset is needed (invalid dir saved)</returns>
        public static async void LoadProfiles()
        {
            // Normal:
            Globals.DebugWriteLine($@"[Func:Ubisoft\UbisoftSwitcherFuncs.LoadProfiles] Loading Steam profiles");
            
            var localCachePath = $"LoginCache\\Ubisoft\\";
            if (!Directory.Exists(localCachePath) || !File.Exists(Path.Join(localCachePath, "ids.json"))) return;
            var allIds = ReadAllIds();
            foreach (var (userId, username) in allIds)
            {
                var element =
                    $"<div class=\"acc_list_item\"><input type=\"radio\" id=\"{userId}\" Username=\"{username}\" class=\"acc\" name=\"accounts\" onchange=\"SelectedItemChanged()\" />\r\n" +
                    $"<label for=\"{userId}\" class=\"acc\">\r\n" +
                    $"<img src=\"" + $"\\img\\profiles\\Ubisoft\\{userId}.png" + "\" draggable=\"false\" />\r\n" +
                    $"<h6>{username}</h6></div>\r\n";
                //$"<p>{UnixTimeStampToDateTime(ua.LastLogin)}</p>\r\n</label>";  TODO: Add some sort of "Last logged in" json file
                try
                {
                    await AppData.ActiveIJsRuntime.InvokeVoidAsync("jQueryAppend", new object[] { "#acc_list", element });
                }
                catch (TaskCanceledException e)
                {
                    Console.WriteLine(e);  
                }
            }
            await AppData.ActiveIJsRuntime.InvokeVoidAsync("initContextMenu");
            await AppData.ActiveIJsRuntime.InvokeVoidAsync("initAccListSortable");
        }

        public static void UbisoftAddCurrent()
        {
            Globals.DebugWriteLine($@"[Func:Ubisoft\UbisoftSwitcherFuncs.UbisoftAddCurrent]");

            // To add account section:
            var userId = GetLastLoginUserId();
            if (userId == "NOTFOUND")
            {
                _ = GeneralInvocableFuncs.ShowToast("error", "Could not find last logged in user. Try logging into Ubisoft again first.", "Error", "toastarea");
                return;
            }

            // Find username from users.dat file
            var username = FindUsername(userId);
            if (username == "ERR") return;
            // Import profile picture to $"LoginCache\\Ubisoft\\{userId}\\pfp.png"
            ImportAvatar(userId);

            // Notification
            AppData.ActiveNavMan?.NavigateTo("/Ubisoft/?cacheReload&toast_type=success&toast_title=Success&toast_message=" + Uri.EscapeUriString("Saved: " + username), true);
        }

        private static string GetLastLoginUserId()
        {
            Globals.DebugWriteLine($@"[Func:Ubisoft\UbisoftSwitcherFuncs.GetLastLoginUserId]");
            File.Copy(Path.Join(Ubisoft.FolderPath, "logs", "launcher_log.txt"), "templog");
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

        //private static string FindUsername(string userId)
        public static string FindUsername(string userId, bool copyFiles = true)
        {
            _ubisoftAppData = Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "Ubisoft Game Launcher");


            Globals.DebugWriteLine($@"[Func:Ubisoft\UbisoftSwitcherFuncs.FindUsername]");
            Directory.CreateDirectory("LoginCache\\Ubisoft\\temp\\");
            var tempUsersDat = $"LoginCache\\Ubisoft\\temp\\users.dat";
            File.Copy(Path.Join(_ubisoftAppData, "users.dat"), $"LoginCache\\Ubisoft\\temp\\users.dat", true);

            var username = "";
            var nextLineIsUsername = false;
            using (var reader = new StreamReader(File.Open(tempUsersDat, FileMode.Open)))
            {
                var line = "";
                while ((line = reader.ReadLine()) != null)
                {
                    if (!nextLineIsUsername) // Will only be true if character separating userId and username is counted as a newline.
                    {
                        var userIdIndex = line.IndexOf(userId, StringComparison.Ordinal);
                        if (userIdIndex == -1) continue; // userId is not on this line >> Next

                        var indexAfterUserId = userIdIndex + userId.Length; // Find end of userId in string
                        if (indexAfterUserId == line.Length) // If username is on 'next line', grab it on next iteration.
                        {
                            nextLineIsUsername = true;
                            continue;
                        }
                        // Otherwise: it was found on this line, grab it.
                        line = line.Substring(indexAfterUserId, line.Length - indexAfterUserId);
                    }
                    // This grabs the username if on the line after userId
                    line = line.Split(":")[0];
                    username = new Regex(@"[^a-zA-Z0-9_\-.-]").Split(line).Last();
                    break;
                }
            }

            if (username == "")
            {
                _ = GeneralInvocableFuncs.ShowToast("error", "Could not find username", "Error", "toastarea");
                return "ERR";
            }

            SetUsername(userId, username);
            GeneralFuncs.RecursiveDelete(new DirectoryInfo("LoginCache\\Ubisoft\\temp\\"), false);

            if (!copyFiles)
            {
                AppData.ActiveNavMan?.NavigateTo("/Ubisoft/?cacheReload&toast_type=success&toast_title=Success&toast_message=" + Uri.EscapeUriString("Refreshed username from file"), true);
                return username; // Used for refreshing username.
            }
            Directory.CreateDirectory($"LoginCache\\Ubisoft\\{userId}\\");
            File.Copy(Path.Join(_ubisoftAppData, "settings.yml"), $"LoginCache\\Ubisoft\\{userId}\\settings.yml", true);
            File.Copy(Path.Join(_ubisoftAppData, "users.dat"), $"LoginCache\\Ubisoft\\{userId}\\users.dat", true);
            return username;
        }

        public static void SetUsername(string id, string username, bool reload = false)
        {
            var allIds = ReadAllIds();
            allIds[id] = username;
            File.WriteAllText($"LoginCache\\Ubisoft\\ids.json", JsonConvert.SerializeObject(allIds));
            if (reload) AppData.ActiveNavMan?.NavigateTo("/Ubisoft/?cacheReload&toast_type=success&toast_title=Success&toast_message=" + Uri.EscapeUriString("Set username"), true);
        }


        public static Dictionary<string, string> ReadAllIds()
        {
            Globals.DebugWriteLine($@"[Func:Ubisoft\UbisoftSwitcherFuncs.ReadAllIds]");
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

        public static void ImportAvatar(string userId)
        {
            Globals.DebugWriteLine($@"[Func:Ubisoft\UbisoftSwitcherFuncs.ImportAvatar] userId: {userId}");
            string i64 = Path.Join(_ubisoftAvatarFolder, userId + "_64.png"),
                i128 = Path.Join(_ubisoftAvatarFolder, userId + "_128.png"),
                i256 = Path.Join(_ubisoftAvatarFolder, userId + "_256.png");
            Directory.CreateDirectory($"wwwroot\\img\\profiles\\Ubisoft\\");
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

            var allIds = ReadAllIds();
            allIds.Remove(userId);
            File.WriteAllText("LoginCache\\Ubisoft\\ids.json", JsonConvert.SerializeObject(allIds));

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
            if (!CloseUbisoft()) return;
            UbisoftAddCurrent();

            if (userId != "")
            {
                UbisoftCopyInAccount(userId, state);
                Globals.AddTrayUser("Ubisoft", "+u:" + userId, ReadAllIds()[userId], Ubisoft.TrayAccNumber); // Add to Tray list
            }
            else
                ClearCurrentUser();

            AppData.ActiveIJsRuntime.InvokeVoidAsync("updateStatus", "Starting Ubisoft");
            
            GeneralFuncs.StartProgram(Ubisoft.Exe(), Ubisoft.Admin);
        }

        private static void ClearCurrentUser()
        {
            Globals.DebugWriteLine($@"[Func:Ubisoft\UbisoftSwitcherFuncs.ClearCurrentUser]");
            var settingsYml = File.ReadAllLines(Path.Join(_ubisoftAppData, "settings.yml"));
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
            File.WriteAllLines(Path.Join(_ubisoftAppData, "settings.yml"), settingsYml);
        }

        /// <summary>
        /// Copies user's account files from LoginCache\\Ubisoft\\{userId} to 
        /// </summary>
        /// <param name="userId"></param>
        /// <param name="state"></param>
        private static void UbisoftCopyInAccount(string userId, int state = 0)
        {
            Globals.DebugWriteLine($@"[Func:Ubisoft\UbisoftSwitcherFuncs.UbisoftCopyInAccount]");
            if (state == -1)
            {
                File.Copy($"LoginCache\\Ubisoft\\{userId}\\settings.yml", Path.Join(_ubisoftAppData, "settings.yml"), true);
                File.Copy($"LoginCache\\Ubisoft\\{userId}\\users.dat", Path.Join(_ubisoftAppData, "users.dat"), true);
                return;
            }
            using var fs = new StreamWriter(Path.Join(_ubisoftAppData, "settings.yml"));
            foreach (var l in File.ReadAllLines($"LoginCache\\Ubisoft\\{userId}\\settings.yml"))
            {
                if (l.Contains("forceoffline")) fs.WriteLine("  forceoffline: " + (state != 0 ? "true" : "false")); 
                else fs.WriteLine(l);
            }
            fs.Close();

            File.Copy($"LoginCache\\Ubisoft\\{userId}\\users.dat", Path.Join(_ubisoftAppData, "users.dat"), true);
            File.Copy(Path.Join(_ubisoftAppData, "settings.yml"), $"LoginCache\\Ubisoft\\{userId}\\settings.yml", true);
        }
        

        #region UBISOFT_MANAGEMENT
        /// <summary>
        /// Kills Origin processes when run via cmd.exe
        /// </summary>
        public static bool CloseUbisoft()
        {
            Globals.DebugWriteLine($@"[Func:Ubisoft\UbisoftSwitcherFuncs.CloseUbisoft]");
            if (!GeneralFuncs.CanKillProcess("upc")) return false;
            Globals.KillProcess("upc");
            return true;
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
        #endregion
    }
}

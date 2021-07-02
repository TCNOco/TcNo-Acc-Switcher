using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Runtime.Versioning;
using System.Security.Cryptography;
using System.Text;
using System.Text.RegularExpressions;
using System.Threading.Tasks;
using Microsoft.JSInterop;
using Microsoft.Win32;
using Newtonsoft.Json;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Pages.General.Classes;
using Task = System.Threading.Tasks.Task;

namespace TcNo_Acc_Switcher_Server.Pages.Discord
{
    public class DiscordSwitcherFuncs
    {
        private static readonly Data.Settings.Discord Discord = Data.Settings.Discord.Instance;
        private static readonly string DiscordRoaming = Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "discord");
        private static readonly string DiscordCookies = Path.Join(DiscordRoaming, "Cookies");
        private static readonly string DiscordCacheFolder = Path.Join(DiscordRoaming, "Cache");
        private static readonly string DiscordStorageFolder = Path.Join(DiscordRoaming, "Local Storage\\leveldb");

        /// <summary>
        /// Main function for Discord Account Switcher. Run on load.
        /// Collects accounts from cache folder
        /// Prepares HTML Elements string for insertion into the account switcher GUI.
        /// </summary>
        /// <returns>Whether account loading is successful, or a path reset is needed (invalid dir saved)</returns>
        public static void LoadProfiles()
        {
            // Normal:
            Globals.DebugWriteLine(@"[Func:Discord\DiscordSwitcherFuncs.LoadProfiles] Loading Discord profiles");
            GenericFunctions.GenericLoadAccounts("Discord");
        }

        /// <summary>
        /// Used in JS. Gets whether forget account is enabled (Whether to NOT show prompt, or show it).
        /// </summary>
        /// <returns></returns>
        [JSInvokable]
        public static Task<bool> GetDiscordForgetAcc() => Task.FromResult(Discord.ForgetAccountEnabled);

        /// <summary>
        /// Remove requested account from loginusers.vdf
        /// </summary>
        /// <param name="accName">Discord account name</param>
        public static bool ForgetAccount(string accName)
        {
            Globals.DebugWriteLine(@"[Func:DiscordDiscordSwitcherFuncs.ForgetAccount] Forgetting account: hidden");
            // Remove ID from list of ids
            var allIds = ReadAllIds();
            allIds.Remove(allIds.Single(x => x.Value == accName).Key);
            File.WriteAllText("LoginCache\\Discord\\ids.json", JsonConvert.SerializeObject(allIds));
            
            // Remove cached files
            GeneralFuncs.RecursiveDelete(new DirectoryInfo($"LoginCache\\Discord\\{accName}"), false);
            // Remove image
            var img = Path.Join(GeneralFuncs.WwwRoot(), $"\\img\\profiles\\discord\\{Uri.EscapeUriString(accName)}.jpg");
            if (File.Exists(img)) File.Delete(img);
            // Remove from Tray
            Globals.RemoveTrayUser("Discord", accName); // Add to Tray list
            return true;
        }
        
        public static string GetHashString(string text)
        {
	        if (string.IsNullOrEmpty(text))
		        return string.Empty;

	        using var sha = new System.Security.Cryptography.SHA256Managed();
	        var textData = System.Text.Encoding.UTF8.GetBytes(text);
	        var hash = sha.ComputeHash(textData);
	        return BitConverter.ToString(hash).Replace("-", string.Empty);
        }
        private static string GetHashedDiscordToken()
        {
	        // Loop through log/ldb files:
            var token = "";
	        foreach (var file in new DirectoryInfo(DiscordStorageFolder).GetFiles("*"))
	        {
		        if (!file.Name.EndsWith(".ldb") && !file.Name.EndsWith(".log")) continue;
		        var text = File.ReadAllText(file.FullName);
		        var test1 = new Regex(@"[\w-]{24}\.[\w-]{6}\.[\w-]{27}").Match(text).Value;
		        var test2 = new Regex(@"mfa\.[\w-]{84}").Match(text).Value;
		        token = test1 != "" ? test1 : test2 != "" ? test2 : token;
	        }

	        if (token == "")
		        GeneralInvocableFuncs.ShowToast("error", "Failed to find user's token!", "Error", "toastarea");
	        else
	        {
		        token = GetHashString(token);

	        }

	        return token;
        }


        /// <summary>
        /// Restart Discord with a new account selected. Leave args empty to log into a new account.
        /// </summary>
        /// <param name="accName">(Optional) User's login username</param>
        [SupportedOSPlatform("windows")]
        public static void SwapDiscordAccounts(string accName = "")
        {
            Globals.DebugWriteLine(@"[Func:Discord\DiscordSwitcherFuncs.SwapDiscordAccounts] Swapping to: hidden.");
            AppData.InvokeVoidAsync("updateStatus", "Closing Discord");
            if (!CloseDiscord()) return;
            ClearCurrentLoginDiscord();
            if (accName != "")
            {
                if (!DiscordCopyInAccount(accName)) return;
                Globals.AddTrayUser("Discord", "+e:" + accName, accName, Discord.TrayAccNumber); // Add to Tray list
            }
            AppData.InvokeVoidAsync("updateStatus", "Starting Discord");

            GeneralFuncs.StartProgram(Discord.Exe(), Discord.Admin);

            Globals.RefreshTrayArea();
        }

        [SupportedOSPlatform("windows")]
        private static void ClearCurrentLoginDiscord()
        {
            Globals.DebugWriteLine(@"[Func:Discord\DiscordSwitcherFuncs.ClearCurrentLoginDiscord]");
            // Save current
            var hash = GetHashedDiscordToken();
            var allIds = ReadAllIds();
            if (allIds.ContainsKey(hash))
	            DiscordAddCurrent(allIds[hash]);

			// Remove Cookies file
			if (File.Exists(DiscordCookies)) File.Delete(DiscordCookies);

            // Loop through Cache files:
            foreach (var file in new DirectoryInfo(DiscordCacheFolder).GetFiles("*"))
	            if (file.Name.StartsWith("data_") || file.Name == "index")
                    File.Delete(file.FullName);

			// Loop through log/ldb files:
			foreach (var file in new DirectoryInfo(DiscordStorageFolder).GetFiles("*"))
				if (file.Name.EndsWith(".ldb") || file.Name.EndsWith(".log"))
					File.Delete(file.FullName);
        }

        [SupportedOSPlatform("windows")]
        private static bool DiscordCopyInAccount(string accName)
        {
            Globals.DebugWriteLine(@"[Func:Discord\DiscordSwitcherFuncs.DiscordCopyInAccount]");
            GeneralInvocableFuncs.ShowToast("info", "Hint: Changed Discord settings? Save again, with the same name!",
	            "Discord note", "toastarea");

			var localCachePath = $"LoginCache\\Discord\\{accName}\\";
            if (!Directory.Exists(localCachePath))
            {
	            _ = GeneralInvocableFuncs.ShowToast("error", $"Could not find {localCachePath}", "Directory not found", "toastarea");
	            return false;
            }

            // Clear files first, as some extra files from different accounts can have different names.
            ClearCurrentLoginDiscord();

			foreach (var file in new DirectoryInfo(localCachePath).GetFiles("*"))
            {
	            if (file.Name == "Cookies") File.Copy(file.FullName, DiscordCookies, true);
                else if (file.Name.StartsWith("data_") || file.Name == "index") File.Copy(file.FullName, Path.Join(DiscordCacheFolder, file.Name), true);
                else if (file.Name.EndsWith(".ldb") || file.Name.EndsWith(".log")) File.Copy(file.FullName, Path.Join(DiscordStorageFolder, file.Name), true);
            }

            return true;
        }

        [SupportedOSPlatform("windows")]
        public static void DiscordAddCurrent(string accName)
        {
            Globals.DebugWriteLine(@"[Func:Discord\DiscordSwitcherFuncs.DiscordAddCurrent]");
            GeneralInvocableFuncs.ShowToast("info", "Hint: You must sign out of Discord in your browser",
	            "Discord note", "toastarea");

            var closedBySwitcher = false;
            var localCachePath = $"LoginCache\\Discord\\{accName}\\";
            Directory.CreateDirectory(localCachePath);
            var hash = "";
            var attempts = 0;
            while (attempts < 5)
            {
	            try
	            {
		            hash = GetHashedDiscordToken();
		            break;
	            }
	            catch (IOException e)
	            {
		            if (attempts > 2 && e.HResult == -2147024864) // File is in use - but wait 2 seconds to see...
                    {
			            CloseDiscord();
			            closedBySwitcher = true;

		            }
		            else if (attempts < 4)
		            {
			            AppData.InvokeVoidAsync("updateStatus", $"Files in use... Timeout: ({attempts}/4 seconds)");
			            System.Threading.Thread.Sleep(1000);
                    }
		            else throw;
	            }

	            attempts++;
            }

            if (string.IsNullOrEmpty(hash)) return;

            var allIds = ReadAllIds();
            allIds[hash] = accName;
            File.WriteAllText("LoginCache\\Discord\\ids.json", JsonConvert.SerializeObject(allIds));

            // Copy Cookies file
            if (File.Exists(DiscordCookies)) File.Copy(DiscordCookies, Path.Join(localCachePath, "Cookies"), true);

            // Loop through Cache files:
            foreach (var file in new DirectoryInfo(DiscordCacheFolder).GetFiles("*"))
	            if (file.Name.StartsWith("data_") || file.Name == "index")
		            File.Copy(file.FullName, Path.Join(localCachePath, file.Name), true);

            // Loop through log/ldb files:
            foreach (var file in new DirectoryInfo(DiscordStorageFolder).GetFiles("*"))
	            if (file.Name.EndsWith(".ldb") || file.Name.EndsWith(".log"))
		            File.Copy(file.FullName, Path.Join(localCachePath, file.Name), true);
            
            // Copy in profile image from default
            Directory.CreateDirectory(Path.Join(GeneralFuncs.WwwRoot(), "\\img\\profiles\\discord"));
            var profileImg = Path.Join(GeneralFuncs.WwwRoot(), $"\\img\\profiles\\discord\\{Uri.EscapeUriString(accName)}.jpg");
            if (!File.Exists(profileImg)) File.Copy(Path.Join(GeneralFuncs.WwwRoot(), "\\img\\DiscordDefault.png"), profileImg, true);
            
            if (closedBySwitcher) GeneralFuncs.StartProgram(Discord.Exe(), Discord.Admin);

            try
            {
	            AppData.ActiveNavMan?.NavigateTo("/Discord/?cacheReload&toast_type=success&toast_title=Success&toast_message=" + Uri.EscapeUriString("Saved: " + accName), true);
            }
            catch (Microsoft.AspNetCore.Components.NavigationException e) // Page was reloaded.
            {
	            //
            }
        }

        public static void ChangeUsername(string oldName, string newName, bool reload = true)
        {
            File.Move(Path.Join(GeneralFuncs.WwwRoot(), $"\\img\\profiles\\discord\\{Uri.EscapeUriString(oldName)}.jpg"),
                Path.Join(GeneralFuncs.WwwRoot(), $"\\img\\profiles\\discord\\{Uri.EscapeUriString(newName)}.jpg")); // Rename image
            Directory.Move($"LoginCache\\Discord\\{oldName}\\", $"LoginCache\\Discord\\{newName}\\"); // Rename login cache folder

            if (reload) AppData.ActiveNavMan?.NavigateTo("/Discord/?cacheReload&toast_type=success&toast_title=Success&toast_message=" + Uri.EscapeUriString("Changed username"), true);
        }

        private static Dictionary<string, string> ReadAllIds()
        {
	        Globals.DebugWriteLine(@"[Func:Discord\DiscordSwitcherFuncs.ReadAllIds]");
	        const string localAllIds = "LoginCache\\Discord\\ids.json";
	        var s = JsonConvert.SerializeObject(new Dictionary<string, string>());
	        if (!File.Exists(localAllIds)) return JsonConvert.DeserializeObject<Dictionary<string, string>>(s);
	        try
	        {
		        s = File.ReadAllText(localAllIds);
	        }
	        catch (Exception)
	        {
		        //
	        }

	        return JsonConvert.DeserializeObject<Dictionary<string, string>>(s);
        }

        #region DISCORD_MANAGEMENT
        /// <summary>
        /// Kills Discord processes when run via cmd.exe
        /// </summary>
        public static bool CloseDiscord()
        {
            Globals.DebugWriteLine(@"[Func:Discord\DiscordSwitcherFuncs.CloseDiscord]");
            if (!GeneralFuncs.CanKillProcess("Discord")) return false;
            Globals.KillProcess("Discord");
            return GeneralFuncs.WaitForClose("Discord");
        }
        #endregion
    }
}

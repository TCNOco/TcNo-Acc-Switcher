// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2021 TechNobo (Wesley Pyburn)
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
using System.Globalization;
using System.IO;
using System.Linq;
using System.Net;
using System.Runtime.Versioning;
using System.Text;
using System.Threading.Tasks;
using System.Xml;
using Gameloop.Vdf;
using Gameloop.Vdf.JsonConverter;
using Microsoft.JSInterop;
using Microsoft.Win32;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Converters;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General;

namespace TcNo_Acc_Switcher_Server.Pages.Steam
{
    public class SteamSwitcherFuncs
    {
        private static readonly Data.Settings.Steam Steam = Data.Settings.Steam.Instance;

        #region STEAM_SWITCHER_MAIN
        public static bool SteamSettingsValid()
        {
            // Checks if Steam path set properly, and can load.
            Steam.LoadFromFile();
            return Steam.LoginUsersVdf() != "RESET_PATH";
        }

        /// <summary>
        /// Main function for Steam Account Switcher. Run on load.
        /// Collects accounts from Steam's loginusers.vdf
        /// Prepares images and VAC/Limited status
        /// Prepares HTML Elements string for insertion into the account switcher GUI.
        /// </summary>
        /// <returns>Whether account loading is successful, or a path reset is needed (invalid dir saved)</returns>
        public static async Task LoadProfiles()
        {
            Globals.DebugWriteLine(@"[Func:Steam\SteamSwitcherFuncs.LoadProfiles] Loading Steam profiles");
            var userAccounts = GetSteamUsers(Steam.LoginUsersVdf()); 
            var vacStatusList = new List<VacStatus>();
            var loadedVacCache = LoadVacInfo(ref vacStatusList);

            // Order
            if (File.Exists("LoginCache\\Steam\\order.json"))
            {
                var savedOrder = JsonConvert.DeserializeObject<List<string>>(await File.ReadAllTextAsync("LoginCache\\Steam\\order.json").ConfigureAwait(false));
                if (savedOrder != null)
                {
                    var index = 0;
                    if (savedOrder is { Count: > 0 })
                        foreach (var acc in from i in savedOrder where userAccounts.Any(x => x.SteamId == i) select userAccounts.Single(x => x.SteamId == i))
                        {
                            userAccounts.Remove(acc);
                            userAccounts.Insert(index, acc);
                            index++;
                        }
                }
            }

            foreach (var ua in userAccounts)
            {
                var va = new VacStatus();
                if (loadedVacCache)
                {
                    PrepareProfileImage(ua); // Just get images
                    foreach (var vsi in vacStatusList.Where(vsi => vsi.SteamId == ua.SteamId))
                    {
                        va = vsi;
                        break;
                    }
                }
                else
                {
                    va = PrepareProfileImage(ua); // Get VAC status as well
                    va.SteamId = ua.SteamId;
                    vacStatusList.Add(va);
                }
                
                var extraClasses = (Steam.ShowVac && va.Vac ? " status_vac" : "") + (Steam.ShowLimited && va.Ltd ? " status_limited" : "");

                var element =
                    $"<div class=\"acc_list_item\"><input type=\"radio\" id=\"{ua.SteamId}\" DisplayName=\"{ua.Name}\" class=\"acc\" name=\"accounts\" Username=\"{ua.AccName}\" SteamId64=\"{ua.SteamId}\" Line1=\"{ua.AccName}\" Line2=\"{ua.Name}\" Line3=\"{ua.LastLogin}\" ExtraClasses=\"{extraClasses}\" onchange=\"selectedItemChanged()\" />\r\n" +
                    $"<label for=\"{ua.AccName}\" class=\"acc {extraClasses}\">\r\n" +
                    $"<img class=\"{extraClasses}\" src=\"{ua.ImgUrl}?{Globals.GetUnixTime()}\" draggable=\"false\" />\r\n" +
                    $"<p class=\"streamerCensor\">{ua.AccName}</p>\r\n" +
                    $"<h6>{ua.Name}</h6>\r\n" +
                    $"<p class=\"streamerCensor steamId\">{ua.SteamId}</p>\r\n" +
                    $"<p>{UnixTimeStampToDateTime(ua.LastLogin)}</p></label></div>\r\n";

                await AppData.InvokeVoidAsync("jQueryAppend", "#acc_list", element);
            }

            SaveVacInfo(vacStatusList);
            AppData.InvokeVoid("jQueryProcessAccListSize");
            AppData.InvokeVoid("initContextMenu");
            AppData.InvokeVoid("initAccListSortable");
        }

        /// <summary>
        /// Takes loginusers.vdf and iterates through each account, loading details into output Steamuser list.
        /// </summary>
        /// <param name="loginUserPath">loginusers.vdf path</param>
        /// <returns>List of Steamuser classes, from loginusers.vdf</returns>
        public static List<Index.Steamuser> GetSteamUsers(string loginUserPath)
        {
            Globals.DebugWriteLine($@"[Func:Steam\SteamSwitcherFuncs.GetSteamUsers] Getting list of Steam users from {loginUserPath}");
            var userAccounts = new List<Index.Steamuser>();

            userAccounts.Clear();
            Directory.CreateDirectory("wwwroot/img/profiles");
            try
            {
	            var vdfText = VerifyVdfText(File.ReadAllText(loginUserPath), loginUserPath);

                var loginUsersVToken = VdfConvert.Deserialize(vdfText);
                var loginUsers = new JObject { loginUsersVToken.ToJson() };

                if (loginUsers["users"] != null)
                {
                    userAccounts.AddRange(from user in loginUsers["users"]
                    let steamId = user.ToObject<JProperty>()?.Name
                    where !string.IsNullOrEmpty(steamId) && !string.IsNullOrEmpty(user.First?["AccountName"]?.ToString())
                    select new Index.Steamuser
                    {
                        Name = user.First?["PersonaName"]?.ToString(),
                        AccName = user.First?["AccountName"]?.ToString(),
                        SteamId = steamId,
                        ImgUrl = "img/QuestionMark.jpg",
                        LastLogin = user.First?["Timestamp"]?.ToString(),
                        OfflineMode = !string.IsNullOrEmpty(user.First?["WantsOfflineMode"]?.ToString()) ? user.First?["WantsOfflineMode"]?.ToString() : "0"
                    });
                }
            }
            catch (FileNotFoundException)
            {
                Environment.Exit(2); // The system cannot find the file specified.
            }

            return userAccounts;
        }

        private static string VerifyVdfText(string vdf, string loginUserPath)
        {
	        var original = vdf;
	        // Replaces double quotes, sometimes added by mistake (?) with single, as they should be.
            vdf = vdf.Replace("\"\"", "\"");
            if (original == vdf) return vdf;

            // Save original file if different.
            try
            {
	            File.WriteAllText(loginUserPath, vdf);
            }
            catch (Exception)
            {
	            //
            }
            return vdf;
        }

        /// <summary>
        /// Deletes cached VAC/Limited status file
        /// </summary>
        /// <returns>Whether deletion successful</returns>
        public static bool DeleteVacCacheFile()
        {
            Globals.DebugWriteLine(@"[Func:Steam\SteamSwitcherFuncs.DeleteVacCacheFile] Deleting VAC cache file:hidden");
            if (!File.Exists(Steam.VacCacheFile)) return true;
            File.Delete(Steam.VacCacheFile);
            return true;
        }

        /// <summary>
        /// Loads List of VacStatus classes into input cache from file, or deletes if outdated.
        /// </summary>
        /// <param name="vsl">Reference to List of VacStatus</param>
        /// <returns>Whether file was loaded. False if deleted ~ failed to load.</returns>
        public static bool LoadVacInfo(ref List<VacStatus> vsl)
        {
            Globals.DebugWriteLine(@"[Func:Steam\SteamSwitcherFuncs.LoadVacInfo] Loading VAC info: hidden");
            GeneralFuncs.DeletedOutdatedFile(Steam.VacCacheFile);
            if (!File.Exists(Steam.VacCacheFile)) return false;
            vsl = JsonConvert.DeserializeObject<List<VacStatus>>(File.ReadAllText(Steam.VacCacheFile));

            if (vsl != null) return true;
            File.Delete(Steam.VacCacheFile);
            vsl = new List<VacStatus>();
            return true;
        }

        /// <summary>
        /// Saves List of VacStatus into cache file as JSON.
        /// </summary>
        public static void SaveVacInfo(List<VacStatus> vsList)
        {
            Directory.CreateDirectory(Path.GetDirectoryName(Steam.VacCacheFile) ?? string.Empty);
            File.WriteAllText(Steam.VacCacheFile, JsonConvert.SerializeObject(vsList));
        }

        /// <summary>
        /// Converts Unix Timestamp string to DateTime
        /// </summary>
        public static string UnixTimeStampToDateTime(string stringUnixTimeStamp)
        {
            if (!double.TryParse(stringUnixTimeStamp, out var unixTimeStamp)) return "";
            // Unix timestamp is seconds past epoch
            var dtDateTime = new DateTime(1970, 1, 1, 0, 0, 0, 0, DateTimeKind.Utc);
            dtDateTime = dtDateTime.AddSeconds(unixTimeStamp).ToLocalTime();
            return dtDateTime.ToString(CultureInfo.InvariantCulture);
        }

        /// <summary>
        /// Class for storing SteamID, VAC status and Limited status.
        /// </summary>
        public class VacStatus
        {
            [JsonProperty("SteamID", Order = 0)] public string SteamId { get; set; }
            [JsonProperty("Vac", Order = 1)] public bool Vac { get; set; }
            [JsonProperty("Ltd", Order = 2)] public bool Ltd { get; set; }
        }

        /// <summary>
        /// Deletes outdated/invalid profile images (If they exist)
        /// Then downloads a new copy from Steam
        /// </summary>
        /// <param name="su"></param>
        /// <returns></returns>
        private static VacStatus PrepareProfileImage(Index.Steamuser su)
        {
            Globals.DebugWriteLine($@"[Func:Steam\SteamSwitcherFuncs.PrepareProfileImage] Preparing profile image for: {su.SteamId.Substring(su.SteamId.Length-4, 4)}");
            Directory.CreateDirectory(Steam.SteamImagePath);
            var dlDir = $"{Steam.SteamImagePath}{su.SteamId}.jpg";
            // Delete outdated file, if it exists
            GeneralFuncs.DeletedOutdatedFile(dlDir, Steam.ImageExpiryTime);
            // ... & invalid files
            GeneralFuncs.DeletedInvalidImage(dlDir);

            var vs = new VacStatus();
            
            // Download new copy of the file
            if (!File.Exists(dlDir))
            {
                var imageUrl = GetUserImageUrl(ref vs, su);
                if (string.IsNullOrEmpty(imageUrl)) return vs;
                try
                {
                    using (var client = new WebClient())
                    {
                        client.DownloadFile(new Uri(imageUrl), dlDir);
                    }
                    su.ImgUrl = $"{Steam.SteamImagePathHtml}{su.SteamId}.jpg";
                }
                catch (WebException ex)
                {
                    if (ex.HResult == -2146233079) return vs;
                    su.ImgUrl = "img/QuestionMark.jpg";
                    Globals.WriteToLog("ERROR: Could not connect and download Steam profile's image from Steam servers.\nCheck your internet connection.\n\nDetails: " + ex);
                    //MessageBox.Show($"{Strings.ErrImageDownloadFail} {ex}", Strings.ErrProfileImageDlFail, MessageBoxButton.OK, MessageBoxImage.Error);
                }
            }
            else
            {
                su.ImgUrl = $"{Steam.SteamImagePathHtml}{su.SteamId}.jpg";
                var profileXml = new XmlDocument();
                var cachedFile = $"LoginCache/Steam/VACCache/{su.SteamId}.xml";
                Directory.CreateDirectory("LoginCache/Steam/VACCache/");
                profileXml.Load(File.Exists(cachedFile)? cachedFile : $"https://steamcommunity.com/profiles/{su.SteamId}?xml=1");
                if (!File.Exists(cachedFile)) profileXml.Save(cachedFile);

                if (profileXml.DocumentElement == null ||
                    profileXml.DocumentElement.SelectNodes("/profile/privacyMessage")?.Count != 0) return vs;

                    XmlGetVacLimitedStatus(ref vs, profileXml);
            }

            return vs;
        }

        /// <summary>
        /// Read's Steam's public XML data on user (& Caches).
        /// Gets user's image URL and checks for VAC bans, and limited account.
        /// </summary>
        /// <param name="vs">Reference to VacStatus variable</param>
        /// <param name="su">Steamuser to be checked</param>
        /// <returns>User's image URL for downloading</returns>
        private static string GetUserImageUrl(ref VacStatus vs, Index.Steamuser su)
        {
            Globals.DebugWriteLine($@"[Func:Steam\SteamSwitcherFuncs.GetUserImageUrl] Reading XML for: {su.SteamId.Substring(su.SteamId.Length - 4, 4)}");
            var imageUrl = "";
            var profileXml = new XmlDocument();
            try
            {
                profileXml.Load($"https://steamcommunity.com/profiles/{su.SteamId}?xml=1");
                // Cache for later
                Directory.CreateDirectory("LoginCache/Steam/VACCache");
                profileXml.Save($"LoginCache/Steam/VACCache/{su.SteamId}.xml");

                if (profileXml.DocumentElement != null && profileXml.DocumentElement.SelectNodes("/profile/privacyMessage")?.Count == 0) // Fix for accounts that haven't set up their Community Profile
                {
                    try
                    {
                        imageUrl = profileXml.DocumentElement.SelectNodes("/profile/avatarFull")[0].InnerText;
                        XmlGetVacLimitedStatus(ref vs, profileXml);
                    }
                    catch (NullReferenceException) // User has not set up their account, or does not have an image.
                    {
                        imageUrl = "";
                    }
                }
            }
            catch (Exception e)
            {
                imageUrl = "";
                Globals.DebugWriteLine($@"[Func:Steam\SteamSwitcherFuncs.GetUserImageUrl] ERROR: {e}");
            }
            return imageUrl;
        }

        /// <summary>
        /// Gets VAC & Limited status from input XML Document.
        /// </summary>
        /// <param name="vs">Reference to VacStatus object to be edited</param>
        /// <param name="profileXml">User's profile XML string</param>
        private static void XmlGetVacLimitedStatus(ref VacStatus vs, XmlDocument profileXml)
        {
            Globals.DebugWriteLine(@"[Func:Steam\SteamSwitcherFuncs.XmlGetVacLimitedStatus] Get VAC/Limited status for account.");
            if (profileXml.DocumentElement == null) return;
            try
            {
                if (profileXml.DocumentElement.SelectNodes("/profile/vacBanned")?[0] != null)
                    vs.Vac = profileXml.DocumentElement.SelectNodes("/profile/vacBanned")?[0].InnerText == "1";
                if (profileXml.DocumentElement.SelectNodes("/profile/isLimitedAccount")?[0] != null)
                    vs.Ltd = profileXml.DocumentElement.SelectNodes("/profile/isLimitedAccount")?[0].InnerText == "1";
            }
            catch (NullReferenceException)
            {
                Globals.DebugWriteLine(@"[Func:Steam\SteamSwitcherFuncs.XmlGetVacLimitedStatus] SUPPRESSED ERROR: NullReferenceException");
            }
        }

        /// <summary>
        /// Restart Steam with a new account selected. Leave args empty to log into a new account.
        /// </summary>
        /// <param name="steamId">(Optional) User's SteamID</param>
        /// <param name="autoStartSteam">(Optional) Whether Steam should start after switching [Default: true]</param>
        /// <param name="ePersonaState">(Optional) Persona state for user [0: Offline, 1: Online...]</param>
        public static void SwapSteamAccounts(string steamId = "", bool autoStartSteam = true, int ePersonaState = -1)
        {
            Globals.DebugWriteLine($@"[Func:Steam\SteamSwitcherFuncs.SwapSteamAccounts] Swapping to: hidden. autoStartSteam={autoStartSteam}, ePersonaState={ePersonaState}");
            if (steamId != "" && !VerifySteamId(steamId))
            {
                return;
            }
            AppData.InvokeVoid("updateStatus", "Closing Steam");
            if (!CloseSteam()) return;
            if (OperatingSystem.IsWindows()) UpdateLoginUsers(steamId, ePersonaState);
            AppData.InvokeVoid("updateStatus", "Starting Steam");
            if (!autoStartSteam) return;

            GeneralFuncs.StartProgram(Steam.Exe(), Steam.Admin);

            Globals.RefreshTrayArea();
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
        #endregion

        #region STEAM_MANAGEMENT
        /// <summary>
        /// Kills Steam processes when run via cmd.exe
        /// </summary>
        public static bool CloseSteam()
        {
            Globals.DebugWriteLine(@"[Func:Steam\SteamSwitcherFuncs.CloseSteam]");
            if (!GeneralFuncs.CanKillProcess("steam")) return false;
            Globals.KillProcess("steam");
            return true;
        }

        /// <summary>
        /// Updates loginusers and registry to select an account as "most recent"
        /// </summary>
        /// <param name="selectedSteamId">Steam ID64 to switch to</param>
        /// <param name="pS">[PersonaState]0-7 custom persona state [0: Offline, 1: Online...]</param>
        [SupportedOSPlatform("windows")]
        public static void UpdateLoginUsers(string selectedSteamId, int pS)
        {
            Globals.DebugWriteLine($@"[Func:Steam\SteamSwitcherFuncs.UpdateLoginUsers] Updating loginusers: selectedSteamId={(selectedSteamId.Length > 0 ? selectedSteamId.Substring(selectedSteamId.Length - 4, 4) : "")}, pS={pS}");
            var userAccounts = GetSteamUsers(Steam.LoginUsersVdf());
            // -----------------------------------
            // ----- Manage "loginusers.vdf" -----
            // -----------------------------------
            AppData.InvokeVoid("updateStatus", "Updating loginusers.vdf");
            var tempFile = Steam.LoginUsersVdf() + "_temp";
            File.Delete(tempFile);

            // MostRec is "00" by default, just update the one that matches SteamID.
            userAccounts.Where(x => x.SteamId == selectedSteamId).ToList().ForEach(u =>
            {
                u.MostRec = "1";
                u.RememberPass = "1";
                u.OfflineMode = pS == -1 ? u.OfflineMode : pS > 1 ? "0" : pS == 1 ? "0" : "1";
                // u.OfflineMode: Set ONLY if defined above
                // If defined & > 1, it's custom, therefor: Online
                // Otherwise, invert [0 == Offline => Online, 1 == Online => Offline]
            });
            //userAccounts.Single(x => x.SteamId == selectedSteamId).MostRec = "1";
            
            // Save updated loginusers.vdf
            SaveSteamUsersIntoVdf(userAccounts);

            // -----------------------------------
            // - Update localconfig.vdf for user -
            // -----------------------------------
            if (pS != -1) SetPersonaState(selectedSteamId, pS); // Update persona state, if defined above.

            Index.Steamuser user = new() { AccName = "" };
            if (selectedSteamId != "")
                user = userAccounts.Single(x => x.SteamId == selectedSteamId);
            // -----------------------------------
            // --------- Manage registry ---------
            // -----------------------------------
            /*
            ------------ Structure ------------
            HKEY_CURRENT_USER\Software\Valve\Steam\
                --> AutoLoginUser = username
                --> RememberPassword = 1
            */
            AppData.InvokeVoid("updateStatus", "Updating registry");
            using var key = Registry.CurrentUser.CreateSubKey(@"Software\Valve\Steam");
            key?.SetValue("AutoLoginUser", user.AccName); // Account name is not set when changing user accounts from launch arguments (part of the viewmodel). -- Can be "" if no account
            key?.SetValue("RememberPassword", 1);

            // -----------------------------------
            // ------Update Tray users list ------
            // -----------------------------------
            if (selectedSteamId != "")
                Globals.AddTrayUser("Steam", "+s:" + user.SteamId, Steam.TrayAccName ? user.AccName : user.Name, Steam.TrayAccNumber);
        }

        /// <summary>
        /// Save updated list of Steamuser into loginusers.vdf, in vdf format.
        /// </summary>
        /// <param name="userAccounts">List of Steamuser to save into loginusers.vdf</param>
        public static void SaveSteamUsersIntoVdf(List<Index.Steamuser> userAccounts)
        {
            Globals.DebugWriteLine($@"[Func:Steam\SteamSwitcherFuncs.SaveSteamUsersIntoVdf] Saving updated loginusers.vdf. Count: {userAccounts.Count}");
            // Convert list to JObject list, ready to save into vdf.
            var outJObject = new JObject();
            foreach (var ua in userAccounts)
            {
                outJObject[ua.SteamId] = (JObject)JToken.FromObject(ua);
            }

            // Write changes to files.
            var tempFile = Steam.LoginUsersVdf() + "_temp";
            File.WriteAllText(tempFile, @"""users""" + Environment.NewLine + outJObject.ToVdf());
            File.Replace(tempFile, Steam.LoginUsersVdf(), Steam.LoginUsersVdf() + "_last");
        }

        /// <summary>
        /// Clears backups of forgotten accounts
        /// </summary>
        public static async Task ClearForgotten()
        {
            Globals.DebugWriteLine(@"[Func:Steam\SteamSwitcherFuncs.ClearForgotten] Clearing forgotten backups.");
            await GeneralInvocableFuncs.ShowModal("confirm:ClearSteamBackups:" + "Are you sure you want to clear backups of forgotten accounts?".Replace(' ', '_'));
            // Confirmed in GeneralInvocableFuncs.GiConfirmAction for rest of function
        }
        /// <summary>
        /// Fires after being confirmed by above function, and actually performs task.
        /// </summary>
        public static void ClearForgotten_Confirmed()
        {
            Globals.DebugWriteLine(@"[Func:Steam\SteamSwitcherFuncs.ClearForgotten_Confirmed] Confirmation received to clear forgotten backups.");
            var legacyBackupPath = Path.Join(Steam.FolderPath, "config\\\\TcNo-Acc-Switcher-Backups\\\\");
            if (Directory.Exists(legacyBackupPath))
                    Directory.Delete(legacyBackupPath, true);

            // Handle new method:
            if (File.Exists("SteamForgotten.json")) File.Delete("SteamForgotten.json");
        }

        /// <summary>
        /// Clears images folder of contents, to re-download them on next load.
        /// </summary>
        /// <returns>Whether files were deleted or not</returns>
        public static async Task ClearImages()
        {
            Globals.DebugWriteLine(@"[Func:Steam\SteamSwitcherFuncs.ClearImages] Clearing images.");
            if (!Directory.Exists(Steam.SteamImagePath))
            {
                await GeneralInvocableFuncs.ShowToast("error", "Could not clear images", "Error", "toastarea"); 
            }
            foreach (var file in Directory.GetFiles(Steam.SteamImagePath))
            {
                File.Delete(file);
            }
            // Reload page, then display notification using a new thread.
            AppData.ActiveNavMan?.NavigateTo("/steam/?cacheReload&toast_type=success&toast_title=Success&toast_message=" + Uri.EscapeUriString("Cleared images"), true);
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
            var localConfigFilePath = Path.Join(Steam.FolderPath, "userdata", id32, "config", "localconfig.vdf");
            if (!File.Exists(localConfigFilePath)) return;
            var localConfigText = File.ReadAllText(localConfigFilePath); // Read relevant localconfig.vdf

            // Find index of range needing to be changed.
            var positionOfVar = localConfigText.IndexOf("ePersonaState", StringComparison.Ordinal); // Find where the variable is being set
            if (positionOfVar == -1) return;
            var indexOfBefore = localConfigText.IndexOf(":", positionOfVar, StringComparison.Ordinal) + 1; // Find where the start of the variable's value is
            var indexOfAfter = localConfigText.IndexOf(",", positionOfVar, StringComparison.Ordinal); // Find where the end of the variable's value is

            // The variable is now in-between the above numbers. Remove it and insert something different here.
            var sb = new StringBuilder(localConfigText);
            sb.Remove(indexOfBefore, indexOfAfter - indexOfBefore);
            sb.Insert(indexOfBefore, ePersonaState);
            localConfigText = sb.ToString();

            // Output
            File.WriteAllText(localConfigFilePath, localConfigText);
        }

        /// <summary>
        /// Returns string representation of Steam ePersonaState int
        /// </summary>
        /// <param name="ePersonaState">integer state to return string for</param>
        public static string PersonaStateToString(int ePersonaState)
        {
            return ePersonaState switch
            {
                -1 => "",
                0 => "Offline",
                1 => "Online",
                2 => "Busy",
                3 => "Away",
                4 => "Snooze",
                5 => "Looking to Trade",
                6 => "Looking to Play",
                7 => "Invisible",
                _ => "Unrecognised ePersonaState"
            };
        }
        #endregion

        #region STEAM_SETTINGS
        /* OTHER FUNCTIONS*/
        // STEAM SPECIFIC -- Move to a new file in the future.

        /// <summary>
        /// Used in JS. Gets whether forget account is enabled (Whether to NOT show prompt, or show it).
        /// </summary>
        /// <returns></returns>
        [JSInvokable]
        public static Task<bool> GetSteamForgetAcc() => Task.FromResult(Steam.ForgetAccountEnabled);

        /// <summary>
        /// Purely a class used for backing up forgotten Steam users, used in ForgetAccount() and RestoreAccount()
        /// </summary>
        public class ForgottenSteamuser
        {
            [JsonProperty("SteamId", Order = 0)] public string SteamId { get; set; }
            [JsonProperty("SteamUser", Order = 1)] public Index.Steamuser Steamuser { get; set; }
        }

        /// <summary>
        /// Remove requested account from loginusers.vdf
        /// </summary>
        /// <param name="steamId">SteamId of account to be removed</param>
        public static bool ForgetAccount(string steamId)
        {
            Globals.DebugWriteLine($@"[Func:Steam\SteamSwitcherFuncs.ForgetAccount] Forgetting account: {steamId.Substring(steamId.Length - 4, 4)}");
            // Load and remove account that matches SteamID above.
            var userAccounts = GetSteamUsers(Steam.LoginUsersVdf());
            var forgottenUser = userAccounts.First(x => x.SteamId == steamId); // Get the removed user to save into restore file
            userAccounts.RemoveAll(x => x.SteamId == steamId);

            // Instead of backing up EVERY TIME like the previous version (Was: BackupLoginUsers(settings: settings);)
            // Rather just save the users in a file, for better restoring later if necessary.
            var fFileContents = File.Exists(Steam.ForgottenFile) ? File.ReadAllText(Steam.ForgottenFile) : "";
            var fUsers = fFileContents == "" ? new List<ForgottenSteamuser>() : JsonConvert.DeserializeObject<List<ForgottenSteamuser>>(fFileContents);
            if (fUsers!.All(x => x.SteamId != forgottenUser.SteamId)) fUsers.Add(new ForgottenSteamuser { SteamId = forgottenUser.SteamId, Steamuser = forgottenUser }); // Add to list if user with SteamID doesn't exist in it.
            File.WriteAllText(Steam.ForgottenFile, JsonConvert.SerializeObject(fUsers));

            // Save updated loginusers.vdf file
            SaveSteamUsersIntoVdf(userAccounts);

            // Remove image
            var img = $"{Steam.SteamImagePathHtml}{steamId}.jpg";
            if (File.Exists(img)) File.Delete(img);

            // Remove from Tray
            Globals.RemoveTrayUserByArg("Steam", "+s:" + steamId);
            return true;
        }

        /// <summary>
        /// Restores requested SteamIds: Moves them from ForgottenFile, back into loginusers.vdf
        /// </summary>
        /// <param name="requestedSteamIds">List of SteamID64s (strings)</param>
        /// <returns>Whether the forgotten file even exists or not</returns>
        public static bool RestoreAccounts(string[] requestedSteamIds)
        {
            foreach (var s in requestedSteamIds) Globals.DebugWriteLine($@"[Func:Steam\SteamSwitcherFuncs.RestoreAccounts] Restoring account: {s.Substring(s.Length - 4, 4)}");
            
            if (!File.Exists(Steam.ForgottenFile)) return false;
            var forgottenAccounts = JsonConvert.DeserializeObject<List<ForgottenSteamuser>>(File.ReadAllText(Steam.ForgottenFile));
            if (forgottenAccounts == null) return false;
            // Load existing accounts
            var userAccounts = GetSteamUsers(Steam.LoginUsersVdf());
            // Create list of existing SteamIds (as to not add duplicates)
            var existingIds = userAccounts.Select(ua => ua.SteamId).ToList();

            var selectedForgottenPossibleDuplicates = forgottenAccounts.Where(fsu => requestedSteamIds.Contains(fsu.SteamId)).ToList(); // To remove items in Loginusers from forgotten list
            var selectedForgotten = selectedForgottenPossibleDuplicates.Where(fsu => !existingIds.Contains(fsu.SteamId)).ToList(); // To add new items to Loginusers (So there's no duplicates)
            foreach (var fa in selectedForgotten)
            {
                var su = fa.Steamuser;
                su.SteamId = fa.SteamId;
                userAccounts.Add(su);
            }
            
            // Save updated loginusers.vdf file
            SaveSteamUsersIntoVdf(userAccounts);

            // Update & Save SteamForgotten.json
            forgottenAccounts = forgottenAccounts.Except(selectedForgottenPossibleDuplicates).ToList();
            File.WriteAllText(Steam.ForgottenFile, JsonConvert.SerializeObject(forgottenAccounts));
            return true;
        }
        #endregion
    }
}

using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Globalization;
using System.IO;
using System.Linq;
using System.Net;
using System.Runtime.InteropServices;
using System.Threading;
using System.Threading.Tasks;
using System.Xml;
using Gameloop.Vdf;
using Gameloop.Vdf.JsonConverter;
using Microsoft.AspNetCore.Components;
using Microsoft.JSInterop;
using Microsoft.Win32;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Globals;

using Steamuser = TcNo_Acc_Switcher_Server.Pages.Steam.Index.Steamuser;

namespace TcNo_Acc_Switcher_Server.Pages.Steam
{
    public class SteamSwitcherFuncs
    {
        private const string SteamVacCacheFile = "profilecache/SteamVACCache.json";
        
        /// <summary>
        /// Main function for Steam Account Switcher. Run on load.
        /// Collects accounts from Steam's loginusers.vdf
        /// Prepares images and VAC/Limited status
        /// Prepares HTML Elements string for insertion into the account switcher GUI.
        /// </summary>
        /// <param name="jsRuntime"></param>
        /// <returns>Whether account loading is successful, or a path reset is needed (invalid dir saved)</returns>
        public static async ValueTask<bool> LoadProfiles(IJSRuntime jsRuntime)
        {
            Console.WriteLine("LOADING PROFILES!");
            var settings = GeneralFuncs.LoadSettings("SteamSettings");
            var steamPath = SteamSwitcherFuncs.LoginUsersVdf(settings);
            if (steamPath == "RESET_PATH") return false;
            var userAccounts = GetSteamUsers(steamPath); 
            var vacStatusList = new List<VacStatus>();
            var loadedVacCache = LoadVacInfo(ref vacStatusList);

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
                
                var extraClasses = (va.Vac ? " status_vac" : "") + (va.Ltd ? " status_limited" : "");

                var element =
                    $"<input type=\"radio\" id=\"{ua.AccName}\" class=\"acc\" name=\"accounts\" Username=\"{ua.AccName}\" SteamId64=\"{ua.SteamId}\" Line1=\"{ua.AccName}\" Line2=\"{ua.Name}\" Line3=\"{ua.LastLogin}\" ExtraClasses=\"{extraClasses}\" onchange=\"SelectedItemChanged()\" />\r\n" +
                    $"<label for=\"{ua.AccName}\" class=\"acc {extraClasses}\">\r\n" +
                    $"<img class=\"{extraClasses}\" src=\"{ua.ImgUrl}\" draggable=\"false\" />\r\n" +
                    $"<p>{ua.AccName}</p>\r\n" +
                    $"<h6>{ua.Name}</h6>\r\n" +
                    $"<p>{UnixTimeStampToDateTime(ua.LastLogin)}</p>\r\n</label>";

                await jsRuntime.InvokeVoidAsync("jQueryAppend", new object[] { "#acc_list", element });
            }

            SaveVacInfo(vacStatusList);
            await jsRuntime.InvokeVoidAsync("initContextMenu");

            return true;
        }

        /// <summary>
        /// Takes loginusers.vdf and iterates through each account, loading details into output Steamuser list.
        /// </summary>
        /// <param name="loginUserPath">loginusers.vdf path</param>
        /// <returns>List of Steamuser classes, from loginusers.vdf</returns>
        public static List<Steamuser> GetSteamUsers(string loginUserPath)
        {
            var userAccounts = new List<Steamuser>();

            userAccounts.Clear();
            Directory.CreateDirectory("wwwroot/img/profiles");
            try
            {
                var loginUsersVToken = VdfConvert.Deserialize(File.ReadAllText(loginUserPath));
                var loginUsers = new JObject() { loginUsersVToken.ToJson() };

                if (loginUsers["users"] != null)
                {
                    userAccounts.AddRange(from user in loginUsers["users"]
                    let steamId = user.ToObject<JProperty>()?.Name
                    where !string.IsNullOrEmpty(steamId) && !string.IsNullOrEmpty(user.First?["AccountName"]?.ToString())
                    select new Steamuser()
                    {
                        Name = user.First?["PersonaName"]?.ToString(),
                        AccName = user.First?["AccountName"]?.ToString(),
                        SteamId = steamId,
                        ImgUrl = "img/QuestionMark.jpg",
                        LastLogin = user.First?["Timestamp"]?.ToString(),
                        OfflineMode = (!string.IsNullOrEmpty(user.First?["WantsOfflineMode"]?.ToString()) ? user.First?["WantsOfflineMode"]?.ToString() : "0")
                    });
                }
            }
            catch (FileNotFoundException ex)
            {
                //MessageBox.Show(Strings.ErrLoginusersNonExist, Strings.ErrLoginusersNonExistHeader, MessageBoxButton.OK, MessageBoxImage.Error);
                //MessageBox.Show($"{Strings.ErrInformation} {ex}", Strings.ErrLoginusersNonExistHeader, MessageBoxButton.OK, MessageBoxImage.Error);
                Environment.Exit(2);
            }

            return userAccounts;
        }

        /// <summary>
        /// Deletes cached VAC/Limited status file
        /// </summary>
        /// <returns>Whether deletion successful</returns>
        public static bool DeleteVacCacheFile()
        {
            if (!File.Exists(SteamVacCacheFile)) return true;
            File.Delete(SteamVacCacheFile);
            return true;
        }

        /// <summary>
        /// Loads List of VacStatus classes into input cache from file, or deletes if outdated.
        /// </summary>
        /// <param name="vsl">Reference to List of VacStatus</param>
        /// <returns>Whether file was loaded. False if deleted ~ failed to load.</returns>
        public static bool LoadVacInfo(ref List<VacStatus> vsl)
        {
            GeneralFuncs.DeletedOutdatedFile(SteamVacCacheFile);
            if (!File.Exists(SteamVacCacheFile)) return false;
            vsl = JsonConvert.DeserializeObject<List<VacStatus>>(File.ReadAllText(SteamVacCacheFile));
            return true;
        }

        /// <summary>
        /// Saves List of VacStatus into cache file as JSON.
        /// </summary>
        public static void SaveVacInfo(List<VacStatus> vsList) => File.WriteAllText(SteamVacCacheFile, JsonConvert.SerializeObject(vsList));

        /// <summary>
        /// Converts Unix Timestamp string to DateTime
        /// </summary>
        public static string UnixTimeStampToDateTime(string stringUnixTimeStamp)
        {
            double.TryParse(stringUnixTimeStamp, out var unixTimeStamp);
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
        private static VacStatus PrepareProfileImage(Steamuser su)
        { 
            var dlDir = $"wwwroot/img/profiles/{su.SteamId}.jpg";
            // Delete outdated file, if it exists
            GeneralFuncs.DeletedOutdatedFile(dlDir);
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
                    su.ImgUrl = $"img/profiles/{su.SteamId}.jpg";
                }
                catch (WebException ex)
                {
                    if (ex.HResult != -2146233079) // Ignore currently in use error, for when program is still writing to file.
                    {
                        su.ImgUrl = "img/QuestionMark.jpg";
                        Console.WriteLine("ERROR: Could not connect and download Steam profile's image from Steam servers.\nCheck your internet connection.\n\nDetails: " + ex);
                        //MessageBox.Show($"{Strings.ErrImageDownloadFail} {ex}", Strings.ErrProfileImageDlFail, MessageBoxButton.OK, MessageBoxImage.Error);
                    }
                }
            }
            else
            {
                su.ImgUrl = $"img/profiles/{su.SteamId}.jpg";
                var profileXml = new XmlDocument();
                var cachedFile = $"profilecache/{su.SteamId}.xml";
                profileXml.Load((File.Exists(cachedFile))? cachedFile : $"https://steamcommunity.com/profiles/{su.SteamId}?xml=1");
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
        private static string GetUserImageUrl(ref VacStatus vs, Steamuser su)
        {
            var imageUrl = "";
            var profileXml = new XmlDocument();
            try
            {
                profileXml.Load($"https://steamcommunity.com/profiles/{su.SteamId}?xml=1");
                // Cache for later
                Directory.CreateDirectory("profilecache");
                profileXml.Save($"profilecache/{su.SteamId}.xml");

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
                // TODO: Is this necessary? Catch errors from the whole project later in crash handler?
                imageUrl = "";
                Directory.CreateDirectory("Errors");
                using (var sw = File.AppendText($"Errors\\AccSwitcher-Error-{DateTime.Now:dd-MM-yy_hh-mm-ss.fff}.txt"))
                {
                    sw.WriteLine(DateTime.Now.ToString(CultureInfo.InvariantCulture) + "\t" + /*Strings.ErrUnhandledCrash +*/ "Unhandled error crash: " + e + Environment.NewLine + Environment.NewLine);
                }
                using (var sw = File.AppendText($"Errors\\AccSwitcher-Error-{DateTime.Now:dd-MM-yy_hh-mm-ss.fff}.txt"))
                {
                    sw.WriteLine(JsonConvert.SerializeObject(profileXml));
                }
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
            if (profileXml.DocumentElement == null) return;
            try
            {
                if (profileXml.DocumentElement.SelectNodes("/profile/vacBanned")?[0] != null)
                    vs.Vac = profileXml.DocumentElement.SelectNodes("/profile/vacBanned")?[0].InnerText == "1";
                if (profileXml.DocumentElement.SelectNodes("/profile/isLimitedAccount")?[0] != null)
                    vs.Ltd = profileXml.DocumentElement.SelectNodes("/profile/isLimitedAccount")?[0].InnerText == "1";
            }
            catch (NullReferenceException) { }
        }

        /// <summary>
        /// Restart Steam with a new account selected. Leave args empty to log into a new account.
        /// </summary>
        /// <param name="steamId">(Optional) User's SteamID</param>
        /// <param name="accName">(Optional) User's login username</param>
        /// <param name="autoStartSteam">(Optional) Whether Steam should start after switching [Default: true]</param>
        public static void SwapSteamAccounts(string steamId = "", string accName = "", bool autoStartSteam = true)
        {
            JObject settings = GeneralFuncs.LoadSettings("SteamSettings");
            if (steamId != "" && !VerifySteamId(steamId))
            {
                // await JsRuntime.InvokeVoidAsync("createAlert", "Invalid SteamID" + steamid);
                return;
            }

            CloseSteam();
            UpdateLoginUsers(settings, steamId, accName);

            if (!autoStartSteam) return;
            if ((bool)settings["Steam_Admin"])
                Process.Start(SteamSwitcherFuncs.SteamExe(settings));
            else
                Process.Start(new ProcessStartInfo("explorer.exe", SteamExe(settings)));
        }
        
        /// <summary>
        /// Verify whether input Steam64ID is valid or not
        /// </summary>
        public static bool VerifySteamId(string steamId)
        {
            const long steamIdMin = 0x0110000100000001;
            const long steamIdMax = 0x01100001FFFFFFFF;
            if (!IsDigitsOnly(steamId) || steamId.Length != 17) return false;
            // Size check: https://stackoverflow.com/questions/33933705/steamid64-minimum-and-maximum-length#40810076
            var steamIdVal = double.Parse(steamId);
            return steamIdVal > steamIdMin && steamIdVal < steamIdMax;
        }
        private static bool IsDigitsOnly(string str) => str.All(c => c >= '0' && c <= '9');

        #region SteamManagement
        /// <summary>
        /// Kills Steam processes when run via cmd.exe
        /// </summary>
        public static void CloseSteam()
        {
            // This is what Administrator permissions are required for.
            var startInfo = new ProcessStartInfo
            {
                WindowStyle = ProcessWindowStyle.Hidden,
                FileName = "cmd.exe",
                Arguments = "/C TASKKILL /F /T /IM steam*"
            };
            var process = new Process { StartInfo = startInfo };
            process.Start();
            process.WaitForExit();
        }

        /// <summary>
        /// 
        /// </summary>
        /// <param name="settings"></param>
        /// <param name="loginNone">Whether it's a fresh account or not</param>
        /// <param name="selectedSteamId"></param>
        /// <param name="accName"></param>
        public static void UpdateLoginUsers(JObject settings, string selectedSteamId, string accName = "")
        {
            var userAccounts = SteamSwitcherFuncs.GetSteamUsers(SteamSwitcherFuncs.LoginUsersVdf(settings));
            // -----------------------------------
            // ----- Manage "loginusers.vdf" -----
            // -----------------------------------
            var tempFile = SteamSwitcherFuncs.LoginUsersVdf(settings) + "_temp";
            File.Delete(tempFile);

            var outJObject = new JObject();
            foreach (var ua in userAccounts)
            {
                if (ua.SteamId == selectedSteamId) ua.MostRec = "1";
                outJObject[ua.SteamId] = (JObject)JToken.FromObject(ua);
            }
            File.WriteAllText(tempFile, @"""users""" + Environment.NewLine + outJObject.ToVdf());
            File.Replace(tempFile, SteamSwitcherFuncs.LoginUsersVdf(settings), SteamSwitcherFuncs.LoginUsersVdf(settings) + "_last");

            // -----------------------------------
            // --------- Manage registry ---------
            // -----------------------------------
            /*
            ------------ Structure ------------
            HKEY_CURRENT_USER\Software\Valve\Steam\
                --> AutoLoginUser = username
                --> RememberPassword = 1
            */
            using var key = Registry.CurrentUser.CreateSubKey(@"Software\Valve\Steam");
            key.SetValue("AutoLoginUser", accName); // Account name is not set when changing user accounts from launch arguments (part of the viewmodel). -- Can be "" if no account
            key.SetValue("RememberPassword", 1);
        }

        /// <summary>
        /// Clears backups of forgotten accounts
        /// </summary>
        public static async void ClearForgottenBackups(IJSRuntime js = null)
        {
            await GeneralInvocableFuncs.ShowModal(js, "confirm:ClearSteamBackups:" + "Are you sure you want to clear backups of forgotten accounts?".Replace(' ', '_'));
            // Confirmed in GeneralInvocableFuncs.GiConfirmAction for rest of function
        }
        public static void ClearForgottenBackups_Confirmed() {
            var backupPath = SteamSwitcherFuncs.GetForgottenBackupPath();
            try
            {
                if (Directory.Exists(backupPath))
                    Directory.Delete(backupPath, true);
            }
            catch (Exception ex)
            {
                throw;
                // Handle this better in the future somehow
            }
        }
        #endregion

        #region Settings
        /// <summary>
        /// Default JSON Object containing the default config for SteamSettings.json
        /// </summary>
        public static JObject DefaultSettings_Steam()
        {
            return JObject.Parse(@"{
                ForgetAccountEnabled: false,
                Path: ""C:\\Program Files (x86)\\Steam\\"",
                WindowSize: ""800, 450"",
                Steam_Admin: false,
                Steam_ShowSteamID: false,
                Steam_ShowVAC: true,
                Steam_ShowLimited: false,
                Steam_DesktopShortcut: false,
                Steam_StartMenu: false,
                Steam_TrayStartup: false,
                Steam_TrayAccountName: false,
                Steam_ImageExpiryTime: ""7"",
                Steam_TrayAccNumber: ""3""
            }");
        }

        /* OTHER FUNCTIONS*/
        // STEAM SPECIFIC -- Move to a new file in the future.

        public static void ResetSettings_Steam()
        {
            GeneralFuncs.SaveSettings("SteamSettings", SteamSwitcherFuncs.DefaultSettings_Steam());
        }

        /// <summary>
        /// Clears images folder of contents, to re-download them on next load.
        /// </summary>
        /// <returns>Whether files were deleted or not</returns>
        public static bool ClearImages()
        {
            if (!Directory.Exists("wwwroot/img/profiles/")) return false;
            foreach (var file in Directory.GetFiles("wwwroot/img/profiles/"))
            {
                File.Delete(file);
            }
            return true;
        }

        /// <summary>
        /// Get path of loginusers.vdf
        /// </summary>
        /// <param name="settings">(Optional) Existing settings loaded into memory by another function</param>
        /// <returns>(Steam's path)\config\loginuisers.vdf</returns>
        public static string LoginUsersVdf(JObject settings = null)
        {
            GeneralFuncs.InitSettingsIfNull(ref settings, "SteamSettings");
            var path = Path.Combine(SteamFolder(settings), "config\\loginusers.vdf");
            if (!File.Exists(path))
            {
                ResetSteamFolder();
                return "RESET_PATH";
            }
            return Path.Combine(SteamFolder(settings), "config\\loginusers.vdf");
        }

        /// <summary>
        /// Get path of folder containing forgotten Steam accounts
        /// </summary>
        /// <param name="settings">(Optional) Existing settings loaded into memory by another function</param>
        /// <returns>(Steam's directory)\config\TcNo-Acc-Switcher-Backups\</returns>
        public static string GetForgottenBackupPath(JObject settings = null)
        {
            // TODO: Currently unused.
            GeneralFuncs.InitSettingsIfNull(ref settings, "SteamSettings");
            return Path.Combine(SteamFolder(settings), "config\\\\TcNo-Acc-Switcher-Backups\\\\");
        }

        /// <summary>
        /// Get Steam's config folder
        /// </summary>
        /// <param name="settings">(Optional) Existing settings loaded into memory by another function</param>
        /// <returns>(Steam's Path)\config\</returns>
        public static string SteamConfigFolder(JObject settings = null)
        {
            GeneralFuncs.InitSettingsIfNull(ref settings, "SteamSettings");
            return Path.Combine(SteamFolder(settings), "config\\");
        }

        /// <summary>
        /// Get Steam.exe path from SteamSettings.json 
        /// </summary>
        /// <param name="settings">(Optional) Existing settings loaded into memory by another function</param>
        /// <returns>Steam.exe's path string</returns>
        public static string SteamExe(JObject settings = null)
        {
            GeneralFuncs.InitSettingsIfNull(ref settings, "SteamSettings");
            return Path.Combine(SteamFolder(settings), "Steam.exe");
        }

        /// <summary>
        /// Get Steam Folder Path from SteamSettings.json
        /// </summary>
        /// <param name="settings">(Optional) Existing settings loaded into memory by another function</param>
        /// <returns>Steam's path string (containing steam.exe)</returns>
        public static string SteamFolder(JObject settings = null)
        {
            GeneralFuncs.InitSettingsIfNull(ref settings, "SteamSettings");
            return (string)settings["Path"];
        }

        /// <summary>
        /// Resets Steam Folder location in SteamSettings.json file.
        /// </summary>
        /// <param name="settings">(Optional) Existing settings loaded into memory by another function</param>
        public static void ResetSteamFolder(JObject settings = null)
        {
            GeneralFuncs.InitSettingsIfNull(ref settings, "SteamSettings");
            settings["Path"] = "";
            GeneralFuncs.SaveSettings("SteamSettings", settings);
            return;
        }

        /// <summary>
        /// Updatse the ForgetAccountEnabled bool in Steam settings file
        /// </summary>
        /// <param name="enabled">Whether will NOT prompt user if they're sure or not</param>
        /// <param name="settings">(Optional) Existing settings loaded into memory by another function</param>
        public static void UpdateSteamForgetAcc(bool enabled, JObject settings = null)
        {
            GeneralFuncs.InitSettingsIfNull(ref settings, "SteamSettings");
            if ((bool)settings["ForgetAccountEnabled"] == enabled) return; // Ignore if already set
            settings["ForgetAccountEnabled"] = enabled;
            GeneralFuncs.SaveSettings("SteamSettings", settings);
            return;
        }

        /// <summary>
        /// Creates a backup of the LoginUsers.vdf file
        /// </summary>
        /// <param name="backupName">(Optional) Name for the backup file (including .vdf)</param>
        /// <param name="settings">(Optional) Existing settings loaded into memory by another function</param>
        public static void BackupLoginUsers(string backupName = "", JObject settings = null)
        {
            GeneralFuncs.InitSettingsIfNull(ref settings, "SteamSettings");
            var steamFolder = SteamFolder(settings);
            var steamVdf = LoginUsersVdf(settings);

            var backup = Path.Combine(steamFolder, $"config\\TcNo-Acc-Switcher-Backups\\");
            var backupFileName = backupName != "" ? backupName : $"loginusers-{DateTime.Now:dd-MM-yyyy_HH-mm-ss.fff}.vdf";

            Directory.CreateDirectory(backup);
            File.Copy(steamVdf, Path.Combine(backup, backupFileName), true);
        }

        /// <summary>
        /// Remove requested account from loginusers.vdf
        /// </summary>
        /// <param name="steamId"></param>
        public static void ForgetAccount(string steamId)
        {
            JObject settings = GeneralFuncs.LoadSettings("SteamSettings");
            BackupLoginUsers(settings: settings);

            // Load and remove account that matches SteamID above.
            var steamPath = SteamSwitcherFuncs.LoginUsersVdf(settings);
            var userAccounts = GetSteamUsers(steamPath);
            userAccounts.RemoveAll(x => x.SteamId == steamId);

            // Convert list to JObject list, ready to save into vdf.
            var outJObject = new JObject();
            foreach (var ua in userAccounts)
            {
                outJObject[ua.SteamId] = (JObject)JToken.FromObject(ua);
            }

            // Write changes to files.
            var tempFile = steamPath + "_temp";
            File.WriteAllText(tempFile, @"""users""" + Environment.NewLine + outJObject.ToVdf());
            File.Replace(tempFile, steamPath, steamPath + "_last");

            // Refresh browser page, to show new list.

        }

        #endregion
    }
}

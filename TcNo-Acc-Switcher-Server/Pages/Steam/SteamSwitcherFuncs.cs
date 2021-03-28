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
using Microsoft.JSInterop;
using Microsoft.Win32;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Globals;

using Steamuser = TcNo_Acc_Switcher_Server.Pages.Index.Steamuser;

namespace TcNo_Acc_Switcher_Server.Pages.Steam
{
    public class SteamSwitcherFuncs
    {
        #region Profiles
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
                    foreach (var user in loginUsers["users"])
                    {
                        var steamId = user.ToObject<JProperty>()?.Name;
                        if (string.IsNullOrEmpty(steamId) || string.IsNullOrEmpty(user.First?["AccountName"]?.ToString())) continue;

                        var newSu = new Steamuser()
                        {
                            Name = user.First?["PersonaName"]?.ToString(),
                            AccName = user.First?["AccountName"]?.ToString(),
                            SteamId = steamId,
                            ImgUrl = "img/QuestionMark.jpg",
                            LastLogin = user.First?["Timestamp"]?.ToString(),
                            OfflineMode = (!string.IsNullOrEmpty(user.First?["WantsOfflineMode"]?.ToString()) ? user.First?["WantsOfflineMode"]?.ToString() : "0")
                        };
                        userAccounts.Add(newSu);
                    }
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


        private const string SteamVacCacheFile = "profilecache/SteamVACCache.json";

        public static bool LoadVacInfo(ref List<VacStatus> vsl)
        {
            GeneralFuncs.DeletedOutdatedFile(SteamVacCacheFile);
            if (File.Exists(SteamVacCacheFile))
            {
                vsl = JsonConvert.DeserializeObject<List<VacStatus>>(File.ReadAllText(SteamVacCacheFile));
                return true;
            }
            return false;
        }
        public static void SaveVacInfo(List<VacStatus> vsList) => File.WriteAllText(SteamVacCacheFile, JsonConvert.SerializeObject(vsList));

        //public static List<Steamuser> loadProfiles()
        public static async Task LoadProfiles(IJSRuntime jsRuntime)
        {
            Console.WriteLine("LOADING PROFILES!");
            JObject settings = GeneralFuncs.LoadSettings("SteamSettings");
            var userAccounts = GetSteamUsers(Path.Combine((string)settings["SteamFolder"], "config\\loginusers.vdf"));  //////////////// TO GET TO: CHECK IF NULL, ASK USER IN POPUP
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

                await jsRuntime.InvokeVoidAsync("jQueryAppend", new string[] { "#acc_list", element });
            }

            SaveVacInfo(vacStatusList);
            await jsRuntime.InvokeVoidAsync("initContextMenu");
            
            //return _userAccounts;
        }
        
        public static string UnixTimeStampToDateTime(string stringUnixTimeStamp)
        {
            double.TryParse(stringUnixTimeStamp, out var unixTimeStamp);
            // Unix timestamp is seconds past epoch
            var dtDateTime = new DateTime(1970, 1, 1, 0, 0, 0, 0, System.DateTimeKind.Utc);
            dtDateTime = dtDateTime.AddSeconds(unixTimeStamp).ToLocalTime();
            return dtDateTime.ToString(CultureInfo.InvariantCulture);
        }


        public class VacStatus
        {
            [JsonProperty("SteamID", Order = 0)] public string SteamId { get; set; }
            [JsonProperty("Vac", Order = 1)] public bool Vac { get; set; }
            [JsonProperty("Ltd", Order = 2)] public bool Ltd { get; set; }
        }

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
                if (!string.IsNullOrEmpty(imageUrl))
                {
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
            }
            else
            {
                su.ImgUrl = $"img/profiles/{su.SteamId}.jpg";
                var profileXml = new XmlDocument();
                var cachedFile = $"profilecache/{su.SteamId}.xml";
                profileXml.Load((File.Exists(cachedFile))? cachedFile : $"https://steamcommunity.com/profiles/{su.SteamId}?xml=1");
                if (!File.Exists(cachedFile)) profileXml.Save(cachedFile);

                if (profileXml.DocumentElement != null && profileXml.DocumentElement.SelectNodes("/profile/privacyMessage")?.Count == 0)
                {
                    try
                    {
                        var isVac = false;
                        var isLimited = true;
                        if (profileXml.DocumentElement != null)
                        {
                            if (profileXml.DocumentElement.SelectNodes("/profile/vacBanned")?[0] != null)
                                isVac = profileXml.DocumentElement.SelectNodes("/profile/vacBanned")?[0].InnerText == "1";
                            if (profileXml.DocumentElement.SelectNodes("/profile/isLimitedAccount")?[0] != null)
                                isLimited = profileXml.DocumentElement.SelectNodes("/profile/isLimitedAccount")?[0].InnerText == "1";
                        }

                        vs.Vac = isVac;
                        vs.Ltd = isLimited;
                    }
                    catch (NullReferenceException) { }
                }
            }

            return vs;
        }
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
                        var isVac = false;
                        var isLimited = true;
                        if (profileXml.DocumentElement != null)
                        {
                            if (profileXml.DocumentElement.SelectNodes("/profile/vacBanned")?[0] != null)
                                isVac = profileXml.DocumentElement.SelectNodes("/profile/vacBanned")?[0].InnerText == "1";
                            if (profileXml.DocumentElement.SelectNodes("/profile/isLimitedAccount")?[0] != null)
                                isLimited = profileXml.DocumentElement.SelectNodes("/profile/isLimitedAccount")?[0].InnerText == "1";
                        }

                        vs.Vac = isVac;
                        vs.Ltd = isLimited;
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


        #region SteamSwapper
        public static void SwapSteamAccounts(bool loginNone, string steamId, string accName, bool autoStartSteam = true)
        {
            JObject settings = GeneralFuncs.LoadSettings("SteamSettings");
            if (steamId != "" && !VerifySteamId(steamId))
            {
                // await JsRuntime.InvokeVoidAsync("createAlert", "Invalid SteamID" + steamid);
                return;
            }

            CloseSteam();
            UpdateLoginUsers(settings, loginNone, steamId, accName);

            if (!autoStartSteam) return;
            if ((bool)settings["Steam_Admin"])
                Process.Start((string)settings["SteamFolder"]);
            else
                Process.Start(new ProcessStartInfo("explorer.exe", (string)settings["SteamFolder"]));
        }

        public static void NewSteamLogin()
        {
            JObject settings = GeneralFuncs.LoadSettings("SteamSettings");
            // Kill Steam
            CloseSteam();
            // Set all accounts to 'not used last' status
            UpdateLoginUsers(settings, true, "", "");
            // Start Steam
            if ((bool)settings["Steam_Admin"])
                Process.Start(GeneralFuncs.SteamExe(settings));
            else
                Process.Start(new ProcessStartInfo("explorer.exe", GeneralFuncs.SteamExe(settings)));
            //LblStatus.Content = Strings.StatusStartedSteam;
        }


        #region Verification and Checks
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
        #endregion



        #region SteamManagement

        public static void CloseSteam()
        {
            // This is what Administrator permissions are required for.
            var startInfo = new ProcessStartInfo
            {
                WindowStyle = System.Diagnostics.ProcessWindowStyle.Hidden,
                FileName = "cmd.exe",
                Arguments = "/C TASKKILL /F /T /IM steam*"
            };
            var process = new Process { StartInfo = startInfo };
            process.Start();
            process.WaitForExit();
        }
        public static void UpdateLoginUsers(JObject settings, bool loginNone, string selectedSteamId, string accName)
        {
            var userAccounts = SteamSwitcherFuncs.GetSteamUsers(GeneralFuncs.LoginusersVDF(settings));
            // -----------------------------------
            // ----- Manage "loginusers.vdf" -----
            // -----------------------------------
            var targetUsername = accName;
            var tempFile = GeneralFuncs.LoginusersVDF(settings) + "_temp";
            File.Delete(tempFile);

            var outJObject = new JObject();
            foreach (var ua in userAccounts)
            {
                if (ua.SteamId == selectedSteamId) ua.MostRec = "1";
                outJObject[ua.SteamId] = (JObject)JToken.FromObject(ua);
            }
            File.WriteAllText(tempFile, @"""users""" + Environment.NewLine + outJObject.ToVdf());
            File.Replace(tempFile, GeneralFuncs.LoginusersVDF(settings), GeneralFuncs.LoginusersVDF(settings) + "_last");

            // -----------------------------------
            // --------- Manage registry ---------
            // -----------------------------------
            /*
            ------------ Structure ------------
            HKEY_CURRENT_USER\Software\Valve\Steam\
                --> AutoLoginUser = username
                --> RememberPassword = 1
            */
            ////////LblStatus.Content = Strings.StatusEditingRegistry; --------------------
            using var key = Registry.CurrentUser.CreateSubKey(@"Software\Valve\Steam");
            if (loginNone)
            {
                key.SetValue("AutoLoginUser", "");
                key.SetValue("RememberPassword", 1);
            }
            else
            {
                key.SetValue("AutoLoginUser", targetUsername); // Account name is not set when changing user accounts from launch arguments (part of the viewmodel).
                key.SetValue("RememberPassword", 1);
            }
        }

        #endregion
        #endregion


        #endregion

        #region Settings

        public static JObject DefaultSettings_Steam()
        {
            return JObject.Parse(@"{
                ForgetAccountEnabled: false,
                SteamFolder: ""C:\\Program Files (x86)\\Steam\\"",
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
        #endregion
    }
}

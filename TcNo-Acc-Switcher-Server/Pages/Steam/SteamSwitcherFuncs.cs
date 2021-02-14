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
using TcNo_Acc_Switcher.Pages.General;
using TcNo_Acc_Switcher_Globals;

using Steamuser = TcNo_Acc_Switcher.Pages.Index.Steamuser;
using UserSteamSettings = TcNo_Acc_Switcher.Pages.Index.UserSteamSettings;

namespace TcNo_Acc_Switcher.Pages.Steam
{
    public class SteamSwitcherFuncs
    {
        #region Profiles
        public static List<Steamuser> GetSteamUsers(string loginUserPath)
        {
            List<Steamuser> _userAccounts = new List<Steamuser>();
            
            _userAccounts.Clear();
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
                        double timestampLastLogin = 0;
                        Double.TryParse(user.First?["Timestamp"]?.ToString(), out timestampLastLogin);

                        Steamuser newSU = new Steamuser()
                        {
                            Name = user.First?["PersonaName"]?.ToString(),
                            AccName = user.First?["AccountName"]?.ToString(),
                            SteamID = steamId,
                            ImgURL = "img/QuestionMark.jpg",
                            lastLogin = UnixTimeStampToDateTime(timestampLastLogin).ToString(),
                            OfflineMode = (!string.IsNullOrEmpty(user.First?["WantsOfflineMode"]?.ToString()) ? user.First?["WantsOfflineMode"]?.ToString() : "0"),
                            ExtraClasses = ""
                        };
                        _userAccounts.Add(newSU);
                    }
                }
            }
            catch (FileNotFoundException ex)
            {
                //MessageBox.Show(Strings.ErrLoginusersNonExist, Strings.ErrLoginusersNonExistHeader, MessageBoxButton.OK, MessageBoxImage.Error);
                //MessageBox.Show($"{Strings.ErrInformation} {ex}", Strings.ErrLoginusersNonExistHeader, MessageBoxButton.OK, MessageBoxImage.Error);
                Environment.Exit(2);
            }

            return _userAccounts;
        }


        //public static List<Steamuser> loadProfiles()
        public static async Task LoadProfiles(UserSteamSettings _persistentSettings, IJSRuntime JsRuntime)
        {
            Console.WriteLine("LOADING PROFILES!");
            //await JsRuntime.InvokeVoidAsync("jQueryClearInner", "#acc_list");
            List<Steamuser> _userAccounts = GetSteamUsers(Path.Combine(_persistentSettings.SteamFolder, "config\\loginusers.vdf"));

            foreach (Steamuser ua in _userAccounts)
            {
                PrepareProfileImages(ua);

                string element =
                    $"<input type=\"radio\" id=\"{ua.AccName}\" class=\"acc\" name=\"accounts\" Username=\"{ua.AccName}\" SteamId64=\"{ua.SteamID}\" Line1=\"{ua.AccName}\" Line2=\"{ua.Name}\" Line3=\"{ua.lastLogin}\" ExtraClasses=\"{ua.ExtraClasses}\" onchange=\"SelectedItemChanged()\" />\r\n" +
                    $"<label for=\"{ua.AccName}\" class=\"acc @ExtraClasses\">\r\n" +
                    $"<img src=\"{ua.ImgURL}\" />\r\n" +
                    $"<p>{ua.AccName}</p>\r\n" +
                    $"<h6>{ua.Name}</h6>\r\n" +
                    $"<p>{ua.lastLogin}</p>\r\n</label>";

                await JsRuntime.InvokeVoidAsync("jQueryAppend", new string[] { "#acc_list", element });
            }
            
            await JsRuntime.InvokeVoidAsync("initContextMenu");
            
            //return _userAccounts;
        }
        
        public static DateTime UnixTimeStampToDateTime(double unixTimeStamp)
        {
            // Unix timestamp is seconds past epoch
            System.DateTime dtDateTime = new DateTime(1970, 1, 1, 0, 0, 0, 0, System.DateTimeKind.Utc);
            dtDateTime = dtDateTime.AddSeconds(unixTimeStamp).ToLocalTime();
            return dtDateTime;
        }

        private static void PrepareProfileImages(Steamuser su)
        { 
            var dlDir = $"wwwroot/img/profiles/{su.SteamID}.jpg";
            // Delete outdated file, if it exists
            GeneralFuncs.DeletedOutdatedImage(dlDir);
            // ... & invalid files
            GeneralFuncs.DeletedInvalidImage(dlDir);

            var temp = DateTime.Now.Subtract(File.GetLastWriteTime(dlDir)).Days;
            
            // Download new copy of the file
            if (!File.Exists(dlDir))
            {
                var imageUrl = GetUserImageUrl(su);
                if (!string.IsNullOrEmpty(imageUrl))
                {
                    try
                    {
                        using (var client = new WebClient())
                        {
                            client.DownloadFile(new Uri(imageUrl), dlDir);
                        }
                        su.ImgURL = $"img/profiles/{su.SteamID}.jpg";
                    }
                    catch (WebException ex)
                    {
                        if (ex.HResult != -2146233079) // Ignore currently in use error, for when program is still writing to file.
                        {
                            su.ImgURL = "img/QuestionMark.jpg";
                            Console.WriteLine("ERROR: Could not connect and download Steam profile's image from Steam servers.\nCheck your internet connection.\n\nDetails: " + ex);
                            //MessageBox.Show($"{Strings.ErrImageDownloadFail} {ex}", Strings.ErrProfileImageDlFail, MessageBoxButton.OK, MessageBoxImage.Error);
                        }
                    }
                }
            }
            else
            {
                su.ImgURL = $"img/profiles/{su.SteamID}.jpg";
            }
        }
        private static string GetUserImageUrl(Steamuser su)
        {
            var imageUrl = "";
            var profileXml = new XmlDocument();
            try
            {
                profileXml.Load($"https://steamcommunity.com/profiles/{su.SteamID}?xml=1");
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

                        su.ExtraClasses += (isVac ? "status_vac" : "") + (isLimited ? "status_limited" : "");
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
        public static async Task SwapSteamAccounts(bool loginNone, string steamId, string accName, bool autoStartSteam = true)
        {
            Index.UserSteamSettings _persistentSettings = SteamSwitcherFuncs.LoadSettings();
            if (!VerifySteamId(steamId))
            {
                // await JsRuntime.InvokeVoidAsync("createAlert", "Invalid SteamID" + steamid);
                return;
            }

            CloseSteam();
            UpdateLoginUsers(_persistentSettings, loginNone, steamId, accName);

            if (!autoStartSteam) return;
            if (_persistentSettings.StartAsAdmin)
                Process.Start(_persistentSettings.SteamExe());
            else
                Process.Start(new ProcessStartInfo("explorer.exe", _persistentSettings.SteamExe()));
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
        public static void UpdateLoginUsers(Index.UserSteamSettings _persistentSettings, bool loginNone, string selectedSteamId, string accName)
        {
            List<Index.Steamuser> _userAccounts = SteamSwitcherFuncs.GetSteamUsers(Path.Combine(_persistentSettings.SteamFolder, "config\\loginusers.vdf"));
            // -----------------------------------
            // ----- Manage "loginusers.vdf" -----
            // -----------------------------------
            var targetUsername = accName;
            var tempFile = _persistentSettings.LoginusersVdf() + "_temp";
            File.Delete(tempFile);

            var outJObject = new JObject();
            foreach (var ua in _userAccounts)
            {
                if (ua.SteamID == selectedSteamId) ua.MostRec = "1";
                outJObject[ua.SteamID] = (JObject)JToken.FromObject(ua);
            }
            File.WriteAllText(tempFile, @"""users""" + Environment.NewLine + outJObject.ToVdf());
            File.Replace(tempFile, _persistentSettings.LoginusersVdf(), _persistentSettings.LoginusersVdf() + "_last");

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
            using (var key = Registry.CurrentUser.CreateSubKey(@"Software\Valve\Steam"))
            {
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
        }

        #endregion
        #endregion


        #endregion

        #region Settings

        public static void SaveSettings(UserSteamSettings _persistentSettings)
        {
            //_persistentSettings.WindowSize = new Size(this.Width, this.Height);
            var serializer = new JsonSerializer() { NullValueHandling = NullValueHandling.Ignore };

            using (var sw = new StreamWriter(@"SteamSettings.json"))
            using (JsonWriter writer = new JsonTextWriter(sw))
            {
                serializer.Serialize(writer, _persistentSettings);
            }
        }
        public static UserSteamSettings LoadSettings()
        {
            UserSteamSettings _persistentSettings = new UserSteamSettings();
            if (!File.Exists("SteamSettings.json")) { SaveSettings(_persistentSettings);
                return _persistentSettings;
            }

            using (var sr = new StreamReader(@"SteamSettings.json"))
            {
                // persistentSettings = JsonConvert.DeserializeObject<UserSettings>(sr.ReadToEnd()); -- Entirely replaces, instead of merging. New variables won't have values.
                // Using a JSON UnionUnion Merge means that settings that are missing will have default values, set at the top of this file.
                var jCurrent = JObject.Parse(JsonConvert.SerializeObject(_persistentSettings));
                try
                {
                    jCurrent.Merge(JObject.Parse(sr.ReadToEnd()), new JsonMergeSettings
                    {
                        MergeArrayHandling = MergeArrayHandling.Union
                    });
                    _persistentSettings = jCurrent.ToObject<UserSteamSettings>();
                }
                catch (Exception)
                {
                    if (File.Exists("SteamSettings.json"))
                    {
                        if (File.Exists("SteamSettings.old.json"))
                            File.Delete("SteamSettings.old.json");
                        File.Copy("SteamSettings.json", "SteamSettings.old.json");
                    }

                    SaveSettings(_persistentSettings);
                    //MessageBox.Show(Strings.ErrSteamSettingsLoadFail, Strings.ErrSteamSettingsLoadFailHeader, MessageBoxButton.OK, MessageBoxImage.Error);
                }
            }
            return _persistentSettings;
        }
        #endregion
    }
}

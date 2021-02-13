using System;
using System.Collections.Generic;
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
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher.Pages.General;
using TcNo_Acc_Switcher_Globals;

using Steamuser = TcNo_Acc_Switcher.Pages.Index.Steamuser;

namespace TcNo_Acc_Switcher.Pages.Steam
{
    public class SteamSwitcherFuncs
    {
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
        public static async Task LoadProfiles(IJSRuntime JsRuntime)
        {
            Console.WriteLine("LOADING PROFILES!");
            //await JsRuntime.InvokeVoidAsync("jQueryClearInner", "#acc_list");
            List<Steamuser> _userAccounts = GetSteamUsers("C:\\Program Files (x86)\\Steam\\config\\loginusers.vdf");

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
    }
}

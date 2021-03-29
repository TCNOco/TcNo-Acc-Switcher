using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Server.Pages.Steam;

namespace TcNo_Acc_Switcher_Server.Pages.General
{
    public class GeneralFuncs
    {
        public static bool DeletedOutdatedFile(string filename)
        {
            if (!File.Exists(filename)) return true;
            if (DateTime.Now.Subtract(File.GetLastWriteTime(filename)).Days <= 7) return false;
            File.Delete(filename);
            return true;
        }

        public static bool DeletedInvalidImage(string filename)
        {
            try
            {
                if (!IsValidGdiPlusImage(filename)) // Delete image if is not as valid, working image.
                {
                    File.Delete(filename);
                    return true;
                }
            }
            catch (Exception ex)
            {
                try
                {
                    File.Delete(filename);
                    return true;
                }
                catch (Exception)
                {
                    Console.WriteLine("Empty profile image detected (0 bytes). Can't delete to redownload.\nInfo: \n" + ex);
                    throw;
                }
            }

            return false;
        }
        private static bool IsValidGdiPlusImage(string filename)
        {
            //From https://stackoverflow.com/questions/8846654/read-image-and-determine-if-its-corrupt-c-sharp
            try
            {
                using (var bmp = new System.Drawing.Bitmap(filename))
                    return true;
            }
            catch
            {
                return false;
            }
        }

        #region SETTINGS

        public static void ResetSettings_Steam()
        {
            SaveSettings("SteamSettings", SteamSwitcherFuncs.DefaultSettings_Steam());
        }

        public static void SaveSettings(string file, JObject joNewSettings)
        {
            string sFilename = file + ".json";

            // Get existing settings
            JObject joSettings = new JObject();
            try
            {
                joSettings = JObject.Parse(File.ReadAllText(sFilename));
            }
            catch (Exception e)
            {
                Console.WriteLine(e);
            }

            // Merge existing settings with settings from site
            joSettings.Merge(joNewSettings, new JsonMergeSettings
            {
                MergeArrayHandling = MergeArrayHandling.Union
            });

            // Save all settings back into file
            File.WriteAllText(sFilename, joSettings.ToString());
        }

        public static JObject LoadSettings(string file)
        {
            string sFilename = file + ".json";
            if (File.Exists(sFilename))
            {
                try
                {
                    return JObject.Parse(File.ReadAllText(sFilename));
                }
                catch (Exception e)
                {
                    Console.WriteLine(e);
                }
            }

            switch (file)
            {
                case "SteamSettings":
                    return SteamSwitcherFuncs.DefaultSettings_Steam();
                case "WindowSettings":
                    return DefaultSettings();
            }

            return new JObject();
        }
        public static JObject DefaultSettings() => JObject.Parse(@"WindowSize: ""800, 450""");

        public static void InitSettingsIfNull(ref JObject settings, string settingsType) => settings ??= GeneralFuncs.LoadSettings(settingsType);

        #endregion
    }
}

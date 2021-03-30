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
        /// <summary>
        /// Checks if input file is older than 7 days, then deletes if it is
        /// </summary>
        /// <param name="filename">File path to be checked, and possibly deleted</param>
        /// <returns>Whether file was deleted or not (Outdated or not)</returns>
        public static bool DeletedOutdatedFile(string filename)
        {
            if (!File.Exists(filename)) return true;
            if (DateTime.Now.Subtract(File.GetLastWriteTime(filename)).Days <= 7) return false;
            File.Delete(filename);
            return true;
        }

        /// <summary>
        /// Checks if images is a valid GDI+ image, deleted if not.
        /// </summary>
        /// <param name="filename">File path of image to be checked</param>
        /// <returns>Whether file was deleted, or file was not deleted and was valid</returns>
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

        /// <summary>
        /// Checks if image is a valid GDI+ image
        /// </summary>
        /// <param name="filename">File path of image to be checked</param>
        /// <returns>Whether image is a valid file or not</returns>
        private static bool IsValidGdiPlusImage(string filename)
        {
            //From https://stackoverflow.com/questions/8846654/read-image-and-determine-if-its-corrupt-c-sharp
            try
            {
                using var bmp = new System.Drawing.Bitmap(filename);
                return true;
            }
            catch
            {
                return false;
            }
        }

        #region SETTINGS
        /// <summary>
        /// Saves input JObject of settings to input file path
        /// </summary>
        /// <param name="file">File path to save JSON string to</param>
        /// <param name="joNewSettings">JObject of settings to be saved</param>
        public static void SaveSettings(string file, JObject joNewSettings)
        {
            var sFilename = file + ".json";

            // Get existing settings
            var joSettings = new JObject();
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

        /// <summary>
        /// Loads settings from input file (JSON string to JObject)
        /// </summary>
        /// <param name="file">JSON file to be read</param>
        /// <returns>JObject created from file</returns>
        public static JObject LoadSettings(string file)
        {
            var sFilename = file + ".json";
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

            return file switch
            {
                "SteamSettings" => SteamSwitcherFuncs.DefaultSettings_Steam(),
                "WindowSettings" => DefaultSettings(),
                _ => new JObject()
            };
        }

        /// <summary>
        /// Returns default settings for program (Just the Window size currently)
        /// </summary>
        public static JObject DefaultSettings() => JObject.Parse(@"WindowSize: ""800, 450""");

        /// <summary>
        /// If input settings from another function are null, load settings from file OR default settings for [Platform]
        /// </summary>
        /// <param name="settings">JObject of settings to be initialized if null</param>
        /// <param name="file">"[Platform]Settings" to be used later in reading file, and setting default if can't</param>
        public static void InitSettingsIfNull(ref JObject settings, string file) => settings ??= GeneralFuncs.LoadSettings(file);

        #endregion
    }
}

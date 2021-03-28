using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using Microsoft.JSInterop;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;

namespace TcNo_Acc_Switcher_Server.Pages.General
{
    public class GeneralInvocableFuncs
    {
        [JSInvokable]
        public static async void GISaveSettings(string file, string jsonString)
        {
            string sFilename = file + ".json";

            // Get existing settings
            JObject joSettings = new JObject();
            try
            {
                joSettings = JObject.Parse(await File.ReadAllTextAsync(sFilename));
            }
            catch (Exception e)
            {
                Console.WriteLine(e);
            }
            JObject joNewSettings = JObject.Parse(jsonString);

            // Merge existing settings with settings from site
            joSettings.Merge(joNewSettings, new JsonMergeSettings
            {
                MergeArrayHandling = MergeArrayHandling.Union
            });

            // Save all settings back into file
            await File.WriteAllTextAsync(sFilename, joSettings.ToString());
        }

        [JSInvokable]
        public static Task GILoadSettings(string file)
        {
            string sFilename = file + ".json";
            string output = "";
            try
            {
                output = File.ReadAllText(sFilename);
            }
            catch (Exception e)
            {
                Console.WriteLine(e);
            }
            return Task.FromResult(output);
        }
    }
}

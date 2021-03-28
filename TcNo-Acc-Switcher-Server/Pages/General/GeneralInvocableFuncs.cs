using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using Microsoft.JSInterop;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Server.Pages.Steam;

using Microsoft.Win32;
using System.Windows;


namespace TcNo_Acc_Switcher_Server.Pages.General
{
    public class GeneralInvocableFuncs
    {
        [JSInvokable]
        public static void GiSaveSettings(string file, string jsonString)
        {
            GeneralFuncs.SaveSettings(file, JObject.Parse(jsonString));
        }
        
        [JSInvokable]
        public static Task GiLoadSettings(string file)
        {
            return Task.FromResult(GeneralFuncs.LoadSettings(file).ToString());
        }

        [JSInvokable]
        public static void GiUpdatePath(string file, string path)
        {
            JObject settings = GeneralFuncs.LoadSettings(file);
            settings["Path"] = path;
            GeneralFuncs.SaveSettings(file, settings);
        }
    }
}

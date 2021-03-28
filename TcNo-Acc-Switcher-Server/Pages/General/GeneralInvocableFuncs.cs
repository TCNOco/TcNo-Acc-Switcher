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

namespace TcNo_Acc_Switcher_Server.Pages.General
{
    public class GeneralInvocableFuncs
    {
        [JSInvokable]
        public static async void GISaveSettings(string file, string jsonString)
        {
            GeneralFuncs.SaveSettings(file, JObject.Parse(jsonString));
        }
        
        [JSInvokable]
        public static Task GILoadSettings(string file)
        {
            return Task.FromResult(GeneralFuncs.LoadSettings(file).ToString());
        }
    }
}

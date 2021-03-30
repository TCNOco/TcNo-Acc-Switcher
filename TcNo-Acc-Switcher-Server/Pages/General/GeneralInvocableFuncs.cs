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
using Microsoft.AspNetCore.Components;


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

        public static async Task ShowModal(IJSRuntime jsRuntime, string args)
        {
            await jsRuntime.InvokeAsync<string>("ShowModal", args);
        }
        public static async Task ShowToast(IJSRuntime jsRuntime, string toastType, string toastMessage, string toastTitle = "", string renderTo = "body", string position = "button-right", int duration = 5000)
        {
            //dynamic testD = new { type = toastType, title = toastTitle, message = toastMessage };
            await jsRuntime.InvokeVoidAsync($"window.notification.new", new { type = toastType, title = toastTitle, message = toastMessage, renderTo = renderTo, duration = duration, position = position });
        }
    }
}

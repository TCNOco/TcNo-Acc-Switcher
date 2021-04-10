// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2021 TechNobo (Wesley Pyburn)
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

using System;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.ComponentModel;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Text;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Data;
using System.Windows.Documents;
using System.Windows.Input;
using System.Windows.Media;
using System.Windows.Media.Imaging;
using System.Windows.Navigation;
using System.Windows.Shapes;
using System.Threading;
using System.Windows.Interop;
using Microsoft.Web.WebView2.Core;
using Microsoft.Web.WebView2.Wpf;
using Microsoft.Win32;
using Microsoft.Win32.TaskScheduler;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Server.Pages.Steam;
using TcNo_Acc_Switcher_Server.Shared;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server;
using TcNo_Acc_Switcher_Server.Pages.General;
using Index = TcNo_Acc_Switcher_Server.Pages.Index;
using Path = System.IO.Path;
using Point = System.Windows.Point;
using Strings = TcNo_Acc_Switcher_Client.Localisation.Strings;

namespace TcNo_Acc_Switcher_Client
{
    /// <summary>
    /// Interaction logic for MainWindow.xaml
    /// </summary>

    public partial class MainWindow : Window
    {
        private readonly Thread _server = new Thread(RunServer);
        private static void RunServer() { Program.Main(new string[0]); }
        public static readonly TcNo_Acc_Switcher_Server.Data.AppSettings AppSettings = TcNo_Acc_Switcher_Server.Data.AppSettings.Instance;


        public MainWindow()
        {
            // Start web server
            _server.IsBackground = true;
            _server.Start();
            
            // Initialise and connect to web server above
            // Somehow check ports and find a different one if it doesn't work? We'll see...
            InitializeComponent();

            AppSettings.LoadFromFile();
            //MainBackground.Background = (Brush)new BrushConverter().ConvertFromString(AppSettings.Stylesheet["headerbarBackground"]);

            //MView2.Source = new Uri("http://localhost:44305/");
            MView2.Source = new Uri("http://localhost:5000/" + App.StartPage);
            MView2.NavigationStarting += UrlChanged;
            MView2.CoreWebView2InitializationCompleted += WebView_CoreWebView2Ready;
            //MView2.MouseDown += MViewMDown;

            this.Width = AppSettings.WindowSize.X;
            this.Height = AppSettings.WindowSize.Y;
            StateChanged += WindowStateChange;
            // Each window in the program would have its own size. IE Resize for Steam, and more.
        }
        
        // For draggable regions:
        // https://github.com/MicrosoftEdge/WebView2Feedback/issues/200
        private void WebView_CoreWebView2Ready(object sender, EventArgs e)
        {
            var eventForwarder = new Headerbar.EventForwarder(new WindowInteropHelper(this).Handle);

            MView2.CoreWebView2.AddHostObjectToScript("eventForwarder", eventForwarder);
        }

        private void WindowStateChange(object sender, EventArgs e)
        {
            var state = "";
            switch (WindowState)
            {
                case WindowState.Maximized:
                    state = "add";
                    break;
                case WindowState.Normal:
                    state = "remove";
                    break;
            }
            MView2.ExecuteScriptAsync("document.body.classList." + state + "('maximized')");
        }

        protected override void OnClosing(CancelEventArgs e)
        {
            SaveSettings(MView2.Source.AbsolutePath);
        }

        private void SaveSettings(string windowUrl)
        {
            // IN THE FUTURE: ONLY DO THIS FOR THE MAIN PAGE WHERE YOU CAN CHOOSE WHAT PLATFORM TO SWAP ACCOUNTS ON
            // This will only be when that's implemented. Easier to leave it until then.
            MessageBox.Show(windowUrl);
            AppSettings.WindowSize = new System.Drawing.Point(){ X = Convert.ToInt32(this.Width), Y = Convert.ToInt32(this.Height) };
            AppSettings.SaveSettings();
        }

        private void UrlChanged(object sender, CoreWebView2NavigationStartingEventArgs args)
        {
            var uri = args.Uri.Split("/").Last();
            Console.WriteLine(args.Uri);

            if (!args.Uri.Contains("?")) return;
            // Needs to be here as:
            // Importing Microsoft.Win32 and System.Windows didn't get OpenFileDialog to work.
            var uriArg = args.Uri.Split("?").Last();
            if (!uriArg.StartsWith("selectFile")) return;
            args.Cancel = true;
            var argValue = uriArg.Split("=")[1];
            var dlg = new OpenFileDialog
            {
                FileName = Path.GetFileNameWithoutExtension(argValue),
                DefaultExt = Path.GetExtension(argValue),
                Filter = $"{argValue}|{argValue}"
            };

            var result = dlg.ShowDialog();
            if (result != true) return;
            MView2.ExecuteScriptAsync("Modal_RequestedLocated(true)");
            MView2.ExecuteScriptAsync("Modal_SetFilepath(" + JsonConvert.SerializeObject(dlg.FileName.Substring(0, dlg.FileName.LastIndexOf('\\'))) + ")");
            //VerifySteamPath();

        }
        public static async Task<string> ExecuteScriptFunctionAsync(WebView2 webView2, string functionName, params object[] parameters)
        {
            string script = functionName + "(";
            for (int i = 0; i < parameters.Length; i++)
            {
                script += JsonConvert.SerializeObject(parameters[i]);
                if (i < parameters.Length - 1)
                {
                    script += ", ";
                }
            }
            script += ");";
            return await webView2.ExecuteScriptAsync(script);
        }
    }
}

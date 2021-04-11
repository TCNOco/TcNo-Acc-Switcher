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
using System.Configuration;
using System.Data;
using System.IO;
using System.Linq;
using System.Runtime.InteropServices;
using System.Threading;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Input;
using System.Windows.Shapes;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using Path = System.IO.Path;

namespace TcNo_Acc_Switcher_Client
{
    /// <summary>
    /// Interaction logic for App.xaml
    /// </summary>
    public partial class App : Application
    {
        private readonly Dictionary<string, List<TrayUser>> TrayUsers = new();
        public static string StartPage = "";


        internal static class NativeMethods
        {
            // http://msdn.microsoft.com/en-us/library/ms681944(VS.85).aspx
            // See: http://www.codeproject.com/tips/68979/Attaching-a-Console-to-a-WinForms-application.aspx
            // And: https://stackoverflow.com/questions/2669463/console-writeline-does-not-show-up-in-output-window/2669596#2669596
            [DllImport("kernel32.dll", SetLastError = true)]
            internal static extern int AllocConsole();
            [DllImport("kernel32.dll", SetLastError = true)]
            internal static extern int FreeConsole();
        }

        protected override void OnExit(ExitEventArgs e)
        {
            NativeMethods.FreeConsole();
            TcNo_Acc_Switcher_Client.MainWindow.KillServer();
            mutex.ReleaseMutex();
        }

        public bool debugMode = true;

        static Mutex mutex = new Mutex(true, "{A240C23D-6F45-4E92-9979-11E6CE10A22C}");
        [STAThread]
        protected override void OnStartup(StartupEventArgs e)
        {
            // Single instance:
            if (!mutex.WaitOne(TimeSpan.Zero, true))
            {
                MessageBox.Show("Another TcNo Account Switcher instance has been detected.");
                Environment.Exit(1056); // 1056	An instance of the service is already running.
            }


            #if DEBUG
            if (debugMode)
            {
                NativeMethods.AllocConsole();
                Console.WriteLine("Debug Console started");
            }
            #endif
            
            base.OnStartup(e);
            var quitArg = false;
            for (var i = 0; i != e.Args.Length; ++i)
            {
                if (e.Args[i]?[0] == '+')
                {
                    var command = e.Args[i].Substring(1); // Drop '+'
                    var platform = command.Split(':')[0];
                    var account = command.Split(':')[1];
                    
                    switch (platform)
                    {
                        case "s": // Steam
                            // Steam format: +s:<steamId>[:<PersonaState (0-7)>]
                            if (!account.Contains(":"))
                                TcNo_Acc_Switcher_Server.Pages.Steam.SteamSwitcherFuncs.SwapSteamAccounts(account);
                            else TcNo_Acc_Switcher_Server.Pages.Steam.SteamSwitcherFuncs.SwapSteamAccounts(
                                    account.Split(":")[0], ePersonaState: int.Parse(account.Split(":")[1])); // Request has a PersonaState in it
                            break;
                    }

                    quitArg = true;
                }
                else switch (e.Args[i])
                {
                    case "steam":
                        StartPage = "Steam";
                        break;
                    case "logout":
                        TcNo_Acc_Switcher_Server.Pages.Steam.SteamSwitcherFuncs.SwapSteamAccounts("", "");
                        quitArg = true;
                        break;
                    case "quit":
                        quitArg = true;
                        break;
                    default:
                        Console.WriteLine($"Unknown argument: \"{e.Args[i]}\"");
                        break;
                }
            }

            if (quitArg)
                Environment.Exit(1);
        }

        #region ResizeWindows
        // https://stackoverflow.com/a/27157947/5165437
        bool _resizeInProcess = false;
        private void Resize_Init(object sender, MouseButtonEventArgs e)
        {
            var senderRect = sender as Rectangle;
            if (senderRect != null)
            {
                _resizeInProcess = true;
                senderRect.CaptureMouse();
            }
        }

        private void Resize_End(object sender, MouseButtonEventArgs e)
        {
            var senderRect = sender as Rectangle;
            if (senderRect != null)
            {
                _resizeInProcess = false; ;
                senderRect.ReleaseMouseCapture();
            }
        }

        private void Resizeing_Form(object sender, MouseEventArgs e)
        {
            if (_resizeInProcess)
            {
                var senderRect = sender as Rectangle;
                var mainWindow = senderRect.Tag as Window;
                if (senderRect != null)
                {
                    var width = e.GetPosition(mainWindow).X;
                    var height = e.GetPosition(mainWindow).Y;
                    senderRect.CaptureMouse();
                    if (senderRect.Name.ToLower().Contains("right"))
                    {
                        width += 5;
                        if (width > 0)
                            mainWindow.Width = width;
                    }
                    if (senderRect.Name.ToLower().Contains("left"))
                    {
                        width -= 5;
                        mainWindow.Left += width;
                        width = mainWindow.Width - width;
                        if (width > 0)
                        {
                            mainWindow.Width = width;
                        }
                    }
                    if (senderRect.Name.ToLower().Contains("bottom"))
                    {
                        height += 5;
                        if (height > 0)
                            mainWindow.Height = height;
                    }
                    if (senderRect.Name.ToLower().Contains("top"))
                    {
                        height -= 5;
                        mainWindow.Top += height;
                        height = mainWindow.Height - height;
                        if (height > 0)
                        {
                            mainWindow.Height = height;
                        }
                    }
                }
            }
        }
        #endregion
    }
}

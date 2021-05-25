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
using System.Threading;
using System.Windows;
using TcNo_Acc_Switcher_Globals;

namespace TcNo_Acc_Switcher_Updater
{
    /// <summary>
    /// Interaction logic for App.xaml
    /// </summary>
    public partial class App
    {
        static readonly Mutex Mutex = new(true, "{A240C23D-6F45-4E92-9979-11E6CE10A22C}");
        [STAThread]
        protected override void OnStartup(StartupEventArgs e)
        {
            // Crash handler
            AppDomain.CurrentDomain.UnhandledException += Globals.CurrentDomain_UnhandledException;
            // Single instance:
            if (!Mutex.WaitOne(TimeSpan.Zero, true))
            {
                MessageBox.Show("Another TcNo Account Switcher Updater instance has been detected.");
                Environment.Exit(1056); // 1056	An instance of the service is already running.
            }

            base.OnStartup(e);
            for (var i = 0; i != e.Args.Length; ++i)
            {
                switch (e.Args[i])
                {
                    case "verify":
                        TcNo_Acc_Switcher_Updater.MainWindow.VerifyAndClose = true;
                        break;
                    case "hashlist":
                        TcNo_Acc_Switcher_Updater.MainWindow.QueueHashList = true;
                        break;
                    case "createupdate":
                        TcNo_Acc_Switcher_Updater.MainWindow.QueueCreateUpdate = true;
                        break;
                    default:
                        Console.WriteLine($@"Unknown argument: ""{e.Args[i]}""");
                        break;
                }
            }
        }
    }
}

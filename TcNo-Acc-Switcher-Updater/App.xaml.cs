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
using System.Globalization;
using System.IO;
using System.Reflection;
using System.Threading;
using System.Windows;
using Newtonsoft.Json.Linq;

namespace TcNo_Acc_Switcher_Updater
{
    /// <summary>
    /// Interaction logic for App.xaml
    /// </summary>
    public partial class App
    {
        private static readonly Mutex Mutex = new(true, "{1523A82D-9AD3-4B01-B970-4F06633AD41C}");

        /// <summary>
        /// Shows error and exits program is program is already running
        /// </summary>
        private static void IsRunningAlready()
        {
	        try
	        {
		        if (Mutex.WaitOne(TimeSpan.Zero, true)) return;

		        // Otherwise: It has probably just closed. Wait a few and try again
		        Thread.Sleep(2000); // 2 seconds before just making sure -- Might be an admin restart

		        if (Mutex.WaitOne(TimeSpan.Zero, true)) return;
				// Try to show from tray, as user may not know it's hidden there.
				MessageBox.Show("Another TcNo Account Switcher Updater instance has been detected.");
				Environment.Exit(1056); // An instance of the service is already running.
	        }
	        catch (AbandonedMutexException)
	        {
		        // Just restarted 
	        }
        }

        [STAThread]
        protected override void OnStartup(StartupEventArgs e)
        {
            // Crash handler
            AppDomain.CurrentDomain.UnhandledException += CurrentDomain_UnhandledException;
            // Single instance:
            IsRunningAlready();

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

        public static string AppDataFolder =>
	        Directory.GetParent(Path.GetDirectoryName(Assembly.GetEntryAssembly()?.Location) ?? string.Empty)?.FullName;
        
        /// <summary>
        /// Exception handling for all programs
        /// </summary>
        public static void CurrentDomain_UnhandledException(object sender, UnhandledExceptionEventArgs e)
        {
            // Set working directory to parent
            if (File.Exists(Path.Join(AppDataFolder, "userdata_path.txt")))
	            Directory.SetCurrentDirectory(File.ReadAllLines(Path.Join(AppDataFolder, "userdata_path.txt"))[0].Trim());
            else
	            Directory.SetCurrentDirectory(Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "TcNo Account Switcher\\"));
            var version = "unknown";
            try
            {
                if (File.Exists("WindowSettings.json"))
                {
                    var o = JObject.Parse(File.ReadAllText("WindowSettings.json"));
                    version = o["Version"]?.ToString();
                }
            }
            catch (Exception)
            {
                //
            }

            // Log Unhandled Exception
            var exceptionStr = e.ExceptionObject.ToString();
            Directory.CreateDirectory("CrashLogs");
            var filePath = $"CrashLogs\\AccSwitcher-Updater-Crashlog-{DateTime.Now:dd-MM-yy_hh-mm-ss.fff}.txt";
            using var sw = File.AppendText(filePath);
            sw.WriteLine($"{DateTime.Now.ToString(CultureInfo.InvariantCulture)}({version})\tUNHANDLED CRASH: {exceptionStr}{Environment.NewLine}{Environment.NewLine}");

            //if (e.ExceptionObject.)
            //{
	           // if (e.HResult == -2147024671)
		          //  MessageBox.Show(
			         //   "Error: Windows has 'detected a virus or potentially unwanted software'. This is a false positive, and can be safely ignored. User action is required." +
			         //   Environment.NewLine +
			         //   "1. Please whitelist the program's directory." + Environment.NewLine +
			         //   "2. Run the Updater in the programs folder, in the 'updater' folder." + Environment.NewLine +
			         //   "OR download a fresh installer/copy from GitHub." + Environment.NewLine +
			         //   Environment.NewLine +
			         //   "I am aware of these and work as fast as possible to report false positives, and stop them being detected.",
			         //   "Windows error", MessageBoxButton.OK, MessageBoxImage.Error);
	           // else
		          //  throw;
            //})
            MessageBox.Show("This crashlog will be automatically submitted next launch." + Environment.NewLine + Environment.NewLine + "Error: " + e.ExceptionObject, "Fatal error occurred!", MessageBoxButton.OK, MessageBoxImage.Error, MessageBoxResult.OK, MessageBoxOptions.DefaultDesktopOnly);
        }
    }
}

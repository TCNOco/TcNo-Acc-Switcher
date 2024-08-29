// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2024 TroubleChute (Wesley Pyburn)
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
using System.Threading.Tasks;
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
                _ = MessageBox.Show("Another TcNo Account Switcher Updater instance has been detected.");
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
            AppDomain.CurrentDomain.ProcessExit += CurrentDomain_ProcessExit;
            TaskScheduler.UnobservedTaskException += (sender, e) =>
            {
                Logger.WriteLine("Unobserved task exception:");
                Logger.WriteLine(e.Exception.ToString());
                e.SetObserved(); // Prevents the exception from being escalated to a crash
            };

            // Single instance:
            IsRunningAlready();

            base.OnStartup(e);
            Logger.WriteLine($"Updater started with: {e.Args}");
            for (var i = 0; i != e.Args.Length; ++i)
            {
                switch (e.Args[i].ToLowerInvariant())
                {
                    case "downloadcef":
                        TcNo_Acc_Switcher_Updater.MainWindow.DownloadCef = true;
                        break;
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

        public static void LogToErrorFile(string log)
        {
            // Log Unhandled Exception
            _ = Directory.CreateDirectory("UpdaterErrorLogs");
            var filePath = $"UpdaterErrorLogs\\AccSwitcher-Updater-{DateTime.Now:dd-MM-yy_hh-mm-ss.fff}.txt";
            using var sw = File.AppendText(filePath);
            sw.WriteLine($"{DateTime.Now.ToString(CultureInfo.InvariantCulture)}({GetVersion()}){Environment.NewLine}{log}");

            Logger.WriteLine($"\nUpdater encountered an error!\n: {DateTime.Now.ToString(CultureInfo.InvariantCulture)}({GetVersion()}){Environment.NewLine}{log}\n\n");
        }
        
        private static string GetVersion()
        {
            try
            {
                if (File.Exists("WindowSettings.json"))
                {
                    var o = JObject.Parse(UGlobals.ReadAllText("WindowSettings.json"));
                    return o["Version"]?.ToString();
                }
            }
            catch (Exception)
            {
                //
            }
            return "unknown";
        }

        /// <summary>
        /// Exception handling for all programs
        /// </summary>
        public static void CurrentDomain_UnhandledException(object sender, UnhandledExceptionEventArgs e)
        {
            // Set working directory to parent
            Directory.SetCurrentDirectory(File.Exists(Path.Join(AppDataFolder, "userdata_path.txt"))
                ? UGlobals.ReadAllLines(Path.Join(AppDataFolder, "userdata_path.txt"))[0].Trim()
                : Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "TcNo Account Switcher\\"));

            // Log Unhandled Exception
            var exceptionStr = e.ExceptionObject.ToString();
            _ = Directory.CreateDirectory("CrashLogs");
            var filePath = $"CrashLogs\\AccSwitcher-Updater-Crashlog-{DateTime.Now:dd-MM-yy_hh-mm-ss.fff}.txt";
            using var sw = File.AppendText(filePath);
            sw.WriteLine($"{DateTime.Now.ToString(CultureInfo.InvariantCulture)}({GetVersion()})\tUNHANDLED CRASH: {exceptionStr}{Environment.NewLine}{Environment.NewLine}");
            Logger.WriteLine($"{DateTime.Now.ToString(CultureInfo.InvariantCulture)}({GetVersion()})\tUNHANDLED CRASH: {exceptionStr}{Environment.NewLine}{Environment.NewLine}");

            if (e.ExceptionObject is FileNotFoundException && (exceptionStr?.Contains("SevenZipExtractor") ?? false))
            {
                _ = MessageBox.Show($"A fatal error was hit. A required file was not found. Please make sure the x64 and x86 folders exist in the install directory AND the updater folder. If one has files, and the other not: Copy so they are the same and try again.{Environment.NewLine}Currently installed to: {AppDataFolder}", "Fatal error occurred!", MessageBoxButton.OK, MessageBoxImage.Error, MessageBoxResult.OK, MessageBoxOptions.DefaultDesktopOnly);
            }
            else
                _ = MessageBox.Show("Error: " + e.ExceptionObject, "Fatal error occurred!", MessageBoxButton.OK, MessageBoxImage.Error, MessageBoxResult.OK, MessageBoxOptions.DefaultDesktopOnly);


            // Throw for bad image errors (bad dlls in computer):
            if (e.ExceptionObject is BadImageFormatException exception) throw exception;

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
        }
        public static void CurrentDomain_ProcessExit(object sender, EventArgs e)
        {
            Logger.WriteLine("Application is exiting...");
            Logger.WriteLine((new System.Diagnostics.StackTrace()).ToString());
            // Perform cleanup or logging here
        }

    }
}

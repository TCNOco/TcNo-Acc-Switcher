using System;
using System.Diagnostics;
using System.Globalization;
using System.IO;
using System.Runtime.InteropServices;
using System.Windows;
using System.Windows.Input;
using Newtonsoft.Json;

namespace TcNo_Acc_Switcher_Globals
{
    public class Globals
    {
        public string WorkingDirectory { get; set; }
        public DateTime UpdateLastChecked { get; set; } = DateTime.Now;

        // Read existing settings. If they don't exist, create them.
        // --> Reads from current directory. This will only work with the main TcNo Account Switcher app.
        //     Other apps will need to find the correct directory first.
        public static Globals LoadExisting(string fromDir)
        {
            var globalsFile = Path.Combine(fromDir, "globals.json");

            Globals g;
            if (File.Exists(globalsFile))
                g = JsonConvert.DeserializeObject<Globals>(File.ReadAllText(globalsFile));
            else
            {
                g = new Globals();
                File.WriteAllText(globalsFile, JsonConvert.SerializeObject(g));
            }

            g.WorkingDirectory = fromDir;
            return g;
        }

        public static void Save(Globals g)
        {
            var globalsFile = Path.Combine(g.WorkingDirectory, "globals.json");
            File.WriteAllText(globalsFile, JsonConvert.SerializeObject(g, Formatting.Indented));
        }

        // Saves the last time an update was checked.
        public void LastCheckedNow()
        {
            UpdateLastChecked = DateTime.Now;
        }

        // Did the account switcher check for an update within the last day?
        public bool NeedsUpdateCheck()
        {
            return (UpdateLastChecked < DateTime.Now.AddDays(-1));
        }
        // Was the account switcher launched within the last few minutes?
        // It reports launches, so I know how many people are using it from where, but it won't count launches < 5 mins apart.
        public bool NeedsUpdateCheck_Launch()
        {
            return (UpdateLastChecked < DateTime.Now.AddMinutes(-5));
        }

        // Launch main software and check for updates if not already running
        public void RunUpdateCheck()
        {
            string mainExeName = "TcNo Account Switcher.exe";
            string mainExeFullName = Path.Combine(WorkingDirectory, mainExeName);
            if (!File.Exists(mainExeFullName)) return;
            Process[] pList = Process.GetProcessesByName(Path.GetFileNameWithoutExtension(mainExeName).ToLower());
            if (pList.Length > 0) return; // Return because software is already running.

            try
            {
                var processName = mainExeFullName;
                var startInfo = new ProcessStartInfo
                {
                    FileName = processName,
                    CreateNoWindow = false,
                    UseShellExecute = true,
                    Arguments = "-updatecheck"
                };

                Process.Start(startInfo);
            }
            catch (Exception)
            {
                Console.WriteLine(@"Failed to start for update check.");
            }
        }





        /// <summary>
        /// Exception handling for all programs
        /// </summary>
        public static void CurrentDomain_UnhandledException(object sender, UnhandledExceptionEventArgs e)
        {
            // Log Unhandled Exception
            var exceptionStr = e.ExceptionObject.ToString();
            Directory.CreateDirectory("Errors");
            var filePath = $"Errors\\AccSwitcher-Crashlog-{DateTime.Now:dd-MM-yy_hh-mm-ss.fff}.txt";
            using (var sw = File.AppendText(filePath))
            {
                sw.WriteLine(DateTime.Now.ToString(CultureInfo.InvariantCulture) + "\t" + Strings.ErrUnhandledCrash + ": " + exceptionStr + Environment.NewLine + Environment.NewLine);
            }
            Console.WriteLine(Strings.ErrUnhandledException + Path.GetFullPath(filePath));
            //MessageBox.Show(Strings.ErrUnhandledException + Path.GetFullPath(filePath), Strings.ErrUnhandledExceptionHeader, MessageBoxButton.OK, MessageBoxImage.Error);
            Console.WriteLine(Strings.ErrSubmitCrashlog);
            //MessageBox.Show(Strings.ErrSubmitCrashlog, Strings.ErrUnhandledExceptionHeader, MessageBoxButton.OK, MessageBoxImage.Information);
        }

    }
}

using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Text;
using System.Threading.Tasks;
using Newtonsoft.Json;

namespace TcNo_Acc_Switcher_Globals
{
    public class Globals
    {
        static void Main(string[] args)
        {
        }

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
                string processName = mainExeFullName;
                ProcessStartInfo startInfo = new ProcessStartInfo();
                startInfo.FileName = processName;
                startInfo.CreateNoWindow = false;
                startInfo.UseShellExecute = true;
                startInfo.Arguments = "-updatecheck";

                Process.Start(startInfo);
            }
            catch (Exception)
            {
                Console.WriteLine("Failed to start for update check.");
            }
        }
    }
}

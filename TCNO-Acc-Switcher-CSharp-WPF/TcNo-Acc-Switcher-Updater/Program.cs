using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Net;
using System.Text;
using System.Threading;

namespace TcNo_Acc_Switcher_Updater
{
    class Program
    {
        static void Main(string[] args)
        {
            Console.Title = @"TcNo Account Switcher - Auto-updater";
            Directory.SetCurrentDirectory(Path.GetDirectoryName(System.Reflection.Assembly.GetEntryAssembly().Location)); // Set working directory to the same as the actual .exe
#if X64
            string arch = "x64";
#else
            string arch = "x32";
#endif
            string updzip = arch + ".zip";
            string currentDirectory = Directory.GetCurrentDirectory();


            // Extract 7-zip for update
            Console.WriteLine(@"Extracting 7-zip");
            Extract7Zip();

            // Download update package
            // -- Will later have platforms separated, and download individual updates.
            Console.WriteLine(@"Starting download");
            Thread thread = new Thread(new ParameterizedThreadStart(DownloadUpdate));
            thread.Start(arch);


            // Get running all running applications in the program's directory
            Console.WriteLine(@"Checking for running processes...");
            var exeList = DirSearch(currentDirectory).Where(path => path.EndsWith(".exe")).ToList();
            List<string> runningPrograms = GetRunningProgramsList(exeList);

            RestartTrayCheck(runningPrograms);

            // Attempt to close running programs
            Console.WriteLine(@"Closing processes (if any)");
            CloseRunningPrograms(runningPrograms);

            // Wait for download to finish
            Console.WriteLine(@"Waiting for download to finish...");
            thread.Join();


            // Run 7-zip.
            // Extracts and replaces existing files with updated ones (-aoa)
            Console.WriteLine(@"Extracting ZIP");
            ProcessStartInfo pro = new ProcessStartInfo
            {
                WindowStyle = ProcessWindowStyle.Hidden,
                FileName = "7za.exe",
                Arguments = string.Format("x \"{0}\" -y -aoa -o\"{1}\"", updzip, currentDirectory),
                UseShellExecute = false,
                RedirectStandardOutput = true,
                CreateNoWindow = true
            };
            Process x = Process.Start(pro);
            x.WaitForExit();

            File.Create("Update_Complete");

            Console.WriteLine();
            Console.WriteLine(@"Starting TcNo Account Switcher");

            int attempts = 0;

            while (attempts < 3)
            {
                try
                {
                    attempts++;
                    string processName = "TcNo Account Switcher.exe";
                    ProcessStartInfo startInfo = new ProcessStartInfo();
                    startInfo.FileName = processName;
                    startInfo.CreateNoWindow = false;
                    startInfo.UseShellExecute = true;

                    Process.Start(startInfo);
                    break;
                }
                catch (Exception)
                {
                    Console.WriteLine("Failed to start. Trying again in 3 seconds. (Attempt " + attempts + "/3)");
                    Thread.Sleep(3000);
                }
            }
            Console.WriteLine(@"Closing in 3 seconds");
            Thread.Sleep(3000);
            Environment.Exit(1);
        }

        private static void RestartTrayCheck(List<string> runningPrograms)
        {
            foreach (string runningProgram in runningPrograms)
            {
                if (runningProgram.Contains("TcNo Acc Switcher SteamTray"))
                {
                    if (!File.Exists("Restart_SteamTray")) File.Create("Restart_SteamTray");
                }
            }
        }
        private static void DownloadUpdate(object arch)
        {
            using (WebClient wc = new WebClient())
            {
                string[] updateInfo = wc.DownloadString("https://tcno.co/Projects/AccSwitcher/net_update.php").Split('|');
                string downloadLink = updateInfo[((string)arch == "x64" ? 0 : 1)].Replace("\n", ""),
                    downloadZip = (string)arch + ".zip";

                Console.WriteLine($@"Downloading: {downloadLink}");

                File.Delete(downloadZip);

                wc.DownloadFile(downloadLink, downloadZip);
            }
        }

        // Recursively check <string>Folder, and return a string array of files.
        static List<string> DirSearch(string sDir)
        {
            List<string> paths = new List<string>();
            try
            {
                // Loop through files in directory
                foreach (string f in Directory.GetFiles(sDir))
                    paths.Add(f);
                // Search through each subfolder
                foreach (string d in Directory.GetDirectories(sDir))
                    paths.AddRange(DirSearch(d));
                
            }
            catch (System.Exception e)
            {
                Console.WriteLine(e.Message);
            }

            return paths;
        }

        // Check if .exe at path is running.
        // - Partly based off https://stackoverflow.com/a/34380469
        private static List<string> GetRunningProgramsList(List<string> fullPaths)
        {
            var runningProgramList = new List<string>();
            foreach (string fullPath in fullPaths)
            {
                // Skip self
                if (fullPath.EndsWith("TcNo Acc Switcher Updater.exe")) continue;

                var filePath = Path.GetDirectoryName(fullPath);
                var fileName = Path.GetFileNameWithoutExtension(fullPath).ToLower();

                Process[] pList = Process.GetProcessesByName(fileName);

                foreach (Process p in pList)
                {
                    try
                    {
                        if (p.MainModule != null && !p.MainModule.FileName.StartsWith(filePath, StringComparison.InvariantCultureIgnoreCase)) continue;
                        runningProgramList.Add(fullPath);
                        break;
                    }
                    catch (Exception)
                    {
                        // Relatively save to ignore errors here.
                        continue;
                    }
                }
            }

            return runningProgramList;
        }

        // Close applications in list
        private static void CloseRunningPrograms(List<string> fullPaths)
        {
            var runningProgramList = new List<string>();
            foreach (string fullPath in fullPaths)
            {
                Console.WriteLine($@"Closing: {fullPath}");
                if (!KillProgram(fullPath))
                {
                    Console.WriteLine($@"ERROR: Could not close: {fullPath}");
                    while (!KillProgram(fullPath))
                    {
                        Console.WriteLine(@"Please exit the running program, and press any key to continue.");
                        Console.ReadKey();
                    }
                }
            }
        }
        private static bool KillProgram(string fullPath)
        {
            var filePath = Path.GetDirectoryName(fullPath);
            var fileName = Path.GetFileNameWithoutExtension(fullPath).ToLower();
            Process[] pList = Process.GetProcessesByName(fileName);
            foreach (Process p in pList)
            {
                try
                {
                    if (p.MainModule != null
                        && !p.MainModule.FileName.StartsWith(filePath, StringComparison.InvariantCultureIgnoreCase)
                        && !p.MainModule.FileName.EndsWith(fileName, StringComparison.InvariantCultureIgnoreCase))
                        continue;

                    p.Kill();
                    return true;
                }
                catch (Exception)
                {
                    // Program can not be closed automatically.
                    return false;
                }
            }
            // Program already closed
            return true;
        }

        // Extract 7-zip and license from Resources.
        private static void Extract7Zip()
        {
            File.WriteAllBytes("7za.exe", Properties.Resources._7za);
            File.WriteAllText("7za-license.txt", Properties.Resources.License);
        }
    }
}

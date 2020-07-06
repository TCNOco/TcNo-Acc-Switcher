using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Net;
using System.Threading;
using TcNo_Acc_Switcher_Globals;

namespace TcNo_Acc_Switcher_Updater
{
    internal class Program
    {
        private static void Main(string[] args)
        {
            // Crash handler
            AppDomain.CurrentDomain.UnhandledException += Globals.CurrentDomain_UnhandledException;
            Console.Title = @"TcNo Account Switcher - Auto-updater";
            Directory.SetCurrentDirectory(Path.GetDirectoryName(System.Reflection.Assembly.GetEntryAssembly().Location)); // Set working directory to the same as the actual .exe
#if X64
            const string arch = "x64";
#else
            const string arch = "x32";
#endif
            var updZip = arch + ".zip";
            var currentDirectory = Directory.GetCurrentDirectory();


            // Extract 7-zip for update
            Console.WriteLine(@"Extracting 7-zip");
            Extract7Zip();

            // Download update package
            // -- Will later have platforms separated, and download individual updates.
            Console.WriteLine(@"Starting download");
            var thread = new Thread(DownloadUpdate);
            thread.Start(arch);


            // Get running all running applications in the program's directory
            Console.WriteLine(@"Checking for running processes...");
            var exeList = DirSearch(currentDirectory).Where(path => path.EndsWith(".exe")).ToList();
            var runningPrograms = GetRunningProgramsList(exeList);

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
            var pro = new ProcessStartInfo
            {
                WindowStyle = ProcessWindowStyle.Hidden,
                FileName = "7za.exe",
                Arguments = $"x \"{updZip}\" -y -aoa -o\"{currentDirectory}\"",
                UseShellExecute = false,
                RedirectStandardOutput = true,
                CreateNoWindow = true
            };
            var x = Process.Start(pro);
            x?.WaitForExit();

            File.Create("Update_Complete");

            Console.WriteLine();
            Console.WriteLine(@"Starting TcNo Account Switcher");

            var attempts = 0;

            while (attempts < 3)
            {
                try
                {
                    attempts++;
                    const string processName = "TcNo Account Switcher.exe";
                    var startInfo = new ProcessStartInfo
                    {
                        FileName = processName, CreateNoWindow = false, UseShellExecute = true
                    };

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
            foreach (var unused in runningPrograms.Where(runningProgram => 
                runningProgram.Contains("TcNo Acc Switcher SteamTray")).Where(runningProgram => 
                !File.Exists("Restart_SteamTray")))
                File.Create("Restart_SteamTray");
        }
        private static void DownloadUpdate(object arch)
        {
            using (var wc = new WebClient())
            {
                var updateInfo = wc.DownloadString("https://tcno.co/Projects/AccSwitcher/net_update.php").Split('|');
                string downloadLink = updateInfo[((string)arch == "x64" ? 0 : 1)].Replace("\n", ""),
                    downloadZip = (string)arch + ".zip";

                Console.WriteLine($@"Downloading: {downloadLink}");

                File.Delete(downloadZip);

                wc.DownloadFile(downloadLink, downloadZip);
            }
        }

        // Recursively check <string>Folder, and return a string array of files.
        private static IEnumerable<string> DirSearch(string sDir)
        {
            var paths = new List<string>();
            try
            {
                // Loop through files in directory
                paths.AddRange(Directory.GetFiles(sDir));
                // Search through each subfolder
                foreach (var d in Directory.GetDirectories(sDir))
                    paths.AddRange(DirSearch(d));
            }
            catch (Exception e)
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
            foreach (var fullPath in fullPaths)
            {
                // Skip self
                if (fullPath.EndsWith("TcNo Acc Switcher Updater.exe")) continue;

                var filePath = Path.GetDirectoryName(fullPath);
                var fileName = Path.GetFileNameWithoutExtension(fullPath).ToLower();

                var pList = Process.GetProcessesByName(fileName);

                foreach (var p in pList)
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
                    }
                }
            }

            return runningProgramList;
        }

        // Close applications in list
        private static void CloseRunningPrograms(IEnumerable<string> fullPaths)
        {
            foreach (var fullPath in fullPaths)
            {
                Console.WriteLine($@"Closing: {fullPath}");
                if (KillProgram(fullPath)) continue;
                Console.WriteLine($@"ERROR: Could not close: {fullPath}");
                while (!KillProgram(fullPath))
                {
                    Console.WriteLine(@"Please exit the running program, and press any key to continue.");
                    Console.ReadKey();
                }
            }
        }
        private static bool KillProgram(string fullPath)
        {
            var filePath = Path.GetDirectoryName(fullPath);
            var fileName = Path.GetFileNameWithoutExtension(fullPath).ToLower();
            var pList = Process.GetProcessesByName(fileName);
            foreach (var p in pList)
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

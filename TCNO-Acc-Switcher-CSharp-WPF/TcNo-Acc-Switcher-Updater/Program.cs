using System;
using System.Diagnostics;
using System.IO;
using System.Net;
using System.Text;
using System.Threading;

namespace TcNo_Acc_Switcher_Updater
{
    class Program
    {
        static void Main(string[] args)
        {
            Console.Title = "TcNo Account Switcher - Autoupdater";
            // Get main process .exe name, as users might set it to their own
            if (!File.Exists("UpdateInformation.txt"))
                Environment.Exit(1);

            // Wait until the program has finished closing
            string[] information = File.ReadAllText("UpdateInformation.txt").Split("|");
            string  MainExeName = information[0],
                    Arch = information[1],
                    Version = versionToString(information[2]);
            Console.WriteLine("Waiting for TcNo Account Switcher to close...");
            while (File.Exists(MainExeName))
            {
                try
                {
                    File.Delete(MainExeName);
                }
                catch (Exception)
                {
                    Thread.Sleep(500);
                }
            }
            Console.WriteLine("TcNo Account Switcher closed!");
            Console.WriteLine($"Old version [{Version}] was deleted");
            Console.WriteLine();

            using (WebClient wc = new WebClient())
            {

                string[] updateInfo = wc.DownloadString("https://tcno.co/Projects/AccSwitcher/update.php").Split("|");
                string downloadLink = updateInfo[(Arch == "x64" ? 0 : 1)],
                        downloadZip = Arch + ".zip";

                Console.WriteLine("Downloading: " + downloadLink);

                if (File.Exists(downloadZip))
                    File.Delete(downloadZip);

                wc.DownloadFile(downloadLink, downloadZip);

                Console.WriteLine("Extracting Zip");
                string ePath = Path.GetDirectoryName(Process.GetCurrentProcess().MainModule.FileName),
                       zPath = Path.Combine("Resources", "7za.exe");
                try
                {
                    ProcessStartInfo pro = new ProcessStartInfo();
                    pro.WindowStyle = ProcessWindowStyle.Hidden;
                    pro.FileName = zPath;
                    pro.Arguments = string.Format("x \"{0}\" -y -o\"{1}\"", downloadZip, ePath);
                    pro.UseShellExecute = false;
                    pro.RedirectStandardOutput = true;
                    pro.CreateNoWindow = true;

                    //Console.WriteLine(pro.FileName + " | " + pro.Arguments);
                    //Console.ReadLine();

                    Process x = Process.Start(pro);
                    x.WaitForExit();
                }
                catch (System.Exception Ex)
                {
                    Console.WriteLine("Error extracting new version! Error: " + Ex.ToString());
                }
                Console.WriteLine("Zip extracted!");

                Console.WriteLine("");
                Console.WriteLine("Starting TcNo Account Switcher");

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

                        //Console.WriteLine(startInfo.FileName);
                        //Console.ReadLine();

                        Process.Start(startInfo);
                        break;
                    }
                    catch (Exception)
                    {
                        Console.WriteLine("Failed to start. Trying again in 3 seconds. (Attempt " + attempts + "/3)");
                        Thread.Sleep(3000);
                    }
                }
                Console.WriteLine("Closing in 2 seconds");
                Thread.Sleep(3000);
                Environment.Exit(1);
            }
            Console.WriteLine("Failed to update.");
            Console.WriteLine("Please extract either x64.zip or x32.zip manually and start TcNo Account Switcher.exe");
        }
        static string versionToString(string version)
        {
            StringBuilder sb = new StringBuilder();
            foreach (char c in version)
            {
                sb.Append(c).Append(".");
            }
            sb.Remove(sb.Length - 1, 1);
            return sb.ToString();
        }
    }
}

using System;
using System.Collections.Generic;
using System.Configuration;
using System.Data;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Reflection;
using System.Threading.Tasks;
using System.Windows;

namespace TcNo_Acc_Switcher
{
    /// <summary>
    /// Interaction logic for App.xaml
    /// </summary>
    public partial class App : Application
    {
        static void AssemblyResolver()
        {
            AppDomain.CurrentDomain.AssemblyResolve += new ResolveEventHandler((sender, args) => Assembly.LoadFrom("libs\\TcNo-Acc-Switcher-Globals.dll"));
        }
        protected override void OnStartup(StartupEventArgs e)
        {
            AssemblyResolver();

            Directory.SetCurrentDirectory(Path.GetDirectoryName(System.Reflection.Assembly.GetEntryAssembly().Location)); // Set working directory to the same as the actual .exe
            MainWindow mainWindow = new MainWindow();
            mainWindow.Topmost = true;

            for (int i = 0; i != e.Args.Length; ++i)
            {
                Console.WriteLine(e.Args[i]);
                if (e.Args[i]?[0] == '+')
                {
                    string arg = e.Args[i].Substring(1); // Get command

                    switch (arg)
                    {
                        case "steam":
                            Process.Start("Steam\\TcNo Account Switcher Steam.exe");
                            mainWindow.Process_Update(true);
                            Environment.Exit(1);
                            break;
                        //case "origin":
                    }
                }

                if (e.Args[i]?[0] == '-')
                {
                    string arg = e.Args[i].Substring(1); // Get command
                    switch (arg)
                    {
                        case "updatecheck":
                            mainWindow.Process_Update(true);
                            Environment.Exit(1);
                            break;
                        //case "update_steam":
                    }
                }
            }

            // Bypass this platform picker launcher for now.
            // This keeps shortcuts working, and can be replaced with an update.
            //ProcessStartInfo startInfo = new ProcessStartInfo();
            //startInfo.FileName = System.IO.Path.GetFullPath("Steam\\TcNo Account Switcher Steam.exe");
            //startInfo.WorkingDirectory = Path.GetDirectoryName("Steam\\TcNo Account Switcher Steam.exe");
            //startInfo.CreateNoWindow = false;
            //startInfo.UseShellExecute = false;
            //Process.Start(startInfo);
            Process.Start("Steam\\TcNo Account Switcher Steam.exe");

            mainWindow.Process_Update(false);
            Environment.Exit(1);
        }
    }
}

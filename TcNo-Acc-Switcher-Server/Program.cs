// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2022 TechNobo (Wesley Pyburn)
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
using System.IO;
using System.Linq;
using System.Net.NetworkInformation;
using System.Text.RegularExpressions;
using Microsoft.AspNetCore.Components;
using Microsoft.AspNetCore.Hosting;
using Microsoft.Extensions.Hosting;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;


namespace TcNo_Acc_Switcher_Server
{
    public class Program
    {
        [STAThread]
        public static void Main(string[] args)
        {
            // Set working directory to documents folder
            Globals.CreateDataFolder(false);
            Directory.SetCurrentDirectory(Globals.UserDataFolder);

            try
            {
                CreateHostBuilder(args).Build().Run();
            }
            catch (IOException ioe)
            {
                // Failed to bind to port
                if (ioe.HResult != -2146232800) throw;
                Globals.WriteToLog(ioe);
            }
        }

        public static IHostBuilder CreateHostBuilder(string[] args)
        {
            var port = "";
            foreach (var arg in args)
            {
                if (!arg.Contains("--url")) continue;
                port = Regex.Match(arg, @"[\d]+", RegexOptions.IgnoreCase).Value;
                Console.WriteLine(@"Using port (from --url arg): " + port);
            }

            if (string.IsNullOrEmpty(port))
            {
                port = FindOpenPort().ToString();
                Console.WriteLine(@"Using saved/random port: " + port);
            }

            // Start browser - if not started with nobrowser
            if (!args.Contains("nobrowser")) Data.GeneralFuncs.OpenLinkInBrowser($"http://localhost:{port}");

            return Host.CreateDefaultBuilder(args)
                .ConfigureWebHostDefaults(webBuilder =>
                {
                    _ = webBuilder.UseStartup<Startup>()
                        .UseUrls($"http://localhost:{port}");
                });
        }

        /// <summary>
        /// Find first available port up from requested
        /// </summary>
        public static int FindOpenPort()
        {
            Globals.DebugWriteLine(@"[Func:(Client)MainWindow.xaml.cs.FindOpenPort]");
            // Check if port available:
            var ipGlobalProperties = IPGlobalProperties.GetIPGlobalProperties();
            var tcpConnInfoArray = ipGlobalProperties.GetActiveTcpConnections();
            var serverPort = 1337;
            if (File.Exists(Path.Join(Globals.UserDataFolder, "WindowSettings.json")))
            {
                var appSettings = JObject.Load(new JsonTextReader(File.OpenText(Globals.UserDataFolder + "WindowSettings.json")));
                serverPort = appSettings.Value<int>("ServerPort");
            }

            while (true)
            {
                if (tcpConnInfoArray.All(x => x.LocalEndPoint.Port != serverPort)) break;
                serverPort = NewPort();
            }

            return serverPort;
        }

        public static int NewPort()
        {
            var r = new Random();
            return r.Next(20000, 40000); // Random int [Why this range? See: https://www.sciencedirect.com/topics/computer-science/registered-port & netsh interface ipv4 show excludedportrange protocol=tcp]
        }
    }
}

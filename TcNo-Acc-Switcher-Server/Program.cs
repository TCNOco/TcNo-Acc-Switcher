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
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Net.NetworkInformation;
using System.Text.RegularExpressions;
using Microsoft.AspNetCore.Hosting;
using Microsoft.Extensions.Hosting;
using Newtonsoft.Json;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.State;

namespace TcNo_Acc_Switcher_Server
{
    public class Program
    {
        public static void Main(string[] args)
        {
            // Empty
            _ = MainProgram(args);
        }
        public static bool MainProgram(string[] args)
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
                return false;
            }

            return true;
        }

        public static IHostBuilder CreateHostBuilder(string[] args)
        {
            // Create WindowSettings instance. This loads from saved file if available.
            WindowSettings windowSettings = new();
            var savedPort = windowSettings.ServerPort;

            var port = "";
            foreach (var arg in args)
            {
                if (!arg.Contains("--url")) continue;
                port = Regex.Match(arg, @"[\d]+", RegexOptions.IgnoreCase).Value;
                Console.WriteLine(@"Using port (from --url arg): " + port);
            }

            if (string.IsNullOrEmpty(port))
            {
                // The server was launched without the client, or a specified port
                windowSettings.ServerPort = FindOpenPort(windowSettings.ServerPort);
                Console.WriteLine(@"Using saved/random port: " + windowSettings.ServerPort);
            }

            // Save new port, if different.
            if (savedPort.ToString() != port)
                windowSettings.Save();

            // Start browser - if not started with nobrowser
            if (!args.Contains("nobrowser")) GeneralInvocableFuncs.OpenLinkInBrowser($"http://localhost:{windowSettings.ServerPort}");

            return Host.CreateDefaultBuilder(args)
                .ConfigureWebHostDefaults(webBuilder =>
                {
                    _ = webBuilder.UseStartup<Startup>()
                        .UseUrls($"http://localhost:{windowSettings.ServerPort}");
                });
        }

        /// <summary>
        /// Find first available port. Returns open port to use.
        /// May return same as input, if it was open.
        /// </summary>
        public static int FindOpenPort(int port = 0)
        {
            Globals.DebugWriteLine(@"[Func:(Client)MainWindow.xaml.cs.FindOpenPort]");
            // Pick random port if none assigned.
            if (port == 0) port = NewPort();

            // Check if port available:
            var ipGlobalProperties = IPGlobalProperties.GetIPGlobalProperties();
            var tcpConnInfoArray = ipGlobalProperties.GetActiveTcpConnections();
            while (true)
            {
                if (tcpConnInfoArray.All(x => x.LocalEndPoint.Port != port)) break;
                port = NewPort();
            }

            return port;
        }

        private static int NewPort()
        {
            var r = new Random();
            return r.Next(20000, 40000); // Random int [Why this range? See: https://www.sciencedirect.com/topics/computer-science/registered-port & netsh interface ipv4 show excludedportrange protocol=tcp]
        }
    }
}

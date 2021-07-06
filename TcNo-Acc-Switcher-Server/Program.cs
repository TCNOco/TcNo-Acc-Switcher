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
using System.IO;
using Microsoft.AspNetCore.Hosting;
using Microsoft.Extensions.Hosting;
using TcNo_Acc_Switcher_Globals;

namespace TcNo_Acc_Switcher_Server
{
    public class Program
    {
	    public static void Main(string[] args)
        {
            // Empty
            MainProgram(args, out _);
        }
        public static bool MainProgram(string[] args, out Exception e)
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
                e = ioe;
                return false;
            }

            e = null;
            return true;
        }

        public static IHostBuilder CreateHostBuilder(string[] args) =>
            Host.CreateDefaultBuilder(args)
                .ConfigureWebHostDefaults(webBuilder =>
                {
                    webBuilder.UseStartup<Startup>();
                });
    }
}

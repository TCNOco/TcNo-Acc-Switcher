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
using System.Linq;
using System.Reflection;
using AKSoftware.Localization.MultiLanguages;
using Microsoft.AspNetCore.Builder;
using Microsoft.AspNetCore.Hosting;
using Microsoft.AspNetCore.Http;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.FileProviders;
using Microsoft.Extensions.Hosting;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General;

namespace TcNo_Acc_Switcher_Server
{
    public class Startup
    {
        public Startup(IConfiguration configuration)
        {
            Configuration = configuration;
        }

        public IConfiguration Configuration { get; }

        // This method gets called by the runtime. Use this method to add services to the container.
        // For more information on how to configure your application, visit https://go.microsoft.com/fwlink/?LinkID=398940
        public void ConfigureServices(IServiceCollection services)
        {
            // Crash handler
            AppDomain.CurrentDomain.UnhandledException += Globals.CurrentDomain_UnhandledException;
            services.AddControllers();
            services.AddLocalization(options => options.ResourcesPath = "Resources");

            services.AddRazorPages();
            services.AddLanguageContainer<EmbeddedResourceKeysProvider>(Assembly.GetExecutingAssembly(), "Resources");
            services.AddServerSideBlazor().AddCircuitOptions(options => { options.DetailedErrors = true; });



            // Persistent settings:
            services.AddSingleton<AppSettings>();
            services.AddSingleton<AppData>();
            services.AddSingleton<Data.Settings.BattleNet>();
            services.AddSingleton<Data.Settings.Discord>();
            services.AddSingleton<Data.Settings.Epic>();
            services.AddSingleton<Data.Settings.Origin>();
            services.AddSingleton<Data.Settings.Riot>();
            services.AddSingleton<Data.Settings.Steam>();
            services.AddSingleton<Data.Settings.Ubisoft>();
        }

        private RequestLocalizationOptions GetLocalisationOptions()
        {
	        var cultures = Configuration.GetSection("Cultures").GetChildren().ToDictionary(x => x.Key, x =>  x.Value);
            var supportedCultures = cultures.Keys.ToArray();
	        var localisationOptions = new RequestLocalizationOptions()
		        .AddSupportedCultures(supportedCultures)
		        .AddSupportedUICultures(supportedCultures);
            return localisationOptions;
        }

        // This method gets called by the runtime. Use this method to configure the HTTP request pipeline.
        public void Configure(IApplicationBuilder app, IWebHostEnvironment env)
        {
            if (env.IsDevelopment())
            {
                app.UseDeveloperExceptionPage();
            }
            else
            {
                app.UseExceptionHandler("/Error");
            }

            // Moves any old files from previous installs.
            foreach (var p in Globals.PlatformList) // Copy across all platform files
            {
	            MoveIfFileExists(p + "Settings.json");
            }
            MoveIfFileExists("SteamForgotten.json");
            MoveIfFileExists("StyleSettings.yaml");
            MoveIfFileExists("Tray_Users.json");
            MoveIfFileExists("WindowSettings.json");

            // Copy LoginCache
            if (Directory.Exists(Path.Join(Globals.AppDataFolder, "LoginCache\\")))
            {
	            if (Directory.Exists(Path.Join(Globals.UserDataFolder, "LoginCache"))) GeneralFuncs.RecursiveDelete(new DirectoryInfo(Path.Join(Globals.UserDataFolder, "LoginCache")), true);
	            Globals.CopyFilesRecursive(Path.Join(Globals.AppDataFolder, "LoginCache"), Path.Join(Globals.UserDataFolder, "LoginCache"));
            }

            try
            {
	            app.UseStaticFiles(new StaticFileOptions
	            {
		            FileProvider = new PhysicalFileProvider(Path.Join(Globals.UserDataFolder, @"wwwroot")),
		            RequestPath = new PathString("")
	            });
            }
            catch (DirectoryNotFoundException)
            {
	            Globals.CopyFilesRecursive(Globals.OriginalWwwroot, "wwwroot");
			}

			app.UseStaticFiles(); // Second call due to: https://github.com/dotnet/aspnetcore/issues/19578

			app.UseRequestLocalization(GetLocalisationOptions());

            app.UseRouting();

            app.UseEndpoints(endpoints =>
            {
                endpoints.MapDefaultControllerRoute();
                endpoints.MapControllers();

                endpoints.MapBlazorHub();
                endpoints.MapFallbackToPage("/_Host");
            });
        }

        private static void MoveIfFileExists(string f)
        {
            if (File.Exists(Path.Join(Globals.AppDataFolder, f)))
                File.Copy(Path.Join(Globals.AppDataFolder, f), Path.Join(Globals.UserDataFolder, f), true);
            File.Delete(Path.Join(Globals.AppDataFolder, f));
        }
    }
}

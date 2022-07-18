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
using Microsoft.AspNetCore.Builder;
using Microsoft.AspNetCore.Components;
using Microsoft.AspNetCore.Hosting;
using Microsoft.AspNetCore.Http;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.FileProviders;
using Microsoft.Extensions.Hosting;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Data.Classes;
using TcNo_Acc_Switcher_Server.Data.Settings;
using TcNo_Acc_Switcher_Server.Shared.Accounts;

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
            AppDomain.CurrentDomain.ProcessExit += CurrentDomain_OnProcessExit;
            _ = services.AddControllers();

            _ = services.AddRazorPages();
            _ = services.AddServerSideBlazor().AddCircuitOptions(options => { options.DetailedErrors = true; });

            _ = services.AddTransient<IHttpContextAccessor, HttpContextAccessor>();

            // Persistent settings:
            _ = services.AddTransient<IAccountFuncs, AccountFuncs>();
            _ = services.AddTransient<Program>();
            _ = services.AddTransient<IAppSettings, AppSettings>();

            _ = services.AddTransient<IAppStats, AppStats>();
            _ = services.AddTransient<IAppData, AppData>();
            _ = services.AddTransient<IAppFuncs, AppFuncs>();
            _ = services.AddTransient<IAppStats, AppStats>();
            _ = services.AddTransient<IGeneralFuncs, GeneralFuncs >();
            _ = services.AddTransient<IModalData, ModalData>();
            _ = services.AddTransient<IBasicPlatforms, BasicPlatforms>();
            _ = services.AddTransient<IBasicStats, BasicStats>();
            _ = services.AddTransient<ICurrentPlatform, CurrentPlatform>();
            _ = services.AddTransient<IBasic, Basic>();
            _ = services.AddTransient<ISteam, Steam>();
            _ = services.AddTransient<ILang, Lang>();
        }

        [Inject] private ILang Lang { get; }
        [Inject] private IAppStats AppStats { get; }
        [Inject] private IAppSettings AppSettings { get; }

        // This method gets called by the runtime. Use this method to configure the HTTP request pipeline.
        public void Configure(IApplicationBuilder app, IWebHostEnvironment env)
        {
            //var prog = app.ApplicationServices.GetRequiredService<Program>();
            //app.ApplicationServices.GetRequiredService<IAccountFuncs>();
            //app.ApplicationServices.GetRequiredService<ILang>();

            _ = env.IsDevelopment() ? app.UseDeveloperExceptionPage() : app.UseExceptionHandler("/Error");

            MoveIfFileExists("Tray_Users.json");
            MoveIfFileExists("WindowSettings.json");

            // Copy LoginCache
            if (Directory.Exists(Path.Join(Globals.AppDataFolder, "LoginCache\\")))
            {
                Globals.RecursiveDelete(Path.Join(Globals.UserDataFolder, "LoginCache"), true);
                Globals.CopyFilesRecursive(Path.Join(Globals.AppDataFolder, "LoginCache"), Path.Join(Globals.UserDataFolder, "LoginCache"));
            }

            try
            {
                _ = app.UseStaticFiles(new StaticFileOptions
                {
                    FileProvider = new PhysicalFileProvider(Path.Join(Globals.UserDataFolder, @"wwwroot")),
                    RequestPath = new PathString("")
                });
            }
            catch (DirectoryNotFoundException)
            {
                Globals.CopyFilesRecursive(Globals.OriginalWwwroot, "wwwroot", throwOnError: true);
            }

            _ = app.UseStaticFiles(); // Second call due to: https://github.com/dotnet/aspnetcore/issues/19578

            _ = app.UseRouting();

            _ = app.UseEndpoints(endpoints =>
              {
                  _ = endpoints.MapDefaultControllerRoute();
                  _ = endpoints.MapControllers();

                  _ = endpoints.MapBlazorHub();
                  _ = endpoints.MapFallbackToPage("/_Host");
              });
        }

        public void CurrentDomain_OnProcessExit(object sender, EventArgs e)
        {
            try
            {
                AppStats.SaveSettings();
            }
            catch (Exception)
            {
                // Do nothing, just close.
            }
        }

        private static void MoveIfFileExists(string f)
        {
            Globals.CopyFile(Path.Join(Globals.AppDataFolder, f), Path.Join(Globals.UserDataFolder, f));
            Globals.DeleteFile(Path.Join(Globals.AppDataFolder, f));
        }
    }
}

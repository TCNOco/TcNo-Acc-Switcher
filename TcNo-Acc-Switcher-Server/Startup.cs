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
using Microsoft.Extensions.DependencyInjection;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.State;
using TcNo_Acc_Switcher_Server.State.Interfaces;

namespace TcNo_Acc_Switcher_Server;

public class Startup
{
    public Startup(IConfiguration configuration)
    {
        Configuration = configuration;
    }

    private IConfiguration Configuration { get; }

    // This method gets called by the runtime. Use this method to add services to the container.
    // For more information on how to configure your application, visit https://go.microsoft.com/fwlink/?LinkID=398940
    public void ConfigureServices(IServiceCollection services)
    {
        // Crash handler
        AppDomain.CurrentDomain.UnhandledException += Globals.CurrentDomain_UnhandledException;
        _ = services.AddControllers();

        _ = services.AddRazorPages();
        _ = services.AddServerSideBlazor().AddCircuitOptions(options => { options.DetailedErrors = true; });

        _ = services.AddSingleton<IHttpContextAccessor, HttpContextAccessor>();

        // Proper singletons. This is the correct load order.
        _ = services.AddSingleton<IWindowSettings, WindowSettings>(); // #1 - NONE
        _ = services.AddSingleton<ILang, Lang>(); // After WindowSettings
        _ = services.AddSingleton<IToasts, Toasts>(); // After Lang
        _ = services.AddSingleton<IModals, Modals>(); // After Lang
        _ = services.AddSingleton<IStatistics, Statistics>(); // After WindowSettings
        _ = services.AddSingleton<IAppState, AppState>(); // Lang, Modals, Statistics, Toasts, WindowSettings
        _ = services.AddSingleton<ISharedFunctions, SharedFunctions>(); // AppState, Lang, Modals, Statistics, Toasts (+JsRuntime)

        _ = services.AddSingleton<IGameStatsRoot, GameStatsRoot>(); // Toasts, WindowSettings
        _ = services.AddSingleton<IGameStats, GameStats>(); // AppState, GameStatsRoot

        _ = services.AddSingleton<ISteamSettings, SteamSettings>(); // (No depends)
        _ = services.AddSingleton<ISteamState, SteamState>(); // AppState, GameStats, Lang, Modals, SharedFunctions, Statistics, SteamSettings, Toasts
        _ = services.AddSingleton<ISteamFuncs, SteamFuncs>(); // AppState, Lang, Modals, SharedFunctions, Statistics, SteamSettings, SteamState, Toasts, WindowSettings (+JSRuntime)

        _ = services.AddSingleton<ITemplatedPlatformState, TemplatedPlatformState>(); // AppState, GameStats, Modals, SharedFunctions, Statistics, Toasts
        _ = services.AddSingleton<ITemplatedPlatformSettings, TemplatedPlatformSettings>(); // Statistics, TemplatedPlatformState
        _ = services.AddSingleton<ITemplatedPlatformFuncs, TemplatedPlatformFuncs>(); // AppState, Lang, Modals, SharedFunctions, Statistics, TemplatedPlatformState, TemplatedPlatformSettings, Toasts, WindowSettings (+JSRuntime, NavigationManager)
    }

    // This method gets called by the runtime. Use this method to configure the HTTP request pipeline.
    public void Configure(IApplicationBuilder app, IWebHostEnvironment env)
    {
        _ = env.IsDevelopment() ? app.UseDeveloperExceptionPage() : app.UseExceptionHandler("/Error");
        // This is to make sure the services are loaded.

        // Previously settings files for previous platforms, as well as Tray_Users.json and WindowSettings.json
        // were moved from the app directory, to the userdata directory.
        // It has been a long time since these files were saved here and it's probably time to sunset this compatibility feature.
        // Also prevents me having to load a list of all games here, as well as in the app itself.

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
}
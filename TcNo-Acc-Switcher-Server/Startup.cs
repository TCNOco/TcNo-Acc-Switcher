// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2024 TroubleChute (Wesley Pyburn)
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
using System.Net.Http;
using System.Runtime.InteropServices;
using System.Runtime.Versioning;
using System.Text.Json;
using Microsoft.AspNetCore.Builder;
using Microsoft.AspNetCore.Hosting;
using Microsoft.AspNetCore.Http;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.FileProviders;
using Microsoft.Extensions.Hosting;
using Microsoft.Win32;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Data.Settings;
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
            AppDomain.CurrentDomain.ProcessExit += CurrentDomain_OnProcessExit;
            _ = services.AddControllers();

            _ = services.AddRazorPages();
            _ = services.AddServerSideBlazor().AddCircuitOptions(options => { options.DetailedErrors = true; });

            _ = services.AddSingleton<IHttpContextAccessor, HttpContextAccessor>();

            // Persistent settings:
            _ = services.AddSingleton<AppSettings>();
            _ = services.AddSingleton<AppStats>();
            _ = services.AddSingleton<AppData>();
            _ = services.AddSingleton<BasicPlatforms>();
            _ = services.AddSingleton<BasicStats>();
            _ = services.AddSingleton<CurrentPlatform>();
            _ = services.AddSingleton<Basic>();
            _ = services.AddSingleton<Steam>();
            _ = services.AddSingleton<Lang>();
        }

        // This method gets called by the runtime. Use this method to configure the HTTP request pipeline.
        public void Configure(IApplicationBuilder app, IWebHostEnvironment env)
        {
            Lang.LoadLocalised();

            _ = env.IsDevelopment() ? app.UseDeveloperExceptionPage() : app.UseExceptionHandler("/Error");

            // Moves any old files from previous installs.
            foreach (var p in AppData.Instance.PlatformList) // Copy across all platform files
            {
                MoveIfFileExists(p + "Settings.json");
            }
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

            // Increment launch count. I don't know if this should be here, but it is.
            AppStats.LaunchCount++;

            // Update installed version number, if uninstaller preset.
            if (OperatingSystem.IsWindows()) UpdateRegistryVersion(Globals.Version);

            // Check for Platforms.json update
            if (!AppSettings.OfflineMode) UpdatePlatformsJson();
        }

        public static void CurrentDomain_OnProcessExit(object sender, EventArgs e)
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

        private static readonly string[] RegistryKeys =
        [
            @"SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\TcNo-Acc-Switcher",
            @"SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\TcNo-Acc-Switcher"
        ];

        [SupportedOSPlatform("windows")]
        public static void UpdateRegistryVersion(string version)
        {
            var dotVersion = version.Replace("-", ".").Replace("_", ".");

            string exeDirectory = AppDomain.CurrentDomain.BaseDirectory;
            string uninstallExePath = Path.Combine(exeDirectory, "Uninstall TcNo Account Switcher.exe");

            // Check if uninstaller exists. If not, this copy isn't installed. Portable, or otherwise.
            if (!File.Exists(uninstallExePath)) return;

            foreach (var key in RegistryKeys)
            {
                try
                {
                    using var registryKey = Registry.LocalMachine.OpenSubKey(key, writable: true);
                    if (registryKey == null) continue;

                    registryKey.SetValue("DisplayVersion", version, RegistryValueKind.String);
                    registryKey.SetValue("ProductVersion", dotVersion, RegistryValueKind.String);
                    registryKey.SetValue("FileVersion", dotVersion, RegistryValueKind.String);
                    Globals.WriteToLog($"Updated registry key: {key}");
                }
                catch (Exception ex)
                {
                    Globals.WriteToLog($"Failed to update registry key {key}: {ex.Message}");
                }
            }
        }

        private static int DateStringToInt(string s)
        {
            s = s.Replace("-", "").Replace("_", "");
            bool success = int.TryParse(s, out var i);
            return success ? i : 0;
        }

        private async static void UpdatePlatformsJson()
        {
            try
            {
                if (!AppSettings.AutoUpdatePlatforms) return;

                var basicPlatformsPath = Path.Join(Globals.AppDataFolder, "Platforms.json");

                // Get current version
                var currentPlatformsVersion = 0;
                var latestPlatformsVersion = 0;
                if (File.Exists(basicPlatformsPath))
                {
                    using JsonDocument document = JsonDocument.Parse(File.ReadAllText(basicPlatformsPath));
                    JsonElement root = document.RootElement;

                    if (root.TryGetProperty("Version", out JsonElement versionElement))
                        currentPlatformsVersion = DateStringToInt(versionElement.GetString());
                }

                // Get latest version from GitHub
                using HttpClient client = new();
                string latestPlatformsString = await client.GetStringAsync("https://raw.githubusercontent.com/TCNOco/TcNo-Acc-Switcher/master/TcNo-Acc-Switcher-Server/Platforms.json");

                using (JsonDocument document = JsonDocument.Parse(latestPlatformsString))
                {
                    JsonElement root = document.RootElement;

                    if (root.TryGetProperty("Version", out JsonElement versionElement))
                        latestPlatformsVersion = DateStringToInt(versionElement.GetString());
                }

                // Compare and copy if newer
                if (latestPlatformsVersion > currentPlatformsVersion)
                {
                    Globals.DeleteFile(basicPlatformsPath + ".bak");
                    Globals.CopyFile(basicPlatformsPath, basicPlatformsPath + ".bak");
                    Globals.WriteToLog("Updated Platforms.json");

                    using StreamWriter sw = new(basicPlatformsPath);
                    await sw.WriteAsync(latestPlatformsString);
                }

                // Download missing images
                var newPlatforms = "";
                using (JsonDocument document = JsonDocument.Parse(File.ReadAllText(basicPlatformsPath))) {
                    JsonElement root = document.RootElement;

                    // Iterate over root > "Platforms". Each key is the name we're looking for.
                    foreach (var platform in root.GetProperty("Platforms").EnumerateObject())
                    {
                        string platformName = platform.Name;
                        string platformImage = Globals.GetCleanFilePath(platformName);
                        string platformImageUrl = $"https://raw.githubusercontent.com/TCNOco/TcNo-Acc-Switcher/master/TcNo-Acc-Switcher-Server/wwwroot/img/platform/{platformImage}.svg";

                        if (string.IsNullOrEmpty(platformImage)) continue;

                        string imagePath = Path.Join(Globals.UserDataFolder, "wwwroot", "img", "platform", platformImage + ".svg");

                        if (!File.Exists(imagePath))
                        {
                            try
                            {
                                // Download latest image from GitHub:
                                byte[] imageBytes = await client.GetByteArrayAsync(platformImageUrl);

                                using FileStream fs = new(imagePath, FileMode.Create);
                                await fs.WriteAsync(imageBytes);

                                newPlatforms += platformName + ", ";
                            }
                            catch (Exception ex)
                            {
                                Globals.WriteToLog($"Failed to download image for {platformName}: {ex.Message}");
                            }
                        }
                    }
                }

                if (newPlatforms.Length != 0)
                {
                    _ = GeneralInvocableFuncs.ShowToast("info", Lang.Instance["Toast_NewPlatforms", new { listOfNames = newPlatforms }], renderTo: "toastarea", duration: 000);

                    Globals.WriteToLog($"Updatedd Platforms.json. Added new platforms: {newPlatforms}");
                }
            }
            catch (Exception ex)
            {
                Globals.WriteToLog($"Failed to update Platforms.json: {ex.Message}");
            }
        }
    }
}

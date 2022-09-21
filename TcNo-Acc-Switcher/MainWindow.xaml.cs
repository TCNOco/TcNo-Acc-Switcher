using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Reflection;
using System.Security.Principal;
using System.Text;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Data;
using System.Windows.Documents;
using System.Windows.Input;
using System.Windows.Media;
using System.Windows.Media.Imaging;
using System.Windows.Navigation;
using Microsoft.Extensions.DependencyInjection;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher.State;
using TcNo_Acc_Switcher.State.Interfaces;


namespace TcNo_Acc_Switcher
{
    /// <summary>
    /// Interaction logic for MainWindow.xaml
    /// </summary>
    public partial class MainWindow : Window
    {
        public MainWindow()
        {
            // Crash handler
            AppDomain.CurrentDomain.UnhandledException += CrashHandler;
            
            InitializeFiles();
            
            InitializeComponent();

            var serviceCollection = new ServiceCollection();
            serviceCollection.AddWpfBlazorWebView();

            // Proper singletons. This is the correct load order.
            serviceCollection.AddSingleton<IWindowSettings, WindowSettings>(); // #1 - NONE
            serviceCollection.AddSingleton<ILang, Lang>(); // After WindowSettings
            serviceCollection.AddSingleton<IToasts, Toasts>(); // After Lang
            serviceCollection.AddSingleton<IModals, Modals>(); // After Lang
            serviceCollection.AddSingleton<IStatistics, Statistics>(); // After WindowSettings
            serviceCollection.AddSingleton<IAppState, AppState>(); // Lang, Modals, Statistics, Toasts, WindowSettings
            serviceCollection.AddSingleton<ISharedFunctions, SharedFunctions>(); // AppState, Lang, Modals, Statistics, Toasts (+JsRuntime)

            serviceCollection.AddSingleton<IGameStatsRoot, GameStatsRoot>(); // Toasts, WindowSettings
            serviceCollection.AddSingleton<IGameStats, GameStats>(); // AppState, GameStatsRoot

            serviceCollection.AddSingleton<ISteamSettings, SteamSettings>(); // (No depends)
            serviceCollection.AddSingleton<ISteamState, SteamState>(); // AppState, GameStats, Lang, Modals, SharedFunctions, Statistics, SteamSettings, Toasts
            serviceCollection.AddSingleton<ISteamFuncs, SteamFuncs>(); // AppState, Lang, Modals, SharedFunctions, Statistics, SteamSettings, SteamState, Toasts, WindowSettings (+JSRuntime)

            serviceCollection.AddSingleton<ITemplatedPlatformState, TemplatedPlatformState>(); // AppState, GameStats, Modals, SharedFunctions, Statistics, Toasts
            serviceCollection.AddSingleton<ITemplatedPlatformSettings, TemplatedPlatformSettings>(); // Statistics, TemplatedPlatformState
            serviceCollection.AddSingleton<ITemplatedPlatformFuncs, TemplatedPlatformFuncs>(); // AppState, Lang, Modals, SharedFunctions, Statistics, TemplatedPlatformState, TemplatedPlatformSettings, Toasts, WindowSettings (+JSRuntime, NavigationManager)
#if DEBUG
            serviceCollection.AddBlazorWebViewDeveloperTools();
#endif
            
            Resources.Add("services", serviceCollection.BuildServiceProvider());
        }

        private void CrashHandler(object sender, UnhandledExceptionEventArgs e)
        {
            if (e.ExceptionObject.ToString()!.Contains("WebView2RuntimeNotFoundException"))
            {
                // Does not have WebView2 installed
                // Show error and download link
                if (MessageBox.Show(
                        "WebView2 Runtime is not installed. Click Yes to visit the Microsoft download page.",
                        "WebView2 Runtime not found!",
                        MessageBoxButton.YesNo,
                        MessageBoxImage.Error) == MessageBoxResult.Yes)
                {
                    Process.Start(new ProcessStartInfo { FileName = "https://developer.microsoft.com/en-us/microsoft-edge/webview2/#download-section", UseShellExecute = true });
                    MessageBox.Show("Download and install the 'Evergreen Bootstrapper", "WebView2 Install Tip", MessageBoxButton.OK, MessageBoxImage.Information);
                }
                return;
            }
            
            Globals.CurrentDomain_UnhandledException(sender, e);

            // Show message
            MessageBox.Show($"This crash log will be submitted on the next launch. See %AppData%\\TcNo Account Switcher\\LastError.txt for more details.{Environment.NewLine + Environment.NewLine + e.GetType().FullName + Environment.NewLine + Environment.NewLine}If you manually report this error: please include a copy of the log file!", "Fatal error", MessageBoxButton.OK, MessageBoxImage.Error);
        }

        #region Init files (Windows)
        /// <summary>
        /// Set userdata folder as working directory, and make sure can access own files.
        /// </summary>
        private void InitializeFiles(){
            // Set working directory to "%AppData%\TcNo Account Switcher"
            if (!Directory.Exists(Globals.UserDataFolder)) Directory.CreateDirectory(Globals.UserDataFolder);
            Directory.SetCurrentDirectory(Globals.UserDataFolder);

            if (Globals.InstalledToProgramFiles() && !IsAdmin() || !Globals.HasFolderAccess(Globals.AppDataFolder))
                Restart("", true);
            // Previously wwwroot was moved to AppData, but for ease-of-use and deduplication:
            //  - The program will now install to Local AppData,
            //  - The data will still be located in Roaming AppData.
            // Updates will no longer need admin, as well as a few other things. Makes life easier for all.
            // Same as Discord :)
        }
        
        private static bool IsAdmin()
        {
            // Checks whether program is running as Admin or not
            var securityIdentifier = WindowsIdentity.GetCurrent().Owner;
            return securityIdentifier is not null && securityIdentifier.IsWellKnown(WellKnownSidType.BuiltinAdministratorsSid);
        }
        private static void Restart(string args = "", bool admin = false)
        {
            var proc = new ProcessStartInfo
            {
                WorkingDirectory = Environment.CurrentDirectory,
                FileName = Assembly.GetEntryAssembly()?.Location.Replace(".dll", ".exe") ?? "TcNo-Acc-Switcher_main.exe",
                UseShellExecute = true,
                Arguments = args,
                Verb = admin ? "runas" : ""
            };
            try
            {
                _ = Process.Start(proc);
                Environment.Exit(0);
            }
            catch (Exception ex)
            {
                Globals.WriteToLog(@"This program must be run as an administrator!" + Environment.NewLine + ex);
                Environment.Exit(0);
            }
        }
        #endregion
    }
}

using System;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.ComponentModel;
using System.Diagnostics;
using System.Drawing;
using System.IO;
using System.Linq;
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
using System.Windows.Shapes;
using System.Threading;
using System.Windows.Interop;
using Microsoft.Web.WebView2.Core;
using Microsoft.Web.WebView2.Wpf;
using Microsoft.Win32;
using Microsoft.Win32.TaskScheduler;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Server.Pages.Steam;
using TcNo_Acc_Switcher_Server.Shared;
using TcNo_Acc_Switcher_Client.Classes;
using TcNo_Acc_Switcher_Server;
using TcNo_Acc_Switcher_Server.Pages.General;
using Index = TcNo_Acc_Switcher_Server.Pages.Index;
using Path = System.IO.Path;
using Point = System.Windows.Point;
using Strings = TcNo_Acc_Switcher_Client.Localisation.Strings;

namespace TcNo_Acc_Switcher_Client
{
    /// <summary>
    /// Interaction logic for MainWindow.xaml
    /// </summary>

    public partial class MainWindow : Window
    {
        private readonly Thread _server = new Thread(RunServer);
        private static void RunServer() { Program.Main(new string[0]); }
        private readonly TrayUsers _trayUsers = new TrayUsers();
        private JObject _settings = new JObject();


        public MainWindow()
        {
            // Start web server
            _server.IsBackground = true;
            _server.Start();
            
            // Initialise and connect to web server above
            // Somehow check ports and find a different one if it doesn't work? We'll see...
            InitializeComponent();

            // Load settings (If they exist, otherwise creates).
            _settings = GeneralFuncs.LoadSettings("WindowSetting");


            //MView2.Source = new Uri("http://localhost:44305/");
            MView2.Source = new Uri("http://localhost:5000/");
            MView2.NavigationStarting += UrlChanged;
            MView2.CoreWebView2InitializationCompleted += WebView_CoreWebView2Ready;
            //MView2.MouseDown += MViewMDown;

            Point windowSize = Point.Parse((string)_settings["WindowSize"] ?? "800,450");
            this.Width = windowSize.X;
            this.Height = windowSize.Y;
            // Each window in the program would have its own size. IE Resize for Steam, and more.
        }

        #region Windows Shortcuts
        //private void CheckShortcuts()
        //{
        //    //MainViewmodel.DesktopShortcut = ;
        //   //MainViewmodel.StartWithWindows = CheckStartWithWindows();
        //    //MainViewmodel.StartMenuIcon = ShortcutExist(System.IO.Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.Programs), @"TcNo Account Switcher\"));
        //}

        // Library class used to see if a shortcut exists.
        private static bool ShortcutExist(string location) => File.Exists(System.IO.Path.Combine(location, "TcNo Account Switcher - Steam.lnk"));
        private static string ParentDirectory(string dir) =>
            dir.Substring(0, dir.LastIndexOf(System.IO.Path.DirectorySeparatorChar));
        private static void WriteShortcut(string exe, string selfLocation, string iconDirectory, string description, string settingsLink, string arguments)
        {
            if (File.Exists(settingsLink)) return;
            if (File.Exists("CreateShortcut.vbs"))
                File.Delete("CreateShortcut.vbs");
            
            string[] lines = {"set WshShell = WScript.CreateObject(\"WScript.Shell\")",
                "set oShellLink = WshShell.CreateShortcut(\"" + settingsLink  + "\")",
                "oShellLink.TargetPath = \"" + exe + "\"",
                "oShellLink.WindowStyle = 1",
                "oShellLink.IconLocation = \"" + iconDirectory + "\"",
                "oShellLink.Description = \"" + description + "\"",
                "oShellLink.WorkingDirectory = \"" + selfLocation + "\"",
                "oShellLink.Arguments = \"" + arguments + "\"",
                "oShellLink.Save()"
            };
            File.WriteAllLines("CreateShortcut.vbs", lines);

            var vbsProcess = new Process
            {
                StartInfo =
                {
                    FileName = "cscript",
                    Arguments = "//nologo \"" + System.IO.Path.GetFullPath("CreateShortcut.vbs") + "\"",
                    UseShellExecute = false,
                    RedirectStandardOutput = true,
                    CreateNoWindow = true
                }
            };


            vbsProcess.Start();
            vbsProcess.StandardOutput.ReadToEnd();
            vbsProcess.Close();

            File.Delete("CreateShortcut.vbs");
            MessageBox.Show("Shortcut created!\n\nLocation: " + settingsLink);
        }

        private static void CreateShortcut(string location)
        {
            Directory.CreateDirectory(location);
            // Starts the main picker, with the Steam argument.
            string selfExe = System.IO.Path.Combine(ParentDirectory(Directory.GetCurrentDirectory()), "TcNo Account Switcher.exe"),
                selfLocation = ParentDirectory(Directory.GetCurrentDirectory()),
                iconDirectory = System.IO.Path.Combine(selfLocation, "wwwroot/favicon.ico"),
                settingsLink = System.IO.Path.Combine(location, "TcNo Account Switcher - Steam.lnk");
            const string description = "TcNo Account Switcher - Steam";
            const string arguments = "+steam";

            WriteShortcut(selfExe, selfLocation, iconDirectory, description, settingsLink, arguments);
        }
        private static void DeleteShortcut(string location, string name, bool delFolder)
        {
            var settingsLink = System.IO.Path.Combine(location, name);
            if (File.Exists(settingsLink))
                File.Delete(settingsLink);
            if (delFolder)
            {
                if (Directory.GetFiles(location).Length == 0)
                    Directory.Delete(location);
                else
                    MessageBox.Show($"{Strings.ErrDeleteFolderNonempty} {location}");
            }
            MessageBox.Show(Strings.InfoShortcutDeleted.Replace("{}", name));
        }
        private static void CreateTrayShortcut(string location)
        {
            string selfExe = System.IO.Path.Combine(Directory.GetCurrentDirectory(), "TcNo Acc Switcher SteamTray.exe"),
                selfLocation = Directory.GetCurrentDirectory(),
                iconDirectory = System.IO.Path.Combine(selfLocation, "icon.ico"),
                settingsLink = System.IO.Path.Combine(location, "TcNo Account Switcher - Steam tray.lnk");
            const string description = "TcNo Account Switcher - Steam tray";
            const string arguments = "";

            WriteShortcut(selfExe, selfLocation, iconDirectory, description, settingsLink, arguments);
        }

        // ICON - Desktop Icon
        private static bool DesktopShortcut_Exists() =>
            ShortcutExist(Environment.GetFolderPath(Environment.SpecialFolder.DesktopDirectory));
        private static void DesktopShortcut_Toggle()
        {
            var desktopPath = Environment.GetFolderPath(Environment.SpecialFolder.DesktopDirectory);
            if (!DesktopShortcut_Exists())
                CreateShortcut(desktopPath);
            else
                DeleteShortcut(desktopPath, "TcNo Account Switcher - Steam.lnk", false);
        }
        // -------------------

        // ICON - Start Menu
        private static bool StartMenuIcon_Exists() => ShortcutExist(
            System.IO.Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.Programs),
                @"TcNo Account Switcher\"));
        private static void StartMenuIcon_Toggle()
        {
            string programsPath = Environment.GetFolderPath(Environment.SpecialFolder.Programs),
                shortcutFolder = System.IO.Path.Combine(programsPath, @"TcNo Account Switcher\");
            if (!StartMenuIcon_Exists())
            {
                CreateShortcut(shortcutFolder);
                CreateTrayShortcut(shortcutFolder);
            }
            else
            {
                DeleteShortcut(shortcutFolder, "TcNo Account Switcher - Steam.lnk", false);
                DeleteShortcut(shortcutFolder, "TcNo Account Switcher - Steam tray.lnk", true);
            }
        }
        // -------------------

        // TRAY - Run when Windows starts
        private static bool StartWithWindows_Enabled()
        {
            using (var ts = new TaskService())
            {
                var tasks = ts.RootFolder.Tasks;
                return tasks.Exists("TcNo Account Switcher - Tray start with logon");
            }
        }
        private static void StartWithWindows_Toggle()
        {
            if (!StartWithWindows_Enabled())
            {
                var ts = new TaskService();
                var td = ts.NewTask();
                td.Principal.RunLevel = TaskRunLevel.Highest;
                td.Triggers.AddNew(TaskTriggerType.Logon);
                var programPath = System.IO.Path.GetFullPath("TcNo Acc Switcher SteamTray.exe");
                td.Actions.Add(new ExecAction(programPath));
                ts.RootFolder.RegisterTaskDefinition("TcNo Account Switcher - Steam Tray start with logon", td);
                MessageBox.Show(Strings.InfoTrayWindowsStart);
            }
            else
            {
                var ts = new TaskService();
                ts.RootFolder.DeleteTask("TcNo Account Switcher - Steam Tray start with logon");
                MessageBox.Show(Strings.InfoTrayWindowsStartOff);
            }
        }
        // -------------------
        #endregion


        
        

        // For draggable regions:
        // https://github.com/MicrosoftEdge/WebView2Feedback/issues/200
        private void WebView_CoreWebView2Ready(object sender, EventArgs e)
        {
            var eventForwarder = new Headerbar.EventForwarder(new WindowInteropHelper(this).Handle);

            MView2.CoreWebView2.AddHostObjectToScript("eventForwarder", eventForwarder);
        }

        private void SaveSettings(string windowUrl)
        {
            // IN THE FUTURE: ONLY DO THIS FOR THE MAIN PAGE WHERE YOU CAN CHOOSE WHAT PLATFORM TO SWAP ACCOUNTS ON
            // This will only be when that's implimented. Easier to leave it until then.
            MessageBox.Show(windowUrl);
            _settings["WindowSize"] = Convert.ToInt32(this.Width).ToString() + ',' + Convert.ToInt32(this.Height).ToString();
            GeneralFuncs.SaveSettings("WindowSetting", _settings);
        }

        private void UrlChanged(object sender, CoreWebView2NavigationStartingEventArgs args)
        {
            var uri = args.Uri.Split("/").Last();
            Console.WriteLine(args.Uri);
            switch (uri)
            {
                case "Win_min":
                    args.Cancel = true;
                    this.WindowState = WindowState.Minimized;
                    break;
                case "Win_max":
                    args.Cancel = true;
                    this.WindowState = WindowState.Maximized;
                    break;
                case "Win_restore":
                    args.Cancel = true;
                    this.WindowState = WindowState.Normal;
                    break;
                case "Win_close":
                    args.Cancel = true;
                    SaveSettings(args.Uri);
                    Environment.Exit(1);
                    break;
            }
            if (uri.Contains("Win_min"))
            {
                args.Cancel = true;
                this.WindowState = WindowState.Minimized;
            }

            if (args.Uri.Contains("?"))
            {
                // Needs to be here as:
                // Importing Microsoft.Win32 and System.Windows didn't get OpenFileDialog to work.
                var uriArg = args.Uri.Split("?").Last();
                if (uriArg.StartsWith("selectFile"))
                {
                    args.Cancel = true;
                    var argValue = uriArg.Split("=")[1];
                    var dlg = new OpenFileDialog
                    {
                        FileName = Path.GetFileNameWithoutExtension(argValue),
                        DefaultExt = Path.GetExtension(argValue),
                        Filter = $"{argValue}|{argValue}"
                    };

                    var result = dlg.ShowDialog();
                    if (result != true) return;
                    MView2.ExecuteScriptAsync("Modal_RequestedLocated(true)");
                    MView2.ExecuteScriptAsync("Modal_SetFilepath(" + JsonConvert.SerializeObject(dlg.FileName.Substring(0, dlg.FileName.LastIndexOf('\\'))) + ")");
                    //VerifySteamPath();
                }
            }

        }
        public static async Task<string> ExecuteScriptFunctionAsync(WebView2 webView2, string functionName, params object[] parameters)
        {
            string script = functionName + "(";
            for (int i = 0; i < parameters.Length; i++)
            {
                script += JsonConvert.SerializeObject(parameters[i]);
                if (i < parameters.Length - 1)
                {
                    script += ", ";
                }
            }
            script += ");";
            return await webView2.ExecuteScriptAsync(script);
        }
    }
}

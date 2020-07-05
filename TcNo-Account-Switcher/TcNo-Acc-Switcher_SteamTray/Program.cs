using System;
using System.Linq;
using System.Windows.Forms;
using TcNo_Acc_Switcher_SteamTray.Properties;
using System.Drawing;
using System.IO;
using System.Diagnostics;
using System.Runtime.InteropServices;
using TcNo_Acc_Switcher_Globals;

namespace TcNo_Acc_Switcher_SteamTray
{
    internal static class Program
    {
        /// <summary>
        ///  The main entry point for the application.
        /// </summary>
        public static TrayUsers TrayUsers = new TrayUsers();
        
        [STAThread]
        private static void Main()
        {
            // Crash handler
            AppDomain.CurrentDomain.UnhandledException += Globals.CurrentDomain_UnhandledException;
            if (SelfAlreadyRunning())
            {
                Console.WriteLine(@"TcNo Account Switcher SteamTray is already running");
                Environment.Exit(99);
            }
            Directory.SetCurrentDirectory(Path.GetDirectoryName(System.Reflection.Assembly.GetEntryAssembly().Location)); // Set working directory to the same as the actual .exe
            TrayUsers.LoadTrayUsers();
            Application.EnableVisualStyles();
            Application.SetCompatibleTextRenderingDefault(false);
            Application.Run(new AppCont());
        }
        private static bool SelfAlreadyRunning()
        {
            var processes = Process.GetProcesses();
            var currentProc = Process.GetCurrentProcess();
            return processes.Any(process => currentProc.ProcessName == process.ProcessName && currentProc.Id != process.Id);
        }
    }
    public class AppCont : ApplicationContext
    {
        private readonly string _mainProgram = Path.Combine(Path.GetDirectoryName(Process.GetCurrentProcess().MainModule.FileName), "TcNo Account Switcher Steam.exe");

        private readonly NotifyIcon _trayIcon;

        public AppCont()
        {
            var contextMenu = new ContextMenuStrip {Renderer = new ContentRenderer()};
            contextMenu.Items.Add(new ToolStripMenuItem()
            {
                Name = "START",
                Text = "Start TcNo Steam Account Switcher",
                ForeColor = Color.White,
                BackColor = Color.FromArgb(255, 34, 34, 34)
            });
            // Insert last 3 used accounts from json file
            // Name will be number from top.
            // Then start main program to switch? Yeah.
            // - Launch argument
            if (File.Exists("Tray_Users.json"))
                if (Program.TrayUsers.ListTrayUsers != null && Program.TrayUsers.ListTrayUsers.Count >= 1)
                {
                    Program.TrayUsers.ListTrayUsers.Reverse(); // Last item was last to be logged into.
                    foreach (var trayUser in Program.TrayUsers.ListTrayUsers)
                        contextMenu.Items.Add(new ToolStripMenuItem()
                        {
                            Name = trayUser.SteamId,
                            Text = $@"Switch to: {trayUser.DisplayAs}",
                            ForeColor = Color.White,
                            BackColor = Color.FromArgb(255, 34, 34, 34)
                        });
                }

            contextMenu.Items.Add(new ToolStripMenuItem()
            {
                Name = "EXIT",
                Text = "Exit",
                ForeColor = Color.White,
                BackColor = Color.FromArgb(255, 34, 34, 34)
            });
            contextMenu.ItemClicked += contextMenu_ItemClicked;


            // Initialize Tray Icon
            _trayIcon = new NotifyIcon()
            {
                Icon = Resources.icon,
                ContextMenuStrip = contextMenu,
                Visible = true
            };
            _trayIcon.DoubleClick += NotifyIcon_DoubleClick;
        }

        private void contextMenu_ItemClicked(object sender, ToolStripItemClickedEventArgs e)
        {
            var item = e.ClickedItem;
            if (item.Name != "START")
            {
                if (item.Name == "EXIT")
                {
                    _trayIcon.Visible = false;
                    CloseMainProcess();
                    Application.Exit();
                }
                else if (Program.TrayUsers.AlreadyInList(item.Name))
                        StartSwitcher($"+{item.Name} quit");
            }
            else
                StartSwitcher("");
        }
        private void NotifyIcon_DoubleClick(object sender, EventArgs e) => StartSwitcher("");

        private static bool AlreadyRunning() => Process.GetProcessesByName("TcNo Account Switcher Steam").Length > 0;

        // Start with Windows login, using https://stackoverflow.com/questions/15191129/selectively-disabling-uac-for-specific-programs-on-windows-programatically for automatic administrator.
        // Adding to Start Menu shortcut also creates "Start in Tray", which is a shortcut to this program. 


        [DllImport("user32")]
        private static extern bool SetForegroundWindow(IntPtr hwnd);
        [DllImport("user32")]
        private static extern bool ShowWindow(IntPtr hWnd, int nCmdShow);

        private static void BringToFront()
        {
            var proc = Process.GetProcessesByName("TcNo Account Switcher Steam").FirstOrDefault();
            if (proc == null || proc.MainWindowHandle == IntPtr.Zero) return;
            const int swRestore = 9;
            ShowWindow(proc.MainWindowHandle, swRestore);
            SetForegroundWindow(proc.MainWindowHandle);
        }

        private static void CloseMainProcess()
        {
            var proc = Process.GetProcessesByName("TcNo Account Switcher Steam").FirstOrDefault();
            if (proc == null || proc.MainWindowHandle == IntPtr.Zero) return;
            proc.CloseMainWindow();
            proc.WaitForExit();
        }
        private void StartSwitcher(string args) {
            if (AlreadyRunning())
                BringToFront();
            else
            {
                var processName = _mainProgram;
                if (File.Exists(_mainProgram))
                {
                    var startInfo = new ProcessStartInfo();
                    try
                    {
                        startInfo.FileName = processName;
                        startInfo.CreateNoWindow = false;
                        startInfo.UseShellExecute = false;
                        startInfo.Arguments = args;
                        Process.Start(startInfo);
                    }
                    catch (System.ComponentModel.Win32Exception win32Exception)
                    {
                        if (win32Exception.HResult != -2147467259) throw; // Throw is error is not: Requires elevation
                        try
                        {
                            startInfo.UseShellExecute = true;
                            startInfo.Verb = "runas";
                            Process.Start(startInfo);
                        }

                        catch (System.ComponentModel.Win32Exception win32Exception2)
                        {
                            if (win32Exception2.HResult != -2147467259) throw; // Throw is error is not: cancelled by user
                        }
                    }
                }
                else
                    MessageBox.Show("Could not open the main .exe. Make sure it exists.\n\nI attempted to open: " + _mainProgram, "TcNo Account Switcher - Tray launch fail");
            }
        }

        private void Exit(object sender, EventArgs e)
        {
            _trayIcon.Visible = false;
            Application.Exit();
        }

        #region ContextMenuStrip Style Section
        private class ContentRenderer : ToolStripProfessionalRenderer
        {
            public ContentRenderer() : base(new MyColors()) { }
        }

        private class MyColors : ProfessionalColorTable
        {
            public override Color MenuItemSelected => Color.FromArgb(255, 24, 24, 24);
            public override Color MenuItemSelectedGradientBegin => Color.FromArgb(255, 34, 34, 34);
            public override Color MenuItemSelectedGradientEnd => Color.FromArgb(255, 34, 34, 34);
        }
        #endregion
    }
}

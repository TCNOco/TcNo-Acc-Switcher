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
using System.Collections.Generic;
using System.ComponentModel;
using System.Diagnostics;
using System.Drawing;
using System.IO;
using System.Linq;
using System.Windows.Forms;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Tray.Properties;

namespace TcNo_Acc_Switcher_Tray
{
    internal static class Program
    {
        public static Dictionary<string, List<TrayUser>> TrayUsers = new();
        public static string LastHash = "";

        /// <summary>
        ///  The main entry point for the application.
        /// </summary>
        [STAThread]
        private static void Main()
        {
            // Crash handler
            AppDomain.CurrentDomain.UnhandledException += Globals.CurrentDomain_UnhandledException;
            if (SelfAlreadyRunning())
            {
                Environment.Exit(1056); // An instance of the service is already running.
            }

            // Set working directory to documents folder
            Directory.SetCurrentDirectory(Globals.UserDataFolder);
            TrayUsers = TrayUser.ReadTrayUsers();

            _ = Application.SetHighDpiMode(HighDpiMode.SystemAware);
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
        private readonly string _mainProgram = Path.Join(Path.GetDirectoryName(Process.GetCurrentProcess().MainModule?.FileName)!, "TcNo-Acc-Switcher_main.exe");

        private NotifyIcon _trayIcon;

        public AppCont()
        {
            InitMenu();
        }

        private void InitMenu(bool first = true)
        {
            if (!File.Exists("Tray_Users.json"))
            {
                _ = MessageBox.Show(GLang.Instance["Tray_NoAccs"],
                    GLang.Instance["Tray_Error"], MessageBoxButtons.OK, MessageBoxIcon.Error);
                Environment.Exit(0);
            }
            if (string.Equals(Program.LastHash, Globals.GetFileMd5("Tray_Users.json"), StringComparison.Ordinal)) return; // Don't init again
            Program.LastHash = Globals.GetFileMd5("Tray_Users.json");

            Program.TrayUsers = TrayUser.ReadTrayUsers();

            var contextMenu = new ContextMenuStrip { Renderer = new ContentRenderer() };
            _ = contextMenu.Items.Add(new ToolStripMenuItem
            {
                Name = "START",
                Text = @"TcNo Account Switcher",
                ForeColor = Color.White,
                BackColor = Color.FromArgb(255, 34, 34, 34)
            });

            foreach (var (key, value) in Program.TrayUsers)
            {
                var tsi = (ToolStripMenuItem)contextMenu.Items.Add(key, null, null);
                tsi.ForeColor = Color.White;
                tsi.BackColor = Color.FromArgb(255, 34, 34, 34);
                foreach (var trayUsers in value)
                {
                    _ = tsi.DropDownItems.Add(new ToolStripMenuItem
                    {
                        Name = trayUsers.Arg,
                        Text = GLang.Instance["Tray_Switch", new { account = trayUsers.Name}],
                        ForeColor = Color.White,
                        BackColor = Color.FromArgb(255, 34, 34, 34)
                    });
                }
                tsi.DropDownItemClicked += ContextMenu_ItemClicked;
            }

            _ = contextMenu.Items.Add(new ToolStripMenuItem
            {
                Name = "EXIT",
                Text = @"Exit",
                ForeColor = Color.White,
                BackColor = Color.FromArgb(255, 34, 34, 34)
            });
            contextMenu.ItemClicked += ContextMenu_ItemClicked;


            // Initialise Tray Icon
            if (first)
                _trayIcon = new NotifyIcon
                {
                    Icon = Resources.icon,
                    ContextMenuStrip = contextMenu,
                    Visible = true
                };
            else
                _trayIcon.ContextMenuStrip = contextMenu;

            _trayIcon.MouseDown += TrayIconOnMouseDown;
            _trayIcon.DoubleClick += NotifyIcon_DoubleClick;
        }

        private void TrayIconOnMouseDown(object sender, MouseEventArgs e)
        {
            if (e.Button == MouseButtons.Right)
                InitMenu(false);
        }

        private void ContextMenu_ItemClicked(object sender, ToolStripItemClickedEventArgs e)
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
                else StartSwitcher($"{item.Name} quit");
            }
            else
                StartSwitcher();
        }
        private void NotifyIcon_DoubleClick(object sender, EventArgs e) => StartSwitcher();

        private static bool AlreadyRunning() => Process.GetProcessesByName("TcNo-Acc-Switcher_main").Length > 0;

        // Start with Windows login, using https://stackoverflow.com/questions/15191129/selectively-disabling-uac-for-specific-programs-on-windows-programatically for automatic administrator.
        // Adding to Start Menu shortcut also creates "Start in Tray", which is a shortcut to this program.


        private static void CloseMainProcess()
        {
            var proc = Process.GetProcessesByName("TcNo-Acc-Switcher_main").FirstOrDefault();
            if (proc == null || proc.MainWindowHandle == IntPtr.Zero) return;
            _ = proc.CloseMainWindow();
            proc.WaitForExit();
        }
        private void StartSwitcher(string args = "")
        {
            if (AlreadyRunning() && args == "")
                _ = NativeFuncs.BringToFront();
            else
            {
                if (File.Exists(_mainProgram))
                {
                    var startInfo = new ProcessStartInfo();
                    try
                    {
                        startInfo.FileName = _mainProgram;
                        startInfo.CreateNoWindow = false;
                        startInfo.UseShellExecute = false;
                        startInfo.Arguments = args;
                        _ = Process.Start(startInfo);
                    }
                    catch (Win32Exception win32Exception)
                    {
                        if (win32Exception.HResult != -2147467259) throw; // Throw is error is not: Requires elevation
                        try
                        {
                            startInfo.UseShellExecute = true;
                            startInfo.Verb = "runas";
                            _ = Process.Start(startInfo);
                        }

                        catch (Win32Exception win32Exception2)
                        {
                            if (win32Exception2.HResult != -2147467259) throw; // Throw is error is not: cancelled by user
                        }
                    }
                }
                else
                    _ = MessageBox.Show(@$"{GLang.Instance["Tray_CantOpenExe"]} {_mainProgram}", GLang.Instance["Tray_LaunchFail"]);
            }
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

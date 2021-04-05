using System;
using System.Collections.Generic;
using System.Configuration;
using System.Data;
using System.Linq;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Input;
using System.Windows.Shapes;
using TcNo_Acc_Switcher_Client.Classes;

namespace TcNo_Acc_Switcher_Client
{
    /// <summary>
    /// Interaction logic for App.xaml
    /// </summary>
    public partial class App : Application
    {
        private readonly TrayUsers _trayUsers = new TrayUsers();
        public static string StartPage = "";

        protected override void OnStartup(StartupEventArgs e)
        {
            base.OnStartup(e);
            var quitArg = false;
            for (var i = 0; i != e.Args.Length; ++i)
            {
                if (e.Args[i]?[0] == '+')
                {
                    var steamId = e.Args[i].Substring(1);// Get SteamID from launch options
                    _trayUsers.LoadTrayUsers();
                    var accName = _trayUsers.GetAccName(steamId); // Get account name from JSON

                    TcNo_Acc_Switcher_Server.Pages.Steam.SteamSwitcherFuncs.SwapSteamAccounts( steamId, accName);
                    quitArg = true;
                }
                else switch (e.Args[i])
                {
                    case "steam":
                        StartPage = "Steam";
                        break;
                    case "logout":
                        TcNo_Acc_Switcher_Server.Pages.Steam.SteamSwitcherFuncs.SwapSteamAccounts("", "");
                        quitArg = true;
                        break;
                    case "quit":
                        quitArg = true;
                        break;
                    default:
                        Console.WriteLine($"Unknown argument: \"{e.Args[i]}\"");
                        break;
                }
            }

            if (quitArg)
                Environment.Exit(1);
        }

        #region ResizeWindows
        // https://stackoverflow.com/a/27157947/5165437
        bool _resizeInProcess = false;
        private void Resize_Init(object sender, MouseButtonEventArgs e)
        {
            var senderRect = sender as Rectangle;
            if (senderRect != null)
            {
                _resizeInProcess = true;
                senderRect.CaptureMouse();
            }
        }

        private void Resize_End(object sender, MouseButtonEventArgs e)
        {
            var senderRect = sender as Rectangle;
            if (senderRect != null)
            {
                _resizeInProcess = false; ;
                senderRect.ReleaseMouseCapture();
            }
        }

        private void Resizeing_Form(object sender, MouseEventArgs e)
        {
            if (_resizeInProcess)
            {
                var senderRect = sender as Rectangle;
                var mainWindow = senderRect.Tag as Window;
                if (senderRect != null)
                {
                    var width = e.GetPosition(mainWindow).X;
                    var height = e.GetPosition(mainWindow).Y;
                    senderRect.CaptureMouse();
                    if (senderRect.Name.ToLower().Contains("right"))
                    {
                        width += 5;
                        if (width > 0)
                            mainWindow.Width = width;
                    }
                    if (senderRect.Name.ToLower().Contains("left"))
                    {
                        width -= 5;
                        mainWindow.Left += width;
                        width = mainWindow.Width - width;
                        if (width > 0)
                        {
                            mainWindow.Width = width;
                        }
                    }
                    if (senderRect.Name.ToLower().Contains("bottom"))
                    {
                        height += 5;
                        if (height > 0)
                            mainWindow.Height = height;
                    }
                    if (senderRect.Name.ToLower().Contains("top"))
                    {
                        height -= 5;
                        mainWindow.Top += height;
                        height = mainWindow.Height - height;
                        if (height > 0)
                        {
                            mainWindow.Height = height;
                        }
                    }
                }
            }
        }
        #endregion
    }
}

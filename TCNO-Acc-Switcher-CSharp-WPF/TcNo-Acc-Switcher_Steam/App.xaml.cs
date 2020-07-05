using System;
using System.IO;
using System.Windows;

namespace TcNo_Acc_Switcher_Steam
{
    /// <summary>
    /// Interaction logic for App.xaml
    ///
    /// Also contains launch options/argument handling
    /// Possible launch options:
    /// +[SteamId]  : Sets selected account as 'last login', and opens Steam.
    /// logout      : Sets no account to 'last login' state, to allow another account to be signed in.
    /// quit        : Closes TcNo Steam Account Switcher upon argument handling completion.
    ///
    /// Launch options can be used with shortcuts.
    /// - Ability to create these shortcuts may be added to the right-click menu at some stage in the future.
    /// 
    /// </summary>
    public partial class App
    {
        private readonly TrayUsers _trayUsers = new TrayUsers();
        protected override void OnStartup(StartupEventArgs e)
        {
            base.OnStartup(e);
            var mainWindow = new MainWindow();
            var quitArg = false;
            for (var i = 0; i != e.Args.Length; ++i)
            {
                if (e.Args[i]?[0] == '+')
                {
                    if (!File.Exists("Tray_Users.json"))
                    {
                        MessageBox.Show("Error! There are no accounts for the Tray menu!");
                        Environment.Exit(1);
                    }

                    var steamId = e.Args[i].Substring(1);// Get SteamID from launch options
                    _trayUsers.LoadTrayUsers();
                    var accName = _trayUsers.GetAccName(steamId); // Get account name from JSON

                    mainWindow.SwapSteamAccounts(false, steamId, accName);
                }
                else switch (e.Args[i])
                {
                    case "logout":
                        mainWindow.SwapSteamAccounts(true, "", "");
                        break;
                    case "quit":
                        quitArg = true;
                        break;
                }
            }

            if (quitArg)
                Environment.Exit(1);
            this.MainWindow = mainWindow;
            mainWindow.Show();
        }
    }
}

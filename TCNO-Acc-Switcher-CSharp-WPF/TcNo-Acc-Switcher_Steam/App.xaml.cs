using System;
using System.Collections.Generic;
using System.Configuration;
using System.Data;
using System.IO;
using System.Linq;
using System.Reflection;
using System.Threading.Tasks;
using System.Windows;

namespace TcNo_Acc_Switcher_Steam
{
    /// <summary>
    /// Interaction logic for App.xaml
    ///
    /// Also contains launch options/argument handling
    /// Possible launch options:
    /// +[SteamID]  : Sets selected account as 'last login', and opens Steam.
    /// logout      : Sets no account to 'last login' state, to allow another account to be signed in.
    /// quit        : Closes TcNo Steam Account Switcher upon argument handling completion.
    ///
    /// Launch options can be used with shortcuts.
    /// - Ability to create these shortcuts may be added to the right-click menu at some stage in the future.
    /// 
    /// </summary>
    public partial class App : Application
    {
        private TrayUsers trayUsers = new TrayUsers();
        protected override void OnStartup(StartupEventArgs e)
        {
            base.OnStartup(e);
            MainWindow mainWindow = new MainWindow();
            bool quitArg = false;
            for (int i = 0; i != e.Args.Length; ++i)
            {
                if (e.Args[i]?[0] == '+')
                {
                    if (!File.Exists("Tray_Users.json"))
                    {
                        MessageBox.Show("Error! There are no accounts for the Tray menu!");
                        Environment.Exit(1);
                    }

                    string SteamID = e.Args[i].Substring(1);// Get SteamID from launch options
                    trayUsers.LoadTrayUsers();
                    string AccName = trayUsers.GetAccName(SteamID); // Get account name from JSON

                    mainWindow.SwapSteamAccounts(false, SteamID, AccName);
                }
                else if (e.Args[i] == "logout")
                    mainWindow.SwapSteamAccounts(true, "", "");
                else if (e.Args[i] == "quit")
                    quitArg = true;
            }

            if (quitArg)
                Environment.Exit(1);
            this.MainWindow = mainWindow;
            mainWindow.Show();
        }
    }
}

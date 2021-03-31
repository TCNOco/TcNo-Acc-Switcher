using System.IO;
using System.Windows;
using System.Windows.Input;
using TcNo_Acc_Switcher_Client;
using Globals = TcNo_Acc_Switcher_Globals.Globals;

namespace TcNo_Acc_Switcher_Client
{
    /// <summary>
    /// Interaction logic for RestoreForgotten.xaml
    /// </summary>
    public partial class RestoreForgotten
    {
        MainWindow _mw;
        public RestoreForgotten()
        {
            InitializeComponent();
            Loaded += PageLoaded;
        }

        private void PageLoaded(object sender, RoutedEventArgs e)
        {
            //var fileEntries = Directory.GetFiles(TcNo_Acc_Switcher_Server.Pages.Steam.SteamSwitcherFuncs.GetForgottenBackupPath());
            //foreach (var fileName in fileEntries)
            //    FileNamesList.Items.Add(fileName);
        }

        private void btnRestore_Click(object sender, RoutedEventArgs e)
        {
            //File.Copy(FileNamesList.SelectedItem.ToString(), Path.Combine(TcNo_Acc_Switcher_Server.Pages.Steam.SteamSwitcherFuncs.SteamConfigFolder(), "loginusers.vdf"), true);
            //MessageBox.Show("Restored! Refreshing Steam accounts now. This may take a second.");
            //_mw.MView2.Reload();
            //this.Close();
        }
    }
}
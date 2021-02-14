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
        public void ShareMainWindow(MainWindow imw) => _mw = imw;
        private void PageLoaded(object sender, RoutedEventArgs e)
        {
            string[] fileEntries = Directory.GetFiles(Settings.GetForgottenBackupPath());
            foreach (string fileName in fileEntries)
                FileNamesList.Items.Add(fileName);
        }
        private void Window_PreviewKeyDown(object sender, KeyEventArgs e)
        {
            if (e.Key == Key.Escape)
                Close();
        }

        private void BtnExit(object sender, RoutedEventArgs e) => Globals.WindowHandling.BtnExit(sender, e, this);
        private void BtnMinimize(object sender, RoutedEventArgs e) => Globals.WindowHandling.BtnMinimize(sender, e, this);
        private void DragWindow(object sender, MouseButtonEventArgs e) => Globals.WindowHandling.DragWindow(sender, e, this);
        private void btnRestore_Click(object sender, RoutedEventArgs e)
        {
            File.Copy(FileNamesList.SelectedItem.ToString(), Path.Combine(Settings.GetPersistentFolder(), "loginusers.vdf"), true);
            MessageBox.Show("Restored! Refreshing Steam accounts now. This may take a second.");
            _mw.MView2.Reload();
            this.Close();
        }
    }
}
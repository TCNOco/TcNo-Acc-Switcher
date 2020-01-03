using System;
using System.Collections.Generic;
using System.IO;
using System.Text;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Data;
using System.Windows.Documents;
using System.Windows.Input;
using System.Windows.Media;
using System.Windows.Media.Imaging;
using System.Windows.Shapes;

namespace TCNO_Acc_Switcher_CSharp_WPF
{
    /// <summary>
    /// Interaction logic for RestoreForgotten.xaml
    /// </summary>
    public partial class RestoreForgotten : Window
    {
        MainWindow mw;
        public RestoreForgotten()
        {
            InitializeComponent();
            Loaded += PageLoaded;
        }
        public void ShareMainWindow(MainWindow imw)
        {
            mw = imw;
        }
        private void PageLoaded(object sender, RoutedEventArgs e)
        {
            string[] fileEntries = Directory.GetFiles(mw.GetForgottenBackupPath());
            foreach (string fileName in fileEntries)
                filenamesList.Items.Add(fileName);
        }
        private void Window_PreviewKeyDown(object sender, KeyEventArgs e)
        {
            if (e.Key == Key.Escape)
                Close();
        }
        private void btnExit(object sender, RoutedEventArgs e)
        {
            this.Close();
        }

        private void btnMinimize(object sender, RoutedEventArgs e)
        {
            this.WindowState = WindowState.Minimized;
        }

        private void dragWindow(object sender, MouseButtonEventArgs e)
        {
            this.DragMove();
        }

        private void btnRestore_Click(object sender, RoutedEventArgs e)
        {
            File.Copy(filenamesList.SelectedItem.ToString(), System.IO.Path.Combine(mw.GetPersistentFolder(), "loginusers.vdf"), true);
            MessageBox.Show("Restored! Refreshing Steam accounts now. This may take a second.");
            mw.RefreshSteamAccounts();
            this.Close();
        }
    }
}

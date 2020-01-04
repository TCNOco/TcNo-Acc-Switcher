using System;
using System.Collections.Generic;
using System.Diagnostics;
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
    /// Interaction logic for Settings.xaml
    /// </summary>+
    
    public partial class Settings : Window
    {
        MainWindow mw;
        public Settings()
        {
            InitializeComponent();
        }
        public void ShareMainWindow(MainWindow imw)
        {
            mw = imw;
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
        private void Window_PreviewKeyDown(object sender, KeyEventArgs e)
        {
            if (e.Key == Key.Escape)
                Close();
        }
        private void btnResetSettings_Click(object sender, RoutedEventArgs e)
        {
            if (!mw.VACCheckRunning)
            {
                MessageBoxResult messageBoxResult = System.Windows.MessageBox.Show("Are you sure?", "Reset settings", System.Windows.MessageBoxButton.YesNo);
                if (messageBoxResult == MessageBoxResult.Yes)
                {
                    mw.ResetSettings();
                    this.Close();
                }
            }
        }
        private void btnPickSteamFolder_Click(object sender, RoutedEventArgs e)
        {
            mw.PickSteamFolder();
        }
        private void btnResetImages_Click(object sender, RoutedEventArgs e)
        {
           if (!mw.VACCheckRunning)
            {
                MessageBoxResult messageBoxResult = System.Windows.MessageBox.Show("Are you sure?", "Reset settings", System.Windows.MessageBoxButton.YesNo);
                if (messageBoxResult == MessageBoxResult.Yes)
                {
                    mw.ResetImages();
                }
            }
        }
        private void btnCheckVac_Click(object sender, RoutedEventArgs e)
        {
            mw.CheckVac();
        }

        private void ShowSteamID_CheckChanged(object sender, RoutedEventArgs e)
        {
            mw.ShowSteamIDHidden.IsChecked = (bool)ShowSteamID.IsChecked;
        }

        private void ShowVACStatus_CheckChanged(object sender, RoutedEventArgs e)
        {
            mw.toggleVACStatus((bool)ShowVACStatus.IsChecked);
        }

        private void btnRestoreForgotten_Click(object sender, RoutedEventArgs e)
        {
            if (Directory.Exists(mw.GetForgottenBackupPath()))
            {
                RestoreForgotten restoreForgottenDialog = new RestoreForgotten();
                restoreForgottenDialog.ShareMainWindow(mw);
                restoreForgottenDialog.Owner = this;
                restoreForgottenDialog.ShowDialog();
            }
            else
            {
                MessageBox.Show($"No backups available. ({mw.GetForgottenBackupPath()})");
            }
        }

        private void btnClearForgottenBackups_Click(object sender, RoutedEventArgs e)
        {
            mw.ClearForgottenBackups();
        }

        private void btnOpenSteamFolder_Click(object sender, RoutedEventArgs e)
        {
            Process.Start("explorer.exe", mw.GetSteamDirectory() );
        }

        private void btnAdvancedCleaning_Click(object sender, RoutedEventArgs e)
        {
            ClearLogins clearLoginsDialog = new ClearLogins();
            clearLoginsDialog.ShareMainWindow(mw);
            clearLoginsDialog.Owner = this;
            clearLoginsDialog.ShowDialog();
        }
    }
}

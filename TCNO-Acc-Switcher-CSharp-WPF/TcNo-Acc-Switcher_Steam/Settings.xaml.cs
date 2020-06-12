using System;
using System.ComponentModel;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Input;
using TcNo_Acc_Switcher_Globals;


namespace TcNo_Acc_Switcher_Steam
{
    /// <summary>
    /// Interaction logic for Settings.xaml
    /// </summary>+

    public partial class Settings : Window
    {
        MainWindow mw;
        private bool enableButtons = false;
        public Settings()
        {
            InitializeComponent();
        }
        bool _shown;
        protected override void OnContentRendered(EventArgs e)
        {
            base.OnContentRendered(e);

            if (_shown)
                return;

            _shown = true;
            // Enables buttons once the settings page has loaded. This stops them being fired by .NET setting values.
            enableButtons = true;
        }


        public void ShareMainWindow(MainWindow imw)
        {
            mw = imw;
        }

        private void BtnExit(object sender, RoutedEventArgs e)
        {
            Globals.WindowHandling.BtnExit(sender, e, this);
        }
        private void BtnMinimize(object sender, RoutedEventArgs e)
        {
            Globals.WindowHandling.BtnMinimize(sender, e, this);
        }
        private void DragWindow(object sender, MouseButtonEventArgs e)
        {
            Globals.WindowHandling.DragWindow(sender, e, this);
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
            if (enableButtons)
                mw.CheckVac();
        }

        private void ShowSteamID_CheckChanged(object sender, RoutedEventArgs e)
        {
            if (enableButtons)
                mw.ShowSteamIDHidden.IsChecked = (bool)ShowSteamID.IsChecked;
        }

        private void ShowVACStatus_CheckChanged(object sender, RoutedEventArgs e)
        {
            if (enableButtons)
                mw.ToggleVacStatus((bool)ShowVACStatus.IsChecked);
        }

        private void LimitedAsVAC_CheckChanged(object sender, RoutedEventArgs e)
        {
            if (enableButtons)
                mw.ToggleLimitedAsVac((bool)LimitedAsVAC.IsChecked);
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
            Process.Start("explorer.exe", mw.GetSteamDirectory());
        }

        private void btnAdvancedCleaning_Click(object sender, RoutedEventArgs e)
        {
            ClearLogins clearLoginsDialog = new ClearLogins();
            clearLoginsDialog.ShareMainWindow(mw);
            clearLoginsDialog.Owner = this;
            clearLoginsDialog.ShowDialog();
        }

        private void ToggleStartShortcut_CheckChanged(object sender, RoutedEventArgs e)
        {
            if (enableButtons)
                mw.StartMenuShortcut((bool)ToggleStartShortcut.IsChecked);
        }
        private void ToggleStartWithWindows_CheckChanged(object sender, RoutedEventArgs e)
        {
            if (enableButtons)
                mw.StartWithWindows((bool)ToggleStartWithWindows.IsChecked);
        }
        private void ToggleDesktopShortcut_CheckChanged(object sender, RoutedEventArgs e)
        {
            if (enableButtons)
                mw.DesktopShortcut((bool)ToggleDesktopShortcut.IsChecked);
        }

        private void ToggleAccNames_CheckChanged(object sender, RoutedEventArgs e)
        {
            if (enableButtons)
                mw.ToggleAccNames((bool)ToggleAccNames.IsChecked);
        }

        private void NumberRecentAccounts_OnTextChanged(object sender, TextChangedEventArgs e)
        {
            if (enableButtons)
            {
                if ((string) NumberRecentAccounts.Text == "")
                {
                    NumberRecentAccounts.Text = "0";
                    return;
                }
                else if (((string)NumberRecentAccounts.Text).Length > 1)
                {
                    while (((string)NumberRecentAccounts.Text)[0] == '0')
                    {
                        NumberRecentAccounts.Text = (string)NumberRecentAccounts.Text.Substring(1);
                    }
                }

                int x;
                if (Int32.TryParse((string)NumberRecentAccounts.Text, out x))
                    mw.SetTotalRecentAccount((string)NumberRecentAccounts.Text);
                else
                    NumberRecentAccounts.Text = new string(((string)NumberRecentAccounts.Text).Where(c => "0123456789".Contains(c)).ToArray());
            }
        }

        private void Settings_OnClosing(object sender, CancelEventArgs e)
        {
            mw.CapTotalTrayUsers();
        }

        private void ImageExpiry_OnTextChanged(object sender, TextChangedEventArgs e)
        {
            if (enableButtons)
            {
                if ((string)ImageExpiry.Text == "")
                {
                    ImageExpiry.Text = "0";
                    return;
                } else if (((string)ImageExpiry.Text).Length > 1)
                {
                    while (((string)ImageExpiry.Text)[0] == '0')
                    {
                        ImageExpiry.Text = (string)ImageExpiry.Text.Substring(1);
                    }
                }

                int x;
                if (Int32.TryParse((string)ImageExpiry.Text, out x))
                    mw.SetImageExpiry((string)ImageExpiry.Text);
                else
                    ImageExpiry.Text = new string(((string)ImageExpiry.Text).Where(c => "0123456789".Contains(c)).ToArray());
            }
        }
    }
}

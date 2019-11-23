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
//using System.Windows.Shapes;
using System.IO;
using Microsoft.Win32;

namespace TCNO_Acc_Switcher_CSharp_WPF
{
    /// <summary>
    /// Interaction logic for SteamFolderInput.xaml
    /// </summary>
    public partial class SteamFolderInput : Window
    {
        bool SteamFound = false;
        public SteamFolderInput()
        {
            InitializeComponent();
        }

        private void btnSetDirectory_Click(object sender, RoutedEventArgs e)
        {
            this.Close();
        }
        Color DarkGreen = (Color)(ColorConverter.ConvertFromString("#053305"));
        Color DefaultGray = (Color)(ColorConverter.ConvertFromString("#333333"));
        private void txtResponse_KeyUp(object sender, KeyEventArgs e)
        {
            verifySteamPath();
        }
        private void verifySteamPath()
        {
            if (File.Exists(Path.Combine(txtResponse.Text, "Steam.exe")))
            {
                SteamFound = true;
                rectSteamFound.Background = new SolidColorBrush(Colors.Green);
                lblQuery3.Content = "Steam.exe found!";
                btnSetDirectory.Background = new SolidColorBrush(DarkGreen);
                btnSetDirectory.IsEnabled = true;
            }
            else
            {
                SteamFound = false;
                rectSteamFound.Background = new SolidColorBrush(Colors.Red);
                lblQuery3.Content = "Steam.exe not found";
                btnSetDirectory.Background = new SolidColorBrush(DefaultGray);
                btnSetDirectory.IsEnabled = false;
            }
        }

        private void btnSetDirectory_MouseEnter(object sender, MouseEventArgs e)
        {
            btnSetDirectory.Background = new SolidColorBrush(SteamFound ? Colors.Green : DefaultGray);
        }

        private void btnSetDirectory_MouseLeave(object sender, MouseEventArgs e)
        {
            btnSetDirectory.Background = new SolidColorBrush(SteamFound ? DarkGreen : DefaultGray);
        }

        private void Button_Click(object sender, RoutedEventArgs e)
        {
            Microsoft.Win32.OpenFileDialog dlg = new Microsoft.Win32.OpenFileDialog();
            dlg.FileName = "Steam";
            dlg.DefaultExt = ".exe";
            dlg.Filter = "Steam.exe|Steam.exe";

            Nullable<bool> result = dlg.ShowDialog();
            if (result == true)
            {
                txtResponse.Text = Path.GetDirectoryName(dlg.FileName);
                verifySteamPath();
            }
        }
    }
}

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
        private void Window_Loaded(object sender, RoutedEventArgs e)
        {
            lblQuery2.Margin = lblQuery.IsVisible ? new Thickness(0, 0, 0, 0) : new Thickness(0, 16, 0, 0);
            verifySteamPath();
        }

        private void btnSetDirectory_Click(object sender, RoutedEventArgs e)
        {
            this.Close();
        }
        Color DarkGreen = (Color)(ColorConverter.ConvertFromString("#053305"));
        Color DefaultGray = (Color)(ColorConverter.ConvertFromString("#333333"));
        private void verifySteamPath()
        {
            if (File.Exists(Path.Combine(txtResponse.Text, "Steam.exe")))
            {
                SteamFound = true;
                rectSteamFound.Background = new SolidColorBrush(Colors.Green);
                lblQuery3.Background = new SolidColorBrush((Color)(ColorConverter.ConvertFromString("#040")));
                lblQuery3.Content = "Steam.exe found!";
                btnSetDirectory.Background = new SolidColorBrush(DarkGreen);
                btnSetDirectory.IsEnabled = true;
            }
            else
            {
                SteamFound = false;
                rectSteamFound.Background = new SolidColorBrush(Colors.Red);
                lblQuery3.Background = new SolidColorBrush((Color)(ColorConverter.ConvertFromString("#400")));
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

        private void txtResponse_TextChanged(object sender, TextChangedEventArgs e)
        {
            verifySteamPath();
        }


        Point _startPosition;
        bool _isResizing = false;
        private void resizeGrip_PreviewMouseLeftButtonDown(object sender, MouseButtonEventArgs e)
        {
            if (Mouse.Capture(resizeGrip))
            {
                _isResizing = true;
                _startPosition = Mouse.GetPosition(this);
            }
        }

        private void Window_PreviewMouseMove(object sender, MouseEventArgs e)
        {
            if (_isResizing)
            {
                Point currentPosition = Mouse.GetPosition(this);
                double diffX = currentPosition.X - _startPosition.X;
                double diffY = currentPosition.Y - _startPosition.Y;
                this.Width = Math.Max(this.Width + diffX, this.MinWidth);
                this.Height = Math.Max(this.Height + diffY, this.MinHeight);
                _startPosition = currentPosition;
            }
        }

        private void resizeGrip_PreviewMouseLeftButtonUp(object sender, MouseButtonEventArgs e)
        {
            if (_isResizing == true)
            {
                _isResizing = false;
                Mouse.Capture(null);
            }

        }
        private void Window_PreviewKeyDown(object sender, KeyEventArgs e)
        {
            if (e.Key == Key.Escape)
                Close();
        }
    }
}

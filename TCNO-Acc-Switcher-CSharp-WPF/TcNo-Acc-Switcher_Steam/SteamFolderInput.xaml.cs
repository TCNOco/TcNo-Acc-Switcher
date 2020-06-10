using System;
using System.IO;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Input;
using System.Windows.Media;
using Microsoft.Win32;

//using System.Windows.Shapes;

namespace TcNo_Acc_Switcher_Steam
{
    /// <summary>
    ///     Interaction logic for SteamFolderInput.xaml
    /// </summary>
    public partial class SteamFolderInput : Window
    {
        private readonly Color _darkGreen = Color.FromRgb(5, 51, 5);
        private readonly Color _defaultGray = Color.FromRgb(51, 51, 51);
        private bool _isResizing;


        private Point _startPosition;
        private bool SteamFound;

        public SteamFolderInput()
        {
            InitializeComponent();
        }

        private void Window_Loaded(object sender, RoutedEventArgs e)
        {
            lblQuery2.Margin = lblQuery.IsVisible ? new Thickness(0, 0, 0, 0) : new Thickness(0, 16, 0, 0);
            VerifySteamPath();
        }

        private void btnSetDirectory_Click(object sender, RoutedEventArgs e)
        {
            Close();
        }

        private void VerifySteamPath()
        {
            if (File.Exists(Path.Combine(txtResponse.Text, "Steam.exe")))
            {
                SteamFound = true;
                rectSteamFound.Background = new SolidColorBrush(Colors.Green);
                lblQuery3.Background = new SolidColorBrush(Color.FromRgb(0, 68, 0));
                lblQuery3.Content = "Steam.exe found!";
                btnSetDirectory.Background = new SolidColorBrush(_darkGreen);
                btnSetDirectory.IsEnabled = true;
            }
            else
            {
                SteamFound = false;
                rectSteamFound.Background = new SolidColorBrush(Colors.Red);
                lblQuery3.Background = new SolidColorBrush(Color.FromRgb(68, 0, 0));
                lblQuery3.Content = "Steam.exe not found";
                btnSetDirectory.Background = new SolidColorBrush(_defaultGray);
                btnSetDirectory.IsEnabled = false;
            }
        }

        private void btnSetDirectory_MouseEnter(object sender, MouseEventArgs e)
        {
            btnSetDirectory.Background = new SolidColorBrush(SteamFound ? Colors.Green : _defaultGray);
        }

        private void btnSetDirectory_MouseLeave(object sender, MouseEventArgs e)
        {
            btnSetDirectory.Background = new SolidColorBrush(SteamFound ? _darkGreen : _defaultGray);
        }

        private void Button_Click(object sender, RoutedEventArgs e)
        {
            var dlg = new OpenFileDialog
            {
                FileName = "Steam",
                DefaultExt = ".exe",
                Filter = "Steam.exe|Steam.exe"
            };

            var result = dlg.ShowDialog();
            if (result == true)
            {
                txtResponse.Text = Path.GetDirectoryName(dlg.FileName);
                VerifySteamPath();
            }
        }

        private void BtnExit(object sender, RoutedEventArgs e)
        {
            Close();
        }

        private void BtnMinimize(object sender, RoutedEventArgs e)
        {
            WindowState = WindowState.Minimized;
        }

        private void DragWindow(object sender, MouseButtonEventArgs e)
        {
            if (e.LeftButton == MouseButtonState.Pressed) DragMove();
        }

        private void txtResponse_TextChanged(object sender, TextChangedEventArgs e)
        {
            VerifySteamPath();
        }

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
                var currentPosition = Mouse.GetPosition(this);
                var diffX = currentPosition.X - _startPosition.X;
                var diffY = currentPosition.Y - _startPosition.Y;
                Width = Math.Max(Width + diffX, MinWidth);
                Height = Math.Max(Height + diffY, MinHeight);
                _startPosition = currentPosition;
            }
        }

        private void resizeGrip_PreviewMouseLeftButtonUp(object sender, MouseButtonEventArgs e)
        {
            if (_isResizing)
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
using System;
using System.Collections.Generic;
using System.Linq;
using System.Runtime.InteropServices;
using System.Text;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Data;
using System.Windows.Documents;
using System.Windows.Input;
using System.Windows.Media;
using System.Windows.Media.Imaging;
using System.Windows.Navigation;
using System.Windows.Shapes;

namespace TcNo_Acc_Switcher_Updater
{
    /// <summary>
    /// Interaction logic for MainWindow.xaml
    /// </summary>
    public partial class MainWindow : Window
    {
        public MainWindow()
        {
            InitializeComponent();
        }
        // Window interaction
        private void BtnExit(object sender, RoutedEventArgs e)
        {
            WindowHandling.BtnExit(sender, e, this);
        }
        private void BtnMinimize(object sender, RoutedEventArgs e)
        {
            WindowHandling.BtnMinimize(sender, e, this);
        }
        private void DragWindow(object sender, MouseButtonEventArgs e)
        {
            WindowHandling.DragWindow(sender, e, this);
        }
        private void Window_Closed(object sender, EventArgs e)
        {
            Environment.Exit(1);
        }
    }

    /// <summary>
    /// Handles window buttons, resizing and dragging
    /// </summary>
    public class WindowHandling
    {
        public static void BtnMinimize(object sender, RoutedEventArgs e, Window window)
        {
            window.WindowState = WindowState.Minimized;
        }
        public static void BtnExit(object sender, RoutedEventArgs e, Window window)
        {
            window.Close();
        }
        public static void DragWindow(object sender, MouseButtonEventArgs e, Window window)
        {
            if (e.LeftButton == MouseButtonState.Pressed)
            {
                if (e.ClickCount == 2)
                {
                    SwitchState(window);
                }
                else
                {
                    if (window.WindowState == WindowState.Maximized)
                    {
                        var percentHorizontal = e.GetPosition(window).X / window.ActualWidth;
                        var targetHorizontal = window.RestoreBounds.Width * percentHorizontal;

                        var percentVertical = e.GetPosition(window).Y / window.ActualHeight;
                        var targetVertical = window.RestoreBounds.Height * percentVertical;

                        window.WindowState = WindowState.Normal;

                        GetCursorPos(out var lMousePosition);

                        window.Left = lMousePosition.X - targetHorizontal;
                        window.Top = lMousePosition.Y - targetVertical;
                    }


                    window.DragMove();
                }
            }
        }
        [DllImport("user32.dll")]
        [return: MarshalAs(UnmanagedType.Bool)]
        private static extern bool GetCursorPos(out Point lpPoint);


        [StructLayout(LayoutKind.Sequential)]
        public struct Point
        {
            public int X;
            public int Y;
        }
        public static void SwitchState(Window window)
        {
            switch (window.WindowState)
            {
                case WindowState.Normal:
                    window.WindowState = WindowState.Maximized;
                    break;
                case WindowState.Maximized:
                    window.WindowState = WindowState.Normal;
                    break;
            }
        }
    }
}

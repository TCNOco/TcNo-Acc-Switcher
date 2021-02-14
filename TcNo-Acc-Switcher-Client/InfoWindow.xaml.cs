using System.Diagnostics;
using System.Windows;
using System.Windows.Input;
using System.Windows.Navigation;
using TcNo_Acc_Switcher_Globals;

namespace TcNo_Acc_Switcher_Client
{
    /// <summary>
    /// Interaction logic for InfoWindow.xaml
    /// </summary>
    public partial class InfoWindow
    {
        public InfoWindow()
        {
            InitializeComponent();
        }

        private void BtnExit(object sender, RoutedEventArgs e) => Globals.WindowHandling.BtnExit(sender, e, this);
        private void BtnMinimize(object sender, RoutedEventArgs e) => Globals.WindowHandling.BtnMinimize(sender, e, this);
        private void DragWindow(object sender, MouseButtonEventArgs e) => Globals.WindowHandling.DragWindow(sender, e, this);
        private void Hyperlink_RequestNavigate(object sender, RequestNavigateEventArgs e)
        {
            Process.Start(new ProcessStartInfo(e.Uri.AbsoluteUri) { UseShellExecute = true });
            e.Handled = true;
        }

        private void Window_PreviewKeyDown(object sender, KeyEventArgs e)
        {
            if (e.Key == Key.Escape)
                Close();
        }
    }
}
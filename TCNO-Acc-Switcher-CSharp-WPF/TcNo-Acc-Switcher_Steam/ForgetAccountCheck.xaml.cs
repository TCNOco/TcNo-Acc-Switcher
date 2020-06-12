using System.Windows;
using System.Windows.Input;
using TcNo_Acc_Switcher_Globals;

namespace TcNo_Acc_Switcher_Steam
{
    /// <summary>
    /// Interaction logic for ForgetAccountCheck.xaml
    /// </summary>
    public partial class ForgetAccountCheck : Window
    {
        MainWindow mw;
        public ForgetAccountCheck()
        {
            InitializeComponent();
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
    }
}

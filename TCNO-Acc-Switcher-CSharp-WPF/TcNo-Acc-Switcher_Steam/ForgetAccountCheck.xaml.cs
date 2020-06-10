using System.Windows;
using System.Windows.Input;

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
            this.Close();
        }

        private void BtnMinimize(object sender, RoutedEventArgs e)
        {
            this.WindowState = WindowState.Minimized;
        }
        private void DragWindow(object sender, MouseButtonEventArgs e)
        {
            if (e.LeftButton == MouseButtonState.Pressed)
            {
                this.DragMove();
            }
        }
        private void Window_PreviewKeyDown(object sender, KeyEventArgs e)
        {
            if (e.Key == Key.Escape)
                Close();
        }
    }
}

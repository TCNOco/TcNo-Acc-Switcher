using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Data;
using System.Windows.Documents;
using System.Windows.Input;
using System.Windows.Media;
using System.Windows.Media.Animation;
using System.Windows.Media.Imaging;
using System.Windows.Navigation;
using System.Windows.Shapes;

namespace TCNO_Acc_Switcher_CSharp_WPF
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

        private void Button_Click(object sender, RoutedEventArgs e)
        {
            Button loginbutton = sender as Button;

            ColorAnimation animation;
            animation = new ColorAnimation();
            animation.From = Colors.Orange;
            animation.To = (Color)(ColorConverter.ConvertFromString("#0c0c0c"));
            animation.Duration = new Duration(TimeSpan.FromMilliseconds(5));

            loginbutton.Background = new SolidColorBrush(Colors.Orange);
            loginbutton.Background.BeginAnimation(SolidColorBrush.ColorProperty, animation);
        }
    }
}

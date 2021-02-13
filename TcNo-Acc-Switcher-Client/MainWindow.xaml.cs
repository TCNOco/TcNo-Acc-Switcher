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
using System.Windows.Media.Imaging;
using System.Windows.Navigation;
using System.Windows.Shapes;
using TcNo_Acc_Switcher;
using System.Threading;

namespace TcNo_Acc_Switcher_Client
{
    /// <summary>
    /// Interaction logic for MainWindow.xaml
    /// </summary>
    public partial class MainWindow : Window
    {
        private Thread server = new Thread(RunServer);
        private static void RunServer() { TcNo_Acc_Switcher.Program.Main(new string[0]); }

        public MainWindow()
        {
            // Start web server
            server.IsBackground = true;
            server.Start();

            // Initialise and connect to web server above
            // Somehow check ports and find a different one if it doesn't work? We'll see...
            InitializeComponent();
            MView2.Source = new Uri("http://localhost:44305/");
            //MView2.Source = new Uri("http://localhost:5000/");
        }
    }
}

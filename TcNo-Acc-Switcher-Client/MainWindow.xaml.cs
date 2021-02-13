using System;
using System.Collections.Generic;
using System.Diagnostics;
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
using System.Windows.Interop;
using Microsoft.Web.WebView2.Core;
using TcNo_Acc_Switcher.Shared;

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

            //Process.Start("TcNo-Acc-Switcher.exe");

            // Initialise and connect to web server above
            // Somehow check ports and find a different one if it doesn't work? We'll see...
            InitializeComponent();
            //MView2.Source = new Uri("http://localhost:44305/");
            MView2.Source = new Uri("http://localhost:5000/");
            MView2.NavigationStarting += UrlChanged;
            MView2.CoreWebView2InitializationCompleted += WebView_CoreWebView2Ready;
            //MView2.MouseDown += MViewMDown;
        }
        //private void MViewMDown()
        //{

        //}



        // For draggable regions:
        // https://github.com/MicrosoftEdge/WebView2Feedback/issues/200
        private void WebView_CoreWebView2Ready(object sender, EventArgs e)
        {
            var eventForwarder = new Headerbar.EventForwarder(new WindowInteropHelper(this).Handle);

            MView2.CoreWebView2.AddHostObjectToScript("eventForwarder", eventForwarder);
        }

        private void UrlChanged(object sender, CoreWebView2NavigationStartingEventArgs args)
        {
            String uri = args.Uri.Split("/").Last();
            Console.WriteLine(args.Uri);
            switch (uri)
            {
                case "Win_min":
                    args.Cancel = true;
                    this.WindowState = WindowState.Minimized;
                    break;
                case "Win_max":
                    args.Cancel = true;
                    this.WindowState = WindowState.Maximized;
                    break;
                case "Win_restore":
                    args.Cancel = true;
                    this.WindowState = WindowState.Normal;
                    break;
                case "Win_close":
                    args.Cancel = true;
                    Environment.Exit(1);
                    break;

            }
            if (uri.Contains("Win_min"))
            {
                args.Cancel = true;
                this.WindowState = WindowState.Minimized;
            }
        }
    }
}

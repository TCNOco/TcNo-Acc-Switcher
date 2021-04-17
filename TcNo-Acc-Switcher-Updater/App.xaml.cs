using System;
using System.Threading;
using System.Windows;

namespace TcNo_Acc_Switcher_Updater
{
    /// <summary>
    /// Interaction logic for App.xaml
    /// </summary>
    public partial class App : Application
    {
        static readonly Mutex Mutex = new(true, "{A240C23D-6F45-4E92-9979-11E6CE10A22C}");
        [STAThread]
        protected override void OnStartup(StartupEventArgs e)
        {
            // Single instance:
            if (!Mutex.WaitOne(TimeSpan.Zero, true))
            {
                MessageBox.Show("Another TcNo Account Switcher Updater instance has been detected.");
                Environment.Exit(1056); // 1056	An instance of the service is already running.
            }

            base.OnStartup(e);
            for (var i = 0; i != e.Args.Length; ++i)
            {
                switch (e.Args[i])
                {
                    case "verify":
                        TcNo_Acc_Switcher_Updater.MainWindow.VerifyAndClose = true;
                        break;
                    default:
                        Console.WriteLine($"Unknown argument: \"{e.Args[i]}\"");
                        break;
                }
            }
        }
    }
}

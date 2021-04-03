using System;
using System.ComponentModel;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Input;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Server.Pages.Steam;
using TcNo_Acc_Switcher_Client.Localisation;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Pages.General;
using Index = TcNo_Acc_Switcher_Server.Pages.Index;


namespace TcNo_Acc_Switcher_Client
{
    /// <summary>
    /// Interaction logic for Settings.xaml
    /// </summary>

    public partial class Settings
    {
        private readonly JObject _settingsJObject;
        private MainWindow _mw;
        private bool _enableButtons;

        public Settings()
        {
            InitializeComponent();

            _settingsJObject = GeneralFuncs.LoadSettings("SteamSettings");
            // Set tickboxes from here, rather than with a model.
        }

        private void NumberRecentAccounts_OnTextChanged(object sender, TextChangedEventArgs e)
        {
            //if (!_enableButtons) return;
            //if (NumberRecentAccounts.Text == "")
            //{
            //    NumberRecentAccounts.Text = "0";
            //    return;
            //}

            //if ((NumberRecentAccounts.Text).Length > 1)
            //    while ((NumberRecentAccounts.Text)[0] == '0')
            //        NumberRecentAccounts.Text = NumberRecentAccounts.Text.Substring(1);

            //// BELOW WAS COMMENTED OUT FROM BEFORE
            ////if (int.TryParse(NumberRecentAccounts.Text, out _))
            ////    _mw.SetTotalRecentAccount(NumberRecentAccounts.Text);
            ////else
            ////    NumberRecentAccounts.Text = new string((NumberRecentAccounts.Text).Where(c => "0123456789".Contains(c)).ToArray());
        }

        private void Settings_OnClosing(object sender, CancelEventArgs e) => Console.WriteLine("Temp");//_mw.CapTotalTrayUsers();

        private void ImageExpiry_OnTextChanged(object sender, TextChangedEventArgs e)
        {
            //if (!_enableButtons) return;
            //if (ImageExpiry.Text == "")
            //{
            //    ImageExpiry.Text = "0";
            //    return;
            //}

            //if (ImageExpiry.Text.Length > 1)
            //    while (ImageExpiry.Text[0] == '0')
            //        ImageExpiry.Text = ImageExpiry.Text.Substring(1);

            //// BELOW WAS COMMENTED OUT FROM BEFORE
            ////if (int.TryParse(ImageExpiry.Text, out _))
            ////    _mw.SetImageExpiry(ImageExpiry.Text);
            ////else
            ////    ImageExpiry.Text = new string(ImageExpiry.Text.Where(c => "0123456789".Contains(c)).ToArray());
        }
    }
}

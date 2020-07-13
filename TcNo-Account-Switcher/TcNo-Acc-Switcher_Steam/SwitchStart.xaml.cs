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
using System.Windows.Shapes;

namespace TcNo_Acc_Switcher_Steam
{
    /// <summary>
    /// Interaction logic for SwitchStart.xaml
    /// 
    /// PLAN:
    /// Dropdown menu with:
    /// - steam://connect/<IP>
    /// - steam://rungameid/<ID>
    /// Textbox for entering gameID or IP.
    /// - Button to search for game, brings up list with list of games from Steam -- Mentioned in UserDataWindow
    /// Previous starts
    /// 
    /// </summary>
    public partial class SwitchStart : Window
    {
        public SwitchStart()
        {
            InitializeComponent();
        }
    }
}

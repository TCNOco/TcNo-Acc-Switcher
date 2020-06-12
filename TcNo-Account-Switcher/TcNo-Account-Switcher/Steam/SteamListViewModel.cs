using System;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.ComponentModel;
using System.Linq;
using System.Text;
using System.Threading.Tasks;

using TcNo_Account_Switcher.Services.Steam;

namespace TcNo_Account_Switcher.Steam
{
    class SteamListViewModel
    {
        private ISteamUsersRepository _steamUsersRepository = new SteamUsersRepository();

        public SteamListViewModel()
        {
            if (DesignerProperties.GetIsInDesignMode(
                new System.Windows.DependencyObject())) return;

            SteamUsers = new ObservableCollection<SteamUser>(_steamUsersRepository.GetSteamUsersAsync().Result);
        }
        public ObservableCollection<SteamUser> SteamUsers { get; set; }
    }
}

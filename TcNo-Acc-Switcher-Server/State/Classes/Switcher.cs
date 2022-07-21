using System.Collections.ObjectModel;
using Microsoft.AspNetCore.Components;
using TcNo_Acc_Switcher_Globals;

namespace TcNo_Acc_Switcher_Server.State.Classes
{
    public class Switcher
    {
        [Inject] private NewLang Lang { get; set; }

        public Switcher()
        {
            CurrentStatus = Lang["Status_Init"];
        }
        public string CurrentSwitcherSafe { get; set; } = "";

        private string _currentSwitcher = "";
        public string CurrentSwitcher
        {
            get => _currentSwitcher;
            set
            {
                _currentSwitcher = value;
                CurrentSwitcherSafe = Globals.GetCleanFilePath(CurrentSwitcher);
            }
        }

        public string CurrentStatus { get; set; }
        public string SelectedPlatform { get; set; } = "";
        public bool IsCurrentlyExportingAccounts { get; set; }
        public ObservableCollection<Account> SteamAccounts { get; set; } = new();
        public ObservableCollection<Account> BasicAccounts { get; set; } = new();
        public string SelectedAccountId => SelectedAccount?.AccountId ?? "";
        public Account SelectedAccount { get; set; }
    }
}

// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2022 TechNobo (Wesley Pyburn)
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

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

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

using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.ComponentModel;
using System.Runtime.CompilerServices;
using Microsoft.AspNetCore.Components;
using TcNo_Acc_Switcher_Globals;

namespace TcNo_Acc_Switcher_Server.State.Classes
{
    public class Switcher : INotifyPropertyChanged
    {
        public event PropertyChangedEventHandler? PropertyChanged;

        protected void OnPropertyChanged(string propertyName) => PropertyChanged?.Invoke(this, new PropertyChangedEventArgs(propertyName));

        protected bool SetField<T>(ref T field, T value, [CallerMemberName] string propertyName = "")
        {
            if (EqualityComparer<T>.Default.Equals(field, value)) return false;
            field = value;
            OnPropertyChanged(propertyName);
            return true;
        }

        [Inject] private NewLang Lang { get; set; }

        public Switcher()
        {
            CurrentStatus = Lang["Status_Init"];
        }
        public string CurrentSwitcherSafe { get; set; } = "";

        private string _currentSwitcher = "";
        private Account _selectedAccount;
        private string _currentStatus;

        public string CurrentSwitcher
        {
            get => _currentSwitcher;
            set
            {
                SetField(ref _currentSwitcher, value);
                CurrentSwitcherSafe = Globals.GetCleanFilePath(CurrentSwitcher);
            }
        }

        public string CurrentStatus
        {
            get => _currentStatus;
            set => SetField(ref _currentStatus, value);
        }

        public string SelectedPlatform { get; set; } = "";
        public bool IsCurrentlyExportingAccounts { get; set; }
        public ObservableCollection<Account> SteamAccounts { get; set; } = new();
        public ObservableCollection<Account> TemplatedAccounts { get; set; } = new();
        public string SelectedAccountId { get; set; }

        public Account SelectedAccount
        {
            get => _selectedAccount;
            set
            {
                SetField(ref _selectedAccount, value);
                SelectedAccountId = value?.AccountId ?? "";
            }
        }
    }
}

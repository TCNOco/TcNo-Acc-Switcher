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

using System.ComponentModel;
using System.Diagnostics;
using System.IO;
using TcNo_Acc_Switcher_Server.State.Classes;
using TcNo_Acc_Switcher_Server.State.DataTypes;
using TcNo_Acc_Switcher_Server.State.Interfaces;

namespace TcNo_Acc_Switcher_Server.State
{
    public class AppState : IAppState, INotifyPropertyChanged
    {
        public string PasswordCurrent { get; set; }

        public ShortcutsState Shortcuts { get; set; } = new();

        public Toasts Toasts { get; set; } = new();

        public Discord Discord { get; set; } = new();

        public Updates Updates { get; set; } = new();

        public Stylesheet Stylesheet { get; set; } = new();

        public Navigation Navigation { get; set; }

        public Switcher Switcher { get; set; }

        public WindowState WindowState { get; set; } = new();

        // Property change notifications
        public event PropertyChangedEventHandler? PropertyChanged;
        protected void OnPropertyChanged(string propertyName) => PropertyChanged?.Invoke(this, new PropertyChangedEventArgs(propertyName));

        /// <summary>
        /// Is running with the official window, or just the server in a browser.
        /// </summary>
        public bool IsTcNoClientApp { get; set; }
        public AppState()
        {
            // Discord integration
            Discord.RefreshDiscordPresenceAsync(true);

            // Forward state changes.
            Stylesheet.PropertyChanged += (s, e) => PropertyChanged?.Invoke(s, e);
            WindowState.PropertyChanged += (s, e) => PropertyChanged?.Invoke(s, e);

        }


        public void OpenFolder(string folder)
        {
            Directory.CreateDirectory(folder); // Create if doesn't exist
            Process.Start("explorer.exe", folder);
            Toasts.ShowToastLang(ToastType.Info, "Toast_PlaceShortcutFiles");
        }
    }
}

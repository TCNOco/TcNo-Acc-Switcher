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

using System;

namespace TcNo_Acc_Switcher_Server.State.Classes.Stats
{
    public class SwitcherStat
    {
        public SwitcherStat()
        {
            Accounts = 0;
            Switches = 0;
            UniqueDays = 1; // First day is init day
            GameShortcuts = 0;
            GameShortcutsHotbar = 0;
            GamesLaunched = 0;
            FirstActive = DateTime.Now;
            LastActive = DateTime.Now;
        }
        public int Accounts { get; set; }
        public int Switches { get; set; }
        public int UniqueDays { get; set; }
        public int GameShortcuts { get; set; }
        public int GameShortcutsHotbar { get; set; }
        public int GamesLaunched { get; set; }
        public DateTime FirstActive { get; set; }
        public DateTime LastActive { get; set; }
    }
}

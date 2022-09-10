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
using TcNo_Acc_Switcher.State.Classes;

namespace TcNo_Acc_Switcher.State.Interfaces;

public interface IAppState
{
    string PasswordCurrent { get; set; }
    ShortcutsState Shortcuts { get; set; }
    Discord Discord { get; set; }
    Updates Updates { get; set; }
    Stylesheet Stylesheet { get; set; }
    Navigation Navigation { get; set; }
    Switcher Switcher { get; set; }
    WindowState WindowState { get; set; }
    event PropertyChangedEventHandler PropertyChanged;
    void OpenFolder(string folder);
}
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
using System.Collections.Generic;

namespace TcNo_Acc_Switcher.Shared.ContextMenu;

public class MenuItem
{
    public string Text { get; set; }

    public string Content { get; set; }
    public Action MenuAction;
    public List<MenuItem> Children = new();

    /// <summary>
    /// Optional unique ID to remove this section.
    /// </summary>
    public string Id { get; set; }

    public MenuItem() { }
}
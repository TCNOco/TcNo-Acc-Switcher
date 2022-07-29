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

namespace TcNo_Acc_Switcher_Server.State.Classes
{
    /// <summary>
    /// Contains Actions to be used in with Account/s interactions.
    /// </summary>
    public class AccountActions
    {
        /// <summary>
        /// Used in adding to or modifying the context menu before showing on account right-click
        /// </summary>
        public Action<Account> AfterAccountSelect { get; set; }
    }
}

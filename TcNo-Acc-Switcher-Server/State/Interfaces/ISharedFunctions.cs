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
using System.Threading.Tasks;

namespace TcNo_Acc_Switcher_Server.State.Interfaces;

public interface ISharedFunctions
{
    /// <summary>
    /// Runs jQueryProcessAccListSize, initContextMenu and initAccListSortable - Final init needed for account switcher to work.
    /// </summary>
    Task FinaliseAccountList();

    void RunShortcut(string s, string shortcutFolder, string platform = "", bool admin = false);
    bool CanKillProcess(List<string> procNames, string closingMethod = "Combined", bool showModal = true);
    bool CanKillProcess(string processName, string closingMethod = "Combined", bool showModal = true);
    Task<bool> CloseProcesses(string procName, string closingMethod);
    Task<bool> CloseProcesses(List<string> procNames, string closingMethod);

    /// <summary>
    /// Waits for a program to close, and returns true if not running anymore.
    /// </summary>
    /// <param name="procName">Name of process to lookup</param>
    /// <returns>Whether it was closed before this function returns or not.</returns>
    Task<bool> WaitForClose(string procName);

    Task<bool> WaitForClose(List<string> procNames, string closingMethod);
}
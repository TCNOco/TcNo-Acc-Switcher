// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2021 TechNobo (Wesley Pyburn)
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
using System.Linq;
using System.Threading.Tasks;
using Microsoft.VisualBasic;
using Microsoft.Win32.TaskScheduler;

namespace TcNo_Acc_Switcher_Server.Pages.General.Classes
{
    public class Task
    {
        /// <summary>
        /// Checks if the TcNo Account Switcher Tray application has a Task to start with Windows
        /// </summary>
        public static bool StartWithWindows_Enabled()
        {
            using var ts = new TaskService();
            var tasks = ts.RootFolder.Tasks;
            return tasks.Exists("TcNo Account Switcher - Tray start with logon");
        }

        /// <summary>
        /// Toggles whether the TcNo Account Switcher Tray application starts with Windows or not
        /// </summary>
        /// <param name="shouldExist">Whether it should start with Windows, or not</param>
        public static void StartWithWindows_Toggle(bool shouldExist)
        {
            if (shouldExist && !StartWithWindows_Enabled())
            {
                var ts = new TaskService();
                var td = ts.NewTask();
                td.Principal.RunLevel = TaskRunLevel.Highest;
                td.Triggers.AddNew(TaskTriggerType.Logon);
                var programPath = System.IO.Path.GetFullPath("TcNo-Acc-Switcher-Tray.exe");
                td.Actions.Add(new ExecAction(programPath));
                ts.RootFolder.RegisterTaskDefinition("TcNo Account Switcher - Tray start with logon", td);
                //MessageBox.Show(Strings.InfoTrayWindowsStart);
            }
            else if (!shouldExist && StartWithWindows_Enabled())
            {
                var ts = new TaskService();
                ts.RootFolder.DeleteTask("TcNo Account Switcher - Tray start with logon");
                //MessageBox.Show(Strings.InfoTrayWindowsStartOff);
            }
        }
    }
}

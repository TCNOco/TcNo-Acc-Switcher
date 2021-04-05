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
        public static bool StartWithWindows_Enabled()
        {
            using var ts = new TaskService();
            var tasks = ts.RootFolder.Tasks;
            return tasks.Exists("TcNo Account Switcher - Tray start with logon");
        }
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

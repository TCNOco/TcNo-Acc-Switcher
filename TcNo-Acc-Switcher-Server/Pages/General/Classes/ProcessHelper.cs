using System;
using System.Diagnostics;
using System.Linq;
using System.Runtime.InteropServices;
using System.Runtime.Versioning;
using System.Security.Principal;
using TcNo_Acc_Switcher_Globals;

namespace TcNo_Acc_Switcher_Server.Pages.General.Classes
{
    public class ProcessHelper
    {
        //https://stackoverflow.com/a/5479957/5165437

        [DllImport("advapi32.dll", SetLastError = true)]
        private static extern bool OpenProcessToken(IntPtr processHandle, uint desiredAccess, out IntPtr tokenHandle);

        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern bool CloseHandle(IntPtr hObject);

        private const int StandardRightsRequired = 0xF0000;
        private const int AssignPrimary = 0x1;
        private const int Duplicate = 0x2;
        private const int Impersonate = 0x4;
        private const int Query = 0x8;
        private const int QuerySource = 0x10;
        private const int AdjustGroups = 0x40;
        private const int AdjustPrivileges = 0x20;
        private const int AdjustSessionId = 0x100;
        private const int AdjustDefault = 0x80;
        private const int AllAccess = StandardRightsRequired | AssignPrimary | Duplicate | Impersonate | Query | QuerySource | AdjustPrivileges | AdjustGroups | AdjustSessionId | AdjustDefault;

        [SupportedOSPlatform("windows")]
        public static bool IsProcessAdmin(string processName)
        {
	        var processes = Process.GetProcessesByName(processName);
	        if (processes.Length == 0) return false; // Program is not running
	        var proc = processes[0];

	        IntPtr handle;
	        try
	        {
		        handle = proc.Handle;
		        return IsHandleAdmin(handle);
	        }
	        catch (Exception)
	        {
		        try
		        {
			        handle = proc.MainWindowHandle;
			        return IsHandleAdmin(handle);
		        }
		        catch (Exception a)
		        {
			        Globals.WriteToLog(a.ToString());
		        }
	        }

	        return true;
        }

        [SupportedOSPlatform("windows")]
        public static bool IsProcessRunning(string processName)
        {
	        var proc = Process.GetProcessesByName(processName);

            return proc.Length != 0;
        }

        [SupportedOSPlatform("windows")]
        private static bool IsHandleAdmin(IntPtr handle)
        {
            OpenProcessToken(handle, AllAccess, out var ph);

            if (ph == IntPtr.Zero) return true;

            var identity = new WindowsIdentity(ph);

            var result =
                (from role in identity.Groups
                    where role.IsValidTargetType(typeof(SecurityIdentifier))
                    select role as SecurityIdentifier).Any(sid =>
                    sid.IsWellKnown(WellKnownSidType.AccountAdministratorSid) ||
                    sid.IsWellKnown(WellKnownSidType.BuiltinAdministratorsSid));

            CloseHandle(ph);

            return result;
        }
    }
}

using System;
using System.Diagnostics;
using System.Linq;
using System.Runtime.InteropServices;
using System.Runtime.Versioning;
using System.Security.Principal;

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
        private const int TokenAssignPrimary = 0x1;
        private const int TokenDuplicate = 0x2;
        private const int TokenImpersonate = 0x4;
        private const int TokenQuery = 0x8;
        private const int TokenQuerySource = 0x10;
        private const int TokenAdjustGroups = 0x40;
        private const int TokenAdjustPrivileges = 0x20;
        private const int TokenAdjustSessionId = 0x100;
        private const int TokenAdjustDefault = 0x80;
        private const int TokenAllAccess = (StandardRightsRequired | TokenAssignPrimary | TokenDuplicate | TokenImpersonate | TokenQuery | TokenQuerySource | TokenAdjustPrivileges | TokenAdjustGroups | TokenAdjustSessionId | TokenAdjustDefault);

        public static bool IsProcessAdmin(string processName)
        {
            var processes = Process.GetProcessesByName(processName);
            if (processes.Length == 0) return false; // Program is not running
            var proc = processes[0];

            IntPtr handle;
            try
            {
                handle = proc.Handle;
                return isHandleAdmin(handle);
            }
            catch (Exception e)
            {
                try
                {
                    handle = proc.MainWindowHandle;
                    return isHandleAdmin(handle);
                }
                catch (Exception a)
                {
                    Console.WriteLine(a);
                }
            }

            return true;
        }

        [SupportedOSPlatform("windows")]
        private static bool isHandleAdmin(IntPtr handle)
        {
            OpenProcessToken(handle, TokenAllAccess, out var ph);

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

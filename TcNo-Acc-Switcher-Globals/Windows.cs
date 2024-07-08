// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2024 TroubleChute (Wesley Pyburn)
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
using System.ComponentModel;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Runtime.InteropServices;
using System.Security;
using System.Threading;

namespace TcNo_Acc_Switcher_Globals
{
    internal static class NativeMethods
    {
        [DllImport("kernel32.dll", SetLastError = true)]
        public static extern IntPtr GetConsoleWindow();

        [DllImport("user32.dll", SetLastError = true)]
        public static extern int GetWindowLong(IntPtr hWnd, int nIndex);

        [DllImport("user32.dll")]
        public static extern int SetWindowLong(IntPtr hWnd, int nIndex, int dwNewLong);

        public const int GwlExStyle = -20;
        public static readonly int WsExAppWindow = 0x00040000;
        public static readonly int WsExToolWindow = 0x00000080;

        [DllImport("user32")]
        public static extern bool SetForegroundWindow(IntPtr hwnd);

        [DllImport("user32")]
        public static extern bool ShowWindow(IntPtr hWnd, int nCmdShow);

        // See https://stackoverflow.com/a/9500732
        [StructLayout(LayoutKind.Sequential)]
        public struct Rect
        {
            public int left;
            public int top;
            public int right;
            public int bottom;
        }

        [DllImport("user32.dll", CharSet = CharSet.Unicode)]
        public static extern IntPtr FindWindow(string lpClassName, string lpWindowName);

        [DllImport("user32.dll", CharSet = CharSet.Unicode)]
        public static extern IntPtr FindWindowEx(IntPtr hwndParent, IntPtr hwndChildAfter, string lpszClass,
            string lpszWindow);

        [DllImport("user32.dll")]
        public static extern bool GetClientRect(IntPtr hWnd, out Rect lpRect);

        [DllImport("user32.dll")]
        public static extern IntPtr SendMessage(IntPtr hWnd, uint msg, int wParam, int lParam);
        [DllImport("user32.dll")]
        public static extern bool ReleaseCapture();


        public struct TokenPrivileges
        {
            public uint PrivilegeCount;
            [MarshalAs(UnmanagedType.ByValArray, SizeConst = 1)]
            public LuidAndAttributes[] Privileges;
        }

        [StructLayout(LayoutKind.Sequential, Pack = 4)]
        public struct LuidAndAttributes
        {
            public Luid Luid;
            public uint Attributes;
        }

        [StructLayout(LayoutKind.Sequential)]
        public readonly struct Luid
        {
            private readonly uint LowPart;
            private readonly int HighPart;
        }

        [Flags]
        public enum ProcessAccessFlags : uint
        {
            All = 0x001F0FFF,
            /*
            Terminate = 0x00000001,
            CreateThread = 0x00000002,
            VirtualMemoryOperation = 0x00000008,
            VirtualMemoryRead = 0x00000010,
            VirtualMemoryWrite = 0x00000020,
            DuplicateHandle = 0x00000040,
            CreateProcess = 0x000000080,
            SetQuota = 0x00000100,
            SetInformation = 0x00000200,
            QueryLimitedInformation = 0x00001000,
            Synchronize = 0x00100000
            */
            QueryInformation = 0x00000400,
        }

        public enum SecurityImpersonationLevel
        {
            SecurityImpersonation
            /*
            SecurityAnonymous,
            SecurityIdentification,
            SecurityDelegation
            */
        }

        public enum TokenType
        {
            TokenPrimary = 1
            //TokenImpersonation
        }

        [StructLayout(LayoutKind.Sequential)]
        public readonly struct ProcessInformation
        {
            private readonly IntPtr hProcess;
            private readonly IntPtr hThread;
            private readonly int dwProcessId;
            private readonly int dwThreadId;
        }

        [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
        public readonly struct StartupInfo
        {
            private readonly int cb;
            private readonly string lpReserved;
            private readonly string lpDesktop;
            private readonly string lpTitle;
            private readonly int dwX;
            private readonly int dwY;
            private readonly int dwXSize;
            private readonly int dwYSize;
            private readonly int dwXCountChars;
            private readonly int dwYCountChars;
            private readonly int dwFillAttribute;
            private readonly int dwFlags;
            private readonly int wShowWindow;
            private readonly int cbReserved2;
            private readonly int lpReserved2;
            private readonly int hStdInput;
            private readonly int hStdOutput;
            private readonly int hStdError;
        }

        [DllImport("kernel32.dll", ExactSpelling = true)]
        public static extern IntPtr GetCurrentProcess();

        [DllImport("advapi32.dll", ExactSpelling = true, SetLastError = true)]
        public static extern bool OpenProcessToken(IntPtr h, int acc, ref IntPtr phtok);

        [DllImport("advapi32.dll", SetLastError = true, CharSet = CharSet.Unicode)]
        public static extern bool LookupPrivilegeValue(string host, string name, ref Luid pluid);

        [DllImport("advapi32.dll", ExactSpelling = true, SetLastError = true)]
        public static extern bool AdjustTokenPrivileges(IntPtr htok, bool disall, ref TokenPrivileges newst, int len, IntPtr prev, IntPtr relen);

        [DllImport("kernel32.dll", CharSet = CharSet.Auto, SetLastError = true)]
        [return: MarshalAs(UnmanagedType.Bool)]
        public static extern bool CloseHandle(IntPtr hObject);


        [DllImport("user32.dll")]
        public static extern IntPtr GetShellWindow();

        [DllImport("user32.dll", SetLastError = true)]
        public static extern uint GetWindowThreadProcessId(IntPtr hWnd, out uint lpdwProcessId);

        [DllImport("kernel32.dll", SetLastError = true)]
        public static extern IntPtr OpenProcess(ProcessAccessFlags processAccess, bool bInheritHandle, uint processId);

        [DllImport("advapi32.dll", CharSet = CharSet.Auto, SetLastError = true)]
        public static extern bool DuplicateTokenEx(IntPtr hExistingToken, uint dwDesiredAccess, IntPtr lpTokenAttributes, SecurityImpersonationLevel impersonationLevel, TokenType tokenType, out IntPtr phNewToken);

        [DllImport("advapi32", SetLastError = true, CharSet = CharSet.Unicode)]
        public static extern bool CreateProcessWithTokenW(IntPtr hToken, int dwLogonFlags, string lpApplicationName, string lpCommandLine, int dwCreationFlags, IntPtr lpEnvironment, string lpCurrentDirectory, [In] ref StartupInfo lpStartupInfo, out ProcessInformation lpProcessInformation);

        [DllImport("advapi32.dll", SetLastError = true)]
        public static extern bool OpenProcessToken(IntPtr processHandle, uint desiredAccess, out IntPtr tokenHandle);

        // WindowsClipboard
        [DllImport("kernel32.dll", SetLastError = true)]
        public static extern IntPtr GlobalLock(IntPtr hMem);

        [DllImport("kernel32.dll", SetLastError = true)]
        [return: MarshalAs(UnmanagedType.Bool)]
        public static extern bool GlobalUnlock(IntPtr hMem);

        [DllImport("user32.dll", SetLastError = true)]
        [return: MarshalAs(UnmanagedType.Bool)]
        public static extern bool OpenClipboard(IntPtr hWndNewOwner);

        [DllImport("user32.dll", SetLastError = true)]
        [return: MarshalAs(UnmanagedType.Bool)]
        public static extern bool CloseClipboard();

        [DllImport("user32.dll", SetLastError = true)]
        public static extern IntPtr SetClipboardData(uint uFormat, IntPtr data);

        [DllImport("user32.dll")]
        public static extern bool EmptyClipboard();

        // TcNo-Acc-Switcher-Client NativeMethods

        // http://msdn.microsoft.com/en-us/library/ms681944(VS.85).aspx
        // See: http://www.codeproject.com/tips/68979/Attaching-a-Console-to-a-WinForms-application.aspx
        // And: https://stackoverflow.com/questions/2669463/console-writeline-does-not-show-up-in-output-window/2669596#2669596

        [DllImport("kernel32.dll", SetLastError = true)]
        public static extern int AllocConsole();

        [DllImport("kernel32.dll", SetLastError = true)]
        public static extern int FreeConsole();

        [DllImport("user32.dll", SetLastError = true, CharSet = CharSet.Unicode)]
        public static extern bool SetWindowText(IntPtr hwnd, string lpString);

        [DllImport("kernel32.dll", SetLastError = true)]
        public static extern bool AttachConsole(int dwProcessId);


        // For grabbing image from icons:
        public const int IldTransparent = 0x00000001;
        public const int IldImage = 0x00000020;

        [DllImport("shell32.dll", EntryPoint = "#727")]
        public static extern int SHGetImageList(int iImageList, ref Guid riid, ref IImageList ppv);

        [DllImport("user32.dll", EntryPoint = "DestroyIcon", SetLastError = true)]
        public static extern int DestroyIcon(IntPtr hIcon);

        [DllImport("shell32.dll", CharSet = CharSet.Unicode)]
        public static extern IntPtr SHGetFileInfo(
            string pszPath,
            uint dwFileAttributes,
            ref SHFILEINFO psfi,
            uint cbFileInfo,
            uint uFlags
        );

        // For grabbing image from EXEs
        [UnmanagedFunctionPointer(CallingConvention.Winapi, SetLastError = true, CharSet = CharSet.Unicode)]
        [SuppressUnmanagedCodeSecurity]
        internal delegate bool EnumResNameProc(IntPtr hModule, IntPtr lpszType, IntPtr lpszName, IntPtr lParam);
        [DllImport("kernel32.dll", SetLastError = true, CharSet = CharSet.Unicode)]
        public static extern IntPtr LoadLibraryEx(string lpFileName, IntPtr hFile, uint dwFlags);

        [DllImport("kernel32.dll", SetLastError = true, CharSet = CharSet.Unicode)]
        public static extern IntPtr FindResource(IntPtr hModule, IntPtr lpName, IntPtr lpType);

        [DllImport("kernel32.dll", SetLastError = true)]
        public static extern IntPtr LoadResource(IntPtr hModule, IntPtr hResInfo);

        [DllImport("kernel32.dll", SetLastError = true)]
        public static extern IntPtr LockResource(IntPtr hResData);

        [DllImport("kernel32.dll", SetLastError = true)]
        public static extern uint SizeofResource(IntPtr hModule, IntPtr hResInfo);

        [DllImport("kernel32.dll", SetLastError = true, CharSet = CharSet.Unicode)]
        [SuppressUnmanagedCodeSecurity]
        public static extern bool EnumResourceNames(IntPtr hModule, IntPtr lpszType, EnumResNameProc lpEnumFunc, IntPtr lParam);
    }

    public static class WindowsClipboard
    {
        //https://github.com/CopyText/TextCopy/blob/master/src/TextCopy/WindowsClipboard.cs
        public static void SetText(string text)
        {
            Globals.DebugWriteLine(@"[Func:Data\GenericFunctions.SetText] text=hidden");
            if (text == null) return;
            OpenClipboard();

            _ = NativeMethods.EmptyClipboard();
            IntPtr hGlobal = default;
            try
            {
                var bytes = (text.Length + 1) * 2;
                hGlobal = Marshal.AllocHGlobal(bytes);

                if (hGlobal == default) ThrowWin32();
                var target = NativeMethods.GlobalLock(hGlobal);
                if (target == default) ThrowWin32();

                try
                {
                    Marshal.Copy(text.ToCharArray(), 0, target, text.Length);
                }
                finally
                {
                    _ = NativeMethods.GlobalUnlock(target);
                }

                if (NativeMethods.SetClipboardData(CfUnicodeText, hGlobal) == default) ThrowWin32();
                hGlobal = default;
            }
            finally
            {
                if (hGlobal != default) Marshal.FreeHGlobal(hGlobal);
                _ = NativeMethods.CloseClipboard();
            }
        }

        public static void OpenClipboard()
        {
            Globals.DebugWriteLine(@"[Func:Data\GenericFunctions.OpenClipboard]");
            var num = 10;
            while (true)
            {
                if (NativeMethods.OpenClipboard(default)) break;
                if (--num == 0) ThrowWin32();
                Thread.Sleep(100);
            }
        }

        private const uint CfUnicodeText = 13;

        private static void ThrowWin32()
        {
            throw new Win32Exception(Marshal.GetLastWin32Error());
        }
    }

    public class EventForwarder
    {
        public const int WmNclButtonDown = 0xA1;
        public const int WmSysCommand = 0x112;
        public const int HtCaption = 0x2;

        readonly IntPtr _target;

        public EventForwarder(IntPtr target)
        {
            _target = target;
        }

        public void MouseDownDrag()
        {
            NativeMethods.ReleaseCapture();
            NativeMethods.SendMessage(_target, WmNclButtonDown, HtCaption, 0);
        }

        public void MouseResizeDrag(int wParam)
        {
            if (wParam == 0) return;
            NativeMethods.ReleaseCapture();
            NativeMethods.SendMessage(_target, WmNclButtonDown, wParam, 0);
        }

        public void WindowAction(int action)
        {
            NativeMethods.ReleaseCapture();
            if (action != 0x0010) NativeMethods.SendMessage(_target, WmSysCommand, action, 0);
            else NativeMethods.SendMessage(_target, 0x0010, 0, 0);
        }

        public void HideWindow()
        {
            NativeMethods.ReleaseCapture();
            NativeMethods.SendMessage(_target, WmSysCommand, 0xF020, 0); // Minimize
            NativeFuncs.HideWindow(_target); // Hide from start bar
            NativeFuncs.StartTrayIfNotRunning();
        }
    }

    public static class NativeFuncs
    {
        public static int AllocConsole() => NativeMethods.AllocConsole();
        public static int FreeConsole() => NativeMethods.FreeConsole();
        public static void SetWindowText(string text)
        {
            var handle = NativeMethods.GetConsoleWindow();

            _ = NativeMethods.SetWindowText(handle, text);
        }

        public static bool AttachToConsole(int dwProcessId) => NativeMethods.AttachConsole(dwProcessId);

        public static bool BringToFront()
        {
            // This does not work in debug, as the console has the same name
            var proc = Process.GetProcessesByName("TcNo-Acc-Switcher_main").FirstOrDefault();
            if (proc == null || proc.MainWindowHandle == IntPtr.Zero) return false;
            const int swRestore = 9;
            var hwnd = proc.MainWindowHandle;
            ShowWindow(hwnd); // This seems to take ownership of some kind over the main process... So closing the tray closes the main switcher too ~
            _ = NativeMethods.ShowWindow(hwnd, swRestore);
            _ = NativeMethods.SetForegroundWindow(hwnd);
            return true;
        }

        public static void HideWindow(IntPtr handle)
        {
            _ = NativeMethods.SetWindowLong(handle, NativeMethods.GwlExStyle, (NativeMethods.GetWindowLong(handle, NativeMethods.GwlExStyle) | NativeMethods.WsExToolWindow) & ~NativeMethods.WsExAppWindow);
        }

        public static void ShowWindow(IntPtr handle)
        {
            _ = NativeMethods.SetWindowLong(handle, NativeMethods.GwlExStyle, (NativeMethods.GetWindowLong(handle, NativeMethods.GwlExStyle) ^ NativeMethods.WsExToolWindow) | NativeMethods.WsExAppWindow);
        }

        public static int GetWindow(IntPtr handle) => NativeMethods.GetWindowLong(handle, NativeMethods.GwlExStyle);

        public static string StartTrayIfNotRunning()
        {
            if (Process.GetProcessesByName("TcNo-Acc-Switcher-Tray_main").Length > 0) return "Already running";
            if (!File.Exists("Tray_Users.json")) return "Tray users not found";
            var startInfo = new ProcessStartInfo { FileName = Path.Join(Globals.AppDataFolder, "TcNo-Acc-Switcher-Tray.exe"), CreateNoWindow = false, UseShellExecute = false, WorkingDirectory = Globals.AppDataFolder };
            try
            {
                _ = Process.Start(startInfo);
                return "Started Tray";
            }
            catch (Win32Exception win32Exception)
            {
                if (win32Exception.HResult != -2147467259) throw; // Throw is error is not: Requires elevation
                try
                {
                    startInfo.UseShellExecute = true;
                    startInfo.Verb = "runas";
                    _ = Process.Start(startInfo);
                    return "Started Tray";
                }

                catch (Win32Exception win32Exception2)
                {
                    if (win32Exception2.HResult != -2147467259) throw; // Throw is error is not: cancelled by user
                }
            }

            return "Could not start tray";
        }

        public static void RefreshTrayArea()
        {
            // NOTE:
            // "User Promoted Notification Area", and "Notification Area"
            // Need to be translated into other localised languages to work across computers that are NOT english.
            // Not entirely sure where I can get the other locale strings for Windows for something like this.
            // Not to mention detecting language.
            // So, tldr this only works for English Windows at the moment, which is the majority of users.

            try
            {
                var systemTrayContainerHandle = NativeMethods.FindWindow("Shell_TrayWnd", null);
                var systemTrayHandle = NativeMethods.FindWindowEx(systemTrayContainerHandle, IntPtr.Zero, "TrayNotifyWnd", null);
                var sysPagerHandle = NativeMethods.FindWindowEx(systemTrayHandle, IntPtr.Zero, "SysPager", null);
                var notificationAreaHandle = NativeMethods.FindWindowEx(sysPagerHandle, IntPtr.Zero, "ToolbarWindow32", "Notification Area");
                if (notificationAreaHandle == IntPtr.Zero)
                {
                    notificationAreaHandle = NativeMethods.FindWindowEx(sysPagerHandle, IntPtr.Zero, "ToolbarWindow32",
                        "User Promoted Notification Area");
                    var notifyIconOverflowWindowHandle = NativeMethods.FindWindow("NotifyIconOverflowWindow", null);
                    var overflowNotificationAreaHandle = NativeMethods.FindWindowEx(notifyIconOverflowWindowHandle, IntPtr.Zero,
                        "ToolbarWindow32", "Overflow Notification Area");
                    RefreshTrayArea(overflowNotificationAreaHandle);
                }

                RefreshTrayArea(notificationAreaHandle);
            }
            catch (Exception)
            {
                //
            }
        }

        private static void RefreshTrayArea(IntPtr windowHandle)
        {
            const uint wmMousemove = 0x0200;
            _ = NativeMethods.GetClientRect(windowHandle, out var rect);
            for (var x = 0; x < rect.right; x += 5)
            for (var y = 0; y < rect.bottom; y += 5)
                _ = NativeMethods.SendMessage(windowHandle, wmMousemove, 0, (y << 16) + x);
        }
    }
}

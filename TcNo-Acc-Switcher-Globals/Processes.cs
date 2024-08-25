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
using System.Collections.Generic;
using System.ComponentModel;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Management;
using System.Runtime.Versioning;
using System.Security.Principal;
using System.ServiceProcess;

namespace TcNo_Acc_Switcher_Globals
{
    public partial class Globals
    {
        #region PROCESSES
        public static bool IsAdministrator => OperatingSystem.IsWindows() && new WindowsPrincipal(WindowsIdentity.GetCurrent()).IsInRole(WindowsBuiltInRole.Administrator);
        public static void StartProgram(string path, bool elevated) => StartProgram(path, elevated, "");

        /// <summary>
        /// Starts a process with or without Admin
        /// </summary>
        /// <param name="path">Path of process to start</param>
        /// <param name="elevated">Whether the process should start elevated or not</param>
        /// <param name="args">Arguments to pass into the program</param>
        /// <param name="startMethod"></param>
        public static bool StartProgram(string path, bool elevated, string args, string startMethod = "Default")
        {
            var retStatus = true;
            if (!elevated && IsAdministrator)
            {
                // Set working directory
                Directory.SetCurrentDirectory(Path.GetDirectoryName(path) ?? Directory.GetCurrentDirectory());

                var res = ProcessHandler.TryRunAsDesktopUser(path, args);

                // Reset working dir
                Directory.SetCurrentDirectory(UserDataFolder);
            }
            else
            {
                try
                {
                    ProcessHandler.StartProgram(path, elevated, args, startMethod);
                }
                catch (Exception e)
                {
                    WriteToLog($"Failed to start process {path} due to error.", e);
                    retStatus = false;
                }
            }

            Directory.SetCurrentDirectory(UserDataFolder);
            return retStatus;
        }

        [SupportedOSPlatform("windows")]
        public static bool IsProcessService(string processFullPath, out string procName)
        {
            procName = null;

            try
            {
                if (processFullPath.Contains("\\")) // Is path not name
                {
                    using var searcher = new ManagementObjectSearcher("SELECT * FROM Win32_Service");
                    foreach (var o in searcher.Get())
                    {
                        var service = (ManagementObject)o;
                        if ((string)service["PathName"] != processFullPath &&
                            (string)service["PathName"] != '"' + processFullPath + '"') continue;

                        procName = (string)service["Name"];
                        return true;
                    }
                }
                else
                {
                    using var searcher = new ManagementObjectSearcher("SELECT * FROM Win32_Service WHERE Name = \"" + processFullPath + "\"");
                    foreach (var o in searcher.Get())
                    {
                        var service = (ManagementObject)o;
                        procName = (string)service["Name"];
                        return true;
                    }
                }
            }
            catch (ManagementException ex) when (ex.ErrorCode == ManagementStatus.ProviderLoadFailure)
            {
                // Handle specific "Provider load failure" error
                WriteToLog($"WMI Provider load failure when querying process: {processFullPath}. Exception: {ex.Message}");
                // Optionally, retry the operation or return a specific value
                // For now, we'll return false and leave procName as null
            }
            catch (Exception ex)
            {
                // Log any other unexpected exceptions
                WriteToLog($"Unexpected error when querying process: {processFullPath}. Exception: {ex.Message}");
            }

            return false;
        }


        public static bool CanKillProcess(List<string> procNames) => procNames.Aggregate(true, (current, s) => current & CanKillProcess(s));

        public static bool CanKillProcess(string processName)
        {
            // Checks whether program is running as Admin or not
            if (IsAdministrator)
                return true; // Elevated process can kill most other processes.
            if (OperatingSystem.IsWindows())
                return !ProcessHelper.IsProcessAdmin(processName); // If other process is admin, this process can't kill it (because it's not admin)
            return false;
        }

        /// <summary>
        /// Kills requested process. Will Write to Log and Console if unexpected output occurs (Doesn't start with "SUCCESS")
        /// </summary>
        /// <param name="procName">Process name to kill (Will be used as {name}*)</param>
        /// <param name="closingMethod">Closing method (Combined, Close, TaskKill)</param>
        public static bool KillProcess(string procName, string closingMethod = "Combined")
        {
            if (closingMethod == "Combined" || closingMethod == "Close")
            {
                if (!CanKillProcess(procName)) return false;

                // Try normal method first
                var processList = Process.GetProcesses().Where(pr => pr.ProcessName == procName.Split(".exe")[0]);

                foreach (var p in processList)
                {
                    p.Kill();
                }
            }

            if (closingMethod == "Combined" || closingMethod == "TaskKill")
            {
                // Refresh process list
                var processList = Process.GetProcesses().Where(pr => pr.ProcessName == procName.Split(".exe")[0]);

                // Then, just to make sure, run the other method
                if (!processList.Any())
                    return true;

                // Run alternate method if any still running
                var outputText = "";
                var startInfo = new ProcessStartInfo
                {
                    UseShellExecute = false,
                    WindowStyle = ProcessWindowStyle.Hidden,
                    FileName = "cmd.exe",
                    Arguments = $"/C TASKKILL /F /T /IM {procName}*",
                    CreateNoWindow = true,
                    RedirectStandardInput = true,
                    RedirectStandardOutput = true
                };

                var process = new Process { StartInfo = startInfo };
                process.OutputDataReceived += (_, e) => outputText += e.Data + "\n";
                _ = process.Start();
                process.BeginOutputReadLine();
                process.WaitForExit();

                WriteToLog(outputText.StartsWith("SUCCESS") || outputText.Length <= 1
                    ? $"Successfully closed {procName}."
                    : $"Tried to close {procName}. Unexpected output from cmd:\r\n{outputText}");
            }
            return true;
        }

        /// <summary>
        /// Kills requested processes (List of string). Will Write to Log and Console if unexpected output occurs (Doesn't start with "SUCCESS")
        /// </summary>
        /// <param name="inProcesses">List of processes to kill</param>
        /// <param name="closingMethod">Closing method (Combined, Close, TaskKill)</param>
        public static void KillProcess(List<string> inProcesses, string closingMethod = "Combined")
        {
            var procNames = new List<string>(inProcesses); // Don't leave as a reference -- This prevents removing it from the list too...

            if (closingMethod == "Combined" || closingMethod == "TaskKill")
            {
                // Try TaskKill method
                procNames.ForEach(p => KillProcess(p, "TaskKill"));
            }

            if (closingMethod == "Combined" || closingMethod == "Close")
            {
                // Check for services, and close them separately.
                List<string> toRemove = new();
                List<ServiceController> scList = null;
                if (OperatingSystem.IsWindows())
                    scList = new List<ServiceController>();
                var scListProcNames = new List<string>();
                foreach (var procName in procNames)
                {
                    var pathOrName = procName;
                    if (procName.StartsWith("SERVICE:"))
                        pathOrName = procName[8..]; // Remove "SERVICE:"

                    // Get full path if .exe, otherwise it's the name of the service (Hopefully)
                    if (pathOrName.Contains(".exe"))
                    {
                        var processes = Process.GetProcesses();
                        var nameWithoutExe = pathOrName.Replace(".exe", "");
                        foreach (var p in processes)
                        {
                            if (!p.ProcessName.Contains(nameWithoutExe)) continue;
                            try
                            {
                                pathOrName = p.MainModule?.FileName; // Admin is required for this!
                            }
                            catch (Win32Exception e)
                            {
                                WriteToLog("Tried to get MainModule from process but failed.", e);
                            }
                            break;
                        }
                    }

                    if (pathOrName == null) continue; // This process was already closed, or is not running.

                    // Was not explicitly tagged as a service, but checking anyway is good!
                    if (!OperatingSystem.IsWindows() || !IsProcessService(pathOrName, out var sName)) continue;
                    if (sName == null) continue;
                    // Do also try and get the MainModule.Filename (or whatever it is) in the CanKillProcess function, to check if can kill... This seems to be a more reliable way of checking as well? Maybe just for services... use IsProcessService() for that as well?
                    scList?.Add(new ServiceController(sName));
                    scListProcNames.Add(pathOrName);
                    toRemove.Add(procName);
                }

                foreach (var rem in toRemove)
                    procNames.Remove(rem);

                // Kill services all at once to speed everything up.
                if (OperatingSystem.IsWindows() && scList != null)
                    for (var i = 0; i < scList.Count; i++)
                    {
                        try
                        {
                            scList[i].Stop();
                        }
                        catch (Exception)
                        {
                            // Catch cannot stop on computer... Then end using less reliable TaskKill.
                            KillProcess(scListProcNames[i], "TaskKill");
                        }
                    }

                // Try normal method
                var processedNames = procNames.Select(procName => procName.Split(".exe")[0]).ToList();

                var processList = Process.GetProcesses().Where(pr => processedNames.Contains(pr.ProcessName));

                foreach (var process in processList)
                {
                    try
                    {
                        process.Kill();
                    }
                    catch (Win32Exception)
                    {
                        // Already closed
                    }
                }
            }
        }
        public class ProcessHandler
        {
            /// <summary>
            /// Start a program
            /// </summary>
            /// <param name="fileName">Path to file</param>
            /// <param name="elevated">Whether program should be elevated</param>
            /// <param name="args">Arguments for program</param>
            /// <param name="startMethod"></param>
            public static void StartProgram(string fileName, bool elevated, string args = "", string startMethod = "Default")
            {
                ProcessStartInfo prodStartInfo;

                if (startMethod == "Default")
                {
                    var runasPath = Path.Join(AppDataFolder, "runas.exe");
                    if (!File.Exists(runasPath)) throw new Exception("Could not find runas.exe");

                    // This runas.exe program is a temporary workaround for processes closing when this closes.
                    prodStartInfo = new ProcessStartInfo
                    {
                        FileName = runasPath,
                        Arguments = $"\"{fileName}\" {(elevated ? "1" : "0")} {args}",
                        UseShellExecute = true,
                        Verb = elevated ? "runas" : ""
                    };
                }
                else
                {
                    prodStartInfo = new ProcessStartInfo
                    {
                        FileName = fileName,
                        Arguments = args,
                        UseShellExecute = true,
                        Verb = elevated ? "runas" : ""
                    };
                }

                try
                {
                    Process.Start(prodStartInfo);
                }
                catch (Win32Exception e)
                {
                    if (e.HResult != -2147467259) // Not because it was cancelled by user
                        throw;
                }
            }

            // See unmodified code source from link below:
            // https://stackoverflow.com/a/40501607/5165437
            public static bool TryRunAsDesktopUser(string fileName, string args = "")
            {
                var res = false;
                try
                {
                    res = RunAsDesktopUser(fileName, args);
                }
                catch (Exception e)
                {
                    WriteToLog("FAILED: TryRunAsDesktopUser.", e);
                }

                return res;
            }
            public static bool RunAsDesktopUser(string fileName, string args = "")
            {
                if (string.IsNullOrWhiteSpace(fileName))
                {
                    WriteToLog("Value cannot be null or whitespace." + fileName);
                    return false;
                }

                // To start process as shell user you will need to carry out these steps:
                // 1. Enable the SeIncreaseQuotaPrivilege in your current token
                // 2. Get an HWND representing the desktop shell (GetShellWindow)
                // 3. Get the Process ID(PID) of the process associated with that window(GetWindowThreadProcessId)
                // 4. Open that process(OpenProcess)
                // 5. Get the access token from that process (OpenProcessToken)
                // 6. Make a primary token with that token(DuplicateTokenEx)
                // 7. Start the new process with that primary token(CreateProcessWithTokenW)

                var hProcessToken = IntPtr.Zero;
                // Enable SeIncreaseQuotaPrivilege in this process.  (This won't work if current process is not elevated.)
                try
                {
                    var process = NativeMethods.GetCurrentProcess();
                    if (!NativeMethods.OpenProcessToken(process, 0x0020, ref hProcessToken))
                    {
                        WriteToLog("FAILED: RunAsDesktopUser - OpenProcessToken" + fileName);
                        return false;
                    }

                    var tkp = new NativeMethods.TokenPrivileges
                    {
                        PrivilegeCount = 1,
                        Privileges = new NativeMethods.LuidAndAttributes[1]
                    };

                    if (!NativeMethods.LookupPrivilegeValue(null, "SeIncreaseQuotaPrivilege", ref tkp.Privileges[0].Luid))
                    {
                        WriteToLog("FAILED: RunAsDesktopUser - LookupPrivilegeValue" + fileName);
                        return false;
                    }

                    tkp.Privileges[0].Attributes = 0x00000002;

                    if (!NativeMethods.AdjustTokenPrivileges(hProcessToken, false, ref tkp, 0, IntPtr.Zero, IntPtr.Zero))
                    {
                        WriteToLog("FAILED: RunAsDesktopUser - AdjustTokenPrivileges" + fileName);
                        return false;
                    }
                }
                finally
                {
                    NativeMethods.CloseHandle(hProcessToken);
                }

                // Get an HWND representing the desktop shell.
                // CAVEATS:  This will fail if the shell is not running (crashed or terminated), or the default shell has been
                // replaced with a custom shell.  This also won't return what you probably want if Explorer has been terminated and
                // restarted elevated.
                var hwnd = NativeMethods.GetShellWindow();
                if (hwnd == IntPtr.Zero)
                {
                    WriteToLog("FAILED: RunAsDesktopUser - hwnd == IntPtr.Zero" + fileName);
                    return false;
                }

                var hShellProcess = IntPtr.Zero;
                var hShellProcessToken = IntPtr.Zero;
                var hPrimaryToken = IntPtr.Zero;
                try
                {
                    // Get the PID of the desktop shell process.
                    if (NativeMethods.GetWindowThreadProcessId(hwnd, out var dwPid) == 0)
                    {
                        WriteToLog("FAILED: RunAsDesktopUser - GetWindowThreadProcessId" + fileName);
                        return false;
                    }

                    // Open the desktop shell process in order to query it (get the token)
                    hShellProcess = NativeMethods.OpenProcess(NativeMethods.ProcessAccessFlags.QueryInformation, false, dwPid);
                    if (hShellProcess == IntPtr.Zero)
                    {
                        WriteToLog("FAILED: RunAsDesktopUser - hShellProcess == IntPtr.Zero" + fileName);
                        return false;
                    }

                    // Get the process token of the desktop shell.
                    if (!NativeMethods.OpenProcessToken(hShellProcess, 0x0002, ref hShellProcessToken))
                    {
                        WriteToLog("FAILED: RunAsDesktopUser - OpenProcessToken" + fileName);
                        return false;
                    }

                    const uint dwTokenRights = 395U;

                    // Duplicate the shell's process token to get a primary token.
                    // Based on experimentation, this is the minimal set of rights required for CreateProcessWithTokenW (contrary to current documentation).
                    if (!NativeMethods.DuplicateTokenEx(hShellProcessToken, dwTokenRights, IntPtr.Zero, NativeMethods.SecurityImpersonationLevel.SecurityImpersonation, NativeMethods.TokenType.TokenPrimary, out hPrimaryToken))
                    {
                        WriteToLog("FAILED: RunAsDesktopUser - DuplicateTokenEx" + fileName);
                        return false;
                    }

                    // Arguments need a space just before, for some reason.
                    if (args is {Length: > 1} && args[0] != ' ') args = ' ' + args;

                    // Start the target process with the new token.
                    var si = new NativeMethods.StartupInfo();
                    if (!NativeMethods.CreateProcessWithTokenW(hPrimaryToken, 0, fileName, args, 0, IntPtr.Zero, Path.GetDirectoryName(fileName), ref si, out _))
                    {
                        WriteToLog("FAILED: RunAsDesktopUser - CreateProcessWithTokenW" + fileName);
                        return false;
                    }
                }
                finally
                {
                    NativeMethods.CloseHandle(hShellProcessToken);
                    NativeMethods.CloseHandle(hPrimaryToken);
                    NativeMethods.CloseHandle(hShellProcess);
                }

                return true;
            }

            /* Unused
            public static void BringWindowToForeground(string appName) { Process.GetProcessesByName(appName).FirstOrDefault(); }
            public static void BringWindowToForeground(Process p)
            {
                if (p == null) return;

                var h = p.MainWindowHandle;
                NativeMethods.SetForegroundWindow(h);
            }


            #region Interop
            public static void SendMessageToProcess(IntPtr hWnd)
            {
                // I am stuck with this for Discord... https://stackoverflow.com/questions/60893997/how-to-close-discord-programmatically
                // For those looking here's the codes: https://wiki.winehq.org/List_Of_Windows_Messages
                if (hWnd.ToInt32() != 0) NativeMethods.PostMessage(hWnd, NativeMethods.WmQueryEndSession, 1, NativeMethods.EndSessionCloseApp);
                //PostMessage(hWnd, WM_CLOSE, 0, 0);

            }
            #endregion
            */
        }
        public class ProcessHelper
        {
            //https://stackoverflow.com/a/5479957/5165437

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
                var processes = Process.GetProcessesByName(processName.Split(".exe")[0]);
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
                        WriteToLog($"Handled error. Tried to check admin status of process: {processName}", a);
                    }
                }

                return true;
            }

            [SupportedOSPlatform("windows")]
            public static bool IsProcessRunning(string processName)
            {
                var proc = Process.GetProcessesByName(processName.Split(".exe")[0]);

                return proc.Length != 0;
            }

            [SupportedOSPlatform("windows")]
            private static bool IsHandleAdmin(IntPtr handle)
            {
                _ = NativeMethods.OpenProcessToken(handle, AllAccess, out var ph);

                if (ph == IntPtr.Zero) return true;

                var identity = new WindowsIdentity(ph);

                var result =
                    (from role in identity.Groups
                     where role.IsValidTargetType(typeof(SecurityIdentifier))
                     select role as SecurityIdentifier).Any(sid =>
             sid.IsWellKnown(WellKnownSidType.AccountAdministratorSid) ||
             sid.IsWellKnown(WellKnownSidType.BuiltinAdministratorsSid));

                _ = NativeMethods.CloseHandle(ph);

                return result;
            }
        }
        #endregion

    }
}

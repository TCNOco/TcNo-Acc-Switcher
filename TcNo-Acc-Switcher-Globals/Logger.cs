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
using System.Globalization;
using System.IO;
using System.Reflection;
using System.Runtime.InteropServices;
using System.Threading;

namespace TcNo_Acc_Switcher_Globals
{
    public partial class Globals
    {
        #region LOGGER
        public static string MessageFromHResult(int hr)
        {
            return Marshal.GetExceptionForHR(hr)?.Message;
        }
        public static string GetLogPath() => Path.Join(UserDataFolder, "log.txt");
        public static void DebugWriteLine(string s)
        {
            // Toggle here so it only shows in Verbose mode etc.
            if (VerboseMode) Console.WriteLine($@"{DateTime.Now:hh:mm:ss.fff} - {s}");
        }

        public static void WriteToLog(Exception e) => WriteToLog($"HANDLED/IGNORED ERROR: {GetEnglishError(e)}");
        public static void WriteToLog(string desc, Exception e) => WriteToLog($"ERROR: {desc}\n{GetEnglishError(e)}");

        public static string GetEnglishError(Exception e)
        {
            // Set error to English
            var oldCulture = Thread.CurrentThread.CurrentCulture;
            Thread.CurrentThread.CurrentCulture = CultureInfo.InvariantCulture;
            Thread.CurrentThread.CurrentUICulture = CultureInfo.InvariantCulture;

            var toReturn = e.ToString();

            // Reset language
            Thread.CurrentThread.CurrentCulture = oldCulture;
            Thread.CurrentThread.CurrentUICulture = oldCulture;

            return toReturn;
        }

        /// <summary>
        /// Append line to log file
        /// </summary>
        public static void WriteToLog(string s)
        {
            var attempts = 0;
            while (attempts <= 30) // Up to 3 seconds
            {
                try
                {
                    File.AppendAllText(Path.Join(UserDataFolder, "log.txt"), s + Environment.NewLine);
                    break;
                }
                catch (IOException)
                {
                    if (attempts == 5)
                        throw;
                    attempts++;
                    Thread.Sleep(100);
                }
            }

            // Console exists
            if (NativeMethods.GetConsoleWindow() == IntPtr.Zero) return;

            if (s.ToLowerInvariant().Contains("exception"))
            {
                Console.ForegroundColor = ConsoleColor.Red;
                Console.WriteLine(s);
                Console.ForegroundColor = ConsoleColor.Gray;
                return;
            }

            Console.WriteLine(s);
        }

        /// <summary>
        /// Clear log files & Combine other log files.
        /// </summary>
        public static void ClearLogs()
        {
            // Clear original log file
            DeleteFile("log.txt");
            // Check for other log files. Combine them and delete them.
            var filepath = Path.GetDirectoryName(Assembly.GetEntryAssembly()?.Location);
            if (filepath == null) return;
            var d = new DirectoryInfo(filepath);
            var appendText = new List<string>();
            var oldFilesFound = false;
            foreach (var file in d.GetFiles("log*.txt"))
            {
                if (appendText.Count == 0) appendText.Add(Environment.NewLine + "-------- OLD --------");
                try
                {
                    appendText.AddRange(ReadAllLines(file.FullName));
                    File.Delete(file.FullName);
                    oldFilesFound = true;
                }
                catch (Exception)
                {
                    //
                }
            }

            if (oldFilesFound) appendText.Add(Environment.NewLine + "-------- END OF OLD --------");

            // Insert their contents into the actual log file
            try
            {
                File.WriteAllLines("log.txt", appendText);
            }
            catch (IOException)
            {
                // Could not write to log file.
                // Probably in use.
                // Just ignore. Don't crash.
                // Crashing happened way too often because of this. Makes no sense to log these elsewhere.
            }
        }

        /// <summary>
        /// Exception handling for all programs
        /// </summary>
        public static void CurrentDomain_UnhandledException(object sender, UnhandledExceptionEventArgs e)
        {
            // Set working directory to documents folder
            Directory.SetCurrentDirectory(UserDataFolder);

            // Log Unhandled Exception
            try
            {
                // Set error to English
                var oldCulture = Thread.CurrentThread.CurrentCulture;
                Thread.CurrentThread.CurrentCulture = CultureInfo.InvariantCulture;
                Thread.CurrentThread.CurrentUICulture = CultureInfo.InvariantCulture;

                var exceptionStr = e.ExceptionObject.ToString();
                _ = Directory.CreateDirectory("CrashLogs");
                var filePath = $"CrashLogs\\AccSwitcher-Crashlog-{DateTime.Now:dd-MM-yy_hh-mm-ss.fff}.txt";
                if (File.Exists(filePath))
                {
                    Random r = new();
                    filePath = $"CrashLogs\\AccSwitcher-Crashlog-{DateTime.Now:dd-MM-yy_hh-mm-ss.fff}{r.Next(0, 100)}.txt";
                }

                using (var sw = File.AppendText(filePath))
                {
                    sw.WriteLine(
                        $"{DateTime.Now.ToString(CultureInfo.InvariantCulture)} ({Version})\t{Strings.ErrUnhandledCrash}: {exceptionStr}{Environment.NewLine}{Environment.NewLine}");
                }

                WriteToLog(Strings.ErrUnhandledException + Path.GetFullPath(filePath));
                WriteToLog(Strings.ErrSubmitCrashlog);

                //To display an error notification, the main program needs to be started as that's the Windows client. This is a cross-platform compatible binary. Adding the Presentation DLL causes issues.
                File.WriteAllText("LastError.txt",
                    "Fatal error occurred!" + Environment.NewLine +
                    Environment.NewLine + "Error: " + e.ExceptionObject);

                // Reset language
                Thread.CurrentThread.CurrentCulture = oldCulture;
                Thread.CurrentThread.CurrentUICulture = oldCulture;
            }
            catch (Exception)
            {
                // This is just to prevent a complete crash. Sometimes there are multiple errors super close together, that cause it to break and we end up here.
            }
        }

#endregion

    }
}

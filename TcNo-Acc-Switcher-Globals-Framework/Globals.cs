using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Reflection;
using System.Runtime.InteropServices;
using System.Text;
using System.Threading.Tasks;

namespace TcNo_Acc_Switcher_Globals_Framework
{
    internal static class NativeMethods
    {
        [DllImport("kernel32.dll", SetLastError = true)]
        public static extern IntPtr GetConsoleWindow();
    }
    public class Globals
    {
        public static bool VerboseMode;

        private static string _userDataFolder = "";
        public static string UserDataFolder
        {
            get
            {
                if (!string.IsNullOrEmpty(_userDataFolder)) return _userDataFolder;
                if (File.Exists(Path.Combine(AppDataFolder, "userdata_path.txt")))
                    _userDataFolder = ReadAllLines(PathJoin(AppDataFolder, "userdata_path.txt"))[0].Trim();
                else
                    _userDataFolder = PathJoin(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "TcNo Account Switcher\\");

                return _userDataFolder;
            }
            set
            {
                _userDataFolder = value;
                try
                {
                    File.WriteAllText(PathJoin(AppDataFolder, "userdata_path.txt"), value);
                }
                catch (Exception)
                {
                    // Failed to write to file.
                    // This setter will be unused for now at least, so this isn't really necessary.
                }
            }
        }

        public static string AppDataFolder =>
            Path.GetDirectoryName(Assembly.GetEntryAssembly()?.Location) ?? string.Empty;

        #region LOGGER

        public static void DebugWriteLine(string s)
        {
            // Toggle here so it only shows in Verbose mode etc.
            if (VerboseMode) Console.WriteLine(s);
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
                    File.AppendAllText(PathJoin(UserDataFolder, "log.txt"), s + Environment.NewLine);
                    break;
                }
                catch (IOException)
                {
                    if (attempts == 5)
                        throw;
                    attempts++;
                    System.Threading.Thread.Sleep(100);
                }
            }

            if (NativeMethods.GetConsoleWindow() != IntPtr.Zero) // Console exists
                Console.WriteLine(s);
        }
        #endregion

        #region FILES
        /// <summary>
        /// A replacement for File.ReadAllLines() that doesn't crash if a file is in use.
        /// </summary>
        /// <param name="f">File to be read</param>
        /// <returns>string[] of content</returns>
        public static string[] ReadAllLines(string f)
        {
            using (var fs = new FileStream(f, FileMode.Open, FileAccess.Read, FileShare.ReadWrite))
            {
                using (var sr = new StreamReader(fs))
                {
                    var l = new List<string>();
                    while (!sr.EndOfStream)
                    {
                        l.Add(sr.ReadLine());
                    }
                    return l.ToArray();
                }
            }
        }
        #endregion

        #region NETtoFramework
        public static string PathJoin(string s1, string s2)
        {
            if (IsDirectorySeparator(s1[s1.Length - 1])) return s1 + s2;
            return s1 + "\\" + s2;
        }

        private static bool IsDirectorySeparator(char c) => c == '\\' || c == '/';

        #endregion
    }
}

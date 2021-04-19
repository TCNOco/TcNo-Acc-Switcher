using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Globalization;
using System.IO;
using System.Linq;
using Newtonsoft.Json;

namespace TcNo_Acc_Switcher_Globals
{
    public class Globals
    {
        public string WorkingDirectory { get; set; }
        public DateTime UpdateLastChecked { get; set; } = DateTime.Now;
        public static bool VerboseMode = false;

        public static void DebugWriteLine(string s)
        {
            // Toggle here so it only shows in Verbose mode etc.
            if (VerboseMode) Console.WriteLine(s);
        }


        // Read existing settings. If they don't exist, create them.
        // --> Reads from current directory. This will only work with the main TcNo Account Switcher app.
        //     Other apps will need to find the correct directory first.
        public static Globals LoadExisting(string fromDir)
        {
            var globalsFile = Path.Combine(fromDir, "globals.json");

            Globals g;
            if (File.Exists(globalsFile))
                g = JsonConvert.DeserializeObject<Globals>(File.ReadAllText(globalsFile));
            else
            {
                g = new Globals();
                File.WriteAllText(globalsFile, JsonConvert.SerializeObject(g));
            }

            Debug.Assert(g != null, nameof(g) + " != null");
            g.WorkingDirectory = fromDir;
            return g;
        }

        public static void Save(Globals g)
        {
            var globalsFile = Path.Combine(g.WorkingDirectory, "globals.json");
            File.WriteAllText(globalsFile, JsonConvert.SerializeObject(g, Formatting.Indented));
        }
        
        /// <summary>
        /// Exception handling for all programs
        /// </summary>
        public static void CurrentDomain_UnhandledException(object sender, UnhandledExceptionEventArgs e)
        {
            Directory.SetCurrentDirectory(Path.GetDirectoryName(System.Reflection.Assembly.GetEntryAssembly()?.Location) ?? string.Empty); // Set working directory to same as .exe
            // Log Unhandled Exception
            var exceptionStr = e.ExceptionObject.ToString();
            Directory.CreateDirectory("CrashLogs");
            var filePath = $"CrashLogs\\AccSwitcher-Crashlog-{DateTime.Now:dd-MM-yy_hh-mm-ss.fff}.txt";
            using (var sw = File.AppendText(filePath))
            {
                sw.WriteLine(DateTime.Now.ToString(CultureInfo.InvariantCulture) + "\t" + Strings.ErrUnhandledCrash + ": " + exceptionStr + Environment.NewLine + Environment.NewLine);
            }
            Console.WriteLine(Strings.ErrUnhandledException + Path.GetFullPath(filePath));
            Console.WriteLine(Strings.ErrSubmitCrashlog);
        }

        public static void WriteLogLine(string line)
        {
            Directory.SetCurrentDirectory(Path.GetDirectoryName(System.Reflection.Assembly.GetEntryAssembly()?.Location) ?? string.Empty); // Set working directory to same as .exe
            Directory.CreateDirectory("Errors");
            File.AppendAllText("log.txt", $"{DateTime.Now:dd-MM-yy_hh-mm-ss.fff}: {line}\r\n");
        }


        /// <summary>
        /// Kills requested process. Will Write to Log and Console if unexpected output occurs (Anything more than "") 
        /// </summary>
        /// <param name="procName">Process name to kill (Will be used as {name}*)</param>
        public static void KillProcess(string procName)
        {

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
            process.Start();
            process.BeginOutputReadLine();
            process.WaitForExit();

            Console.WriteLine($"Tried to close {procName}. Unexpected output from cmd:\r\n{outputText}");
        }
    }

    public class TrayUser
    {
        public string Name { get; set; } = ""; // Name to display on list
        public string Arg { get; set; } = "";  // Argument used to switch to this account

        /// <summary>
        /// Reads Tray_Users.json, and returns a dictionary of strings, with a list of TrayUsers attached to them.
        /// </summary>
        /// <returns>Dictionary of keys, and associated lists of tray users</returns>
        public static Dictionary<string, List<TrayUser>> ReadTrayUsers()
        {
            if (!File.Exists("Tray_Users.json")) return new();
            var json = File.ReadAllText("Tray_Users.json");
            return JsonConvert.DeserializeObject<Dictionary<string, List<TrayUser>>>(json);
        }

        /// <summary>
        /// Adds a user to the beginning of the [Key]s list of TrayUsers. Moves to position 0 if exists.
        /// </summary>
        /// <param name="trayUsers">Reference to Dictionary of keys & list of TrayUsers</param>
        /// <param name="key">Key to add user to</param>
        /// <param name="newUser">user to add to aforementioned key in dictionary</param>
        public static void AddUser(ref Dictionary<string, List<TrayUser>> trayUsers, string key, TrayUser newUser)
        {
            // Create key and add item if doesn't exist already
            if (!trayUsers.ContainsKey(key))
            {
                trayUsers.Add(key, new List<TrayUser>(new[] {newUser}));
                return;
            }

            // If key contains -> Remove it
            trayUsers[key] = trayUsers[key].Where(x => x.Arg != newUser.Arg).ToList();
            // Add item into first slot
            trayUsers[key].Insert(0, newUser);
            // Shorten list to be a max of 3
            while (trayUsers[key].Count > 3) trayUsers[key].RemoveAt(trayUsers[key].Count - 1);
        }

        /// <summary>
        /// Saves trayUsers list to file.
        /// </summary>
        public static void SaveUsers(Dictionary<string, List<TrayUser>> trayUsers) => File.WriteAllText("Tray_Users.json", JsonConvert.SerializeObject(trayUsers));
    }
}

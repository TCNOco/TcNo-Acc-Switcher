using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Reflection;
using System.Text;
using System.Threading.Tasks;

namespace TcNo_Acc_Switcher_Updater
{
    public class Logger
    {
        private static Logger _instance;
        private static readonly object LockObj = new();
        private StreamWriter _logWriter;

        private Logger() { }

        public static Logger Instance
        {
            get
            {
                lock (LockObj)
                {
                    return _instance ??= new Logger();
                }
            }
        }

        public static StreamWriter LogWriter
        {
            get
            {
                lock (LockObj)
                {
                    return Instance._logWriter ??= InitLogWriter();
                }
            }
        }

        public static void WriteLine(string line)
        {
            LogWriter.WriteLine(line);
            LogWriter.Flush();
        }

        private static string AppDataFolder =>
            Directory.GetParent(Path.GetDirectoryName(Assembly.GetEntryAssembly()?.Location) ?? string.Empty)?.FullName;
        private static StreamWriter InitLogWriter()
        {
            var logPath = File.Exists(Path.Join(AppDataFolder, "userdata_path.txt"))
                ? UGlobals.ReadAllLines(Path.Join(AppDataFolder, "userdata_path.txt"))[0].Trim()
                : Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "TcNo Account Switcher\\");

            if (!Directory.Exists(logPath)) Directory.CreateDirectory(logPath);
            logPath = Path.Join(logPath, "UpdaterLog.txt");

            if (File.Exists(logPath)) File.WriteAllText(logPath, string.Empty);

            return new StreamWriter(logPath, append: true);
        }
    }
}

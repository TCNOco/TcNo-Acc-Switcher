using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Net.Http;
using System.Text;
using System.Threading.Tasks;

namespace TcNo_Acc_Switcher_Globals
{
    public partial class Globals
    {
        #region FILES

        public static bool DeleteFiles(string path, bool throwErr = false)
        {
            return Directory.GetFiles(path).Aggregate(false, (current, f) => DeleteFile(f, true) || current);
        }
        public static bool DeleteFile(string path, bool throwErr = false)
        {
            try
            {
                if (File.Exists(path))
                    File.Delete(path);
                return true;
            }
            catch (Exception e)
            {
                WriteToLog($"Could not delete ({MessageFromHResult(e.HResult)}): {path}");
                if (throwErr)
                    throw;
                return false;
            }
        }

        private static readonly string[] SizeSuffixes =
            { "bytes", "KB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB" };

        /// <summary>
        /// Convert byte length of file to string (KB, MB, GB...)
        /// </summary>
        public static string FileLengthToString(long value, int decimalPlaces = 2)
        {
            if (value < 0) { return "-" + FileLengthToString(-value, decimalPlaces); }

            var i = 0;
            var dValue = (decimal)value;
            while (Math.Round(dValue, decimalPlaces) >= 1000)
            {
                dValue /= 1024;
                i++;
            }

            return string.Format("{0:n" + decimalPlaces + "} {1}", dValue, SizeSuffixes[i]);
        }

        public static bool IsDirectoryEmpty(string path)
        {
            return !Directory.EnumerateFileSystemEntries(path).Any();
        }

        private static readonly HttpClient HClient = new();
        public static string DownloadString(string uri) => HClient.GetStringAsync(uri).Result;
        public static bool DownloadFile(string url, string path)
        {
            try
            {
                if (!Uri.TryCreate(url, UriKind.Absolute, out _))
                    throw new InvalidOperationException("URI is invalid.");

                var fileBytes = HClient.GetByteArrayAsync(url).Result;
                File.WriteAllBytes(path, fileBytes);
            }
            catch (Exception)
            {
                return false;
            }
            return true;
        }

        /// <summary>
        /// A replacement for File.ReadAllText() that doesn't crash if a file is in use.
        /// </summary>
        /// <param name="f">File to be read</param>
        /// <returns>string of content</returns>
        public static string ReadAllText(string f)
        {
            using var fs = new FileStream(f, FileMode.Open, FileAccess.Read, FileShare.ReadWrite);
            using var tr = new StreamReader(fs);
            return tr.ReadToEnd();
        }

        /// <summary>
        /// A replacement for File.ReadAllLines() that doesn't crash if a file is in use.
        /// </summary>
        /// <param name="f">File to be read</param>
        /// <returns>string[] of content</returns>
        public static string[] ReadAllLines(string f)
        {
            using var fs = new FileStream(f, FileMode.Open, FileAccess.Read, FileShare.ReadWrite);
            using var sr = new StreamReader(fs);
            var l = new List<string>();
            while (!sr.EndOfStream)
            {
                l.Add(sr.ReadLine());
            }

            return l.ToArray();
        }

        // Overload for below
        public static void CopyFilesRecursive(string inputFolder, string outputFolder) =>
            CopyFilesRecursive(inputFolder, outputFolder, false);
        /// <summary>
        /// Recursively copy files and directories
        /// </summary>
        /// <param name="inputFolder">Folder to copy files recursively from</param>
        /// <param name="outputFolder">Destination folder</param>
        /// <param name="overwrite">Whether to overwrite files or not</param>
        public static void CopyFilesRecursive(string inputFolder, string outputFolder, bool overwrite)
        {
            _ = Directory.CreateDirectory(outputFolder);
            outputFolder = outputFolder.EndsWith("\\") ? outputFolder : outputFolder + "\\";
            //Now Create all of the directories
            foreach (var dirPath in Directory.GetDirectories(inputFolder, "*", SearchOption.AllDirectories))
                _ = Directory.CreateDirectory(dirPath.Replace(inputFolder, outputFolder));

            //Copy all the files & Replaces any files with the same name
            foreach (var newPath in Directory.GetFiles(inputFolder, "*.*", SearchOption.AllDirectories))
            {
                var dest = newPath.Replace(inputFolder, outputFolder);
                if (!overwrite && File.Exists(dest)) continue;

                File.Copy(newPath, dest, true);
            }
        }

        #endregion
    }
}

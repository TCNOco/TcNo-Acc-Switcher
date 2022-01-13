using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Security.Cryptography;
using System.Text;
using System.Text.RegularExpressions;
using System.Threading.Tasks;

namespace TcNo_Acc_Switcher_Globals
{
    public partial class Globals
    {
        #region FOLDERS
        /// <summary>
        ///  Gets a hash for the provided Data folder (Ignores specific folders in it).
        /// </summary>
        /// <param name="path">Directory to hash</param>
        /// <returns>Hash string</returns>
        public static string GetDataFolderMd5(string path)
        {
            // assuming you want to include nested folders
            var files = Directory.GetFiles(path, "*.*", SearchOption.AllDirectories).OrderBy(p => p).ToList();

            var md5 = MD5.Create();
            byte[] lastBytes = null;
            foreach (var file in files)
            {
                if (lastBytes != null)
                {
                    _ = md5.TransformBlock(lastBytes, 0, lastBytes.Length, lastBytes, 0);
                }

                if (file.Contains("profiles")) // Ignore user-customisable folders.
                    continue;

                var pathBytes = Encoding.UTF8.GetBytes(file[(path.Length + 1)..].ToLower());
                _ = md5.TransformBlock(pathBytes, 0, pathBytes.Length, pathBytes, 0);

                try
                {
                    lastBytes = File.ReadAllBytes(file);
                }
                catch (IOException)
                {
                    // File is in use
                }
            }

            if (lastBytes != null)
                _ = md5.TransformFinalBlock(lastBytes, 0, lastBytes.Length);

            return BitConverter.ToString(md5.Hash ?? Array.Empty<byte>()).Replace("-", "").ToLower();
        }

        public static bool RecursiveDelete(string baseDir, bool keepFolders) =>
            RecursiveDelete(new DirectoryInfo(baseDir), keepFolders);
        public static bool RecursiveDelete(DirectoryInfo baseDir, bool keepFolders)
        {
            if (!baseDir.Exists)
                return true;

            try
            {
                foreach (var dir in baseDir.EnumerateDirectories())
                {
                    RecursiveDelete(dir, keepFolders);
                }
                var files = baseDir.GetFiles();
                foreach (var file in files)
                {
                    if (!file.Exists) continue;
                    file.IsReadOnly = false;
                    file.Delete();
                }

                if (keepFolders) return true;
                baseDir.Delete();
                return true;
            }
            catch (UnauthorizedAccessException e)
            {
                WriteToLog("RecursiveDelete failed", e);
                return false;
            }
        }

        /// <summary>
        /// Remove illegal characters from file path string
        /// </summary>
        public static string GetCleanFilePath(string f)
        {
            var regexSearch = new string(Path.GetInvalidFileNameChars()) + new string(Path.GetInvalidPathChars());
            var r = new Regex($"[{Regex.Escape(regexSearch)}]");
            return r.Replace(f, "");
        }
        #endregion
    }
}

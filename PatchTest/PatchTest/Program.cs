using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Net;
using System.Security.Cryptography;
using System.Text.Json.Serialization;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using SevenZipExtractor;
using VCDiff.Encoders;
using VCDiff.Decoders;
using VCDiff.Includes;
using VCDiff.Shared;

namespace PatchTest
{
    class Program
    {
        // Dictionaries of file paths, as well as MD5 Hashes
        private static Dictionary<string, string> oldDict = new();
        private static Dictionary<string, string> newDict = new();
        private static List<string> patchList = new();


        const string CurrentVersion = "2021-03-25";
        static void Main(string[] args)
        {
            //CreateUpdate();


            // 1. Download update.7z
            // 2. Extract --> Done, mostly
            // 3. Delete files that need to be removed. --> Done, mostly
            // 4. Run patches from working folder and patches --> Done, mostly
            // 5. Copy in new files --> Done
            // 6. Delete working temp folder, and update --> Done
            // 7. Download latest hash list and verify
            // --> Update the updater through the main program with a simple hash check there.
            // ----> Just re-download the whole thing, or make a copy of it to update itself with,
            // ----> then get the main program to replace the updater (Most likely).

            var updaterDirectory = Path.GetDirectoryName(System.Reflection.Assembly.GetEntryAssembly()?.Location); // Where this program is located
            Directory.SetCurrentDirectory(Directory.GetParent(updaterDirectory!)!.ToString()); // Set working directory to same as .exe
            var currentDir = Directory.GetCurrentDirectory();

            // Get info on updates, and get updates since last:
            var updatesAndChanges = new Dictionary<string, string>();
            GetUpdatesList(ref updatesAndChanges);

            // Check if files are in use
            CloseIfRunning(currentDir);
            
            // APPLY UPDATE
            // Cleanup previous updates
            if (Directory.Exists("temp_update")) Directory.Delete("temp_update", true);
            

            foreach (var kvp in updatesAndChanges)
            {
                // This update .exe exists inside an "update" folder, in the TcNo Account Switcher directory.
                // Set the working folder as the parent to this one, where the main program files are located.
                // TODO: Query and download update into new own folder.


                // Apply update
                var updateFilePath = Path.Combine(updaterDirectory, "update1.7z");
                ApplyUpdate(updateFilePath);


                // Compare hash list to files, and download any files that don't match
                //var client = new WebClient();
                var client = new WebClient();
                var verifyDictionary = JsonConvert.DeserializeObject<Dictionary<string, string>>(File.ReadAllText(Path.Join("temp_update", "hashes.json")));
                if (verifyDictionary != null)
                    foreach (var (key, value) in verifyDictionary)
                    {
                        if (!File.Exists(key)) continue;
                        var md5 = GetFileMD5(key);
                        if (value == md5) continue;

                        Console.WriteLine("File: " + key + " has MD5: " + md5 + " EXPECTED: " + value);
                        Console.WriteLine("Deleting: " + key);
                        DeleteFile(key);
                        Console.Write("Downloading file from website... ");
                        var uri = new Uri("https://tcno.co/Projects/AccSwitcher/latest/" + key.Replace('\\', '/'));
                        client.DownloadFile(uri, key);
                        Console.WriteLine("Done.");
                    }
            }


            
            Directory.Delete("temp_update", true);
        }

        static void CloseIfRunning(string currentDir)
        {
            Console.WriteLine("Checking for running instances of TcNo Account Switcher");
            foreach (var exe in Directory.GetFiles(currentDir, "*.exe", SearchOption.AllDirectories))
            {
                if (exe.Contains(Path.GetFileNameWithoutExtension(System.Reflection.Assembly.GetEntryAssembly()?.Location)!)) continue;
                if (Process.GetProcessesByName(Path.GetFileNameWithoutExtension(exe)).Length <= 0) continue;
                Console.WriteLine("Killing: " + exe);
                KillProcess(exe);
            }
        }

        static void GetUpdatesList(ref Dictionary<string, string> updatesAndChanges)
        {
            var client = new WebClient();
            client.Headers.Add("Cache-Control", "no-cache");
            var vUri = new Uri("https://tcno.co/Projects/AccSwitcher/api_v4.php");
            var versions = client.DownloadString(vUri);
            var jUpdates = JObject.Parse(versions)["updates"];
            foreach (JProperty jUpdate in jUpdates)
            {
                if (jUpdate.Name == CurrentVersion) break; // Get up toi the current version
                updatesAndChanges.Add(jUpdate.Name, jUpdate.Value.ToString());
            }
        }





        static void CreateUpdate()
        {
            // CREATE UPDATE
            const string oldFolder = "OldVersion";
            const string newFolder = "NewVersion";
            const string outputFolder = "UpdateOutput";
            var deleteFileList = Path.Join(outputFolder, "filesToDelete.txt");
            RecursiveDelete(new DirectoryInfo(outputFolder), false);
            if (File.Exists(deleteFileList)) File.Delete(deleteFileList);

            List<string> filesToDelete = new(); // Simply ignore these later
            CreateFolderPatches(oldFolder, newFolder, outputFolder, filesToDelete);
            File.WriteAllLines(deleteFileList, filesToDelete);
            Console.WriteLine("Please 7z the update folder.");
            Console.WriteLine("Please 7z the update folder.");
            Console.WriteLine("Please 7z the update folder.");
        }





        static void ApplyUpdate(string updateFilePath)
        {
            var currentDir = Directory.GetCurrentDirectory();
            // Patches, Compression and the rest to be done manually by me.
            // update.7z or version.7z, something along those lines is downloaded
            // Decompressed to a test area
            Directory.CreateDirectory("temp_update");
            using (var archiveFile = new ArchiveFile(updateFilePath))
            {
                archiveFile.Extract("temp_update", true); // extract all
            }
            ApplyPatches(currentDir, "temp_update");

            // Remove files that need to be removed:
            List<string> filesToDelete = File.ReadAllLines(Path.Join("temp_update", "filesToDelete.txt")).ToList();

            MoveFilesRecursive(Path.Join("temp_update", "new"), currentDir);
            MoveFilesRecursive(Path.Join("temp_update", "patched"), currentDir);

            foreach (var f in filesToDelete)
            {
                File.Delete(f);
            }
        }
        


        static void MoveFilesRecursive(string inputFolder, string outputFolder)
        {
            //Now Create all of the directories
            foreach (var dirPath in Directory.GetDirectories(inputFolder, "*", SearchOption.AllDirectories))
                Directory.CreateDirectory(dirPath.Replace(inputFolder, outputFolder));

            //Copy all the files & Replaces any files with the same name
            foreach (var newPath in Directory.GetFiles(inputFolder, "*.*", SearchOption.AllDirectories))
                File.Move(newPath, newPath.Replace(inputFolder, outputFolder), true);
        }


        static void CopyFilesRecursive(string inputFolder, string outputFolder)
        {
            //Now Create all of the directories
            foreach (var dirPath in Directory.GetDirectories(inputFolder, "*", SearchOption.AllDirectories))
                Directory.CreateDirectory(dirPath.Replace(inputFolder, outputFolder));

            //Copy all the files & Replaces any files with the same name
            foreach (var newPath in Directory.GetFiles(inputFolder, "*.*", SearchOption.AllDirectories))
                File.Copy(newPath, newPath.Replace(inputFolder, outputFolder), true);
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
            process.OutputDataReceived += (s, e) => outputText += e.Data + "\n";
            process.Start();
            process.BeginOutputReadLine();
            process.WaitForExit();

            Console.WriteLine($"Tried to close {procName}. Unexpected output from cmd:\r\n{outputText}");
        }
        /// <summary>
        /// Recursively delete files in folders (Choose to keep or delete folders too)
        /// </summary>
        /// <param name="baseDir">Folder to start working inwards from (as DirectoryInfo)</param>
        /// <param name="keepFolders">Set to False to delete folders as well as files</param>
        /// <param name="jsDest">Place to send responses (if any)</param>
        public static void RecursiveDelete(DirectoryInfo baseDir, bool keepFolders, string jsDest = "")
        {
            if (!baseDir.Exists)
                return;

            foreach (var dir in baseDir.EnumerateDirectories())
            {
                RecursiveDelete(dir, keepFolders, jsDest);
            }
            var files = baseDir.GetFiles();
            foreach (var file in files)
            {
                DeleteFile(fileInfo: file, jsDest: jsDest);
            }

            if (keepFolders) return;
            baseDir.Delete();
        }
        /// <summary>
        /// Deletes a single file
        /// </summary>
        /// <param name="file">(Optional) File string to delete</param>
        /// <param name="fileInfo">(Optional) FileInfo of file to delete</param>
        /// <param name="jsDest">Place to send responses (if any)</param>
        public static void DeleteFile(string file = "", FileInfo fileInfo = null, string jsDest = "")
        {
            var f = fileInfo ?? new FileInfo(file);

            try
            {
                if (!f.Exists) Console.WriteLine("err");
                else
                {
                    f.IsReadOnly = false;
                    f.Delete();
                }
            }
            catch (Exception e)
            {

            }
        }


        static void ApplyPatches(string oldFolder, string outputFolder) {
            DirSearch(Path.Join(outputFolder, "patches"), ref patchList);
            foreach (var p in patchList)
            {
                var relativePath = p.Remove(0, p.Split("\\")[0].Length + 1);
                var patchFile = Path.Join(outputFolder, "patches", relativePath);
                var patchedFile = Path.Join(outputFolder, "patched", relativePath);
                Directory.CreateDirectory(Path.GetDirectoryName(patchedFile)!);
                DoDecode(Path.Join(oldFolder, relativePath), patchFile, patchedFile);
            }
        }

        static void CreateFolderPatches(string oldFolder, string newFolder, string outputFolder, List<string> filesToDelete)
        {
            Directory.CreateDirectory(outputFolder);

            DirSearchWithHash(oldFolder, ref oldDict);
            DirSearchWithHash(newFolder, ref newDict);

            // -----------------------------------
            // SAVE DICT OF NEW FILES & HASHES
            // -----------------------------------
            File.WriteAllText(Path.Join(outputFolder,"hashes.json"), JsonConvert.SerializeObject(newDict, Formatting.Indented));


            List<string> differentFiles = new();
            // Files left in newDict are completely new files.

            // COMPARE THE 2 FOLDERS
            foreach (var kvp in oldDict) // Foreach entry in old dictionary:
            {
                if (newDict.TryGetValue(kvp.Key, out string sVal)) // If new dictionary has it, get it's value
                {
                    if (kvp.Value != sVal) // Compare the values. If they don't match ==> FILES ARE DIFFERENT
                    {
                        differentFiles.Add(kvp.Key);
                    }
                    oldDict.Remove(kvp.Key); // Remove from both old
                    newDict.Remove(kvp.Key); // and new dictionaries
                }
                else // New dictionary does NOT have this file
                {
                    filesToDelete.Add(kvp.Key); // Add to list of files to delete
                    oldDict.Remove(kvp.Key); // Remove from old dictionary. They have been added to delete list.
                }
            }

            // -----------------------------------
            // HANDLE NEW FILES
            // -----------------------------------
            // Copy new files into output\\new
            string outputNewFolder = Path.Join(outputFolder, "new");
            Directory.CreateDirectory(outputNewFolder);
            foreach (var newFile in newDict.Keys)
            {
                var newFileInput = Path.Join(newFolder, newFile);
                var newFileOutput = Path.Join(outputNewFolder, newFile);

                Directory.CreateDirectory(Path.GetDirectoryName(newFileOutput)!);
                File.Copy(newFileInput, newFileOutput, true);
                Console.WriteLine("Copied new file to output: " + newFileOutput);
            }


            // -----------------------------------
            // HANDLE DIFFERENT FILES
            // -----------------------------------
            foreach (var differentFile in differentFiles)
            {
                var oldFileInput = Path.Join(oldFolder, differentFile);
                var newFileInput = Path.Join(newFolder, differentFile);
                var patchFileOutput = Path.Join(outputFolder, "patches", differentFile);
                Directory.CreateDirectory(Path.GetDirectoryName(patchFileOutput)!);
                DoEncode(oldFileInput, newFileInput, patchFileOutput);
                Console.WriteLine("Created patch: " + patchFileOutput);
            }
        }






        static void DirSearch(string sDir, ref List<string> list)
        {
            try
            {
                // Foreach file in directory
                foreach (var f in Directory.GetFiles(sDir))
                {
                    Console.WriteLine(f + "|" + GetFileMD5(f));
                    list.Add(f.Remove(0, f.Split("\\")[0].Length + 1));
                }

                // Foreach directory in file
                foreach (var d in Directory.GetDirectories(sDir))
                {
                    DirSearch(d, ref list);
                }
            }
            catch (System.Exception excpt)
            {
                Console.WriteLine(excpt.Message);
            }
        }



        static string GetFileMD5(string filePath)
        {
            using var md5 = MD5.Create();
            using var stream = File.OpenRead(filePath);
            return stream.Length != 0 ? BitConverter.ToString(md5.ComputeHash(stream)).Replace("-", "").ToLowerInvariant() : "0";
        }

        static void DirSearchWithHash(string sDir, ref Dictionary<string, string> dict)
        {
            try
            {
                // Foreach file in directory
                foreach (var f in Directory.GetFiles(sDir))
                {
                    Console.WriteLine(f + "|");
                    dict.Add(f.Remove(0, f.Split("\\")[0].Length + 1), GetFileMD5(f));
                }

                // Foreach directory in file
                foreach (var d in Directory.GetDirectories(sDir))
                {
                    DirSearchWithHash(d, ref dict);
                }
            }
            catch (System.Exception excpt)
            {
                Console.WriteLine(excpt.Message);
            }
        }


        static void DoEncode(string oldFile, string newFile, string patchFileOutput)
        {
            using FileStream output = new FileStream(patchFileOutput, FileMode.Create, FileAccess.Write);
            using FileStream dict = new FileStream(oldFile, FileMode.Open, FileAccess.Read);
            using FileStream target = new FileStream(newFile, FileMode.Open, FileAccess.Read);

            VcEncoder coder = new VcEncoder(dict, target, output);
            VCDiffResult result = coder.Encode(interleaved: true, checksumFormat: ChecksumFormat.SDCH); //encodes with no checksum and not interleaved
            if (result != VCDiffResult.SUCCESS)
            {
                //error was not able to encode properly
                Console.WriteLine("oops :(");
            }
        }
        static void DoDecode(string oldFile, string patchFile, string outputNewFile)
        {
            using FileStream dict = new FileStream(oldFile, FileMode.Open, FileAccess.Read);
            using FileStream target = new FileStream(patchFile, FileMode.Open, FileAccess.Read);
            using FileStream output = new FileStream(outputNewFile, FileMode.Create, FileAccess.Write);
            VcDecoder decoder = new VcDecoder(dict, target, output);

            // The header of the delta file must be available before the first call to decoder.Decode().
            long bytesWritten = 0;
            VCDiffResult result = decoder.Decode(out bytesWritten);

            if (result != VCDiffResult.SUCCESS)
            {
                //error decoding
                Console.WriteLine(result + " - " + bytesWritten);
            }

            // if success bytesWritten will contain the number of bytes that were decoded
        }
    }
}

using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Security.Cryptography;
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


        static void Main(string[] args)
        {
            // 1. Move all files except self into a working folder.
            // 2. Download update.7z
            // 3. Extract
            // 4. Delete files that need to be removed.
            // 5. Run patches from working folder and patches
            // 6. Copy in new files
            // 7. Move out of working into base
            // 8. Delete working temp folder, and update
            // --> Update the updater through the main program with a simple hash check there.
            // ----> Just re-download the whole thing, or make a copy of it to update itself with,
            // ----> then get the main program to replace the updater (Most likely).


            var oldFolder = "releasetest";
            var outputFolder = "output";
            // Patches, Compression and the rest to be done manually by me.
            // update.7z or version.7z, something along those lines is downloaded
            // Decompressed to a test area
            Directory.CreateDirectory("temp_update");
            using (ArchiveFile archiveFile = new ArchiveFile(Path.Combine("output", "update.7z")))
            {
                archiveFile.Extract("temp_update", true); // extract all
            }
            ApplyPatches(oldFolder, outputFolder);

            // Remove files that need to be removed:
            List<string> filesToDelete = File.ReadAllLines("filesToDelete.txt").ToList();

            var patchedFolder = "ALLPATCHED";
            CopyFilesRecursive(oldFolder, patchedFolder);
            CopyFilesRecursive(Path.Join(outputFolder, "patched"), patchedFolder);

            foreach (var f in filesToDelete)
            {
                File.Delete(Path.Join(patchedFolder, f));
            }











            //// Main test of the program v
            //Console.WriteLine("Hello World!");
            //var oldFolder = "net5.0-windows-Release";
            //var newFolder = "net5.0-windows-Debug";
            //var outputFolder = "output";
            //List<string> filesToDelete = new(); // Simply ignore these later
            //CreateFolderPatches(oldFolder, newFolder, outputFolder, filesToDelete);
            //ApplyPatches(oldFolder, outputFolder);
            //File.WriteAllLines("filesToDelete.txt",filesToDelete);


            //var patchedFolder = "ALLPATCHED";
            //CopyFilesRecursive(oldFolder, patchedFolder);
            //CopyFilesRecursive(Path.Join(outputFolder, "patched"), patchedFolder);

            //foreach (var f in filesToDelete)
            //{
            //    File.Delete(Path.Join(patchedFolder, f));
            //}

            //Console.WriteLine("test");
        }

        static void CopyFilesRecursive(string inputFolder, string outputFolder)
        {
            //Now Create all of the directories
            foreach (string dirPath in Directory.GetDirectories(inputFolder, "*", SearchOption.AllDirectories))
                Directory.CreateDirectory(dirPath.Replace(inputFolder, outputFolder));

            //Copy all the files & Replaces any files with the same name
            foreach (string newPath in Directory.GetFiles(inputFolder, "*.*", SearchOption.AllDirectories))
                File.Copy(newPath, newPath.Replace(inputFolder, outputFolder), true);
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
                Console.WriteLine(result);
                Console.WriteLine(bytesWritten);
            }

            // if success bytesWritten will contain the number of bytes that were decoded
        }
    }
}

using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using System.Xml;
using System.Xml.Linq;

namespace Origin_test
{
    class Program
    {

        static string OriginProgramData;
        static string OriginProgramFiles;
        static string OriginRoaming;
        static bool OriginClearLogs = true;
        static void Main(string[] args)
        {
            // Check to see if folders actually exist, and ask user if they don't.
            OriginProgramData = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.CommonApplicationData), "Origin");
            OriginProgramFiles = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ProgramFilesX86), "Origin");
            OriginRoaming = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "Origin");

            Console.WriteLine(OriginProgramData);
            //Console.WriteLine(OriginProgramFiles);
            Console.WriteLine(OriginRoaming);

            // Get user image from cache
            GetAccountImage("7589");


            // Copy account files out
            //copyout("7589");
            // Setup Origin for a new account
            //newacc();
            // Copy and replace existing account files
            //copyin("tech");

            Console.WriteLine("Press any key to close...");
            Console.ReadLine();
        }
        static void copyout(string name)
        {
            //closeOrigin();
            ClearCacheOrigin();
            CopyCurrentUserOut(name);
            CopyCurrentUserOut(name);
        }
        static void newacc()
        {
            closeOrigin();
            ClearCacheOrigin();
            ClearForLoginOrigin();
            PrepareNewLoginXML();
        }
        static void copyin(string name)
        {
            closeOrigin();
            ClearCacheOrigin();
            ClearForLoginOrigin();
            CopyUserIn(name);
        }

        static void GetAccountImage(string username)
        {
            string OriginStorageRoaming = Path.Combine(username, "Roaming");
            FileInfo[] filesRoaming = new DirectoryInfo(OriginStorageRoaming).GetFiles().ToArray();
            string profileimage = "";
            foreach (FileInfo fileRoaming in filesRoaming)
                try
                {
                    if (fileRoaming.Extension == ".xml" && fileRoaming.Name != "local.xml")
                    {
                        using (XmlReader reader = XmlReader.Create(fileRoaming.FullName))
                        {
                            reader.MoveToContent();
                            reader.ReadStartElement("Settings");
                            reader.Read();
                            while (!reader.EOF)
                            {
                                if (reader.GetAttribute("key") != "UserAvatarCacheURL")
                                    reader.Skip();
                                else
                                {
                                    profileimage = reader.GetAttribute("value");
                                    break;
                                }
                            }
                        }
                    }
                }
                catch { }
            Console.WriteLine("Image URL: " + profileimage);
        }

        static void ClearCacheOrigin()
        {
            // Clear ProgramData cache folders.
            TryDeleteFolder(Path.Combine(OriginProgramData, "AchievementCache"));
            TryDeleteFolder(Path.Combine(OriginProgramData, "CatalogCache"));
            TryDeleteFolder(Path.Combine(OriginProgramData, "CustomBoxartCache"));
            //TryDeleteFolder(Path.Combine(OriginProgramData, "DownloadCache")); - Probably not the best cache to clear
            TryDeleteFolder(Path.Combine(OriginProgramData, "EntitlementCache"));
            //TryDeleteFolder(Path.Combine(OriginProgramData, "LocalContent")); - Probably not the best cache to clear
            TryDeleteFolder(Path.Combine(OriginProgramData, "NonOriginContentCache"));

            // Clear logs
            if (OriginClearLogs)
            {
                TryDeleteFolder(Path.Combine(OriginProgramData, "Logs"));
                TryDeleteFile(OriginProgramFiles + "\\debug.log");
                TryDeleteFolder(Path.Combine(OriginRoaming, "Logs"));
            }

            // Clear Roaming cache folders.
            TryDeleteFolder(Path.Combine(OriginRoaming, "CatalogCache"));
        }
        static void ClearForLoginOrigin()
        {
            // Clean ProgramData
            TryDeleteFolder(Path.Combine(OriginProgramData, "Subscription"));
            FileInfo[] filesProgramData = new DirectoryInfo(OriginProgramData).GetFiles().ToArray();
            foreach (FileInfo fileProgramData in filesProgramData)
                try
                {
                    if (fileProgramData.Extension == ".olc" || fileProgramData.Extension == ".xml")
                    {
                        fileProgramData.Attributes = FileAttributes.Normal;
                        TryDeleteFile(fileProgramData.FullName);
                    }
                }
                catch { }
            // Clean Roaming
            TryDeleteFolder(Path.Combine(OriginRoaming, "ConsolidatedCache"));
            TryDeleteFolder(Path.Combine(OriginRoaming, "Logs"));
            TryDeleteFolder(Path.Combine(OriginRoaming, "NucleusCache"));
            DirectoryInfo diRoaming = new DirectoryInfo(OriginRoaming);
            FileInfo[] filesRoaming = new DirectoryInfo(OriginRoaming).GetFiles().ToArray();
            foreach (FileInfo fileRoaming in filesRoaming)
                try
                {
                    fileRoaming.Attributes = FileAttributes.Normal;
                    TryDeleteFile(fileRoaming.FullName);
                }
                catch { }
        }
        static void PrepareNewLoginXML()
        {
            // Modify program XML so Origin starts properly -- Otherwise screen is white
            string localxml = OriginProgramData + "\\local.xml";
            //if (!File.Exists(localxml)) File.Create(localxml);
            bool passed = false;
            byte attempts = 0;
            while (!passed && attempts <= 4)
            {
                try
                {
                    attempts += 1;
                    File.WriteAllText(localxml, "<?xml version=\"1.0\"?><Settings><Setting key=\"OfflineLoginUrl\" value=\"https://signin.ea.com/p/originX/offline\" type=\"10\"/></Settings>");
                    passed = true;
                }
                catch { Thread.Sleep(1000); }
            }
            if (!passed) { Console.WriteLine("Failed to reset login XML file. Screen might be white."); }
        }
        static void CopyCurrentUserOut(string username)
        {
            //
            // Currently user data will be stored in a folder.
            // In future builds it will be stored in an encrypted, compressed file.
            // Just incase there is some chance that these files could be used to login on another computer
            //

            // Create user storage folder
            if (!Directory.Exists(username))
                Directory.CreateDirectory(username);

            // Subfolders to copy
            string OriginProgramDataSubscription = Path.Combine(OriginProgramData, "Subscription");
            string OriginRoamingConsolidatedCache = Path.Combine(OriginRoaming, "ConsolidatedCache");
            string OriginRoamingLogs = Path.Combine(OriginRoaming, "Logs");
            string OriginRoamingNucleusCache = Path.Combine(OriginRoaming, "NucleusCache");

            // Create user storage subfolders
            string OriginStorageProgramData = Path.Combine(username, "ProgramData");
            string OriginStorageRoaming = Path.Combine(username, "Roaming");
            string OriginStorageProgramDataSubscription = Path.Combine(OriginStorageProgramData, "Subscription");
            string OriginStorageRoamingConsolidatedCache = Path.Combine(OriginStorageRoaming, "ConsolidatedCache");
            string OriginStorageRoamingLogs = Path.Combine(OriginStorageRoaming, "Logs");
            string OriginStorageRoamingNucleusCache = Path.Combine(OriginStorageRoaming, "NucleusCache");
            CreateFolder(OriginStorageProgramData);
            CreateFolder(OriginStorageProgramDataSubscription);
            CreateFolder(OriginStorageRoamingConsolidatedCache);
            CreateFolder(OriginStorageRoamingLogs);
            CreateFolder(OriginStorageRoamingNucleusCache);


            // Move all Subscription files
            CopyFoldersInFolder(OriginProgramDataSubscription, OriginStorageProgramDataSubscription);

            FileInfo[] filesProgramData = new DirectoryInfo(OriginProgramData).GetFiles().ToArray();
            foreach (FileInfo fileProgramData in filesProgramData)
                try
                {
                    if (fileProgramData.Extension == ".olc" || fileProgramData.Extension == ".xml")
                    {
                        fileProgramData.Attributes = FileAttributes.Normal;
                        File.Copy(fileProgramData.FullName, fileProgramData.FullName.Replace(OriginProgramData, OriginStorageProgramData), true);
                    }
                }
                catch { }




            CopyFoldersInFolder(OriginRoamingConsolidatedCache, OriginStorageRoamingConsolidatedCache);
            CopyFoldersInFolder(OriginRoamingLogs, OriginStorageRoamingLogs);
            CopyFoldersInFolder(OriginRoamingNucleusCache, OriginStorageRoamingNucleusCache);

            DirectoryInfo diRoaming = new DirectoryInfo(OriginRoaming);
            FileInfo[] filesRoaming = new DirectoryInfo(OriginRoaming).GetFiles().ToArray();
            foreach (FileInfo fileRoaming in filesRoaming)
                try
                {
                    fileRoaming.Attributes = FileAttributes.Normal;
                    File.Copy(fileRoaming.FullName, fileRoaming.FullName.Replace(OriginRoaming, OriginStorageRoaming), true);
                }
                catch { }
        }
        static void CopyUserIn(string username)
        {
            //
            // Currently user data will be stored in a folder.
            // In future builds it will be stored in an encrypted, compressed file.
            // Just incase there is some chance that these files could be used to login on another computer
            //

            // Create user storage subfolders
            string OriginStorageProgramData = Path.Combine(username, "ProgramData");
            string OriginStorageRoaming = Path.Combine(username, "Roaming");

            CopyFoldersInFolder(OriginStorageProgramData, OriginProgramData);
            CopyFoldersInFolder(OriginStorageRoaming, OriginRoaming);

        }




        static void closeOrigin()
        {
            // This is what Administrator permissions are required for.
            System.Diagnostics.Process process = new System.Diagnostics.Process();
            System.Diagnostics.ProcessStartInfo startInfo = new System.Diagnostics.ProcessStartInfo();
            startInfo.WindowStyle = System.Diagnostics.ProcessWindowStyle.Hidden;
            startInfo.FileName = "cmd.exe";
            startInfo.Arguments = "/C TASKKILL /F /T /IM origin*";
            process.StartInfo = startInfo;
            process.Start();
            process.WaitForExit();
        }
        static void MoveFilesInFolder(string infolder, string outfolder)
        {
            try
            {
                foreach (string newPath in Directory.GetFiles(infolder, "*.*", SearchOption.AllDirectories))
                    File.Move(newPath, newPath.Replace(infolder, outfolder));
            }
            catch { }
        }
        static void MoveFoldersInFolder(string infolder, string outfolder)
        {
            try
            {
                foreach (string newPath in Directory.GetDirectories(infolder))
                    Directory.Move(newPath, newPath.Replace(infolder, outfolder));
            }
            catch { }
        }
        static void CopyFoldersInFolder(string infolder, string outfolder)
        {
            try
            {
                var diSource = new DirectoryInfo(infolder);
                var diTarget = new DirectoryInfo(outfolder);

                CopyAll(diSource, diTarget);
            }
            catch { }
        }
        static void TryDeleteFile(string file)
        {
            try
            {
                System.IO.File.Delete(file);
            }
            catch { }
        }
        static void TryDeleteFolder(string fld)
        {
            try
            {
                System.IO.Directory.Delete(fld, true);
                System.IO.Directory.CreateDirectory(fld);
            }
            catch { }
        }
        static void CreateFolder(string fld)
        {
            if (!Directory.Exists(fld)) Directory.CreateDirectory(fld);
        }



        // https://code.4noobz.net/c-copy-a-folder-its-content-and-the-subfolders/
        public static void CopyAll(DirectoryInfo source, DirectoryInfo target)
        {
            Directory.CreateDirectory(target.FullName);

            // Copy each file into the new directory.
            foreach (FileInfo fi in source.GetFiles())
            {
                fi.CopyTo(Path.Combine(target.FullName, fi.Name), true);
            }

            // Copy each subdirectory using recursion.
            foreach (DirectoryInfo diSourceSubDir in source.GetDirectories())
            {
                DirectoryInfo nextTargetSubDir =
                    target.CreateSubdirectory(diSourceSubDir.Name);
                CopyAll(diSourceSubDir, nextTargetSubDir);
            }
        }

    }
}

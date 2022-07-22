using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;

namespace TcNo_Acc_Switcher_Server.Pages.General
{

    public class PathPicker
    {
        public static List<string> LogicalDrivesList
        {
            get
            {
                try
                {
                    return Directory.GetLogicalDrives().ToList();
                }
                catch (Exception e)
                {
                    Globals.WriteToLog("Could not list Logical Drives.", e);
                    _ = GeneralInvocableFuncs.ShowToast("error", Lang["PathPicker_NoLogicalDrives"], renderTo: "toastarea");
                    return new List<string>();
                }
            }
        }


        public class FolderFileList
        {
            public int Depth { get; set; }
            public string FullPath { get; set; }
            public string FolderName { get; set; }

            public Lazy<List<FolderFileList>> ChildFolders =>
                new (() =>
                {
                    try
                    {
                        var childFolders = Directory.GetDirectories(FullPath).Select(x => new FolderFileList
                        {
                            FullPath = x,
                            FolderName = Path.GetFileName(x)
                        }).ToList();
                        return childFolders;
                    }
                    catch (Exception e)
                    {
                        Globals.WriteToLog($"PathPicker: Failed to crawl path or drive: \"{FullPath}\"", e);
                        if (Depth != 0)
                            _ = GeneralInvocableFuncs.ShowToast("error", Lang["PathPicker_FailedGetFolders", new { path = FolderName }], renderTo: "toastarea");
                        return new List<FolderFileList>();
                    }
                });

            public Lazy<List<string>> Files =>
                new(() =>
                {
                    try
                    {
                        return Directory.GetFiles(FullPath).ToList();
                    }
                    catch (Exception e)
                    {
                        Globals.WriteToLog($"PathPicker: Failed to crawl path or drive: \"{FullPath}\"", e);
                        // This drive will be left out of the list.
                        return new List<string>();
                    }
                });

            public FolderFileList(){}
            public FolderFileList(string fullPath, int depth)
            {
                FullPath = fullPath;
                Depth = depth;
                FolderName = depth == 0 ? fullPath : Path.GetFileName(fullPath);
            }
        }
    }
}

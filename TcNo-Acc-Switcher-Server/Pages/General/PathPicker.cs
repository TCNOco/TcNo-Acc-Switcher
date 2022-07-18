using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using Microsoft.AspNetCore.Components;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;

namespace TcNo_Acc_Switcher_Server.Pages.General
{

    public class PathPicker
    {
        [Inject] private IGeneralFuncs GeneralFuncs { get; }
        [Inject] private ILang Lang { get; }


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
                        // TODO: SEE HOW TO DO THIS!
                        //if (Depth != 0)
                        //    _ = GeneralFuncs.ShowToast("error", Lang["PathPicker_FailedGetFolders", new { path = FolderName }], renderTo: "toastarea");
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

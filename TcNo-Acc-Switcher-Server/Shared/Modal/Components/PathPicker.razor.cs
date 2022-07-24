using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using Microsoft.AspNetCore.Components;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.State;
using TcNo_Acc_Switcher_Server.State.DataTypes;
using TcNo_Acc_Switcher_Server.State.Interfaces;

namespace TcNo_Acc_Switcher_Server.Shared.Modal.Components;

public partial class PathPicker
{
    public class FolderFileList
    {
        [Inject] private Toasts Toasts { get; set; }

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
                        Toasts.ShowToastLang(ToastType.Error, new LangSub("PathPicker_FailedGetFolders", new { path = FolderName }));
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
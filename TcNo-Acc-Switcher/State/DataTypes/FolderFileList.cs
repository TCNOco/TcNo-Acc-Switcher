// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2022 TechNobo (Wesley Pyburn)
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher.State.Interfaces;

namespace TcNo_Acc_Switcher.State.DataTypes;

public class FolderFileList
{
    private readonly IToasts _toasts;

    public FolderFileList(IToasts toasts)
    {
        _toasts = toasts;
    }
    public FolderFileList(IToasts toasts, string fullPath, int depth)
    {
        _toasts = toasts;

        FullPath = fullPath;
        Depth = depth;
        FolderName = depth == 0 ? fullPath : Path.GetFileName(fullPath);
    }

    public int Depth { get; set; }
    public string FullPath { get; set; }
    public string FolderName { get; set; }

    public Lazy<List<FolderFileList>> ChildFolders =>
        new(() =>
        {
            try
            {
                var childFolders = Directory.GetDirectories(FullPath).Select(x => new FolderFileList(_toasts)
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
                    _toasts.ShowToastLang(ToastType.Error, new LangSub("PathPicker_FailedGetFolders", new { path = FolderName }));
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
}
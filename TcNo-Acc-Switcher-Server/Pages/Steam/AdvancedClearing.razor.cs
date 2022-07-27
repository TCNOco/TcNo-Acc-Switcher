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
using System.Collections;
using System.Collections.Generic;
using System.IO;
using System.Runtime.Versioning;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Components;
using Microsoft.JSInterop;
using Microsoft.Win32;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.State.Interfaces;

namespace TcNo_Acc_Switcher_Server.Pages.Steam;

public partial class AdvancedClearing
{
    [Inject] private IJSRuntime JsRuntime { get; set; }
    [Inject] private ISteamSettings SteamSettings { get; set; }
    [Inject] private IAppState AppState { get; set; }
    [Inject] private ISharedFunctions SharedFunctions { get; set; }

    protected override void OnInitialized()
    {
        Globals.DebugWriteLine(@"[Auto:Steam\AdvancedClearing.razor.cs.OnInitializedAsync]");
        AppState.WindowState.WindowTitle = Lang["Title_Steam_Cleaning"];
    }


    private void WriteLine(string text, bool followWithNewline = true)
    {
        Globals.DebugWriteLine($@"[Auto:Steam\AdvancedClearing.razor.cs.WriteLine] Line: {text}");
        Lines.Add(text);
        if (followWithNewline) Lines.Add("");
    }

    // BUTTON: Kill Steam process
    public async Task Steam_Close()
    {
        Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Close]");
        WriteLine(await SharedFunctions.CloseProcesses(JsRuntime, SteamSettings.Processes, SteamSettings.ClosingMethod) ? "Closed Steam." : "ERROR: COULD NOT CLOSE STEAM!");
    }

    // BUTTON: ..\Steam\Logs
    public void Steam_Clear_Logs()
    {
        Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_Logs]");
        ClearFolder(Path.Join(SteamSettings.FolderPath, "logs\\"));
        WriteLine("Cleared logs folder.");
    }

    // BUTTON:..\Steam\*.log
    public void Steam_Clear_Dumps()
    {
        Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_Dumps]");
        ClearFolder(Path.Join(SteamSettings.FolderPath, "dumps\\"));
        WriteLine("Cleared dumps folder.");
    }

    // BUTTON: %Local%\Steam\htmlcache
    public void Steam_Clear_HtmlCache()
    {
        Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_HtmlCache]");
        // HTML Cache - %USERPROFILE%\AppData\Local\Steam\htmlcache
        ClearFolder(Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "Steam\\htmlcache"));
        WriteLine("Cleared HTMLCache.");
    }

    // BUTTON: ..\Steam\*.log
    public void Steam_Clear_UiLogs()
    {
        Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_UiLogs]");
        // Overlay UI logs -
        //   Steam\GameOverlayUI.exe.log
        //   Steam\GameOverlayRenderer.log
        ClearFilesOfType(SteamSettings.FolderPath, "*.log|*.last", SearchOption.TopDirectoryOnly);
        WriteLine("Cleared UI Logs.");
    }

    // BUTTON: ..\Steam\appcache
    public void Steam_Clear_AppCache()
    {
        Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_AppCache]");
        // App Cache - Steam\appcache
        ClearFilesOfType(Path.Join(SteamSettings.FolderPath, "appcache"), "*.*", SearchOption.TopDirectoryOnly);
        WriteLine("Cleared AppCache.");
    }

    // BUTTON: ..\Steam\appcache\httpcache
    public void Steam_Clear_HttpCache()
    {
        Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_HttpCache]");
        ClearFilesOfType(Path.Join(SteamSettings.FolderPath, "appcache\\httpcache"), "*.*", SearchOption.AllDirectories);
        WriteLine("Cleared HTTPCache.");
    }

    // BUTTON: ..\Steam\depotcache
    public void Steam_Clear_DepotCache()
    {
        Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_DepotCache]");
        ClearFilesOfType(Path.Join(SteamSettings.FolderPath, "depotcache"), "*.*", SearchOption.TopDirectoryOnly);
        WriteLine("Cleared DepotCache.");
    }

    // BUTTON: ..\Steam\config\config.vdf
    public void Steam_Clear_Config()
    {
        Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_Config]");
        DeleteFile(Path.Join(SteamSettings.FolderPath, "config\\config.vdf"));
        WriteLine("Cleared config\\config.vdf");
    }

    // BUTTON: ..\Steam\config\loginusers.vdf
    public void Steam_Clear_LoginUsers()
    {
        Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_LoginUsers]");
        DeleteFile(Path.Join(SteamSettings.FolderPath, "config\\loginusers.vdf"));
        WriteLine("Cleared config\\loginusers.vdf");
    }

    // BUTTON: ..\Steam\ssfn*
    public void Steam_Clear_Ssfn()
    {
        Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_Ssfn]");
        var d = new DirectoryInfo(SteamSettings.FolderPath);
        var i = 0;
        foreach (var f in d.GetFiles("ssfn*"))
        {
            DeleteFile(f);
            i++;
        }

        WriteLine(i == 0 ? "No SSFN files found." : "Cleared SSFN files.");
    }

    // BUTTON: HKCU\..\AutoLoginUser
    [SupportedOSPlatform("windows")]
    public void Steam_Clear_AutoLoginUser()
    {
        Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_AutoLoginUser]");
        DeleteRegKey(@"Software\Valve\Steam", "AutoLoginuser");
    }
    // BUTTON: HKCU\..\LastGameNameUsed
    [SupportedOSPlatform("windows")]
    public void Steam_Clear_LastGameNameUsed()
    {
        Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_LastGameNameUsed]");
        DeleteRegKey(@"Software\Valve\Steam", "LastGameNameUsed");
    }

    // BUTTON: HKCU\..\PseudoUUID
    [SupportedOSPlatform("windows")]
    public void Steam_Clear_PseudoUUID()
    {
        Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_PseudoUUID]");
        DeleteRegKey(@"Software\Valve\Steam", "PseudoUUID");
    }
    // BUTTON: HKCU\..\RememberPassword
    [SupportedOSPlatform("windows")]
    public void Steam_Clear_RememberPassword()
    {
        Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_RememberPassword]");
        DeleteRegKey(@"Software\Valve\Steam", "RememberPassword");
    }

    // Overload for below
    public void DeleteFile(string file) => DeleteFile(new FileInfo(file));

    /// <summary>
    /// Deletes a single file
    /// </summary>
    /// <param name="f">(Optional) FileInfo of file to delete</param>
    public void DeleteFile(FileInfo f)
    {
        Globals.DebugWriteLine($@"[AdvancedCleaning\DeleteFile] file={f?.FullName ?? ""}");
        try
        {
            if (f is { Exists: false })
                WriteLine(Lang["FileNotFound", new { file = f.FullName }], false);
            else
            {
                if (f == null) return;
                f.IsReadOnly = false;
                f.Delete();
                WriteLine(Lang["DeletedFile", new { file = f.FullName }], false);
            }
        }
        catch (Exception e)
        {
            WriteLine(f is null ? Lang["CouldntDeleteUndefined"] : Lang["CouldntDeleteX", new {x = f.FullName}]);
            WriteLine(e.ToString());
        }
    }

    /// <summary>
    /// Shorter RecursiveDelete (Sets keep folders to true)
    /// </summary>
    public void ClearFolder(string folder)
    {
        Globals.DebugWriteLine($@"[AdvancedCleaning\ClearFolder] folder={folder}");
        RecursiveDelete(new DirectoryInfo(folder), true);
    }

    /// <summary>
    /// Recursively delete files in folders (Choose to keep or delete folders too)
    /// </summary>
    /// <param name="baseDir">Folder to start working inwards from (as DirectoryInfo)</param>
    /// <param name="keepFolders">Set to False to delete folders as well as files</param>1
    public void RecursiveDelete(DirectoryInfo baseDir, bool keepFolders)
    {
        Globals.DebugWriteLine($@"[AdvancedCleaning\RecursiveDelete] baseDir={baseDir.Name}");
        if (!baseDir.Exists)
            return;

        foreach (var dir in baseDir.EnumerateDirectories())
        {
            RecursiveDelete(dir, keepFolders);
        }
        var files = baseDir.GetFiles();
        foreach (var file in files)
        {
            DeleteFile(file);
        }

        if (keepFolders) return;
        baseDir.Delete();

        WriteLine(Lang["DeletingFolder"] + baseDir.FullName);
    }

    /// <summary>
    /// Deletes registry keys
    /// </summary>
    /// <param name="subKey">Subkey to delete</param>
    /// <param name="val">Value to delete</param>
    [SupportedOSPlatform("windows")]
    public void DeleteRegKey(string subKey, string val)
    {
        Globals.DebugWriteLine($@"[AdvancedCleaning\DeleteRegKey] subKey={subKey}, val={val}");
        using var key = Registry.CurrentUser.OpenSubKey(subKey, true);
        if (key == null)
            WriteLine(Lang["Reg_DoesntExist", new { subKey }]);
        else if (key.GetValue(val) == null)
            WriteLine(Lang["Reg_DoesntContain", new { subKey, val }]);
        else
        {
            WriteLine(Lang["Reg_Removing", new { subKey, val }]);
            key.DeleteValue(val);
        }
    }

    /// <summary>
    /// Returns a string array of files in a folder, based on a SearchOption.
    /// </summary>
    /// <param name="sourceFolder">Folder to search for files in</param>
    /// <param name="filter">Filter for files in folder</param>
    /// <param name="searchOption">Option: ie: Sub-folders, TopLevel only etc.</param>
    private static IEnumerable<string> GetFiles(string sourceFolder, string filter, SearchOption searchOption)
    {
        Globals.DebugWriteLine($@"[AdvancedCleaning\GetFiles] sourceFolder={sourceFolder}, filter={filter}");
        var alFiles = new ArrayList();
        var multipleFilters = filter.Split('|');
        foreach (var fileFilter in multipleFilters)
            alFiles.AddRange(Directory.GetFiles(sourceFolder, fileFilter, searchOption));

        return (string[])alFiles.ToArray(typeof(string));
    }

    /// <summary>
    /// Deletes all files of a specific type in a directory.
    /// </summary>
    /// <param name="folder">Folder to search for files in</param>
    /// <param name="extensions">Extensions of files to delete</param>
    /// <param name="so">SearchOption of where to look for files</param>
    public void ClearFilesOfType(string folder, string extensions, SearchOption so)
    {
        Globals.DebugWriteLine($@"[AdvancedCleaning\ClearFilesOfType] folder={folder}, extensions={extensions}");
        if (!Directory.Exists(folder))
        {
            WriteLine(Lang["DirectoryNotFound", new { folder }]);
            return;
        }
        foreach (var file in GetFiles(folder, extensions, so))
        {
            WriteLine(Lang["DeletingFile", new { file }], false);
            try
            {
                Globals.DeleteFile(file);
            }
            catch (Exception ex)
            {
                WriteLine(Lang["ErrorDetails", new { ex = Globals.MessageFromHResult(ex.HResult) }]);
            }
        }

        WriteLine("");
    }
}
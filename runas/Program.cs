// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2025 TroubleChute (Wesley Pyburn)
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

// See https://aka.ms/new-console-template for more information


// What is the purpose of "runas"?
// Put simply: After the addition of the 'wrapper' to auto-install required .NET runtimes,
// using Process.Start() the process is started as a child, and ends when TcNo Account Switcher closes.
// This process starts another process (detatched) and can close itself normally.
// Hence, hopefully, avoiding the above issue.

// Required arguments:
// runas.exe <process path> <1/0 for admin> <optional arguments>

using System.ComponentModel;
using System.Diagnostics;

if (args.Length < 2)
{
    Console.WriteLine("Please do not launch this process directly.");
    Environment.Exit(1);
}

var argString = "";
if (args.Length > 2)
{
    for (var i = 2; i < args.Length; i++)
    {
        argString += args[i] + " ";
    }
}

try
{
    var filePath = args[0].Trim().Trim('"');
    if (string.IsNullOrWhiteSpace(filePath))
        Environment.Exit(1);

    string targetDir;
    
    if (Directory.Exists(filePath))
        targetDir = filePath;
    else if (filePath.EndsWith("\\") || filePath.EndsWith("/"))
    {
        targetDir = filePath.TrimEnd('\\', '/');

        if (!Directory.Exists(targetDir))
            targetDir = Path.GetDirectoryName(targetDir) ?? string.Empty;
    }
    else if (File.Exists(filePath))
        targetDir = Path.GetDirectoryName(filePath) ?? string.Empty;
    else
        targetDir = Path.GetDirectoryName(filePath) ?? string.Empty;
    
    if (string.IsNullOrWhiteSpace(targetDir) || !Directory.Exists(targetDir))
    {
        var exeDir = Path.GetDirectoryName(System.Reflection.Assembly.GetExecutingAssembly().Location);
        if (!string.IsNullOrWhiteSpace(exeDir) && Directory.Exists(exeDir))
            targetDir = exeDir;
        else
            targetDir = Directory.GetCurrentDirectory();
    }

    if (string.IsNullOrWhiteSpace(targetDir))
        targetDir = Environment.CurrentDirectory;

    Directory.SetCurrentDirectory(targetDir);

    Process.Start(new ProcessStartInfo
    {
        FileName = filePath,
        UseShellExecute = true,
        RedirectStandardError = false,
        RedirectStandardOutput = false,
        Arguments = argString,
        Verb = args[1] == "1" ? "runas" : "",
        WorkingDirectory = targetDir
    });
}
catch (Win32Exception e)
{
    if (e.HResult != -2147467259) // Not because it was cancelled by user
        throw;
}
catch
{
    // Silently handle other exceptions to prevent fail-fast
    Environment.Exit(1);
}
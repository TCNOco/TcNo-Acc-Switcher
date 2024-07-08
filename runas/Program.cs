// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2024 TroubleChute (Wesley Pyburn)
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
    Directory.SetCurrentDirectory(Path.GetDirectoryName(args[0]) ?? Directory.GetCurrentDirectory());

    Process.Start(new ProcessStartInfo
    {
        FileName = args[0],
        UseShellExecute = true,
        RedirectStandardError = false,
        RedirectStandardOutput = false,
        Arguments = argString,
        Verb = args[1] == "1" ? "runas" : "",
        WorkingDirectory = Path.GetDirectoryName(args[0]) ?? Directory.GetCurrentDirectory()
    });
}
catch (Win32Exception e)
{
    if (e.HResult != -2147467259) // Not because it was cancelled by user
        throw;
}
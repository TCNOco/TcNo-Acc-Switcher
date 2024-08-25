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

// Goal of this project:
// To allow users to start the program regardless of .NET version.
// If .NET is missing > Open installer, then after install relaunch.

#pragma once
#include <string>

inline std::string required_min_webview = "98.0",
                   required_min_desktop_runtime = "8.0.8",
                   required_min_aspcore = "8.0.8",
                   required_min_vc = "14.30.30704";

const std::string w_runtime_name = "Microsoft WebView2 Runtime",
	w_runtime = "https://go.microsoft.com/fwlink/p/?LinkId=2124703",
	w_exe = "MicrosoftEdgeWebview2Setup.exe",

	d_runtime_name = "Microsoft .NET 8.0.8 Desktop Runtime",
	d_runtime = "https://dotnetcli.azureedge.net/dotnet/WindowsDesktop/8.0.8/windowsdesktop-runtime-8.0.8-win-x64.exe",
	d_exe = "windowsdesktop-runtime-8.0.8-win-x64.exe",

	a_runtime_name = "Microsoft ASP.NET Core 8.0.8 Runtime",
	a_runtime = "https://dotnetcli.azureedge.net/dotnet/aspnetcore/Runtime/8.0.8/aspnetcore-runtime-8.0.8-win-x64.exe",
	a_exe = "aspnetcore-runtime-8.0.8-win-x64.exe",

	c_runtime = "https://aka.ms/vs/17/release/vc_redist.x64.exe",
	c_runtime_name = "C++ Redistributable 2015-2022",
	c_exe = "VC_redist.x64.exe";
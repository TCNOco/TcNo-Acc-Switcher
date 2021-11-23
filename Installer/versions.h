#pragma once
#include <string>

inline std::string required_min_webview = "91.0",
                   required_min_desktop_runtime = "6.0.0",
                   required_min_aspcore = "6.0.0",
                   required_min_vc = "14.30.30704";

const std::string w_runtime_name = "Microsoft WebView2 Runtime",
	w_runtime = "https://go.microsoft.com/fwlink/p/?LinkId=2124703",
	w_exe = "MicrosoftEdgeWebview2Setup.exe",

	d_runtime_name = "Microsoft .NET 6 Desktop Runtime",
	d_runtime = "https://dotnetcli.azureedge.net/dotnet/WindowsDesktop/6.0.0/windowsdesktop-runtime-6.0.0-win-x64.exe",
	d_exe = "windowsdesktop-runtime-6.0.0-win-x64.exe",

	a_runtime_name = "Microsoft ASP.NET Core 6.0 Runtime",
	a_runtime = "https://dotnetcli.azureedge.net/dotnet/aspnetcore/Runtime/6.0.0/aspnetcore-runtime-6.0.0-win-x64.exe",
	a_exe = "aspnetcore-runtime-6.0.0-win-x64.exe",

	c_runtime = "https://aka.ms/vs/17/release/vc_redist.x64.exe",
	c_runtime_name = "C++ Redistributable 2015-2022",
	c_exe = "VC_redist.x64.exe";
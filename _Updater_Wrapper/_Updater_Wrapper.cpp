// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2021 TechNobo (Wesley Pyburn)
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
// To allow users to start the updater regardless of .NET version.
// This program checks .NET version, and downloads/installs missing ones automatically.
// Required by switch from .NET 5 to 6. Existing users on .NET 5 can not run the .NET 6 installer.
// If the installer is kept in .NET 5, new .NET 6 users will need to install .NET 5 just for the installer.
// Compromise? A wrapper that checks .NET version, then parses all arguments into the main program.
// .NET programs have a .dll where things are done, and a .exe for Windows users, that seems to just start the .dll (and a basic check for .NET install).
// This needs to be a bit more here as users need the Desktop runtime as well. Also makes life a little simpler.
//
// If .NET 6 not installed: Download & install automatically


#include "../Installer/runtime_check.hpp"
#include <windows.h>
#include <filesystem>

#include <limits.h>
#include <stdlib.h>
int main()
{
	SetConsoleTitle(_T("TcNo Account Switcher - Updater Wrapper"));
	std::cout << "Verifying .NET versions before attempting to launch Updater." << std::endl;
	bool min_webview_met = false,
		min_desktop_runtime_met = false,
		min_aspcore_met = false;
	find_installed_net_runtimes(false, min_webview_met, min_desktop_runtime_met, min_aspcore_met, false);

	if (!min_webview_met || !min_desktop_runtime_met || !min_aspcore_met)
	{
		// Launch installer to get these!
		const auto args = TEXT("net updater");
		start_program(L"../_First_Run_Installer.exe", args);
	}
	else
	{
		std::string p = '"' + getOperatingPath();
		if (p.back() != '\\') p += '\\';
		p += "TcNo-Acc-Switcher-Updater.dll\"";

		p = "--info";

		std::string dotnet = dotnet_path(); // This is 1024 characters long for some reason...
		// WHY
		// ON
		// EARTH
		// DOES THIS NOT USE THE ARGUMENT?!?!?!?!?
		// TODO: when not 3:30AM: Fix
		start_program(std::wstring(dotnet.begin(), dotnet.end()).c_str(), std::wstring(p.begin(), p.end()).c_str());
		std::cout << "RAN: " << dotnet << " " << p << std::endl;

		system("pause");
	}

	exit(1);
}

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
// To allow users to start the program regardless of .NET version.
// If .NET is missing > Open installer, then after install relaunch.


#include "../Installer/runtime_check.hpp"
#include <windows.h>n
#include <filesystem>

#include <limits.h>
#include <stdlib.h>
int main(int argc, char* argv[])
{
	const std::string self = getSelfName(), window_title = "Runtime Verifier - " + self;
	const size_t last_hyphen = self.find_last_of('-');
	const std::string easy_name = self.substr(last_hyphen + 1, self.find_last_of('.') - last_hyphen - 1); // Will be "Switcher", "Updater", etc.
	SetConsoleTitle(s2ws(window_title).c_str());

	std::cout << "Verifying .NET versions before attempting to launch " << easy_name << std::endl;
	bool min_webview_met = false,
		min_desktop_runtime_met = false,
		min_aspcore_met = false;
	find_installed_net_runtimes(false, min_webview_met, min_desktop_runtime_met, min_aspcore_met, true);

	if (!min_webview_met || !min_desktop_runtime_met || !min_aspcore_met)
	{
		// Launch installer to get these!
		std::string back_path = getOperatingPath();
		std::cout << "LAST: " << back_path.find("updater\\") << std::endl;
		if (back_path.find("updater\\") <= back_path.length()) back_path += +"..\\"; // Go back a folder if in updater folder.
		std::string s_args("net " + self);
		exec_program(std::wstring(back_path.begin(), back_path.end()), L"_First_Run_Installer.exe", std::wstring(s_args.begin(), s_args.end()));
	}
	else
	{
		std::string p = '"' + getOperatingPath();
		if (p.back() != '\\') p += '\\';
		p += self + ".dll\"";

		std::string dotnet = dotnet_path();
		std::cout << "Running: dotnet " << self << ".dll " << std::endl;

		std::vector<char> buffer_temp;

		for (int i = 1; i < argc; ++i)
			p = p + " " + argv[i];

		std::cout << dotnet << "dotnet.exe" << std::endl << p;

		// Show window if running Server.
		bool show_window = false;
		if (self.find("Server") != std::string::npos || self.find("server") != std::string::npos) show_window = true;

		exec_program(std::wstring(dotnet.begin(), dotnet.end()), L"dotnet.exe", std::wstring(p.begin(), p.end()), show_window);
	}

	exit(1);
}

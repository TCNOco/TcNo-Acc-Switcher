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

// Goal of this project:
// To allow users to start the program regardless of .NET version.
// If .NET is missing > Open installer, then after install relaunch.

#pragma comment(linker, "/SUBSYSTEM:windows /ENTRY:mainCRTStartup")

#include "../Installer/runtime_check.hpp"
#include <Windows.h>
#include <filesystem>

#include <cstdlib>

int main(int argc, char* argv[])
{
	const std::string self = get_self_name(), window_title = "Runtime Verifier - " + self;
	const size_t last_hyphen = self.find_last_of('-');
	// Will be "Switcher", "Updater", etc.
	const std::string easy_name = self.substr(last_hyphen + 1, self.find_last_of('.') - last_hyphen - 1);
	SetConsoleTitle(s2_ws(window_title).c_str());

	std::cout << "Verifying .NET versions before attempting to launch " << easy_name << std::endl;
	bool min_webview_met = false,
	     min_desktop_runtime_met = false,
	     min_aspcore_met = false;
	find_installed_net_runtimes(false, min_webview_met, min_desktop_runtime_met, min_aspcore_met, true);

	if (!min_webview_met || !min_desktop_runtime_met || !min_aspcore_met)
	{
		// Launch installer to get these!
		std::string self_path = get_operating_path();
		std::string s_args("net " + self);
		exec_process(s2_ws(self_path), s2_ws("_First_Run_Installer.exe"), s2_ws(s_args));
	}
	else
	{
		std::string operating_path = get_operating_path();
		const std::string exe_name = self + "_main.exe";
		//const std::string exe_name = "TcNo-Acc-Switcher_main.exe";


		std::string full_path = operating_path;
		std::wstring wargs;
		if (full_path.back() != '\\') full_path += '\\';
		full_path += exe_name;

		std::cout << "Running: " << exe_name << std::endl;

		// Build a properly quoted wide args string to preserve spaces in individual arguments
		for (int i = 1; i < argc; ++i)
		{
			std::wstring w = s2_ws(std::string(argv[i]));
			bool needs_quote = w.find(L' ') != std::wstring::npos || w.find(L'\t') != std::wstring::npos;
			if (i > 1) wargs += L' ';
			if (needs_quote)
			{
				wargs += L'"';
				wargs += w;
				wargs += L'"';
			}
			else
			{
				wargs += w;
			}
		}

		std::cout << "FULL PATH: " << full_path << std::endl;

		exec_process(s2_ws(operating_path), s2_ws(exe_name), wargs);
	}

	exit(1);
}

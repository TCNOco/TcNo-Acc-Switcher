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

// Just note that this requires curl to be installed. I used "vcpkg install curl[openssl]:x64-windows-static" in Microsoft's vcpkg to accomplish that.

#include "tcno.hpp"

bool args_contain(const char* needle, int argc, char* argv[])
{
	for (int i = 0; i < argc; ++i)
		if (strcmp(argv[i], needle) == 0) return true;
	return false;
}

void launch_tcno_program(const char* arg)
{
	const std::string program(arg);
	std::string operating_path = get_operating_path();
	const std::string exe_name = program + "_main.exe";

	std::string full_path = operating_path;
	if (full_path.back() != '\\') full_path += '\\';
	full_path += exe_name;

	std::cout << "Running: " << exe_name << std::endl;

	std::vector<char> buffer_temp;

	std::cout << "FULL PATH: " << full_path << std::endl;

	exec_process(std::wstring(operating_path.begin(), operating_path.end()),
	             std::wstring(exe_name.begin(), exe_name.end()), L"");
}

int main(int argc, char* argv[])
{
	// Goal of this application:
	// - Check for the existence of required runtimes, and install them if missing. [First run ever on a computer]
	// --- [Unlikely] Maybe in the future: Verify application folders, and handle zipping and sending error logs? Possibly just the first or none of these.
	const string operating_path = get_operating_path();
	SetConsoleTitle(_T("TcNo Account Switcher - Runtime installer"));
	cout << "Welcome to the TcNo Account Switcher - Runtime installer" << endl <<
		"------------------------------------------------------------------------" << endl << endl;

	// Argument was supplied...
	// Check if need to install anything.
	// Otherwise, launch that, assuming it is a program.
	if (argc > 1)
	{
		if (args_contain("vc", argc, argv))
		{
			verify_vc();
			exit(1);
		}

		if (args_contain("net", argc, argv))
		{
			min_vc_met = true; // Skip over this. Not needed unless CEF enabled --> Checked elsewhere.
			verify_net();
			if (argc > 2)
				launch_tcno_program(argv[argc - 1]);
			exit(1);
		}

		launch_tcno_program(argv[argc - 1]);
	}

	cout << "Currently installed runtimes:" << endl;

	/* Find installed runtimes */
	find_installed_net_runtimes(false, min_webview_met, min_desktop_runtime_met, min_aspcore_met);
	find_installed_c_runtimes(min_vc_met);

	cout << "------------------------------------------------------------------------" << endl << endl;

	download_install_missing_runtimes();

	// Launch main program:
	string exe_name = "TcNo-Acc-Switcher.exe";
	//STARTUPINFO si = { sizeof(STARTUPINFO) };
	//PROCESS_INFORMATION pi;
	//CreateProcess(s2_ws(main_path).c_str(), nullptr, nullptr,
	//    nullptr, 0, 0, nullptr, nullptr, &si, &pi);


	exec_process(std::wstring(operating_path.begin(), operating_path.end()),
	             std::wstring(exe_name.begin(), exe_name.end()), L"");
}

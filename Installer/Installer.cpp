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

// Just note that this requires curl to be installed. I used "vcpkg install curl[openssl]:x64-windows-static" in Microsoft's vcpkg to accomplish that.

#include "tcno.hpp"
#include <iostream>
#include <filesystem>
#include <cstdlib>
#include <chrono>
#include <thread>
#include <fstream>
namespace fs = std::filesystem;

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

std::ofstream* logFile = nullptr;
void log(const std::string& message) {
	if (logFile && logFile->is_open()) {
		(*logFile) << message << std::endl;
	}
	std::cout << message << std::endl;
}

bool move_file(const fs::path& src, const fs::path& dest) {
	for (int attempt = 0; attempt < 3; ++attempt) {
		try {
			if (fs::exists(dest)) {
				fs::remove(dest);
			}
			fs::copy_file(src, dest, fs::copy_options::overwrite_existing);
			fs::remove(src);
			return true;  // Success
		}
		catch (const fs::filesystem_error& e) {
			log("Attempt " + std::to_string(attempt + 1) + " to move " + src.string() + " failed: " + e.what());
			std::this_thread::sleep_for(std::chrono::milliseconds(100));
		}
	}
	return false;  // Failed after retries
}

void move_recursive(const fs::path& from, const fs::path& to) {
	try {
		if (!fs::exists(to)) {
			fs::create_directories(to);
		}

		for (const auto& entry : fs::recursive_directory_iterator(from)) {
			const auto& path = entry.path();
			auto relative_path = fs::relative(path, from);
			auto dest_path = to / relative_path;

			if (fs::is_directory(path)) {
				if (!fs::exists(dest_path)) {
					fs::create_directory(dest_path);
				}
			}
			else if (fs::is_regular_file(path)) {
				if (!move_file(path, dest_path)) {
					log("Failed to move " + path.string() + " to " + dest_path.string());
					std::exit(1);
				}
			}
		}

		// After successful move, remove the source directory
		fs::remove_all(from);

	}
	catch (const fs::filesystem_error& e) {
		log("Filesystem error: " + std::string(e.what()));
		std::exit(1);
	}
}

int finalizeUpdate(std::string sfrom, std::string sto) {
	SetConsoleTitle(_T("TcNo Account Switcher - Finalizing Update..."));
	std::string logFilePath = sto + "\\UpdateFinalizeLog.txt";
	logFile = new std::ofstream(logFilePath);
	
    log("Finalizing update...");
    log("Moving files from temp folder to install folder.");
    log("From: " + sfrom);
    log("To: " + sto);
	fs::path from(sfrom);
	fs::path to(sto);

	if (!fs::exists(from) || !fs::is_directory(from)) {
		log("Source path does not exist or is not a directory.");
		return 1;
	}

	move_recursive(from, to);

	// Launch the main program from sto/TcNo-Acc-Switcher.exe in sto
	std::string exe_name = "TcNo-Acc-Switcher.exe";
	std::string operating_path = sto;
	if (operating_path.back() != '\\') operating_path += '\\';
	std::string full_path = operating_path + exe_name;

	log("Running: " + full_path);

	exec_process(std::wstring(operating_path.begin(), operating_path.end()),
		std::wstring(full_path.begin(), full_path.end()), L"");

	return 0;
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
		if (args_contain("finalizeupdate", argc, argv))
		{
			std::string from = argv[2];
			std::string to = argv[3];
			finalizeUpdate(from, to);
			exit(1);
		}

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

		if (!args_contain("nostart", argc, argv))
			launch_tcno_program(argv[argc - 1]);
	}

	cout << "This script will make sure WebView 2, .NET Runtime 8.0, Desktop Runtime 8.0," << endl << 
			"ASP.NET Core Runtime 8.0, and Visual C++ Redistributable 2015 - 2019 are" << endl << 
			"installed. If they are missing, they will be downloaded." << endl << endl;

	cout << "Please close all open programs for the duration of the installation. If the" << endl <<
			"installation fails, please ensure Windows is fully updated, reboot your" << endl <<
			"computer and try to run this again." << endl << endl;
	
	cout << "You can close this window to stop now. Detailed instructions to install these" << endl <<
			"tools manually are available at:" <<endl << 
			"https://github.com/TCNOco/TcNo-Acc-Switcher/wiki#1-installing-required-runtimes" << endl << endl;

	cout << "Press any key to continue..." << endl;
	std::cin.get();

	system("cls");

	cout << "Welcome to the TcNo Account Switcher - Runtime installer" << endl <<
		"------------------------------------------------------------------------" << endl << endl;

	cout << "This script will download and install packages from Microsoft. By using this" << endl <<
			"script, you are accepting the license for the application, executable(s)," << endl <<
			"or other artifacts delivered to your machine as a result of this install." << endl <<
			"This acceptance occurs whether you know the license terms or not. Read and" << endl <<
			"understand the license terms of the packages being installed and their" << endl <<
			"dependencies prior to installation: " << endl <<
			" - https://developer.microsoft.com/en-us/microsoft-edge/webview2/" << endl <<
			" - https://dotnet.microsoft.com/en-us/download/dotnet/8.0" << endl <<
			" - https://learn.microsoft.com/en-us/cpp/windows/latest-supported-vc-redist" << endl << endl;

	cout << "This script is provided AS-IS without any warranties of any kind" << endl <<
			"Please also see the TcNo Account Switcher's Privacy Policy:" << endl << 
			"https://github.com/TCNOco/TcNo-Acc-Switcher/blob/master/PRIVACY.md" << endl << endl;

	cout << "Press any key to continue..." << endl;
	std::cin.get();

	system("cls");

	cout << "Welcome to the TcNo Account Switcher - Runtime installer" << endl <<
		"------------------------------------------------------------------------" << endl << endl;

	cout << "Currently installed runtimes:" << endl;

	/* Find installed runtimes */
	find_installed_net_runtimes(false, min_webview_met, min_desktop_runtime_met, min_aspcore_met);
	find_installed_c_runtimes(min_vc_met);

	cout << "------------------------------------------------------------------------" << endl << endl;

	download_install_missing_runtimes();

	// Launch main program:
	if (!args_contain("nostart", argc, argv)) {
		string exe_name = "TcNo-Acc-Switcher.exe";

		exec_process(std::wstring(operating_path.begin(), operating_path.end()),
			std::wstring(exe_name.begin(), exe_name.end()), L"");
	}
}

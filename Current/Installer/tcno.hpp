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

#pragma once

#pragma comment(lib, "urlmon.lib")
#pragma comment (lib, "User32.lib")
#pragma comment(lib, "iphlpapi.lib")
#pragma comment(lib, "secur32.lib")
#define CRT_SECURE_NO_WARNINGS
#include <algorithm>
#include <chrono>
#include <iostream>
#include <Windows.h>
#include <conio.h>

#include <iostream>
#include <string>
#include <tchar.h>
#include <urlmon.h>
#include "progress_bar.hpp"

#ifndef CURL_STATICLIB
#define CURL_STATICLIB
#endif
#include <curl/curl.h>

#include "runtime_check.hpp"
#include "versions.h"
using namespace std;

inline bool min_vc_met = false,
	min_webview_met = false,
	min_desktop_runtime_met = false,
	min_aspcore_met = false;

#pragma region Misc Functions
inline void insert_empty_line()
{
	CONSOLE_SCREEN_BUFFER_INFO buffer_info;
	GetConsoleScreenBufferInfo(GetStdHandle(STD_OUTPUT_HANDLE), &buffer_info);

	const int width = buffer_info.srWindow.Right - buffer_info.srWindow.Left;
	const std::string s(static_cast<size_t>(width), ' ');
	std::cout << s << '\r';
}

inline double round_off(const double n) {
	double d = n * 100.0;
	const int i = static_cast<int>((double)0.5 + d);
	d = static_cast<double>(i) / 100.0;
	return d;
}

inline void wait_for_input()
{
	_getch();
}
#pragma endregion



#pragma region File Operations
inline string current_download;
inline chrono::time_point<chrono::system_clock> last_time = std::chrono::system_clock::now();

inline size_t write_data(const void* ptr, const size_t size, const size_t n_mem_b, FILE* stream) {
	const size_t written = fwrite(ptr, size, n_mem_b, stream);
	return written;
}

inline string convert_size(size_t size) {
	static const char* sizes[] = { "B", "KB", "MB", "GB" };
	int div = 0;
	size_t rem = 0;

	while (size >= 1024 && div < std::size(sizes)) {
		rem = (size % 1024);
		div++;
		size /= 1024;
	}

	const double size_d = static_cast<float>(size) + static_cast<float>(rem) / 1024.0;
	string result = to_string(round_off(size_d)) + " " + sizes[div];
	return result;
}

inline int progress_bar(
	void* client_progress_data,
	const double dl_total,
	const double dl_now,
	double ul_total,
	double ul_now)
{
	if (const double n = dl_total; n > 0) {
		const string dls = "Downloading " + current_download + " (" + convert_size(static_cast<size_t>(dl_total)) + ")";
		const auto bar1 = new ProgressBar(static_cast<unsigned long>(n), dls.c_str());
		if (const std::chrono::duration<double> elapsed_seconds = std::chrono::system_clock::now() - last_time; elapsed_seconds.count() >= 0.2)
		{
			last_time = std::chrono::system_clock::now();
			bar1->Progressed(static_cast<unsigned long>(dl_now));
		}
	}
	return 0;
}

inline bool download_file(const char* url, const char* dest) {
	if (CURL* curl = curl_easy_init(); curl) {
		FILE* fp;
		fopen_s(&fp, dest, "wb");
		curl_easy_setopt(curl, CURLOPT_URL, url);
		curl_easy_setopt(curl, CURLOPT_WRITEFUNCTION, write_data);
		curl_easy_setopt(curl, CURLOPT_WRITEDATA, fp);
		curl_easy_setopt(curl, CURLOPT_NOPROGRESS, 0L);
		curl_easy_setopt(curl, CURLOPT_PROGRESSFUNCTION, progress_bar);
		curl_easy_setopt(curl, CURLOPT_FRESH_CONNECT, true);
		curl_easy_setopt(curl, CURLOPT_SSL_VERIFYPEER, false);
		curl_easy_setopt(curl, CURLOPT_FOLLOWLOCATION, true); // Follows 301 redirects, like in the WebView2 link
		const CURLcode res = curl_easy_perform(curl);
		/* always cleanup */
		curl_easy_cleanup(curl);
		fclose(fp);

		insert_empty_line();

		last_time = std::chrono::system_clock::from_time_t(0);
		cout << " Finished downloading " << current_download << endl;
		return res == CURLE_OK;
	}
	return false;
}

inline bool file_exists(const std::string& name) {
	struct stat buffer{};
	return stat(name.c_str(), &buffer) == 0;
}
#pragma endregion


#pragma region Runtime Operations

/// <summary>
/// Checks if winget is installed and available.
/// </summary>
inline bool is_winget_available()
{
	STARTUPINFO si = { sizeof(STARTUPINFO) };
	si.cb = sizeof(STARTUPINFO);
	si.dwFlags = STARTF_USESTDHANDLES;
	si.hStdOutput = GetStdHandle(STD_OUTPUT_HANDLE);
	si.hStdError = GetStdHandle(STD_ERROR_HANDLE);
	PROCESS_INFORMATION pi;
	
	// Use cmd.exe to run winget (winget might not be directly in PATH)
	wstring cmd_line = L"cmd.exe /c winget --version";
	wchar_t* cmd_line_writable = new wchar_t[cmd_line.length() + 1];
	wcscpy_s(cmd_line_writable, cmd_line.length() + 1, cmd_line.c_str());
	
	// Create process with CREATE_NO_WINDOW to hide the window
	bool success = CreateProcessW(nullptr, cmd_line_writable, nullptr, nullptr, FALSE, 
		CREATE_NO_WINDOW | CREATE_UNICODE_ENVIRONMENT, nullptr, nullptr, &si, &pi);
	
	delete[] cmd_line_writable;
	
	if (success)
	{
		WaitForSingleObject(pi.hProcess, 5000); // Wait up to 5 seconds
		DWORD exit_code;
		GetExitCodeProcess(pi.hProcess, &exit_code);
		CloseHandle(pi.hProcess);
		CloseHandle(pi.hThread);
		return exit_code == 0;
	}
	return false;
}

/// <summary>
/// Checks if a winget package is installed.
/// </summary>
inline bool is_winget_package_installed(const string& package_id)
{
	STARTUPINFO si = { sizeof(STARTUPINFO) };
	si.cb = sizeof(STARTUPINFO);
	si.dwFlags = STARTF_USESTDHANDLES | STARTF_USESHOWWINDOW;
	si.wShowWindow = SW_HIDE;
	HANDLE h_read, h_write;
	SECURITY_ATTRIBUTES sa = { sizeof(SECURITY_ATTRIBUTES), nullptr, TRUE };
	CreatePipe(&h_read, &h_write, &sa, 0);
	si.hStdOutput = h_write;
	si.hStdError = h_write;
	PROCESS_INFORMATION pi;
	
	wstring package_id_w = s2_ws(package_id);
	wstring cmd_line = L"cmd.exe /c winget list --id \"" + package_id_w + L"\" --accept-source-agreements --accept-package-agreements";
	wchar_t* cmd_line_writable = new wchar_t[cmd_line.length() + 1];
	wcscpy_s(cmd_line_writable, cmd_line.length() + 1, cmd_line.c_str());
	
	bool installed = false;
	if (CreateProcessW(nullptr, cmd_line_writable, nullptr, nullptr, TRUE, 
		CREATE_NO_WINDOW | CREATE_UNICODE_ENVIRONMENT, nullptr, nullptr, &si, &pi))
	{
		CloseHandle(h_write);
		WaitForSingleObject(pi.hProcess, 10000); // Wait up to 10 seconds
		
		char buffer[4096];
		DWORD bytes_read;
		string output;
		while (ReadFile(h_read, buffer, sizeof(buffer) - 1, &bytes_read, nullptr) && bytes_read > 0)
		{
			buffer[bytes_read] = '\0';
			output += buffer;
		}
		
		// Check exit code and output
		// Exit code 0 typically means package was found
		DWORD exit_code;
		GetExitCodeProcess(pi.hProcess, &exit_code);
		
		// Package is installed if exit code is 0 and output contains the package ID
		installed = (exit_code == 0) && (output.find(package_id) != string::npos);
		
		CloseHandle(h_read);
		CloseHandle(pi.hProcess);
		CloseHandle(pi.hThread);
	}
	else
	{
		CloseHandle(h_read);
		CloseHandle(h_write);
	}
	
	delete[] cmd_line_writable;
	return installed;
}

/// <summary>
/// Installs a package using winget.
/// </summary>
inline bool install_winget_package(const string& package_id, const string& display_name)
{
	cout << "Installing " << display_name << " via winget..." << endl;
	
	STARTUPINFO si = { sizeof(STARTUPINFO) };
	si.cb = sizeof(STARTUPINFO);
	si.dwFlags = STARTF_USESTDHANDLES;
	si.hStdOutput = GetStdHandle(STD_OUTPUT_HANDLE);
	si.hStdError = GetStdHandle(STD_ERROR_HANDLE);
	PROCESS_INFORMATION pi;
	
	wstring package_id_w = s2_ws(package_id);
	wstring cmd_line = L"cmd.exe /c winget install \"" + package_id_w + L"\" --accept-source-agreements --accept-package-agreements --silent";
	wchar_t* cmd_line_writable = new wchar_t[cmd_line.length() + 1];
	wcscpy_s(cmd_line_writable, cmd_line.length() + 1, cmd_line.c_str());
	
	bool success = CreateProcessW(nullptr, cmd_line_writable, nullptr, nullptr, FALSE, 
		0, nullptr, nullptr, &si, &pi);
	
	delete[] cmd_line_writable;
	
	if (success)
	{
		WaitForSingleObject(pi.hProcess, INFINITE);
		DWORD exit_code;
		GetExitCodeProcess(pi.hProcess, &exit_code);
		CloseHandle(pi.hProcess);
		CloseHandle(pi.hThread);
		
		if (exit_code == 0)
		{
			cout << "Successfully installed " << display_name << " via winget." << endl;
			return true;
		}
		else
		{
			cout << "Failed to install " << display_name << " via winget (exit code: " << exit_code << ")." << endl;
			return false;
		}
	}
	
	cout << "Failed to launch winget for " << display_name << "." << endl;
	return false;
}

/// <summary>
/// Installs specified runtime.
/// </summary>
inline void install_runtime(const string& path, const string& name, const bool& passive)
{
	cout << "Installing: " << name << endl;
	STARTUPINFO si = { sizeof(STARTUPINFO) };
	PROCESS_INFORMATION pi;

	wstring args;
	if (passive) args = L"/install /passive /norestart";

	CreateProcessW(s2_ws(path).c_str(), &args[0], nullptr,
		nullptr, 0, 0, nullptr, nullptr, &si, &pi);
	WaitForSingleObject(pi.hProcess, INFINITE);
}

inline void download_install_missing_runtimes()
{
	const string operating_path = get_operating_path();

	/* Warn about runtime downloads */
	if (!min_webview_met || !min_aspcore_met || !min_desktop_runtime_met || !min_vc_met)
	{
		cout << "One or more runtimes were not installed:" << endl;
		int total = 0;
		if (!min_webview_met) {
			cout << " + Microsoft WebView2 Runtime [~1,70 MB + ~120 MB while installing]" << endl;
			total += 122;
		}
		if (!min_aspcore_met) {
			cout << " + Microsoft Microsoft ASP.NET Core 8.0 Runtime [~10,1 MB]" << endl;
			total += 8;
		}
		if (!min_desktop_runtime_met) {
			cout << " + Microsoft .NET 8.0 Desktop Runtime [~57,9 MB]" << endl;
			total += 52;
		}

		if (!min_vc_met) {
			cout << " + C++ Redistributable 2015-2022 [~24 MB]" << endl;
			total += 24;
		}

		cout << " = Total download size: ~" << total << " MB" << endl << endl <<
			"Press any key to start download..." << endl;

		wait_for_input();

		// Check if winget is available
		bool use_winget = is_winget_available();
		if (use_winget)
		{
			cout << "winget detected. Using winget to install missing packages..." << endl << endl;
		}
		else
		{
			cout << "winget not available. Falling back to direct download..." << endl << endl;
		}

		/* Download/Install runtimes */
		bool w_runtime_install = false,
			d_runtime_install = false,
			a_runtime_install = false,
			c_runtime_install = false;
		const string runtime_folder = operating_path + "runtime_installers\\";
		const string w_runtime_local = runtime_folder + w_exe,
			d_runtime_local = runtime_folder + d_exe,
			a_runtime_local = runtime_folder + a_exe,
			c_runtime_local = runtime_folder + c_exe;

		if (!use_winget)
		{
			if (!(CreateDirectoryA(runtime_folder.c_str(), nullptr) || ERROR_ALREADY_EXISTS == GetLastError()))
			{
				cout << "Failed to create folder: " << runtime_folder << endl;
				cout << "Press any key to exit..." << endl;
				_getch();
				return;
			}
		}

		// WebView2 - no winget package, always use download method
		if (!min_webview_met)
		{
			if (!use_winget)
			{
				current_download = w_runtime_name;
				if (!download_file(w_runtime.c_str(), w_runtime_local.c_str()))
				{
					cout << "Failed to download and install Microsoft WebView2 Runtime. To download: 1. Click the link below:" << endl <<
						"https://go.microsoft.com/fwlink/p/?LinkId=2124703" << endl <<
						"2. Click 'Download Hosting Bundle' under Run server apps." << endl << endl;
				}
				else w_runtime_install = true;
			}
		}

		// Desktop Runtime - use winget if available
		if (!min_desktop_runtime_met)
		{
			if (use_winget)
			{
				if (!is_winget_package_installed("Microsoft.DotNet.DesktopRuntime.8"))
				{
					d_runtime_install = install_winget_package("Microsoft.DotNet.DesktopRuntime.8", d_runtime_name);
				}
				else
				{
					cout << d_runtime_name << " is already installed via winget." << endl;
					d_runtime_install = true;
				}
			}
			else
			{
				current_download = d_runtime_name;
				if (!download_file(d_runtime.c_str(), d_runtime_local.c_str()))
				{
					cout << "Failed to download and install .NET 8.0 Desktop Runtime. Please download it here:" << endl <<
						"https://dotnet.microsoft.com/download/dotnet/8.0/runtime" << endl << endl;
				}
				else d_runtime_install = true;
			}
		}

		// ASP.NET Core Runtime - use winget if available
		if (!min_aspcore_met)
		{
			if (use_winget)
			{
				if (!is_winget_package_installed("Microsoft.DotNet.AspNetCore.8"))
				{
					a_runtime_install = install_winget_package("Microsoft.DotNet.AspNetCore.8", a_runtime_name);
				}
				else
				{
					cout << a_runtime_name << " is already installed via winget." << endl;
					a_runtime_install = true;
				}
			}
			else
			{
				current_download = a_runtime_name;
				if (!download_file(a_runtime.c_str(), a_runtime_local.c_str()))
				{
					cout << "Failed to download and install ASP.NET Core 8.0 Runtime. To download: 1. Click the link below:" << endl <<
						"https://dotnet.microsoft.com/en-us/download/dotnet/8.0" << endl <<
						"2. Click 'x64' under ASP.NET Core Runtime 8." << endl << endl;
				}
				else a_runtime_install = true;
			}
		}

		// VC++ Redistributable - use winget if available (install both x64 and x86)
		if (!min_vc_met)
		{
			if (use_winget)
			{
				bool vc_x64_installed = is_winget_package_installed("Microsoft.VCRedist.2015+.x64");
				bool vc_x86_installed = is_winget_package_installed("Microsoft.VCRedist.2015+.x86");
				
				if (!vc_x64_installed)
				{
					if (install_winget_package("Microsoft.VCRedist.2015+.x64", "C++ Redistributable 2015-2022 (x64)"))
					{
						c_runtime_install = true;
					}
				}
				else
				{
					cout << "C++ Redistributable 2015-2022 (x64) is already installed via winget." << endl;
				}
				
				if (!vc_x86_installed)
				{
					if (install_winget_package("Microsoft.VCRedist.2015+.x86", "C++ Redistributable 2015-2022 (x86)"))
					{
						c_runtime_install = true;
					}
				}
				else
				{
					cout << "C++ Redistributable 2015-2022 (x86) is already installed via winget." << endl;
				}
				
				// If either was installed, mark as installed
				if (vc_x64_installed || vc_x86_installed || c_runtime_install)
				{
					c_runtime_install = true;
				}
			}
			else
			{
				current_download = c_runtime_name;
				if (!download_file(c_runtime.c_str(), c_runtime_local.c_str()))
				{
					cout << "Failed to download and install C++ Redistributable 2015-2022. To download: 1. Click the link below:" << endl <<
						"https://docs.microsoft.com/en-us/cpp/windows/latest-supported-vc-redist?view=msvc-170" << endl <<
						"2. Click the link next to 'X64', under the \"Visual Studio 2015, 2017, 2019, and 2022\" heading." << endl << endl;
				}
				else c_runtime_install = true;
			}
		}

		cout << endl;

		// Install downloaded runtimes (only if not using winget)
		if (!use_winget && (w_runtime_install || d_runtime_install || a_runtime_install || c_runtime_install))
		{
			cout << "------------------------------------------------------------------------" << endl << endl <<
				"One or more are ready for install. If and when prompted if you would like to install, click 'Yes'." << endl << endl;

			if (w_runtime_install)
				install_runtime(w_runtime_local, w_runtime_name, false); // Does not support "/passive"

			if (d_runtime_install)
				install_runtime(d_runtime_local, d_runtime_name, true);

			if (a_runtime_install)
				install_runtime(a_runtime_local, a_runtime_name, true);

			if (c_runtime_install)
				install_runtime(c_runtime_local, c_runtime_name, true);
		}

		system("cls");
		cout << "Currently installed runtimes:" << endl;
		find_installed_net_runtimes(false, min_webview_met, min_desktop_runtime_met, min_aspcore_met);
		find_installed_c_runtimes(min_vc_met);
		cout << "------------------------------------------------------------------------" << endl << endl <<
			"Verify you meet the minimum recommended requirements:" << endl;
	}
	else
	{
		cout << "It looks like everything is installed. Verify you meet the minimum recommended requirements:" << endl;
	}

	cout << " - Windows Desktop Runtime 8.0+" << endl <<
		" - ASP.NET Core 8.0+." << endl <<
		" - Edge WebView2 Runtime 91.0+" << endl <<
		" - C++ Redistributable 2015-2022 14.30.30704+" << endl <<
		"------------------------------------------------------------------------" << endl << endl <<
		"That should be everything. The main program, TcNo-Acc-Switcher.exe, will auto-run." << endl << endl <<
		"If it doesn't work, please refer to install instructions, here:" << endl <<
		"https://github.com/TCNOCo/TcNo-Acc-Switcher#required-runtimes-download-and-install" << endl << endl;
}


/// <summary>
/// Downloads and installs latest C++ runtime, if not installed already
/// </summary>
inline void verify_vc()
{
	cout << "Downloading C++ Redistributable 2015-2022" << endl;
	current_download = "Downloading C++ Redistributable 2015-2022";

	bool c_runtime_install = false;
	const string runtime_folder = get_operating_path() + "runtime_installers\\";
	const string c_runtime_local = runtime_folder + "VC_redist.x64.exe",
		c_runtime_name = "Downloading C++ Redistributable 2015-2022";

	if (!(CreateDirectoryA(runtime_folder.c_str(), nullptr) || ERROR_ALREADY_EXISTS == GetLastError()))
	{
		cout << "Failed to create folder: " << runtime_folder << endl;
		cout << "Press any key to exit..." << endl;
		_getch();
		return;
	}

	if (!download_file(c_runtime.c_str(), c_runtime_local.c_str()))
	{
		cout << "Failed to download and install C++ Redistributable 2015-2022. To download: 1. Click the link below:" << endl <<
			"https://docs.microsoft.com/en-us/cpp/windows/latest-supported-vc-redist?view=msvc-170" << endl <<
			"2. Click the link next to 'X64', under the \"Visual Studio 2015, 2017, 2019, and 2022\" heading." << endl << endl;
	}
	else c_runtime_install = true;

	if (c_runtime_install)
		install_runtime(c_runtime_local, c_runtime_name, true);
}

/// <summary>
/// Downloads and installs latest .NEt runtime, if not installed already
/// </summary>
inline void verify_net()
{
	cout << "Checking computer for .NET Runtimes. Currently installed:" << endl;
	find_installed_net_runtimes(false, min_webview_met, min_desktop_runtime_met, min_aspcore_met);
	cout << "------------------------------------------------------------------------" << endl << endl;
	download_install_missing_runtimes();
}
#pragma endregion
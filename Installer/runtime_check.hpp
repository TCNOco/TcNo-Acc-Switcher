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
#include <fstream>
#include <iostream>
#include <vector>
#include <Windows.h>

#include <iostream>
#include <string>
#include <tchar.h>

#include "versions.h"

inline void split_version(const std::string& str, std::vector<int>& arr, const char& delimiter)
{
	std::string s_num;
	for (const char i : str)
	{
		if (i != delimiter)
		{
			if (std::isdigit(i)) s_num += i;
			else if (i == '\0') break; // End of string
		}
		else
		{
			arr.push_back(std::stoi(s_num));
			s_num = "";
		}
	}
	// Add last
	arr.push_back(std::stoi(s_num));
}

/// <summary>
/// Returns whether v2 is newer than, or equal to v1.
/// Works with unequal string sizes.
/// </summary>
inline bool compare_versions(std::string v1, std::string v2, const char& delimiter)
{
	if (v1 == v2) return true;
	try
	{
		std::vector<int> v1_arr;
		std::vector<int> v2_arr;


		split_version(v1, v1_arr, delimiter);
		split_version(v2, v2_arr, delimiter);

		for (int i = 0; i < min(v1_arr.size(), v2_arr.size()); ++i)
		{
			if (v2_arr[i] < v1_arr[i]) return false;
			if (v2_arr[i] > v1_arr[i]) return true;
		}
		return true;
	} catch (std::exception &err)
	{
		std::cout << "Version conversion failed!" << std::endl << "Args: (" << v1 << ") and (" << v2 << ")" << std::endl << "Details:" << std::endl << err.what() << std::endl;
	}

	return true;
}

/// <summary>
/// Gets C++ Runtime versions, and sets min_vc_met
/// </summary>
inline void find_installed_c_runtimes(bool &min_vc_met)
{
	// Get C+++ Runtime info
	HKEY key = nullptr;
	//WCHAR s_key[1024];
	DWORD dw_type = KEY_ALL_ACCESS;
	WCHAR version[1024];
	DWORD dw_v_buffer_size = sizeof(version);
	const auto subkey = L"SOFTWARE\\Microsoft\\DevDiv\\VC\\Servicing\\14.0\\RuntimeMinimum";
	if (!(RegOpenKeyEx(HKEY_LOCAL_MACHINE, subkey, 0, KEY_READ, &key) == ERROR_SUCCESS || RegOpenKeyEx(HKEY_CURRENT_USER, subkey, 0, KEY_READ, &key) == ERROR_SUCCESS))
	{
		RegCloseKey(key);
		return;
	}

	if (RegQueryValueEx(key, L"Version", nullptr, &dw_type, reinterpret_cast<unsigned char*>(version), &dw_v_buffer_size) == ERROR_SUCCESS)
	{
		wprintf(L" - C++ Redistributable 2015-2022 [%s]\n", version);
		const std::string s_version(std::begin(version), std::end(version));
		min_vc_met = compare_versions(required_min_vc, std::string(s_version), '.');
	}
	RegCloseKey(key);
}

/// <summary>
/// Finds existing NET runtimes, and sets min_{runtime}_met for aspcore, webview and desktop_runtime
/// </summary>
inline void find_installed_net_runtimes(const bool x32, bool &min_webview_met, bool &min_desktop_runtime_met, bool &min_aspcore_met, const bool output = true)
{
	// Get .NET info
	// Find installed runtimes, and add them to the list
	const auto s_root1 = L"SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Uninstall";
	const auto s_root2 = L"SOFTWARE\\Wow6432Node\\Microsoft\\Windows\\CurrentVersion\\Uninstall";

	HKEY h_uninst_key = nullptr;
	HKEY h_app_key = nullptr;
	long l_result = ERROR_SUCCESS;
	DWORD dw_type = KEY_ALL_ACCESS;
	DWORD dw_buffer_size = 0;
	DWORD dw_v_buffer_size = 0;

	//Open the "Uninstall" key.
	if (x32)
	{
		// If neither key exists, fail:
		if (!(RegOpenKeyEx(HKEY_LOCAL_MACHINE, s_root1, 0, KEY_READ, &h_uninst_key) == ERROR_SUCCESS ||
			RegOpenKeyEx(HKEY_CURRENT_USER, s_root1, 0, KEY_READ, &h_uninst_key) == ERROR_SUCCESS))
			return;
	} else if (!(RegOpenKeyEx(HKEY_LOCAL_MACHINE, s_root2, 0, KEY_READ, &h_uninst_key) == ERROR_SUCCESS ||
		RegOpenKeyEx(HKEY_CURRENT_USER, s_root2, 0, KEY_READ, &h_uninst_key) == ERROR_SUCCESS))
		return;
	
	for (DWORD dw_index = 0; l_result == ERROR_SUCCESS; dw_index++)
	{
		WCHAR s_app_key_name[1024];
		//Enumerate all sub keys...
		dw_buffer_size = sizeof(s_app_key_name);
		if ((l_result = RegEnumKeyEx(h_uninst_key, dw_index, s_app_key_name, &dw_buffer_size, nullptr, nullptr, nullptr,
			nullptr)) == ERROR_SUCCESS)
		{
			WCHAR version[1024];
			WCHAR s_display_name[1024];
			WCHAR s_sub_key[1024];
			//Open the sub key.
			if (x32) wsprintf(s_sub_key, L"%s\\%s", s_root1, s_app_key_name);
			else wsprintf(s_sub_key, L"%s\\%s", s_root2, s_app_key_name);
			if (!(RegOpenKeyEx(HKEY_LOCAL_MACHINE, s_sub_key, 0, KEY_READ, &h_app_key) == ERROR_SUCCESS ||
				RegOpenKeyEx(HKEY_CURRENT_USER, s_sub_key, 0, KEY_READ, &h_app_key) == ERROR_SUCCESS))
			{
				RegCloseKey(h_app_key);
				RegCloseKey(h_uninst_key);
				continue;
			}

			//Get the display name value from the application's sub key.
			dw_buffer_size = sizeof(s_display_name);
			dw_v_buffer_size = sizeof(version);
			if (RegQueryValueEx(h_app_key, L"DisplayName", nullptr, &dw_type, reinterpret_cast<unsigned char*>(s_display_name), &dw_buffer_size) == ERROR_SUCCESS &&
				RegQueryValueEx(h_app_key, L"DisplayVersion", nullptr, &dw_type, reinterpret_cast<unsigned char*>(version), &dw_v_buffer_size) == ERROR_SUCCESS)
			{
				const std::string s_version(std::begin(version), std::end(version));

				if (wcsstr(s_display_name, L"WebView2") != nullptr)
				{
					min_webview_met = min_webview_met || compare_versions(required_min_webview, std::string(s_version), '.');
					if (output)
					{
						wprintf(L" - %s ", s_display_name);
						printf("[%s]\n", s_version.c_str());
					}
				}

				if (wcsstr(s_display_name, L"Desktop Runtime") != nullptr && wcsstr(s_display_name, L"x64") != nullptr)
				{
					min_desktop_runtime_met = min_desktop_runtime_met || compare_versions(required_min_desktop_runtime, std::string(s_version), '.');
					if (output) wprintf(L" - %s\n", s_display_name);
				}

				if (wcsstr(s_display_name, L"ASP.NET Core 8") != nullptr && wcsstr(s_display_name, L"x64") != nullptr)
				{
					min_aspcore_met = min_aspcore_met || compare_versions(required_min_aspcore, std::string(s_version), '.');
					if (output) wprintf(L" - %s\n", s_display_name);
				}
			}
			RegCloseKey(h_app_key);
		}
	}
	RegCloseKey(h_uninst_key);
}

/**
 * \brief This does not wait for the child process to exit.
 */
inline void exec_process(const std::wstring& path, const std::wstring& exe, const std::wstring& param, const bool show_window = true, const bool wait_for_exit = false)
{
	//JOBOBJECT_EXTENDED_LIMIT_INFORMATION job = { 0 };
	//PROCESS_INFORMATION                  pi = { nullptr };
	//STARTUPINFO                          si = { 0 };


	//HANDLE h_job = CreateJobObject(nullptr, nullptr);

	//job.BasicLimitInformation.LimitFlags = JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE;
	//SetInformationJobObject(h_job, JobObjectExtendedLimitInformation, &job, sizeof(job));

	//si.cb = sizeof(si);
	//const auto show = show_window ? 0 : CREATE_NO_WINDOW;
	//CreateProcess(nullptr, &(path + exe + L" " + param)[0], nullptr, nullptr, FALSE,
	//	CREATE_SUSPENDED | show, nullptr, nullptr, &si, &pi);

	//AssignProcessToJobObject(h_job, pi.hProcess);

	//ResumeThread(pi.hThread);
	////// This is commented out, as it waits for the program to close -> Keeping it as a child. I don't want children...
	////WaitForSingleObject(pi.hProcess, INFINITE);

	//CloseHandle(pi.hThread);
	//CloseHandle(pi.hProcess);
	//CloseHandle(h_job);


	SHELLEXECUTEINFO ShExecInfo = { 0 };
	ShExecInfo.cbSize = sizeof(SHELLEXECUTEINFO);
	ShExecInfo.fMask = SEE_MASK_NOCLOSEPROCESS;
	ShExecInfo.hwnd = nullptr;
	ShExecInfo.lpVerb = _T("open");
	ShExecInfo.lpFile = exe.c_str();
	ShExecInfo.lpParameters = param.c_str();
	ShExecInfo.lpDirectory = path.c_str();
	ShExecInfo.nShow = show_window ? SW_SHOW : SW_HIDE;
	ShExecInfo.hInstApp = nullptr;

	DWORD dw_exit_code;
	if (ShellExecuteEx(&ShExecInfo)) {
		if (wait_for_exit){
			WaitForSingleObject(ShExecInfo.hProcess, INFINITE);
		}

		if (!GetExitCodeProcess(ShExecInfo.hProcess, &dw_exit_code)) {
			//failed to terminate normally
			std::cout << dw_exit_code << std::endl;
		}

		CloseHandle(ShExecInfo.hProcess);
	}
	else {

		//failed to execute the exe file
	}
}


#pragma region Utilities
// This doesn't REALLY belong here, but it's used by both programs that use this file so...
inline std::wstring s2_ws(const std::string& s)
{
	const int s_length = static_cast<int>(s.length()) + 1;
	const int len = MultiByteToWideChar(CP_ACP, 0, s.c_str(), s_length, nullptr, 0);
	const auto buf = new wchar_t[len];
	MultiByteToWideChar(CP_ACP, 0, s.c_str(), s_length, buf, len);
	std::wstring r(buf);
	delete[] buf;
	return r;
}

inline std::string get_self_location()
{
	const HMODULE h_module = GetModuleHandleW(nullptr);
	WCHAR pth[MAX_PATH];
	GetModuleFileNameW(h_module, pth, MAX_PATH);
	std::wstring ws(pth);
	const std::string path(ws.begin(), ws.end());
	return path;
}

inline std::string get_operating_path() {
	const std::string path(get_self_location());
	return path.substr(0, path.find_last_of('\\') + 1);
}

inline std::string get_self_name() {
	const std::string path(get_self_location());
	const size_t last_slash = path.find_last_of('\\');
	return path.substr(last_slash + 1, path.find_last_of('.') - last_slash - 1);
}

inline std::string dotnet_path()
{
	// Get C+++ Runtime info
	HKEY key = nullptr;
	//WCHAR s_key[1024];
	DWORD dw_type = KEY_ALL_ACCESS;
	WCHAR path[1024];
	DWORD dw_v_buffer_size = sizeof(path);

	std::string ret;
	const auto sub_key = L"SOFTWARE\\dotnet\\Setup\\InstalledVersions\\x64\\sharedhost";
	if (!(RegOpenKeyEx(HKEY_LOCAL_MACHINE, sub_key, 0, KEY_READ, &key) == ERROR_SUCCESS ||
		RegOpenKeyEx(HKEY_CURRENT_USER, sub_key, 0, KEY_READ, &key) == ERROR_SUCCESS))
	{
		RegCloseKey(key);
		return ret;
	}

	if (RegQueryValueEx(key, L"Path", nullptr, &dw_type, reinterpret_cast<unsigned char*>(path), &dw_v_buffer_size) == ERROR_SUCCESS)
	{
		//ret = std::string(std::begin(path), std::end(path));
		// For some unknown reason, using +, +=, append, push_back etc just DON'T WORK HERE WTF
		// I have been bashing my head into a wall for over an hour it's 3 AM
		//errno_t e = wcscat_s(path, L"dotnet.exe");
		ret = std::string(std::begin(path), std::end(path));
	}
	RegCloseKey(key);

	ret.resize(strlen(ret.c_str())); // Otherwise this goes for 1024 characters...

	return ret;
}

#pragma endregion